package unindent_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/dackroyd/unindent"
)

// BenchmarkSynthetic analyzes generated code shaped to expose costs that grow faster than the code: long "else if"
// chains whose initialisers move before the "if", blocks with many reported statements, and deep nesting, for both an
// unnecessary "else" and an exit in an else.
func BenchmarkSynthetic(b *testing.B) {
	for _, bc := range []struct {
		name string
		gen  func(*strings.Builder, int)
		n    int
	}{
		{"chain", genChain, 14},
		{"block", genBlock, 400},
		{"nested", genNested, 40},
		{"inverted", genInverted, 400},
		{"nestedinverted", genNestedInverted, 40},
	} {
		b.Run(fmt.Sprintf("%s=%d", bc.name, bc.n), func(b *testing.B) {
			var src strings.Builder

			src.WriteString("package bench\n\nfunc f(int) (int, error) { return 0, nil }\n\n")
			bc.gen(&src, bc.n)

			dir := b.TempDir()
			writeFile(b, filepath.Join(dir, "go.mod"), "module bench\n\ngo 1.26\n")
			writeFile(b, filepath.Join(dir, "bench.go"), src.String())

			benchmarkAnalyze(b, dir, ".")
		})
	}
}

// BenchmarkStdlib analyzes standard library packages, including the dependencies that the control flow analysis
// visits to find functions that never return.
func BenchmarkStdlib(b *testing.B) {
	benchmarkAnalyze(b, "", "fmt", "go/types", "net/http")
}

func benchmarkAnalyze(b *testing.B, dir string, patterns ...string) {
	b.Helper()

	cfg := &packages.Config{Mode: packages.LoadAllSyntax, Dir: dir}
	pkgs, err := packages.Load(cfg, patterns...)
	require.NoError(b, err)
	require.Zero(b, packages.PrintErrors(pkgs))

	analyzers := []*analysis.Analyzer{unindent.NewAnalyzer()}

	for b.Loop() {
		graph, err := checker.Analyze(analyzers, pkgs, nil)
		require.NoError(b, err)

		var diags int
		for _, root := range graph.Roots {
			require.NoError(b, root.Err)
			diags += len(root.Diagnostics)
		}

		require.NotZero(b, diags)
	}
}

// genChain writes an "else if" chain of n links, where each condition uses what the previous initialiser declares,
// so every initialiser moves before the "if".
func genChain(src *strings.Builder, n int) {
	src.WriteString("func Chain() (int, error) {\n\tif v0, err0 := f(0); err0 != nil {\n\t\treturn 0, err0\n")

	for i := 1; i < n; i++ {
		fmt.Fprintf(src, "\t} else if v%d, err%d := f(v%d); err%d != nil {\n\t\treturn v%d, err%d\n", i, i, i-1, i, i-1, i)
	}

	fmt.Fprintf(src, "\t} else {\n\t\treturn v%d, nil\n\t}\n}\n\n", n-1)
}

// genBlock writes a function of n statements that each end in an unnecessary "else".
func genBlock(src *strings.Builder, n int) {
	src.WriteString("func Block() (int, error) {\n\ttotal := 0\n\n")

	for i := range n {
		fmt.Fprintf(src, "\tif v%d, err := f(total); err != nil {\n\t\treturn 0, err\n\t} else {\n", i)
		fmt.Fprintf(src, "\t\tx%d := v%d * 2\n\t\ttotal += x%d\n\t}\n\n", i, i, i)
	}

	src.WriteString("\treturn total, nil\n}\n\n")
}

// genNested writes if statements nested n deep, each with an unnecessary "else".
func genNested(src *strings.Builder, n int) {
	src.WriteString("func Nested(x int) int {\n")

	for i := range n {
		indent := strings.Repeat("\t", i+1)
		fmt.Fprintf(src, "%sif x == %d {\n%s\treturn %d\n%s} else {\n", indent, i, indent, i, indent)
	}

	for i := n - 1; i >= 0; i-- {
		indent := strings.Repeat("\t", i+1)
		fmt.Fprintf(src, "%s\treturn x\n%s}\n", indent, indent)
	}

	src.WriteString("}\n")
}

// genInverted writes a function of n statements that each have an else that exits after a body that doesn't, whose
// initialiser moves before the "if".
func genInverted(src *strings.Builder, n int) {
	src.WriteString("func Inverted() (int, error) {\n\ttotal := 0\n\n")

	for i := range n {
		fmt.Fprintf(src, "\tif v%d, err := f(total); err == nil {\n\t\tx%d := v%d * 2\n\t\ttotal += x%d\n", i, i, i, i)
		src.WriteString("\t} else {\n\t\treturn 0, err\n\t}\n\n")
	}

	src.WriteString("\treturn total, nil\n}\n\n")
}

// genNestedInverted writes if statements nested n deep, each with an else that exits after a body that doesn't.
func genNestedInverted(src *strings.Builder, n int) {
	src.WriteString("func NestedInverted(x int) int {\n")

	for i := range n {
		indent := strings.Repeat("\t", i+1)
		fmt.Fprintf(src, "%sif x != %d {\n%s\ty%d := x + %d\n", indent, i, indent, i, i)
	}

	for i := n - 1; i >= 0; i-- {
		indent := strings.Repeat("\t", i+1)
		fmt.Fprintf(src, "%s\tx += y%d\n%s} else {\n%s\treturn %d\n%s}\n", indent, i, indent, indent, i, indent)
	}

	src.WriteString("\treturn x\n}\n\n")
}

func writeFile(b *testing.B, name, content string) {
	b.Helper()

	require.NoError(b, os.WriteFile(name, []byte(content), 0o600))
}
