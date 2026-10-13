package unindent

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

// Flags that turn each check on or off, named after the checks' headings in docs/checks.md.
const (
	flagElseAfterExit   = "else-after-exit"
	flagElseIfAfterExit = "else-if-after-exit"
)

// checks are the checks that are turned on.
type checks struct {
	elseAfterExit   bool
	elseIfAfterExit bool
}

type analyzer struct {
	checks checks
}

// NewAnalyzer which checks that whether code blocks can be unindented without changing behaviour. Every check is on;
// its flag in the analyzer's Flags turns it off. Each analyzer has its own flags.
func NewAnalyzer() *analysis.Analyzer {
	a := &analyzer{checks: checks{elseAfterExit: true, elseIfAfterExit: true}}

	an := &analysis.Analyzer{
		Doc:      "Identifies unnecessarily indented code",
		Name:     "unindent",
		Requires: []*analysis.Analyzer{inspect.Analyzer, ctrlflow.Analyzer, buildAnalyzer},
		Run:      a.run,
		URL:      "https://github.com/dackroyd/unindent",
	}

	an.Flags.BoolVar(&a.checks.elseAfterExit, flagElseAfterExit, true,
		`report an "else" after a branch that always exits`)
	an.Flags.BoolVar(&a.checks.elseIfAfterExit, flagElseIfAfterExit, true,
		`report an "else if" after a branch that always exits`)

	return an
}

type nodeInfo struct {
	Returns bool
	// Dependent lists the calls that never return only in some builds that the if, or an earlier link of its chain,
	// relies on to exit.
	Dependent []string
}

// checker holds what the analysis of one package needs: the type information, and which functions never return.
type checker struct {
	pass   *analysis.Pass
	cfgs   *ctrlflow.CFGs
	builds *buildDependents
	checks checks
	// strict counts the calls that never return only in some builds as returning, and records them in dependent.
	strict    bool
	dependent []string
}

func (a *analyzer) run(pass *analysis.Pass) (interface{}, error) {
	if !a.checks.elseAfterExit && !a.checks.elseIfAfterExit {
		return analysis.Fact(nil), nil
	}

	ins, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, fmt.Errorf("unexpected analyzer type for initial inspection %T", pass.ResultOf[inspect.Analyzer])
	}

	cfgs, ok := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)
	if !ok {
		return nil, fmt.Errorf("unexpected analyzer type for control flow %T", pass.ResultOf[ctrlflow.Analyzer])
	}

	builds, ok := pass.ResultOf[buildAnalyzer].(*buildDependents)
	if !ok {
		return nil, fmt.Errorf("unexpected analyzer type for build-dependent calls %T", pass.ResultOf[buildAnalyzer])
	}

	c := &checker{pass: pass, cfgs: cfgs, builds: builds, checks: a.checks}

	// Track the analysis of the stack to use when evaluating deeper elements.
	infoStack := []nodeInfo{{Returns: true}}

	ins.WithStack([]ast.Node{&ast.IfStmt{}}, func(node ast.Node, push bool, stack []ast.Node) (proceed bool) {
		stmt, ok := node.(*ast.IfStmt)
		if !ok {
			return false
		}

		if push {
			parentInfo := infoStack[len(infoStack)-1]
			info := c.analyze(stmt, stack, parentInfo)

			infoStack = append(infoStack, info)

			return true
		}

		infoStack = infoStack[:len(infoStack)-1]

		return true
	})

	return analysis.Fact(nil), nil
}

