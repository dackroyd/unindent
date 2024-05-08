package unindent

import (
	"fmt"
	"go/ast"

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

	ins.Preorder([]ast.Node{&ast.IfStmt{}}, func(node ast.Node) {
		stmt, ok := node.(*ast.IfStmt)
		if !ok {
			return
		}

		analyze(pass, stmt)
	})

	return analysis.Fact(nil), nil
}

func analyze(pass *analysis.Pass, stmt *ast.IfStmt) {
	// TODO
}
