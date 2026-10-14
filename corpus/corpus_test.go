package main

import (
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestLostComments(t *testing.T) {
	t.Parallel()

	old := `package p

func f(x int) int {
	if x > 0 {
		return 1
	} else {
		// kept
		// lost
		return 2 // repeated
	}
	return 3 // repeated
}
`
	cur := `package p

func f(x int) int {
	if x > 0 {
		return 1
	}
	// kept
	return 2 // repeated
}
`

	got := lostComments([]byte(old), []byte(cur))
	want := []string{"// lost", "// repeated"}

	if !slices.Equal(got, want) {
		t.Errorf("lostComments() = %q, want %q", got, want)
	}

	got = lostComments([]byte(old), []byte("package p\nfunc {"))
	if len(got) != 1 || !strings.Contains(got[0], "does not parse") {
		t.Errorf("lostComments() on a file that does not parse = %q", got)
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()

	got := diff("a\nb\nb\nc\n", "a\nb\nc\nd\n")
	want := "-b\n+d\n"

	if got != want {
		t.Errorf("diff() = %q, want %q", got, want)
	}

	if d := diff("a\n", "a\n"); d != "" {
		t.Errorf("diff() of equal input = %q, want none", d)
	}
}

func TestModules(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	for _, m := range []string{".", "sub", "sub/nested", "testdata/x", "vendor/x", "_skip", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(dir, m), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(dir, m, "go.mod"), []byte("module x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := modules(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{".", "./sub", "./sub/nested"}
	if !slices.Equal(got, want) {
		t.Errorf("modules() = %q, want %q", got, want)
	}
}

func TestCompare(t *testing.T) {
	t.Parallel()

	src := `package p

func f(x int, c chan int) int {
	if x > 0 {
		return 1
	} else if x < 0 {
		return 2
	} else {
		x++
	}

	select {
	case <-c:
		if x > 1 {
			return 3
		} else {
			x++
		}
	}

	if x > 2 {
		panic(x)
	} else {
		x++
	}

	if x > 3 {
		x++
	} else {
		return x
	}

	if x > 4 {
		x++
	} else {
		panic(x)
	}

	if x > 5 {
		x++
	} else {
		fatal()
	}

	return x
}

func g(x int) int {
	if x > 0 {
		x++
	} else if x < 0 {
		x--
	} else {
		return x
	}

	return x
}
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	at := func(line, col int) position { return position{Project: "stdlib", File: "p.go", Line: line, Col: col} }

	findings := []finding{
		{Pos: at(6, 9)},   // else if x < 0, also reported by revive
		{Pos: at(8, 9)},   // else after return 2
		{Pos: at(16, 10)}, // else in a select case
		{Pos: at(23, 9)},  // else after panic
		{Pos: at(99, 1)},  // not on an if statement
		{Pos: at(16, 10)}, // reported twice, counted once
		{Pos: at(29, 9), Kind: "exit in else ending in return"}, // also reported by revive
		{Pos: at(41, 9), Kind: "exit in else ending in call"},   // a helper that never returns
	}
	revive := []reviveReport{
		{Pos: at(6, 9), Rule: "indent-error-flow"},
		{Pos: at(27, 2), Rule: "early-return"},
		{Pos: at(33, 2), Rule: "early-return"},
		{Pos: at(51, 9), Rule: "early-return"}, // an else if link
	}

	got := compare([]project{{Name: "stdlib"}}, map[string]string{"stdlib": dir}, findings, revive)

	if !strings.Contains(got, "\nstdlib   6         4       2     4              2\n") {
		t.Errorf("compare() has the wrong counts for stdlib in:\n%s", got)
	}

	for _, want := range []string{
		"other: 1",
		"in a select case: 1",
		"ends in a call: 1",
		"exit in else: ends in a call: 1",
		"Reported by revive only:\n  early-return: 1\n  early-return on an else if: 1",
		"not on an if statement with an else:\n  stdlib p.go:99:1 unindent\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("compare() is missing %q in:\n%s", want, got)
		}
	}
}

func TestTestFailures(t *testing.T) {
	t.Parallel()

	out := `{"Action":"run","Package":"p","Test":"TestA"}
{"Action":"fail","Package":"p","Test":"TestA/sub","Elapsed":0.1}
{"Action":"fail","Package":"p","Test":"TestA","Elapsed":0.1}
{"Action":"pass","Package":"p","Test":"TestB"}
not JSON
{"Action":"fail","Package":"p","Elapsed":0.2}
{"Action":"fail","Package":"q","FailedBuild":"q [q.test]"}
{"Action":"fail","Package":"p","Test":"TestA"}
`

	got := testFailures(out)
	want := []string{"p", "p TestA", "p TestA/sub", "q"}

	if !slices.Equal(got, want) {
		t.Errorf("testFailures() = %q, want %q", got, want)
	}
}

func TestConfirmBroken(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name            string
		before          []string
		broken          []string
		runs            [][]string
		wantRerun       [][]string
		want, wantFlaky []string
	}{
		{
			name: "nothing broken reruns nothing",
		},
		{
			name:      "failing every time",
			broken:    []string{"p", "p TestA"},
			runs:      [][]string{{"p", "p TestA"}, {"p", "p TestA"}},
			wantRerun: [][]string{{"p"}, {"p"}},
			want:      []string{"p", "p TestA"},
		},
		{
			name:      "passing on the first rerun",
			broken:    []string{"p", "p TestA", "q", "q TestB"},
			runs:      [][]string{{"q", "q TestB"}, {"q", "q TestB"}},
			wantRerun: [][]string{{"p", "q"}, {"q"}},
			want:      []string{"q", "q TestB"},
			wantFlaky: []string{"p", "p TestA"},
		},
		{
			name:      "passing on the last rerun",
			broken:    []string{"p", "p TestA"},
			runs:      [][]string{{"p", "p TestA"}, nil},
			wantRerun: [][]string{{"p"}, {"p"}},
			wantFlaky: []string{"p", "p TestA"},
		},
		{
			name:      "failures before the fixes don't count",
			before:    []string{"p", "p TestB"},
			broken:    []string{"p TestA"},
			runs:      [][]string{{"p", "p TestB"}},
			wantRerun: [][]string{{"p"}},
			wantFlaky: []string{"p TestA"},
		},
		{
			name:      "a failure without a package reruns every package",
			broken:    []string{"go test: exit status 1", "p TestA"},
			runs:      [][]string{{"go test: exit status 1"}, {"go test: exit status 1"}},
			wantRerun: [][]string{nil, nil},
			want:      []string{"go test: exit status 1"},
			wantFlaky: []string{"p TestA"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var reran [][]string

			got, flaky := confirmBroken(tt.before, tt.broken, 2, func(pkgs []string) []string {
				reran = append(reran, pkgs)

				return tt.runs[len(reran)-1]
			})

			if !slices.Equal(got, tt.want) || !slices.Equal(flaky, tt.wantFlaky) {
				t.Errorf("confirmBroken() = %q, %q, want %q, %q", got, flaky, tt.want, tt.wantFlaky)
			}

			if !slices.EqualFunc(reran, tt.wantRerun, slices.Equal) {
				t.Errorf("confirmBroken() reran %q, want %q", reran, tt.wantRerun)
			}
		})
	}
}

func TestTestStatus(t *testing.T) {
	t.Parallel()

	if got := testStatus(testResult{}); got != "not broken by the fixes" {
		t.Errorf("testStatus() without broken tests = %q", got)
	}

	if got := testStatus(testResult{Broken: []string{"p TestA"}}); got != "BROKEN BY THE FIXES" {
		t.Errorf("testStatus() with broken tests = %q", got)
	}
}

// env returns the value cmd runs with for the environment variable key, the last one set.
func env(cmd *exec.Cmd, key string) string {
	var v string

	for _, kv := range cmd.Env {
		if k, val, ok := strings.Cut(kv, "="); ok && k == key {
			v = val
		}
	}

	return v
}

func TestCommandPlatform(t *testing.T) {
	t.Setenv("GOOS", "plan9")
	t.Setenv("GOARCH", "386")
	t.Setenv("CGO_ENABLED", "1")

	r := &runner{goVersion: "go1.27.0"}

	native := r.command(".", "go", "test")
	for key, want := range map[string]string{
		"GOOS": runtime.GOOS, "GOARCH": runtime.GOARCH, "CGO_ENABLED": "", "GOTOOLCHAIN": "go1.27.0", "GOFLAGS": "",
	} {
		if got := env(native, key); got != want {
			t.Errorf("command() runs with %s=%q, want %q", key, got, want)
		}
	}

	cross := r.targetCommand(".", "go", "build")
	for key, want := range map[string]string{"GOOS": "linux", "GOARCH": "amd64", "CGO_ENABLED": "0"} {
		if got := env(cross, key); got != want {
			t.Errorf("targetCommand() runs with %s=%q, want %q", key, got, want)
		}
	}
}

func TestSets(t *testing.T) {
	t.Parallel()

	if _, ok := findSet(defaultSet); !ok {
		t.Errorf("there is no %q set", defaultSet)
	}

	names := map[string]bool{}

	for _, set := range sets {
		for _, v := range set.Releases {
			if !slices.Contains(goVersions, v) {
				t.Errorf("the %s set is checked with %s, which is not in goVersions", set.Name, v)
			}
		}

		if set.Doc != "" && !slices.Contains(set.Releases, latestGo()) {
			t.Errorf("the %s set has a documentation page but is not checked with %s", set.Name, latestGo())
		}

		for _, p := range set.Projects {
			if names[p.Name] {
				t.Errorf("there is more than one project named %s", p.Name)
			}

			names[p.Name] = true

			if p.Repo != "" && (len(p.Commit) != 40 || p.Date == "") {
				t.Errorf("%s is at %q of %q, want a full commit hash and its date", p.Name, p.Commit, p.Date)
			}

			if p.Description == "" {
				t.Errorf("%s has no description", p.Name)
			}
		}
	}
}

func TestUnits(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	for _, f := range []string{"go.mod", "a/a.go", "b/c/c.go", "testdata/t.go", "_x/x.go", "sub/go.mod", "sub/s.go"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(f)), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(dir, f), []byte("module x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mods := []string{".", "./sub"}

	for _, tt := range []struct {
		batched bool
		want    []unit
	}{
		{false, []unit{{".", "./..."}, {"./sub", "./..."}}},
		{true, []unit{{".", "./a/..."}, {".", "./b/..."}, {"./sub", "."}}},
	} {
		got, err := units(project{Batched: tt.batched}, dir, mods)
		if err != nil {
			t.Fatal(err)
		}

		if !slices.Equal(got, tt.want) {
			t.Errorf("units() with Batched %t = %v, want %v", tt.batched, got, tt.want)
		}
	}
}

func TestCheckStatus(t *testing.T) {
	t.Parallel()

	failed := errors.New("failed")

	for c, want := range map[checkResult]string{
		{}:                              "passes",
		{Before: failed, After: failed}: "fails before and after the fixes",
		{After: failed}:                 "FAILS AFTER THE FIXES",
		{Before: failed}:                "fails before the fixes only",
	} {
		if got := checkStatus(c); got != want {
			t.Errorf("checkStatus(%v) = %q, want %q", c, got, want)
		}
	}

	got := renderFixes([]fixResult{{
		Project: "p",
		Modules: []moduleResult{{Module: ".", Build: checkResult{After: failed}, Vet: checkResult{Before: failed, After: failed}}},
	}})

	if want := "p .: build FAILS AFTER THE FIXES; vet fails before and after the fixes\n"; !strings.Contains(got, want) {
		t.Errorf("renderFixes() is missing %q in:\n%s", want, got)
	}
}

func TestFindingKind(t *testing.T) {
	t.Parallel()

	src := `package p

func f(x int) int {
	if x > 0 {
		return 1
	} else if x < 0 {
		println()
	} else {
		return 2
	}

	switch {
	default:
	}

	if x > 1 {
		switch {
		}
	} else {
		x++
	}

	if x > 2 {
	loop:
		for {
			continue loop
		}
	} else if x > 3 {
		select {}
	} else if x > 4 {
		goto end
	} else {
		x++
	}

	if x > 5 {
		x++
	} else {
		goto end
	}

end:
	return x
}
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	ifs := &ifIndex{dirs: map[string]string{"p": dir}, files: map[string]*fileIfs{}}

	for _, tt := range []struct {
		line, col int
		message   string
		want      string
	}{
		{6, 9, "", "else if after return"},
		{8, 9, "", "else after call"},
		{19, 9, "", "else after switch"},
		{28, 9, "", "else if after for"},
		{30, 9, "", "else if after select"},
		{32, 9, "", "else after goto"},
		{4, 2, "", "not on an else"},
		{19, 9, `Exit in "else": ...`, "exit in else ending in other"},
		{32, 9, `Exit in "else": ...`, "exit in else ending in other"},
		{38, 9, `Exit in "else": ...`, "exit in else ending in goto"},
	} {
		pos := position{Project: "p", File: "p.go", Line: tt.line, Col: tt.col}
		if got := findingKind(ifs.file("p", "p.go").byElse[pos], tt.message); got != tt.want {
			t.Errorf("findingKind() at %d:%d = %q, want %q", tt.line, tt.col, got, tt.want)
		}
	}
}

func TestSummarise(t *testing.T) {
	t.Parallel()

	findings := `# comment
p a.go:1:1 else after return: fixed
p a.go:2:1 else after return: not fixed: "x" is already declared in the enclosing block
p a.go:3:1 else if after call: not fixed: "y" is already declared in the enclosing block; "y" would hide the "y" used after the "if"
p a.go:4:1 not on an else: no fix
`
	fixes := `# comment
p: 2 files changed, 1 comments lost
p lost a.go: // c
p .: build passes; vet passes
p ./sub: build passes; vet fails before and after the fixes
p tests: not broken by the fixes
`

	sums, err := summarise([]project{{Name: "p"}}, findings, fixes)
	if err != nil {
		t.Fatal(err)
	}

	s := sums["p"]
	if s.Findings != 4 || s.Fixed != 1 || s.Withheld != 2 || s.NoFix != 1 || s.Changed != 2 || s.Lost != 1 ||
		s.Modules != 2 || s.Tests != "not broken by the fixes" {
		t.Errorf("summarise() = %+v", *s)
	}

	reasons := map[string]int{
		`"…" is already declared in the enclosing block`: 2,
		`"…" would hide the "…" used after the "if"`:     1,
	}
	if !maps.Equal(s.Reasons, reasons) {
		t.Errorf("summarise() counts the reasons %v, want %v", s.Reasons, reasons)
	}

	if got, want := statusCounts(s.Vet, s.Modules), "fails before and after the fixes in 1, passes in 1"; got != want {
		t.Errorf("statusCounts() = %q, want %q", got, want)
	}

	if _, err := summarise(nil, findings, fixes); err == nil {
		t.Error("summarise() of an unknown project succeeds, want an error")
	}
}

func TestDocs(t *testing.T) {
	t.Parallel()

	for _, set := range sets {
		if set.Doc == "" {
			continue
		}

		want, err := recordedDoc(set)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(set.Doc)
		if err != nil {
			t.Fatal(err)
		}

		if string(got) != want {
			t.Errorf("%s does not match the results of the %s set: run \"go run . docs\"\n%s", set.Doc, set.Name,
				diff(string(got), want))
		}
	}
}
