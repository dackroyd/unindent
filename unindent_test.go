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

	analysistest.Run(t, testdataDir(t), a, "unindent")
}

func testdataDir(t *testing.T) string {
	t.Helper()

	_, testFilename, _, ok := runtime.Caller(1)
	if !ok {
		require.Fail(t, "unable to get current test filename")
	}

	return filepath.Join(filepath.Dir(testFilename), "testdata")
}