func (c *checker) analyze(stmt *ast.IfStmt, stack []ast.Node, parentInfo nodeInfo) nodeInfo {
	parent, ok := stack[len(stack)-2].(*ast.IfStmt)
	if ok && parent.Else == stmt && !parentInfo.Returns {
		// This is an 'else if' where the 'if' doesn't return, so the 'else' is not redundant
		return nodeInfo{}
	}

	if !c.ifAlwaysReturns(stmt) {
		return nodeInfo{}
	}

	var dependent []string
	if ok && parent.Else == stmt {
		dependent = parentInfo.Dependent
	}

	dependent = slices.Concat(dependent, c.buildDependentExits(stmt))

	if c.reports(stmt) {
		c.reportUnnecessaryElse(stmt, stack, dependent)
	}

	return nodeInfo{Returns: true, Dependent: dependent}
}

// reports reports whether the check for the else of stmt, an "else" block or an "else if", is turned on.
func (c *checker) reports(stmt *ast.IfStmt) bool {
	if _, ok := stmt.Else.(*ast.IfStmt); ok {
		return c.checks.elseIfAfterExit
	}

	return c.checks.elseAfterExit
}

// buildDependentExits returns the calls that never return only in some builds that stmt's body needs to exit, if it
// does not exit in every build.
func (c *checker) buildDependentExits(stmt *ast.IfStmt) []string {
	c.strict, c.dependent = true, nil
	defer func() { c.strict = false }()

	if c.ifAlwaysReturns(stmt) {
		return nil
	}

	slices.Sort(c.dependent)

	return slices.Compact(c.dependent)
}

// alwaysReturns reports whether s always leaves the block it is in: one of the Go specification's terminating
// statements, a break or continue, which leave any if around them, or a call that never returns. A for, switch or
// select is rejected when a break refers to it, as the break only leaves that statement.
func (c *checker) alwaysReturns(s ast.Node) bool {
	switch t := s.(type) {
	case *ast.BlockStmt:
		return c.listAlwaysReturns(t.List)
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		// Every branch leaves the block. A fallthrough can only end a case, and the switch checks the case it
		// continues into.
		return true
	case *ast.LabeledStmt:
		return c.alwaysReturns(t.Stmt) && !breaksTo(t.Stmt, t.Label.Name)
	case *ast.IfStmt:
		if !c.ifAlwaysReturns(t) {
			return false
		}

		// When we're looking into nested if/else of the IfStmt being analyzed,
		// we must also evaluate the else cases as part of the body of our main if
		return c.alwaysReturns(t.Else)
	case *ast.ExprStmt:
		call, ok := t.X.(*ast.CallExpr)

		return ok && c.neverReturns(call)
	case *ast.ForStmt:
		return t.Cond == nil && !breaksOut(t.Body)
	case *ast.SwitchStmt:
		return c.switchBodyAlwaysReturns(t.Body) && !breaksOut(t.Body)
	case *ast.TypeSwitchStmt:
		return c.switchBodyAlwaysReturns(t.Body) && !breaksOut(t.Body)
	case *ast.SelectStmt:
		return c.selectBodyAlwaysReturns(t.Body) && !breaksOut(t.Body)
	}

	return false
}

// testingExits are the methods of testing.T, B, F and TB that stop the test by calling runtime.Goexit.
var testingExits = []string{"Fatal", "Fatalf", "FailNow", "Skip", "Skipf", "SkipNow"}

// neverReturns reports whether call never returns to its caller: a panic, a call to testing.TB's methods that stop the
// test, or a function the control flow analysis finds cannot return, such as os.Exit, log.Fatal or a function that
// always calls one of them.
func (c *checker) neverReturns(call *ast.CallExpr) bool {
	if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		if b, ok := c.pass.TypesInfo.Uses[id].(*types.Builtin); ok {
			return b.Name() == "panic"
		}
	}

	fn := typeutil.Callee(c.pass.TypesInfo, call)

	f, ok := fn.(*types.Func)
	if !ok {
		return false
	}

	if f.Pkg() != nil && f.Pkg().Path() == "testing" && slices.Contains(testingExits, f.Name()) {
		return true
	}

	if !c.cfgs.NoReturn(f) {
		return false
	}

	if c.strict && c.builds.Has(f) {
		c.dependent = append(c.dependent, mustFormatNode(call.Fun))
		return false
	}

	return true
}

