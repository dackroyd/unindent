package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

type runner struct {
	// dir holds the corpus checkouts, reused between runs.
	dir string
	// root is the unindent repository.
	root string
	// tmp holds the binaries built for this run.
	tmp string
	// goVersion is the Go release in use, one of goVersions.
	goVersion string
	// cleanCache empties the build cache once a project's builds are checked, before the fixes and after them.
	cleanCache bool
}

func newRunner(dir string) (*runner, error) {
	root, err := filepath.Abs("..")
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(filepath.Join(root, "cmd", "unindent")); err != nil {
		return nil, fmt.Errorf("run from the corpus directory of the unindent repository: %w", err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	tmp, err := os.MkdirTemp("", "unindent-corpus")
	if err != nil {
		return nil, err
	}

	return &runner{dir: dir, root: root, tmp: tmp}, nil
}

func (r *runner) close() {
	_ = os.RemoveAll(r.tmp)
}

// target is the platform the corpus is analysed, fixed, built and vetted for, wherever the check runs. Build constraints
// and cgo select different files on other platforms, so without it the results would depend on the host. Cross builds
// and go vet run nothing they build, so they work on any host.
var target = []string{"GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0"}

// command runs natively with the Go release in use, ignoring GOFLAGS set for other projects, and any platform set in
// the environment.
func (r *runner) command(dir, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GOTOOLCHAIN="+r.goVersion, "GOFLAGS=", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH, "CGO_ENABLED=")

	return cmd
}

// targetCommand is command, building for target instead of the host.
func (r *runner) targetCommand(dir, name string, args ...string) *exec.Cmd {
	cmd := r.command(dir, name, args...)
	cmd.Env = append(cmd.Env, target...)

	return cmd
}

// run runs cmd, returning its standard output, and its standard error in the error when it fails.
func run(cmd *exec.Cmd) (string, error) {
	var stdout, stderr bytes.Buffer

	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s: %w\n%s", strings.Join(cmd.Args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}

func (r *runner) buildUnindent() (string, error) {
	bin := filepath.Join(r.tmp, "unindent-"+r.goVersion)

	_, err := run(r.command(r.root, "go", "build", "-o", bin, "./cmd/unindent"))

	return bin, err
}

func (r *runner) installGolangciLint() (string, error) {
	cmd := r.command(r.root, "go", "install", golangciLint)
	cmd.Env = append(cmd.Env, "GOBIN="+r.tmp)

	if _, err := run(cmd); err != nil {
		return "", err
	}

	return filepath.Join(r.tmp, "golangci-lint"), nil
}

// fetch returns the directory holding p's source at its pinned commit, with any earlier changes undone.
func (r *runner) fetch(p project) (string, error) {
	if p.Repo == "" {
		goroot, err := run(r.command(r.root, "go", "env", "GOROOT"))
		if err != nil {
			return "", err
		}

		return filepath.EvalSymlinks(filepath.Join(strings.TrimSpace(goroot), "src"))
	}

	commit := p.Commit

	// Each commit has a checkout of its own, so a checkout is never reused for another commit by mistake.
	dir := filepath.Join(r.dir, p.Name+"-"+commit[:12])

	if head, err := run(r.command(dir, "git", "rev-parse", "HEAD")); err == nil && strings.TrimSpace(head) == commit {
		if err := r.restore(dir); err != nil {
			return "", err
		}

		return filepath.EvalSymlinks(dir)
	}

	log.Printf("%s: fetching %s", p.Name, commit)

	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	for _, args := range [][]string{
		{"init", "-q"},
		{"fetch", "-q", "--depth", "1", p.Repo, commit},
		{"checkout", "-q", "FETCH_HEAD"},
	} {
		if _, err := run(r.command(dir, "git", args...)); err != nil {
			return "", err
		}
	}

	return filepath.EvalSymlinks(dir)
}

// cleanBuildCache empties the build cache when r.cleanCache is set. Building and vetting hugo once fills it with about
// 14 GB, and the large set fills it with over 30 GB, more than a CI runner's disk.
func (r *runner) cleanBuildCache() error {
	if !r.cleanCache {
		return nil
	}

	_, err := run(r.command(r.root, "go", "clean", "-cache"))

	return err
}

// restore undoes the fixes applied to a checkout.
func (r *runner) restore(dir string) error {
	if _, err := run(r.command(dir, "git", "checkout", "-q", "--", ".")); err != nil {
		return err
	}

	_, err := run(r.command(dir, "git", "clean", "-qfd"))

	return err
}

// modules returns the directories of the Go modules in dir, relative to it, as the go command sees them: directories
// named testdata or vendor, or starting with "." or "_", are skipped.
func modules(dir string) ([]string, error) {
	var mods []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := d.Name()

		if d.IsDir() && path != dir && skipDir(name) {
			return filepath.SkipDir
		}

		if !d.IsDir() && name == "go.mod" {
			rel, err := filepath.Rel(dir, filepath.Dir(path))
			if err != nil {
				return err
			}

			mods = append(mods, "./"+filepath.ToSlash(rel))
		}

		return nil
	})

	for i, m := range mods {
		if m == "./." {
			mods[i] = "."
		}
	}

	slices.Sort(mods)

	return mods, err
}

// unit is one run of unindent: the packages matching Pattern in the module at Module.
type unit struct {
	Module, Pattern string
}

// units splits the modules mods of p, in dir, into the runs of unindent that analyse them: ./... in each module, or
// for a batched project, each top-level directory of each module that is not a module of its own, and the module's
// root package.
func units(p project, dir string, mods []string) ([]unit, error) {
	var list []unit

	for _, mod := range mods {
		if !p.Batched {
			list = append(list, unit{mod, "./..."})
			continue
		}

		entries, err := os.ReadDir(filepath.Join(dir, mod))
		if err != nil {
			return nil, err
		}

		var root bool

		for _, e := range entries {
			name := e.Name()

			if !e.IsDir() {
				root = root || strings.HasSuffix(name, ".go")
				continue
			}

			if skipDir(name) {
				continue
			}

			if _, err := os.Stat(filepath.Join(dir, mod, name, "go.mod")); err == nil {
				continue
			}

			list = append(list, unit{mod, "./" + name + "/..."})
		}

		if root {
			list = append(list, unit{mod, "."})
		}
	}

	return list, nil
}

// skipDir reports whether the go command skips directories named name when matching "./...".
func skipDir(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// record writes results to their files when update is set, and otherwise reports how they differ from the files.
func record(results map[string]string, update bool) error {
	names := make([]string, 0, len(results))
	for name := range results {
		names = append(names, name)
	}

	slices.Sort(names)

	var differ bool

	for _, name := range names {
		if update {
			if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
				return err
			}

			if err := os.WriteFile(name, []byte(results[name]), 0o644); err != nil {
				return err
			}

			continue
		}

		want, err := os.ReadFile(name)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		if d := diff(string(want), results[name]); d != "" {
			differ = true

			fmt.Printf("--- %s (recorded)\n+++ %s (this run)\n%s", name, name, d)
		}
	}

	if differ {
		return errors.New("results differ from testdata: check the changes are intended, then run \"go run . check -update\"")
	}

	return nil
}

// diff lists the lines only in want, prefixed with "-", and those only in got, prefixed with "+", each in order.
func diff(want, got string) string {
	count := func(s string) map[string]int {
		m := map[string]int{}
		for _, l := range strings.Split(s, "\n") {
			m[l]++
		}

		return m
	}

	wantLines, gotLines := count(want), count(got)

	var b strings.Builder

	for _, l := range strings.Split(want, "\n") {
		if gotLines[l] > 0 {
			gotLines[l]--
			continue
		}

		fmt.Fprintf(&b, "-%s\n", l)
	}

	for _, l := range strings.Split(got, "\n") {
		if wantLines[l] > 0 {
			wantLines[l]--
			continue
		}

		fmt.Fprintf(&b, "+%s\n", l)
	}

	return b.String()
}
