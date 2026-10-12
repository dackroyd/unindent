package unindent

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

type analyzer struct{}

// NewAnalyzer which checks that whether code blocks can be unindented without changing behaviour.
func NewAnalyzer() *analysis.Analyzer {
	var a analyzer

	return &analysis.Analyzer{
		Doc:      "Identifies unnecessarily indented code",
		Name:     "unindent",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      a.run,
		URL:      "https://github.com/dackroyd/unindent",
	}
}

type nodeInfo struct {
	Returns bool
}

func (analyzer) run(pass *analysis.Pass) (interface{}, error) {
	ins, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, fmt.Errorf("unexpected analyzer type for initial inspection %T", pass.ResultOf[inspect.Analyzer])
	}

	// Track the analysis of the stack to use when evaluating deeper elements.
	infoStack := []nodeInfo{{Returns: true}}

	ins.WithStack([]ast.Node{&ast.IfStmt{}}, func(node ast.Node, push bool, stack []ast.Node) (proceed bool) {
		stmt, ok := node.(*ast.IfStmt)
		if !ok {
			return false
		}

		if push {
			parentInfo := infoStack[len(infoStack)-1]
			info := analyze(pass, stmt, stack, parentInfo)

			infoStack = append(infoStack, info)

			return true
		}

		infoStack = infoStack[:len(infoStack)-1]

		return true
	})

	return analysis.Fact(nil), nil
}

func analyze(pass *analysis.Pass, stmt *ast.IfStmt, stack []ast.Node, parentInfo nodeInfo) nodeInfo {
	parent, ok := stack[len(stack)-2].(*ast.IfStmt)
	if ok && parent.Else == stmt && !parentInfo.Returns {
		// This is an 'else if' where the 'if' doesn't return, so the 'else' is not redundant
		return nodeInfo{}
	}

	if !ifAlwaysReturns(stmt, false) {
		return nodeInfo{}
	}

	reportUnnecessaryElse(pass, stmt, stack)

	return nodeInfo{Returns: true}
}

// alwaysReturns reports whether s always leaves the block it is in. inSwitch is set within the cases of a switch, where an
// unlabeled break only leaves the switch.
func alwaysReturns(s ast.Node, inSwitch bool) bool {
	switch t := s.(type) {
	case *ast.BlockStmt:
		return blockAlwaysReturns(t, inSwitch)
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		// Fallthrough doesn't return, and GOTO is unpredictable without deeper
		// understanding of where it ends up, which isn't assessed here
		if t.Tok == token.BREAK && t.Label == nil {
			return !inSwitch
		}

		return t.Tok == token.BREAK || t.Tok == token.CONTINUE
	case *ast.IfStmt:
		if !ifAlwaysReturns(t, inSwitch) {
			return false
		}

		// When we're looking into nested if/else of the IfStmt being analyzed,
		// we must also evaluate the else cases as part of the body of our main if
		return alwaysReturns(t.Else, inSwitch)
	case *ast.SwitchStmt:
		return switchAlwaysReturns(t)
	case *ast.TypeSwitchStmt:
		return typeswitchAlwaysReturns(t)
	}

	return false
}

func ifAlwaysReturns(stmt *ast.IfStmt, inSwitch bool) bool {
	if stmt.Else == nil {
		return false
	}

	return blockAlwaysReturns(stmt.Body, inSwitch)
}

func blockAlwaysReturns(block *ast.BlockStmt, inSwitch bool) bool {
	// Where the return statement is declared directly in the block, it should be the last statement
	// However, Go allows other statements after this, even if they can never be reached
	for i := len(block.List) - 1; i >= 0; i-- {
		b := block.List[i]

		if alwaysReturns(b, inSwitch) {
			return true
		}
	}

	return false
}

func switchAlwaysReturns(s *ast.SwitchStmt) bool {
	return switchBodyAlwaysReturns(s.Body)
}

func typeswitchAlwaysReturns(s *ast.TypeSwitchStmt) bool {
	return switchBodyAlwaysReturns(s.Body)
}

