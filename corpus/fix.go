package main

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// moduleResult holds whether a module builds and vets, before and after the fixes. Build and vet are recorded apart,
// so that one failing before the fixes cannot hide the other failing because of them.
type moduleResult struct {
	Module string
	Build  checkResult
	Vet    checkResult
}

// checkResult is the error of a check before and after the fixes, nil when it passes.
type checkResult struct {
	Before, After error
}

type fixResult struct {
	Project string
	Changed int
	// Lost lists the comments missing after the fixes, as "file: comment".
	Lost    []string
	Modules []moduleResult
	// Tests holds the result of running the project's tests before and after the fixes, when it has tests to run.
	Tests *testResult
}

// testResult compares the tests that fail before and after the fixes. Tests run natively, so which tests fail before
// the fixes depends on the host, and only the failures the fixes add are recorded.
type testResult struct {
	// Broken lists the tests and packages that fail after the fixes but not before, on the first run and every rerun.
	Broken []string
}

// fix applies unindent's fixes to every module of p, checks the result, then undoes them.
func (r *runner) fix(bin string, p project, dir string) (fixResult, error) {
	res := fixResult{Project: p.Name}

	mods, err := modules(dir)
	if err != nil {
		return res, err
	}

	runs, err := units(p, dir, mods)
	if err != nil {
		return res, err
	}

	before := map[string]moduleResult{}
	for _, mod := range mods {
		before[mod] = moduleResult{
			Build: checkResult{Before: r.build(filepath.Join(dir, mod))},
			Vet:   checkResult{Before: r.vet(filepath.Join(dir, mod))},
		}
	}

	var testsBefore []string
	if len(p.Tests) > 0 {
		testsBefore = r.runTests(dir, p.Tests)

		if len(testsBefore) > 0 {
			log.Printf("%s: on %s/%s, these tests fail before the fixes, so only other failures count:\n%s",
				p.Name, runtime.GOOS, runtime.GOARCH, strings.Join(testsBefore, "\n"))
		}
	}

	if err := r.cleanBuildCache(); err != nil {
		return res, err
	}

	for _, u := range runs {
		// unindent exits with a non-zero status when it reports anything. Whether the fixes applied cleanly is what the
		// checks after it are for.
		_, _ = run(r.targetCommand(filepath.Join(dir, u.Module), bin, "-fix", u.Pattern))
	}

	out, err := run(r.command(dir, "git", "diff", "--name-only"))
	if err != nil {
		return res, err
	}

	changed := strings.Fields(out)
	res.Changed = len(changed)

	for _, file := range changed {
		lost, err := r.lostInFile(dir, file)
		if err != nil {
			return res, err
		}

		for _, c := range lost {
			res.Lost = append(res.Lost, file+": "+c)
		}
	}

	for _, mod := range mods {
		m := before[mod]
		m.Module = mod
		m.Build.After = r.build(filepath.Join(dir, mod))
		m.Vet.After = r.vet(filepath.Join(dir, mod))
		res.Modules = append(res.Modules, m)

		for _, c := range []struct {
			name   string
			result checkResult
		}{{"go build", m.Build}, {"go vet", m.Vet}} {
			switch {
			case c.result.After != nil && c.result.Before == nil:
				log.Printf("%s %s: %s fails after the fixes:\n%s", p.Name, mod, c.name, excerpt(c.result.After))
			case c.result.Before != nil:
				log.Printf("%s %s: %s fails before the fixes:\n%s", p.Name, mod, c.name, excerpt(c.result.Before))
			}
		}
	}

	if len(p.Tests) > 0 {
		failed := newFailures(testsBefore, r.runTests(dir, p.Tests))

		broken, flaky := confirmBroken(testsBefore, failed, testReruns, func(pkgs []string) []string {
			if pkgs == nil {
				pkgs = p.Tests
			}

			return r.runTests(dir, append([]string{"-count=1"}, pkgs...))
		})
		res.Tests = &testResult{Broken: broken}

		if len(flaky) > 0 {
			log.Printf("%s: these tests failed after the fixes but passed on a rerun, so they count as flaky, not broken:\n%s",
				p.Name, strings.Join(flaky, "\n"))
		}

		if len(res.Tests.Broken) > 0 {
			log.Printf("%s: the fixes break these tests:\n%s", p.Name, strings.Join(res.Tests.Broken, "\n"))
		}
	}

	if err := r.cleanBuildCache(); err != nil {
		return res, err
	}

	return res, r.restore(dir)
}

// testReruns is how many more times the packages of tests that fail only after the fixes run before the failures
// count, so that a flaky test does not record the fixes as breaking it.
const testReruns = 2

// newFailures returns the failures in after that are not in before.
func newFailures(before, after []string) []string {
	var added []string

	for _, t := range after {
		if !slices.Contains(before, t) {
			added = append(added, t)
		}
	}

	return added
}