func (c *checker) ifAlwaysReturns(stmt *ast.IfStmt) bool {
	if stmt.Else == nil {
		return false
	}

	return c.blockAlwaysReturns(stmt.Body)
}

func (c *checker) blockAlwaysReturns(block *ast.BlockStmt) bool {
	return c.listAlwaysReturns(block.List)
}

// listAlwaysReturns reports whether a statement in list always leaves it. Go allows statements after it, which can
// only be reached through a label among them, so a label after it carries on to the end of the list unless the
// labelled statement always leaves too.
func (c *checker) listAlwaysReturns(list []ast.Stmt) bool {
	for i := len(list) - 1; i >= 0; i-- {
		if c.alwaysReturns(list[i]) {
			return true
		}

		if _, ok := list[i].(*ast.LabeledStmt); ok {
			return false
		}
	}

	return false
}

// breaksOut reports whether body holds an unlabeled break that leaves the statement body belongs to, rather than a
// loop, switch or select nested inside it.
func breaksOut(body *ast.BlockStmt) bool {
	var found bool

	ast.Inspect(body, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.BranchStmt:
			found = found || (t.Tok == token.BREAK && t.Label == nil)
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.FuncLit:
			return false
		}

		return !found
	})

	return found
}

// breaksTo reports whether n holds a break to label, which leaves the statement the label is on.
func breaksTo(n ast.Node, label string) bool {
	var found bool

	ast.Inspect(n, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.BranchStmt:
			found = found || (t.Tok == token.BREAK && t.Label != nil && t.Label.Name == label)
		case *ast.FuncLit:
			return false
		}

		return !found
	})

	return found
}

func (c *checker) switchBodyAlwaysReturns(body *ast.BlockStmt) bool {
	var defaultReturns bool

	for _, b := range body.List {
		clause, ok := b.(*ast.CaseClause)
		if !ok {
			panic(fmt.Sprintf("unexpected switch body statement %+v", b))
		}

		if clause.List == nil {
			defaultReturns = c.caseAlwaysReturns(clause)
			continue
		}

		if !c.caseAlwaysReturns(clause) {
			return false
		}
	}

	return defaultReturns
}

func (c *checker) caseAlwaysReturns(clause *ast.CaseClause) bool {
	return c.listAlwaysReturns(clause.Body)
}

// selectBodyAlwaysReturns reports whether every clause of a select always leaves it. A select without clauses blocks
// forever.
func (c *checker) selectBodyAlwaysReturns(body *ast.BlockStmt) bool {
	for _, b := range body.List {
		clause, ok := b.(*ast.CommClause)
		if !ok {
			panic(fmt.Sprintf("unexpected select body statement %+v", b))
		}

		if !c.listAlwaysReturns(clause.Body) {
			return false
		}
	}

	return true
}

