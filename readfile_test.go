package unindent

import (
	"errors"
	"testing"
)

func TestCachedReadFile(t *testing.T) {
	t.Parallel()

	reads := map[string]int{}
	errMissing := errors.New("missing")

	readFile := cachedReadFile(func(name string) ([]byte, error) {
		reads[name]++
		if name == "missing.go" {
			return nil, errMissing
		}

		return []byte(name), nil
	})

	for range 3 {
		if got, err := readFile("a.go"); err != nil || string(got) != "a.go" {
			t.Errorf(`readFile("a.go") = %q, %v, want "a.go", nil`, got, err)
		}

		if _, err := readFile("missing.go"); !errors.Is(err, errMissing) {
			t.Errorf(`readFile("missing.go") error = %v, want %v`, err, errMissing)
		}
	}

	if reads["a.go"] != 1 || reads["missing.go"] != 1 {
		t.Errorf("reads = %v, want each file read once", reads)
	}
}
