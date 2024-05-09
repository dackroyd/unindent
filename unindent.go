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

func analyze(pass *analysis.Pass, stmt *ast.IfStmt, _ []ast.Node) bool {
	if stmt.Else == nil {
		return false
	}

	var alwaysReturns bool

	// Where the return statement is declared directly in the "if" block, it should be the last statement
	// However, Go allows other statements after this, even if they can never be reached
	for i := len(stmt.Body.List) - 1; i >= 0; i-- {
		b := stmt.Body.List[i]
		if _, ok := b.(*ast.ReturnStmt); ok {
			alwaysReturns = true
			break
		}
	}

	if !alwaysReturns {
		return false
	}

	var (
		args []any
		msg  strings.Builder
	)

	msg.WriteString(`Unnecessary "else": preceding conditions always end in a "return". `)

	if stmt.Init == nil {
		msg.WriteString(`Remove the "else"`)
	} else {
		msg.WriteString(`Move variable declaration %q before the "if", and remove the "else"`)
		args = append(args, mustFormatNode(stmt.Init))
	}

	switch els := stmt.Else.(type) {
	case *ast.IfStmt:
		// "else if <cond>"
		msg.WriteString(`, leaving "if %s { ... }"`)
		args = append(args, mustFormatNode(els.Cond))
	case *ast.BlockStmt:
		// "else"
		msg.WriteString(` wrapping the block of statements`)
	default:
		pass.Reportf(els.Pos(), `Unexpected ast in "else": %s`, mustFormatNode(stmt.Else))
		return false
	}

	pass.Reportf(stmt.Else.Pos(), msg.String(), args...)

	return true
}

func mustFormatNode(n any) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), n); err != nil {
		panic(fmt.Errorf("unable to generate formatted node for %+v", n))
	}

	return buf.String()
}