// reportUnnecessaryElse reports the else of stmt, with a fix unless the fix would not compile or would change
// behaviour, as it would in a build where one of the dependent calls returns.
func (c *checker) reportUnnecessaryElse(stmt *ast.IfStmt, stack []ast.Node, dependent []string) {
	pass := c.pass

	var (
		args      []any
		msg       strings.Builder
		conflicts []string
	)

	edits := []analysis.TextEdit{}
	links, enclosing := chainLinks(stack)

	for _, call := range dependent {
		conflicts = append(conflicts, fmt.Sprintf("%q never returns only in some builds", call))
	}

	conflicts = append(conflicts, c.initConflicts(pass.TypesInfo, links, enclosing)...)

	msg.WriteString(`Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. `)

	if hoistsInit(stmt) {
		in := nodeSource(pass, stmt.Init)
		edits = append(edits, analysis.TextEdit{
			Pos:     stmt.Pos(),
			End:     stmt.Init.End(),
			NewText: []byte(in + "\nif "),
		})

		msg.WriteString(`Move variable declaration %q before the "if", and remove the "else"`)
		args = append(args, in)
	} else {
		msg.WriteString(`Remove the "else"`)
	}

	switch els := stmt.Else.(type) {
	case *ast.IfStmt:
		// "else if <cond>"
		// Remove the 'else' from the 'else if', and shift the 'if' down
		edits = append(edits, analysis.TextEdit{
			Pos:     stmt.Body.Rbrace + 1,
			End:     els.If,
			NewText: []byte("\n\n"),
		})

		msg.WriteString(`, leaving "if %s { ... }"`)
		args = append(args, mustFormatNode(els.Cond))
	case *ast.BlockStmt:
		// "else"
		// Remove the 'else {', shifting the body down. Anything after the brace, such as a comment, stays.
		edits = append(edits, analysis.TextEdit{
			Pos:     stmt.Body.Rbrace + 1,
			End:     els.Lbrace + 1,
			NewText: []byte("\n\n"),
		})
		// Remove the closing '}' of the 'else'
		edits = append(edits, analysis.TextEdit{
			Pos:     els.Rbrace - 1,
			End:     els.End() + 1,
			NewText: []byte(""),
		})

		msg.WriteString(` wrapping the block of statements`)

		conflicts = append(conflicts, c.moveConflicts(pass.TypesInfo, els, links, enclosing)...)
	default:
		pass.Reportf(els.Pos(), `Unexpected ast in "else": %s`, mustFormatNode(stmt.Else))
		return
	}

	diag := analysis.Diagnostic{
		Pos:     stmt.Else.Pos(),
		Message: fmt.Sprintf(msg.String(), args...),
	}

	if len(conflicts) > 0 {
		diag.Message += ". Not fixed automatically: " + strings.Join(conflicts, "; ")
	} else {
		diag.SuggestedFixes = []analysis.SuggestedFix{
			{Message: "Remove unnecessary else", TextEdits: edits},
		}
	}

	pass.Report(diag)
}

// chainLinks returns the if statements of the if/else chain from its head down to the one at the top of the stack, and
// the node enclosing the chain: the block or case clause that the else block and hoisted initialisers move into.
func chainLinks(stack []ast.Node) ([]*ast.IfStmt, ast.Node) {
	stmt, _ := stack[len(stack)-1].(*ast.IfStmt)
	links := []*ast.IfStmt{stmt}

	for i := len(stack) - 2; i >= 0; i-- {
		switch n := stack[i].(type) {
		case *ast.IfStmt:
			if n.Else == links[0] {
				links = append([]*ast.IfStmt{n}, links...)
				continue
			}
		case *ast.LabeledStmt:
			continue
		}

		return links, stack[i]
	}

	return links, nil
}

// hoistsInit reports whether the fix for stmt moves its initialiser before the "if", as the else uses what it declares.
func hoistsInit(stmt *ast.IfStmt) bool {
	return stmt.Init != nil && elseUsesInit(stmt)
}

// initConflicts describes why the fix for the last link cannot move initialisers into the block enclosing the chain.
// Every earlier link with a hoisted initialiser moves it into the same block. The rest of the chain uses what that
// initialiser declares, so when an earlier one cannot move, no later link can be fixed either.
func (c *checker) initConflicts(info *types.Info, links []*ast.IfStmt, enclosing ast.Node) []string {
	if enclosing == nil {
		return nil
	}

	if len(links) > 1 && !c.checks.elseIfAfterExit {
		return keptInitConflicts(info, links)
	}

	var conflicts []string

	for i := range links[:len(links)-1] {
		if len(c.hoistConflicts(info, links[:i+1], enclosing)) > 0 {
			conflicts = append(conflicts, `an earlier "else" in the chain is not fixed automatically`)
			break
		}
	}

	return append(conflicts, c.hoistConflicts(info, links, enclosing)...)
}

