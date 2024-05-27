package unindent

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
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

	if !ifAlwaysReturns(stmt) {
		return nodeInfo{}
	}

	reportUnnecessaryElse(pass, stmt, stack)

	return nodeInfo{Returns: true}
}

func alwaysReturns(s ast.Node) bool {
	switch t := s.(type) {
	case *ast.BlockStmt:
		return blockAlwaysReturns(t)
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		// Fallthrough doesn't return, and GOTO is unpredictable without deeper
		// understanding of where it ends up, which isn't assessed here
		return t.Tok == token.BREAK || t.Tok == token.CONTINUE
	case *ast.IfStmt:
		if !ifAlwaysReturns(t) {
			return false
		}

		// When we're looking into nested if/else of the IfStmt being analyzed,
		// we must also evaluate the else cases as part of the body of our main if
		return alwaysReturns(t.Else)
	case *ast.SwitchStmt:
		return switchAlwaysReturns(t)
	case *ast.TypeSwitchStmt:
		return typeswitchAlwaysReturns(t)
	}

	return false
}

func ifAlwaysReturns(stmt *ast.IfStmt) bool {
	if stmt.Else == nil {
		return false
	}

	return blockAlwaysReturns(stmt.Body)
}

func blockAlwaysReturns(block *ast.BlockStmt) bool {
	// Where the return statement is declared directly in the block, it should be the last statement
	// However, Go allows other statements after this, even if they can never be reached
	for i := len(block.List) - 1; i >= 0; i-- {
		b := block.List[i]

		if alwaysReturns(b) {
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

		if alwaysReturns(cb) {
			return true
		}
	}

	return false
}

func reportUnnecessaryElse(pass *analysis.Pass, stmt *ast.IfStmt, _ []ast.Node) {
	var (
		args []any
		msg  strings.Builder
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
	default:
		pass.Reportf(els.Pos(), `Unexpected ast in "else": %s`, mustFormatNode(stmt.Else))
		return
	}

	pass.Report(
		analysis.Diagnostic{
			Pos:     stmt.Else.Pos(),
			Message: fmt.Sprintf(msg.String(), args...),
			SuggestedFixes: []analysis.SuggestedFix{
				{Message: "Remove unnecessary else", TextEdits: edits},
			},
		},
	)
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