func switchBodyAlwaysReturns(body *ast.BlockStmt) bool {
	var defaultReturns bool

	for _, b := range body.List {
		c, ok := b.(*ast.CaseClause)
		if !ok {
			panic(fmt.Sprintf("unexpected switch body statement %+v", c))
		}

		if c.List == nil {
			defaultReturns = caseAlwaysReturns(c)
			continue
		}

		if !caseAlwaysReturns(c) {
			return false
		}
	}

	return defaultReturns
}

func caseAlwaysReturns(c *ast.CaseClause) bool {
	for i := len(c.Body) - 1; i >= 0; i-- {
		cb := c.Body[i]

		if alwaysReturns(cb, true) {
			return true
		}
	}

	return false
}

func reportUnnecessaryElse(pass *analysis.Pass, stmt *ast.IfStmt, stack []ast.Node) {
	var (
		args      []any
		msg       strings.Builder
		conflicts []string
	)

	edits := []analysis.TextEdit{}

	msg.WriteString(`Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". `)

	if stmt.Init != nil && elseUsesInit(stmt) {
		in := mustFormatNode(stmt.Init)
		edits = append(edits, analysis.TextEdit{
			Pos:     stmt.Pos(),
			End:     stmt.Init.End(),
			NewText: []byte(in + "\nif "),
		})

		msg.WriteString(`Move variable declaration %q before the "if", and remove the "else"`)
		args = append(args, mustFormatNode(stmt.Init))
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
		var endOpen token.Pos

		if len(els.List) > 0 {
			endOpen = els.List[0].Pos()
		} else {
			endOpen = els.Lbrace + 1
		}

		// Remove the 'else {', shifting the body down
		edits = append(edits, analysis.TextEdit{
			Pos:     stmt.Body.Rbrace + 1,
			End:     endOpen,
			NewText: []byte("\n\n"),
		})
		// Remove the closing '}' of the 'else'
		edits = append(edits, analysis.TextEdit{
			Pos:     els.Rbrace - 1,
			End:     els.End() + 1,
			NewText: []byte(""),
		})

		msg.WriteString(` wrapping the block of statements`)

		head, enclosing := chainHead(stack)
		conflicts = moveConflicts(pass.TypesInfo, els, head, enclosing)
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

// chainHead returns the first if statement of the if/else chain at the top of the stack, and the node enclosing the
// chain: the block or case clause that the else block moves into.
func chainHead(stack []ast.Node) (*ast.IfStmt, ast.Node) {
	head, _ := stack[len(stack)-1].(*ast.IfStmt)

	for i := len(stack) - 2; i >= 0; i-- {
		switch n := stack[i].(type) {
		case *ast.IfStmt:
			if n.Else == head {
				head = n
				continue
			}
		case *ast.LabeledStmt:
			continue
		}

		return head, stack[i]
	}

	return head, nil
}

// moveConflicts describes the names declared at the top level of els that cannot move into the block enclosing the
// if/else chain: a name already declared in that block, or one that would hide a variable used after the chain.
func moveConflicts(info *types.Info, els *ast.BlockStmt, head *ast.IfStmt, enclosing ast.Node) []string {
	elseScope, headScope := info.Scopes[els], info.Scopes[head]
	if elseScope == nil || headScope == nil || enclosing == nil {
		return nil
	}

	outer := headScope.Parent()

	var conflicts []string

	for _, name := range elseScope.Names() {
		if outer.Lookup(name) != nil {
			conflicts = append(conflicts, fmt.Sprintf("%q is already declared in the enclosing block", name))
			continue
		}

		if usedAfter(info, enclosing, head.End(), name, outer) {
			conflicts = append(conflicts, fmt.Sprintf("%q would hide the %q used after the \"if\"", name, name))
		}
	}

	return conflicts
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

func mustFormatNode(n any) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), n); err != nil {
		panic(fmt.Errorf("unable to generate formatted node for %+v", n))
	}

	return buf.String()
}
