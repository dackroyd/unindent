package unindent

import "testing"

func TestConstrainedName(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]bool{
		"race0.go":             false,
		"os_linux.go":          true,
		"stubs_amd64.go":       true,
		"sys_darwin_arm64.go":  true,
		"exec_windows_test.go": true,
		"linux.go":             false,
		"x_unix.go":            false,
		"x_test.go":            false,
		"x_linux.s.go":         true,
	} {
		if got := constrainedName(name); got != want {
			t.Errorf("constrainedName(%q) = %t, want %t", name, got, want)
		}
	}
}
