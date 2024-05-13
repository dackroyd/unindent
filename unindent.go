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

func (analyzer) run(pass *analysis.Pass) (interface{}, error) {
	ins, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, fmt.Errorf("unexpected analyzer type for initial inspection %T", pass.ResultOf[inspect.Analyzer])
	}

	ins.WithStack([]ast.Node{&ast.IfStmt{}}, func(node ast.Node, push bool, stack []ast.Node) (proceed bool) {
		stmt, ok := node.(*ast.IfStmt)
		if !ok {
			return false
		}

		if push {
			return analyze(pass, stmt, stack)
		}

		return true
	})

	return analysis.Fact(nil), nil
}

func analyze(pass *analysis.Pass, stmt *ast.IfStmt, stack []ast.Node) bool {
	if !alwaysReturns(stmt) {
		return false
	}

	return reportUnnecessaryElse(pass, stmt, stack)
}

func alwaysReturns(s ast.Node) bool {
	switch t := s.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.IfStmt:
		return ifAlwaysReturns(t)
	}

	return false
}

func ifAlwaysReturns(stmt *ast.IfStmt) bool {
	if stmt.Else == nil {
		return false
	}

	// Where the return statement is declared directly in the "if" block, it should be the last statement
	// However, Go allows other statements after this, even if they can never be reached
	for i := len(stmt.Body.List) - 1; i >= 0; i-- {
		b := stmt.Body.List[i]

		if alwaysReturns(b) {
			return true
		}
	}

	return false
}

func reportUnnecessaryElse(pass *analysis.Pass, stmt *ast.IfStmt, _ []ast.Node) bool {
	var (
		args []any
		msg  strings.Builder
	)

	edits := []analysis.TextEdit{}

	msg.WriteString(`Unnecessary "else": preceding conditions always end in a "return". `)

	if stmt.Init == nil {
		msg.WriteString(`Remove the "else"`)
	} else {
		// Pull the 'init' statement our of the 'if'
		// FIXME: this only needs to be done if the 'else' part uses the variable
		in := mustFormatNode(stmt.Init)
		edits = append(edits, analysis.TextEdit{
			Pos:     stmt.Pos(),
			End:     stmt.Init.End(),
			NewText: []byte(in + "\nif "),
		})

		msg.WriteString(`Move variable declaration %q before the "if", and remove the "else"`)
		args = append(args, mustFormatNode(stmt.Init))
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
		return false
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

	return true
}

func mustFormatNode(n any) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), n); err != nil {
		panic(fmt.Errorf("unable to generate formatted node for %+v", n))
	}

	return buf.String()
}