// keptInitConflicts describes why the fix for the last link of a chain cannot be applied while the earlier links keep
// their "else", as the check for an "else if" is off. The initialisers of the chain stay in it, so the last link's own
// initialiser cannot move before the "if", and its else block, which moves out of the chain, cannot use what one of them
// declares.
func keptInitConflicts(info *types.Info, links []*ast.IfStmt) []string {
	stmt := links[len(links)-1]

	var scopes []*types.Scope

	for _, l := range links {
		if s := info.Scopes[l]; s != nil {
			scopes = append(scopes, s)
		}
	}

	var names []string

	ast.Inspect(stmt.Else, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if obj := info.Uses[ident]; obj != nil && slices.Contains(scopes, obj.Parent()) {
				names = append(names, ident.Name)
			}
		}

		return true
	})

	if s := info.Scopes[stmt]; s != nil && hoistsInit(stmt) && len(names) == 0 {
		names = s.Names()
	}

	slices.Sort(names)

	var conflicts []string

	for _, name := range slices.Compact(names) {
		conflicts = append(conflicts, fmt.Sprintf("%q is declared by an initialiser that stays in the chain, as the %s check is off", name, flagElseIfAfterExit))
	}

	return conflicts
}

// hoistConflicts describes why the initialiser of the last link cannot move into the block enclosing the chain, whether
// or not the earlier links can be fixed.
func (c *checker) hoistConflicts(info *types.Info, links []*ast.IfStmt, enclosing ast.Node) []string {
	stmt := links[len(links)-1]
	headScope, stmtScope := info.Scopes[links[0]], info.Scopes[stmt]

	if !hoistsInit(stmt) || headScope == nil || stmtScope == nil {
		return nil
	}

	var conflicts []string

	outer := headScope.Parent()
	earlier := c.movedBefore(info, enclosing, links[0])

	for _, name := range stmtScope.Names() {
		switch {
		case outer.Lookup(name) != nil:
			conflicts = append(conflicts, fmt.Sprintf("%q is already declared in the enclosing block", name))
		case slices.Contains(earlier, name):
			conflicts = append(conflicts, fmt.Sprintf("%q is also moved into the enclosing block by an earlier fix", name))
		case hoistedBefore(info, links[:len(links)-1], name):
			conflicts = append(conflicts, fmt.Sprintf("%q is also declared by an earlier \"if\" initialiser", name))
		case usedAfter(info, enclosing, links[0].End(), name, outer):
			conflicts = append(conflicts, fmt.Sprintf("%q would hide the %q used after the \"if\"", name, name))
		}
	}

	var vars []movedVar

	for _, name := range stmtScope.Names() {
		vars = append(vars, movedVar{name, stmtScope.Lookup(name).Pos()})
	}

	return append(conflicts, gotoConflicts(enclosing, links[0], vars)...)
}

// hoistedBefore reports whether one of links moves an initialiser that declares name.
func hoistedBefore(info *types.Info, links []*ast.IfStmt, name string) bool {
	for _, l := range links {
		if s := info.Scopes[l]; s != nil && hoistsInit(l) && s.Lookup(name) != nil {
			return true
		}
	}

	return false
}

