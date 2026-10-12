// Command corpus checks unindent against pinned versions of real Go codebases, with each supported Go release. It
// records what unindent reports, applies the fixes, checks that each codebase still builds, passes go vet and its
// tests and keeps its comments, and compares the reports with revive's. The results are compared with those recorded
// in testdata, so a change in behaviour shows up as a change to those files.
//
// Usage:
//
//	go run . check [-set name] [-update] [-fix=false] [-clean-cache] [-go release] [-dir dir]
//	go run . revive [-set name] [-dir dir]
//	go run . fetch [-set name] [-dir dir]
//	go run . docs
//
// The projects come in sets, listed in projects.go: "default", which CI checks on every change, and "large", which
// takes about half an hour and is checked weekly and before each release. -set chooses one, the default set unless
// given.
//
// check fails when the results differ from testdata, and -update records them instead, along with the documentation
// page generated from them, if the set has one. Without -go, it checks with every release the set is checked with.
// -clean-cache empties the build cache after each project's builds, before and after the fixes, for hosts with little
// disk. revive records revive's reports in revive.txt. They only change with the set's projects, so it only needs to
// run when projects.go does. fetch downloads the set's projects into dir, so they can be cached. docs regenerates every
// set's documentation page from its recorded results, for changes to the page itself.
//
// The results do not depend on the host: unindent, go build and go vet target linux/amd64 without cgo on every
// platform. Tests run natively, and the results only record whether the fixes broke them. Tests that fail only after
// the fixes are rerun up to twice, and count as broken only if they fail every time.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// resultsFile is where the results named name of the set for the Go release goVersion are recorded.
func resultsFile(set, goVersion, name string) string {
	return filepath.Join("testdata", set, goVersion, name)
}

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		log.Fatal("usage: go run . check|revive|fetch|docs [flags]")
	}

	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}

	names := make([]string, len(sets))
	for i, s := range sets {
		names[i] = s.Name
	}

	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	dir := fs.String("dir", filepath.Join(cache, "unindent-corpus"), "directory holding the corpus checkouts")
	setName := fs.String("set", defaultSet, fmt.Sprintf("the set of projects to check, one of %q", names))
	update := fs.Bool("update", false, "record the results in testdata instead of comparing them")
	fix := fs.Bool("fix", true, "apply the fixes and check the result builds, vets, passes its tests and keeps its comments")
	cleanCache := fs.Bool("clean-cache", false, "empty the build cache after each project's builds, before and after the fixes")
	goVersion := fs.String("go", "", "the Go release to check with, one of those the set is checked with (default all)")

	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}

	set, ok := findSet(*setName)
	if !ok {
		log.Fatalf("-set must be one of %q", names)
	}

	versions := set.Releases
	if *goVersion != "" {
		if !slices.Contains(set.Releases, *goVersion) {
			log.Fatalf("the %s set is checked with %s, so -go must be one of them", set.Name,
				strings.Join(set.Releases, " and "))
		}

		versions = []string{*goVersion}
	}

	r, err := newRunner(*dir)
	if err != nil {
		log.Fatal(err)
	}
	defer r.close()

	r.cleanCache = *cleanCache

	switch os.Args[1] {
	case "check":
		err = r.check(set, versions, *update, *fix)
	case "revive":
		err = r.revive(set)
	case "fetch":
		err = r.fetchAll(set)
	case "docs":
		err = writeDocs()
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}

	if err != nil {
		log.Fatal(err)
	}
}

func (r *runner) check(set corpusSet, versions []string, update, fix bool) error {
	results := map[string]string{}

	for _, v := range versions {
		r.goVersion = v

		if err := r.checkVersion(set, results, fix); err != nil {
			return err
		}
	}

	return record(results, update)
}

// checkVersion adds the results of checking set with the Go release in use to results.
func (r *runner) checkVersion(set corpusSet, results map[string]string, fix bool) error {
	start := time.Now()

	bin, err := r.buildUnindent()
	if err != nil {
		return err
	}

	var (
		findings []finding
		fixes    []fixResult
	)

	dirs := map[string]string{}

	for _, p := range set.Projects {
		dir, err := r.fetch(p)
		if err != nil {
			return err
		}

		dirs[p.Name] = dir

		log.Printf("%s %s: reporting", r.goVersion, p.Name)

		f, err := r.report(bin, p, dir)
		if err != nil {
			return err
		}

		findings = append(findings, f...)

		if !fix || p.Repo == "" {
			continue
		}

		log.Printf("%s %s: applying fixes", r.goVersion, p.Name)

		res, err := r.fix(bin, p, dir)
		if err != nil {
			return err
		}

		fixes = append(fixes, res)
	}

	findingsText := renderFindings(findings)
	results[resultsFile(set.Name, r.goVersion, "findings.txt")] = findingsText

	if fix {
		fixesText := renderFixes(fixes)
		results[resultsFile(set.Name, r.goVersion, "fixes.txt")] = fixesText

		if set.Doc != "" && r.goVersion == latestGo() {
			doc, err := renderProjectsDoc(set, r.goVersion, findingsText, fixesText)
			if err != nil {
				return err
			}

			results[set.Doc] = doc
		}
	}

	if r.goVersion == latestGo() {
		revive, err := readRevive(resultsFile(set.Name, r.goVersion, "revive.txt"))
		if err != nil {
			return err
		}

		results[resultsFile(set.Name, r.goVersion, "comparison.txt")] = compare(set.Projects, dirs, findings, revive)
	}

	log.Printf("%s %s: finished in %s", set.Name, r.goVersion, time.Since(start).Round(time.Second))

	return nil
}

// writeDocs regenerates the documentation page of every set that has one from its recorded results.
func writeDocs() error {
	results := map[string]string{}

	for _, set := range sets {
		if set.Doc == "" {
			continue
		}

		doc, err := recordedDoc(set)
		if err != nil {
			return err
		}

		results[set.Doc] = doc
	}

	return record(results, true)
}

// recordedDoc renders the documentation page of set from the results recorded with the latest release.
func recordedDoc(set corpusSet) (string, error) {
	var texts [2]string

	for i, name := range []string{"findings.txt", "fixes.txt"} {
		b, err := os.ReadFile(resultsFile(set.Name, latestGo(), name))
		if err != nil {
			return "", err
		}

		texts[i] = string(b)
	}

	return renderProjectsDoc(set, latestGo(), texts[0], texts[1])
}

// fetchAll checks out every project of set.
func (r *runner) fetchAll(set corpusSet) error {
	for _, p := range set.Projects {
		if p.Repo == "" {
			continue
		}

		if _, err := r.fetch(p); err != nil {
			return err
		}
	}

	return nil
}

func (r *runner) revive(set corpusSet) error {
	r.goVersion = latestGo()

	lint, err := r.installGolangciLint()
	if err != nil {
		return err
	}

	var reports []reviveReport

	for _, p := range set.Projects {
		dir, err := r.fetch(p)
		if err != nil {
			return err
		}

		log.Printf("%s: running revive", p.Name)

		rep, err := r.runRevive(lint, p, dir)
		if err != nil {
			return err
		}

		reports = append(reports, rep...)
	}

	return record(map[string]string{resultsFile(set.Name, r.goVersion, "revive.txt"): renderRevive(reports)}, true)
}
