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

type analyzer struct{}

// NewAnalyzer which checks that whether code blocks can be unindented without changing behaviour.
func NewAnalyzer() *analysis.Analyzer {
	var a analyzer

	return &analysis.Analyzer{
		Doc:      "Identifies unnecessarily indented code",
		Name:     "unindent",
		Requires: []*analysis.Analyzer{inspect.Analyzer, ctrlflow.Analyzer, buildAnalyzer},
		Run:      a.run,
		URL:      "https://github.com/dackroyd/unindent",
	}
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
	// strict counts the calls that never return only in some builds as returning, and records them in dependent.
	strict    bool
	dependent []string
}

func (analyzer) run(pass *analysis.Pass) (interface{}, error) {
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

	c := &checker{pass: pass, cfgs: cfgs, builds: builds}

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

	c.reportUnnecessaryElse(stmt, stack, dependent)

	return nodeInfo{Returns: true, Dependent: dependent}
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

// alwaysReturns reports whether s always leaves the block it is in. An unlabeled break counts, as it leaves any if
// around it; instead, a switch is rejected when a break in its cases only leaves the switch.
func (c *checker) alwaysReturns(s ast.Node) bool {
	switch t := s.(type) {
	case *ast.BlockStmt:
		return c.blockAlwaysReturns(t)
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		// Fallthrough doesn't return, and GOTO is unpredictable without deeper
		// understanding of where it ends up, which isn't assessed here
		return t.Tok == token.BREAK || t.Tok == token.CONTINUE
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
	case *ast.SwitchStmt:
		return c.switchBodyAlwaysReturns(t.Body) && !breaksOut(t.Body)
	case *ast.TypeSwitchStmt:
		return c.switchBodyAlwaysReturns(t.Body) && !breaksOut(t.Body)
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
	// Where the return statement is declared directly in the block, it should be the last statement
	// However, Go allows other statements after this, even if they can never be reached
	for i := len(block.List) - 1; i >= 0; i-- {
		b := block.List[i]

		if c.alwaysReturns(b) {
			return true
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
	for i := len(clause.Body) - 1; i >= 0; i-- {
		cb := clause.Body[i]

		if c.alwaysReturns(cb) {
			return true
		}
	}

	return false
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

	msg.WriteString(`Unnecessary "else": preceding conditions always end in a "return", "break", "continue" or a call that never returns. `)

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

	var conflicts []string

	for i := range links[:len(links)-1] {
		if hoistsInit(links[i]) && len(c.initConflicts(info, links[:i+1], enclosing)) > 0 {
			conflicts = append(conflicts, `an earlier "else" in the chain is not fixed automatically`)
			break
		}
	}

	stmt := links[len(links)-1]
	headScope, stmtScope := info.Scopes[links[0]], info.Scopes[stmt]

	if !hoistsInit(stmt) || headScope == nil || stmtScope == nil {
		return conflicts
	}

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

	return conflicts
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

	var conflicts []string

	for _, name := range c.movedNames(info, els) {
		if outer.Lookup(name) != nil {
			conflicts = append(conflicts, fmt.Sprintf("%q is already declared in the enclosing block", name))
			continue
		}

		if slices.Contains(earlier, name) {
			conflicts = append(conflicts, fmt.Sprintf("%q is also moved into the enclosing block by an earlier fix", name))
			continue
		}

		if hoistedBefore(info, links, name) {
			conflicts = append(conflicts, fmt.Sprintf("%q is also declared by an \"if\" initialiser", name))
			continue
		}

		if usedAfter(info, enclosing, head.End(), name, outer) {
			conflicts = append(conflicts, fmt.Sprintf("%q would hide the %q used after the \"if\"", name, name))
		}
	}

	return conflicts
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
func (c *checker) chainMovedNames(info *types.Info, stmt ast.Stmt) []string {
	if labeled, ok := stmt.(*ast.LabeledStmt); ok {
		stmt = labeled.Stmt
	}

	link, _ := stmt.(*ast.IfStmt)

	var names []string

	for link != nil && c.ifAlwaysReturns(link) {
		if s := info.Scopes[link]; s != nil && hoistsInit(link) {
			names = append(names, s.Names()...)
		}

		switch e := link.Else.(type) {
		case *ast.IfStmt:
			link = e
		case *ast.BlockStmt:
			names = append(names, c.movedNames(info, e)...)
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