// moveConflicts describes the names declared at the top level of els that cannot move into the block enclosing the
// if/else chain: a name already declared in that block or by a hoisted initialiser, or one that would hide a variable
// used after the chain.
func (c *checker) moveConflicts(info *types.Info, els *ast.BlockStmt, links []*ast.IfStmt, enclosing ast.Node) []string {
	head := links[0]
	elseScope, headScope := info.Scopes[els], info.Scopes[head]
	if elseScope == nil || headScope == nil || enclosing == nil {
		return nil
	}

	outer := headScope.Parent()
	earlier := c.movedBefore(info, enclosing, head)

	// Without the "else if" check, the earlier links keep their initialisers.
	hoisting := links
	if !c.checks.elseIfAfterExit {
		hoisting = links[len(links)-1:]
	}

	var conflicts []string

	names := c.movedNames(info, els)

	for _, name := range names {
		if outer.Lookup(name) != nil {
			conflicts = append(conflicts, fmt.Sprintf("%q is already declared in the enclosing block", name))
			continue
		}

		if slices.Contains(earlier, name) {
			conflicts = append(conflicts, fmt.Sprintf("%q is also moved into the enclosing block by an earlier fix", name))
			continue
		}

		if hoistedBefore(info, hoisting, name) {
			conflicts = append(conflicts, fmt.Sprintf("%q is also declared by an \"if\" initialiser", name))
			continue
		}

		if usedAfter(info, enclosing, head.End(), name, outer) {
			conflicts = append(conflicts, fmt.Sprintf("%q would hide the %q used after the \"if\"", name, name))
		}
	}

	return append(conflicts, gotoConflicts(enclosing, head, movedVars(elseScope, els, names))...)
}

// movedVar is a variable that a fix declares in the block enclosing the if/else chain, and where it is declared.
type movedVar struct {
	name string
	pos  token.Pos
}

// movedVars returns the variables among names, the names movedNames returns for els. A name declared by a fix inside
// els, rather than in els itself, counts as declared at the end of els.
func movedVars(scope *types.Scope, els *ast.BlockStmt, names []string) []movedVar {
	var vars []movedVar

	for _, name := range names {
		switch obj := scope.Lookup(name).(type) {
		case *types.Var:
			vars = append(vars, movedVar{name, obj.Pos()})
		case nil:
			vars = append(vars, movedVar{name, els.Rbrace})
		}
	}

	return vars
}

// gotoConflicts describes the gotos that would jump over the declaration of one of vars once the fix moves it into
// enclosing: a goto before the declaration to a label in enclosing after the chain that head starts. Before the fix, the
// declaration was in a block the goto jumps out of, or does not enter.
func gotoConflicts(enclosing ast.Node, head *ast.IfStmt, vars []movedVar) []string {
	labels := labelsAfter(enclosing, head.End())
	if len(labels) == 0 || len(vars) == 0 {
		return nil
	}

	var conflicts []string

	ast.Inspect(enclosing, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			if t.Tok != token.GOTO || !slices.Contains(labels, t.Label.Name) {
				return true
			}

			for _, v := range vars {
				conflict := fmt.Sprintf("\"goto %s\" would jump over the declaration of %q", t.Label.Name, v.name)
				if t.Pos() < v.pos && !slices.Contains(conflicts, conflict) {
					conflicts = append(conflicts, conflict)
				}
			}
		}

		return n == nil || n.Pos() < head.End()
	})

	return conflicts
}

// labelsAfter returns the labels of the statements of enclosing, a block or case clause, that start after pos.
func labelsAfter(enclosing ast.Node, pos token.Pos) []string {
	var list []ast.Stmt

	switch n := enclosing.(type) {
	case *ast.BlockStmt:
		list = n.List
	case *ast.CaseClause:
		list = n.Body
	case *ast.CommClause:
		list = n.Body
	}

	var labels []string

	for _, stmt := range list {
		for stmt.Pos() > pos {
			labeled, ok := stmt.(*ast.LabeledStmt)
			if !ok {
				break
			}

			labels = append(labels, labeled.Label.Name)
			stmt = labeled.Stmt
		}
	}

	return labels
}

// movedNames returns the names that end up declared at the top level of els once the fixes inside it are applied: its
// own declarations, the initialisers that fixes of if/else chains directly inside it move up, and recursively the
// declarations of the else blocks those fixes move up.
func (c *checker) movedNames(info *types.Info, els *ast.BlockStmt) []string {
	scope := info.Scopes[els]
	if scope == nil {
		return nil
	}

	names := scope.Names()

	for _, stmt := range els.List {
		names = append(names, c.chainMovedNames(info, stmt)...)
	}

	slices.Sort(names)

	return slices.Compact(names)
}

