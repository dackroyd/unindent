package unindent_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/dackroyd/unindent"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	a := unindent.NewAnalyzer()

	analysistest.RunWithSuggestedFixes(t, testdataDir(t), a, "unindent")
}

func TestAnalyzerFlags(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		pkg   string
		flags map[string]string
	}{
		{"noelseif", map[string]string{"else-if-after-exit": "false"}},
		{"noelse", map[string]string{"else-after-exit": "false"}},
		{"nochecks", map[string]string{"else-after-exit": "false", "else-if-after-exit": "false"}},
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

	for _, name := range []string{"else-after-exit", "else-if-after-exit"} {
		f := a.Flags.Lookup(name)
		require.NotNil(t, f, name)
		require.Equal(t, "true", f.DefValue, name)

		require.NoError(t, a.Flags.Set(name, "false"))
		require.Equal(t, "false", a.Flags.Lookup(name).Value.String(), name)
		require.Equal(t, "true", b.Flags.Lookup(name).Value.String(), "flags are shared between analyzers")
	}
}

func testdataDir(t *testing.T) string {
	t.Helper()

	_, testFilename, _, ok := runtime.Caller(1)
	if !ok {
		require.Fail(t, "unable to get current test filename")
	}

	return filepath.Join(filepath.Dir(testFilename), "testdata")
}