// confirmBroken reruns the packages of the broken tests up to reruns times with rerun, and returns the tests that
// fail on every run, and those that passed on a rerun. Failures in before don't count on a rerun either. rerun gets
// nil when a failure names no package, so that every package reruns.
func confirmBroken(before, broken []string, reruns int, rerun func(pkgs []string) []string) (confirmed, flaky []string) {
	confirmed = broken

	for range reruns {
		if len(confirmed) == 0 {
			break
		}

		again := newFailures(before, rerun(failedPackages(confirmed)))

		var still []string

		for _, t := range confirmed {
			if slices.Contains(again, t) {
				still = append(still, t)
			} else {
				flaky = append(flaky, t)
			}
		}

		confirmed = still
	}

	return confirmed, flaky
}

// failedPackages returns the packages of failures as testFailures and runTests list them, or nil when one names none.
func failedPackages(failures []string) []string {
	var pkgs []string

	for _, f := range failures {
		if strings.HasPrefix(f, "go test: ") {
			return nil
		}

		pkg, _, _ := strings.Cut(f, " ")
		pkgs = append(pkgs, pkg)
	}

	slices.Sort(pkgs)

	return slices.Compact(pkgs)
}

// runTests runs go test with args natively, as tests may need cgo or run what they build, and returns those that fail.
func (r *runner) runTests(dir string, args []string) []string {
	out, err := run(r.command(dir, "go", append([]string{"test", "-short", "-json"}, args...)...))

	failed := testFailures(out)
	if err != nil && len(failed) == 0 {
		failed = []string{"go test: " + strings.SplitN(err.Error(), "\n", 2)[0]}
	}

	return failed
}

// testFailures returns the tests, as "package test", and the packages that fail in the output of go test -json.
func testFailures(out string) []string {
	var failed []string

	for _, line := range strings.Split(out, "\n") {
		var e struct{ Action, Package, Test string }

		if json.Unmarshal([]byte(line), &e) != nil || e.Action != "fail" {
			continue
		}

		failed = append(failed, strings.TrimSpace(e.Package+" "+e.Test))
	}

	slices.Sort(failed)

	return slices.Compact(failed)
}

// build builds the module in dir for target.
func (r *runner) build(dir string) error {
	_, err := run(r.targetCommand(dir, "go", "build", "./..."))

	return err
}

// vet runs go vet on the module in dir for target. It runs whether or not the module builds, as it type-checks the
// test files too.
func (r *runner) vet(dir string) error {
	_, err := run(r.targetCommand(dir, "go", "vet", "./..."))

	return err
}

// excerpt returns the first lines of err, enough to see why a build or test failed.
func excerpt(err error) string {
	lines := strings.Split(err.Error(), "\n")

	return strings.Join(lines[:min(len(lines), 20)], "\n")
}

// lostInFile returns the comments in the committed version of file that its working copy no longer has.
func (r *runner) lostInFile(dir, file string) ([]string, error) {
	if !strings.HasSuffix(file, ".go") {
		return nil, nil
	}

	old, err := run(r.command(dir, "git", "show", "HEAD:"+file))
	if err != nil {
		return nil, err
	}

	cur, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		return nil, err
	}

	return lostComments([]byte(old), cur), nil
}

// lostComments returns the comments in old that are missing from cur, counting repeated comments separately.
func lostComments(old, cur []byte) []string {
	curComments, ok := comments(cur)
	if !ok {
		return []string{"(the fixed file does not parse)"}
	}

	remaining := map[string]int{}
	for _, c := range curComments {
		remaining[c]++
	}

	oldComments, _ := comments(old)

	var lost []string

	for _, c := range oldComments {
		if remaining[c] > 0 {
			remaining[c]--
			continue
		}

		lost = append(lost, c)
	}

	return lost
}

// comments returns the comments in src in order, and whether src parses. Each line of a // comment group is a comment
// of its own.
func comments(src []byte) ([]string, bool) {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, false
	}

	var list []string

	for _, g := range f.Comments {
		for _, c := range g.List {
			list = append(list, strings.TrimSpace(c.Text))
		}
	}

	return list, true
}

func renderFixes(results []fixResult) string {
	var b strings.Builder

	b.WriteString("# The corpus after applying unindent's fixes. Generated by \"go run . check -update\".\n")

	for _, r := range results {
		fmt.Fprintf(&b, "%s: %d files changed, %d comments lost\n", r.Project, r.Changed, len(r.Lost))

		lost := slices.Clone(r.Lost)
		slices.Sort(lost)

		for _, l := range lost {
			fmt.Fprintf(&b, "%s lost %s\n", r.Project, l)
		}

		for _, m := range r.Modules {
			fmt.Fprintf(&b, "%s %s: build %s; vet %s\n", r.Project, m.Module, checkStatus(m.Build), checkStatus(m.Vet))
		}

		if r.Tests != nil {
			fmt.Fprintf(&b, "%s tests: %s\n", r.Project, testStatus(*r.Tests))
		}
	}

	return b.String()
}

func testStatus(t testResult) string {
	if len(t.Broken) > 0 {
		return "BROKEN BY THE FIXES"
	}

	return "not broken by the fixes"
}

func checkStatus(c checkResult) string {
	switch {
	case c.Before == nil && c.After == nil:
		return "passes"
	case c.Before != nil && c.After != nil:
		return "fails before and after the fixes"
	case c.Before == nil:
		return "FAILS AFTER THE FIXES"
	default:
		return "fails before the fixes only"
	}
}