// movedBefore returns the names that fixes of if/else chains before head move into enclosing.
func (c *checker) movedBefore(info *types.Info, enclosing ast.Node, head *ast.IfStmt) []string {
	var list []ast.Stmt

	switch n := enclosing.(type) {
	case *ast.BlockStmt:
		list = n.List
	case *ast.CaseClause:
		list = n.Body
	case *ast.CommClause:
		list = n.Body
	}

	var names []string

	for _, stmt := range list {
		if stmt.End() > head.Pos() {
			break
		}

		names = append(names, c.chainMovedNames(info, stmt)...)
	}

	return names
}

// chainMovedNames returns the names that the fixes of the if/else chain at stmt move into the block holding it: the
// initialisers they hoist, and the declarations of the else block they move up, including what fixes inside it move.
// Only the fixes of checks that are on count, and a link after an "else if" keeps its initialiser when the "else if"
// check is off.
func (c *checker) chainMovedNames(info *types.Info, stmt ast.Stmt) []string {
	if labeled, ok := stmt.(*ast.LabeledStmt); ok {
		stmt = labeled.Stmt
	}

	head, _ := stmt.(*ast.IfStmt)
	link := head

	var names []string

	for link != nil && c.ifAlwaysReturns(link) {
		hoists := c.reports(link) && (link == head || c.checks.elseIfAfterExit)
		if s := info.Scopes[link]; s != nil && hoists && hoistsInit(link) {
			names = append(names, s.Names()...)
		}

		switch e := link.Else.(type) {
		case *ast.IfStmt:
			link = e
		case *ast.BlockStmt:
			if c.checks.elseAfterExit {
				names = append(names, c.movedNames(info, e)...)
			}

			link = nil
		}
	}

	return names
}

// usedAfter reports whether n refers to name after pos, where name resolves to a declaration in outer or a scope
// enclosing it. Uses of a name declared in a scope nested after pos are unaffected by a declaration in outer.
func usedAfter(info *types.Info, n ast.Node, pos token.Pos, name string, outer *types.Scope) bool {
	var used bool

	ast.Inspect(n, func(n ast.Node) bool {
		if n != nil && n.End() <= pos {
			return false
		}

		ident, ok := n.(*ast.Ident)
		if used || !ok || ident.Pos() < pos || ident.Name != name {
			return !used
		}

		obj := info.Uses[ident]
		if obj == nil || obj.Parent() == nil {
			return true
		}

		for s := outer; s != nil; s = s.Parent() {
			if obj.Parent() == s {
				used = true
				break
			}
		}

		return !used
	})

	return used
}

func elseUsesInit(stmt *ast.IfStmt) bool {
	assign, ok := stmt.Init.(*ast.AssignStmt)
	if !ok {
		return false
	}

	vars := make([]string, len(assign.Lhs))

	for i, v := range assign.Lhs {
		ident, ok := v.(*ast.Ident)
		if !ok {
			continue
		}

		vars[i] = ident.Name
	}

	var varUsed bool

	ast.Inspect(stmt.Else, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		for _, v := range vars {
			if ident.Name == v {
				varUsed = true
				return false
			}
		}

		return true
	})

	return varUsed
}

// nodeSource returns the source text of n, comments included. Where the file cannot be read, it returns n formatted,
// without its comments.
func nodeSource(pass *analysis.Pass, n ast.Node) string {
	if tf := pass.Fset.File(n.Pos()); tf != nil && pass.ReadFile != nil {
		if src, err := pass.ReadFile(tf.Name()); err == nil && len(src) == tf.Size() {
			return string(src[tf.Offset(n.Pos()):tf.Offset(n.End())])
		}
	}

	return mustFormatNode(n)
}

func mustFormatNode(n any) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), n); err != nil {
		panic(fmt.Errorf("unable to generate formatted node for %+v", n))
	}

	return buf.String()
}
