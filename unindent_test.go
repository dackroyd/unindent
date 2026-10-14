package unindent_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/dackroyd/unindent"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	a := unindent.NewAnalyzer()

	analysistest.RunWithSuggestedFixes(t, testdataDir(t), a, "unindent", "wrappingrest")
}

func TestAnalyzerFlags(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		pkg   string
		flags map[string]string
	}{
		{"noelseif", map[string]string{"else-if-after-exit": "false"}},
		{"noelse", map[string]string{"else-after-exit": "false"}},
		{"noexitinelse", map[string]string{"exit-in-else": "false"}},
		{"noifwrappingrest", map[string]string{"if-wrapping-rest": "false"}},
		{"wrappingmin", map[string]string{"if-wrapping-rest-min-statements": "2"}},
		{"wrappingmin5", map[string]string{"if-wrapping-rest-min-statements": "5"}},
		{"nochecks", map[string]string{
			"else-after-exit": "false", "else-if-after-exit": "false", "exit-in-else": "false", "if-wrapping-rest": "false",
		}},
	} {
		t.Run(tt.pkg, func(t *testing.T) {
			t.Parallel()

			a := unindent.NewAnalyzer()
			for name, value := range tt.flags {
				require.NoError(t, a.Flags.Set(name, value))
			}

			analysistest.RunWithSuggestedFixes(t, testdataDir(t), a, tt.pkg)
		})
	}
}

func TestNewAnalyzerFlags(t *testing.T) {
	t.Parallel()

	a, b := unindent.NewAnalyzer(), unindent.NewAnalyzer()

	for _, name := range []string{"else-after-exit", "else-if-after-exit", "exit-in-else", "if-wrapping-rest"} {
		f := a.Flags.Lookup(name)
		require.NotNil(t, f, name)
		require.Equal(t, "true", f.DefValue, name)

		require.NoError(t, a.Flags.Set(name, "false"))
		require.Equal(t, "false", a.Flags.Lookup(name).Value.String(), name)
		require.Equal(t, "true", b.Flags.Lookup(name).Value.String(), "flags are shared between analyzers")
	}
}

func TestWrappingMinFlag(t *testing.T) {
	t.Parallel()

	a, b := unindent.NewAnalyzer(), unindent.NewAnalyzer()
	f := a.Flags.Lookup("if-wrapping-rest-min-statements")
	require.NotNil(t, f)
	require.Equal(t, "3", f.DefValue)

	require.NoError(t, a.Flags.Set(f.Name, "1"))
	require.Equal(t, "1", f.Value.String())
	require.Equal(t, "3", b.Flags.Lookup(f.Name).Value.String(), "flags are shared between analyzers")

	require.ErrorContains(t, a.Flags.Set(f.Name, "0"), "must be at least 1")
	require.Error(t, a.Flags.Set(f.Name, "five"))
	require.Equal(t, "1", f.Value.String())
}

// TestFixedNotReported checks that the fixed code of the if-wrapping-rest testdata is reported by no check, so the
// checks never undo each other's fixes.
func TestFixedNotReported(t *testing.T) {
	t.Parallel()

	golden, err := os.ReadFile(filepath.Join(testdataDir(t), "src", "wrappingrest", "wrappingrest.go.golden"))
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src", "fixed"), 0o755))

	// The expectations of the first run stay in the fixed code as comments, and would expect the same reports again.
	src := regexp.MustCompile(`// want .*`).ReplaceAll(golden, nil)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "fixed", "fixed.go"), src, 0o600))

	analysistest.Run(t, dir, unindent.NewAnalyzer(), "fixed")
}

func testdataDir(t *testing.T) string {
	t.Helper()

	_, testFilename, _, ok := runtime.Caller(1)
	if !ok {
		require.Fail(t, "unable to get current test filename")
	}

	return filepath.Join(filepath.Dir(testFilename), "testdata")
}
