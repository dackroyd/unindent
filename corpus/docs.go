package main

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// projectSummary is what the documentation says about a project, read back from the recorded results.
type projectSummary struct {
	Findings, Fixed, Withheld, NoFix int
	Kinds, Reasons                   map[string]int
	Changed, Lost                    int
	// Build and Vet count the modules by their status, such as "passes".
	Build, Vet map[string]int
	Modules    int
	// Tests is how the fixes left the tests, or empty if the project has none to run.
	Tests string
}

var (
	fixedLine  = regexp.MustCompile(`^(\S+): (\d+) files changed, (\d+) comments lost$`)
	moduleLine = regexp.MustCompile(`^(\S+) (\.\S*): build (.+); vet (.+)$`)
	testsLine  = regexp.MustCompile(`^(\S+) tests: (.+)$`)
	// quoted matches the names in a reason, so that reasons can be counted whatever the name.
	quoted = regexp.MustCompile(`"[^"]*"`)
)

// summarise reads the findings and fixes recorded for projects, as renderFindings and renderFixes write them.
func summarise(projects []project, findings, fixes string) (map[string]*projectSummary, error) {
	sums := map[string]*projectSummary{}

	for _, p := range projects {
		sums[p.Name] = &projectSummary{
			Kinds: map[string]int{}, Reasons: map[string]int{}, Build: map[string]int{}, Vet: map[string]int{},
		}
	}

	get := func(name, line string) (*projectSummary, error) {
		s, ok := sums[name]
		if !ok {
			return nil, fmt.Errorf("unknown project in %q", line)
		}

		return s, nil
	}

	for _, line := range strings.Split(findings, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.SplitN(line, " ", 3)
		if len(fields) < 3 {
			return nil, fmt.Errorf("unexpected finding %q", line)
		}

		kind, status, ok := strings.Cut(fields[2], ": ")
		if !ok {
			return nil, fmt.Errorf("unexpected finding %q", line)
		}

		s, err := get(fields[0], line)
		if err != nil {
			return nil, err
		}

		s.Findings++
		s.Kinds[kind]++

		switch {
		case status == "fixed":
			s.Fixed++
		case strings.HasPrefix(status, "not fixed: "):
			s.Withheld++

			reasons := map[string]bool{}
			for _, r := range strings.Split(strings.TrimPrefix(status, "not fixed: "), "; ") {
				reasons[reasonKind(r)] = true
			}

			for r := range reasons {
				s.Reasons[r]++
			}
		default:
			s.NoFix++
		}
	}

	for _, line := range strings.Split(fixes, "\n") {
		if m := fixedLine.FindStringSubmatch(line); m != nil {
			s, err := get(m[1], line)
			if err != nil {
				return nil, err
			}

			s.Changed, _ = strconv.Atoi(m[2])
			s.Lost, _ = strconv.Atoi(m[3])
		} else if m := moduleLine.FindStringSubmatch(line); m != nil {
			s, err := get(m[1], line)
			if err != nil {
				return nil, err
			}

			s.Modules++
			s.Build[m[3]]++
			s.Vet[m[4]]++
		} else if m := testsLine.FindStringSubmatch(line); m != nil {
			s, err := get(m[1], line)
			if err != nil {
				return nil, err
			}

			s.Tests = m[2]
		}
	}

	return sums, nil
}

// reasonKind is reason with the names it quotes elided, except for keywords.
func reasonKind(reason string) string {
	return quoted.ReplaceAllStringFunc(reason, func(q string) string {
		if q == `"if"` || q == `"else"` {
			return q
		}

		return `"…"`
	})
}

// repoName is the owner and name of the repository at url.
func repoName(url string) string {
	return strings.TrimPrefix(url, "https://github.com/")
}

// renderProjectsDoc writes the documentation page about the projects of set, from the findings and fixes recorded
// with the Go release goVersion.
func renderProjectsDoc(set corpusSet, goVersion, findings, fixes string) (string, error) {
	sums, err := summarise(set.Projects, findings, fixes)
	if err != nil {
		return "", err
	}

	var b strings.Builder

	fmt.Fprintf(&b, `# Projects

<!-- Generated from corpus/testdata/%[1]s/%[2]s by "go run . docs" in corpus/. Do not edit. -->

What `+"`unindent`"+` reports on large Go codebases, and what its fixes do to them, with %[2]s. The corpus check runs it
over each codebase at a pinned commit for linux/amd64, skipping generated files and `+"`testdata`"+` directories. It
applies the fixes, and checks that the code still builds, passes `+"`go vet`"+` and its tests, and keeps its comments.

| Project | Commit | Findings | Fixed | Fix withheld | Files changed | Comments lost |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
`, set.Name, goVersion)

	var total projectSummary

	for _, p := range set.Projects {
		s := sums[p.Name]
		fmt.Fprintf(&b, "| [%s](#%s) | [`%s`](%s/commit/%s), %s | %d | %d | %d | %d | %d |\n", repoName(p.Repo),
			anchor(repoName(p.Repo)), p.Commit[:7], p.Repo, p.Commit, p.Date, s.Findings, s.Fixed, s.Withheld, s.Changed,
			s.Lost)

		total.Findings += s.Findings
		total.Fixed += s.Fixed
		total.Withheld += s.Withheld
		total.Changed += s.Changed
		total.Lost += s.Lost
	}

	fmt.Fprintf(&b, "| Total | | %d | %d | %d | %d | %d |\n", total.Findings, total.Fixed, total.Withheld, total.Changed,
		total.Lost)

	b.WriteString(`
A finding's kind is the kind of ` + "`else`" + ` and the last statement of the branch before it, or for an exit in an
` + "`else`" + `, the last statement of the ` + "`else`" + ` block. A fix is withheld when it would not compile or would change
behaviour; the reasons quote the names involved, shown here as "…".
`)

	for _, p := range set.Projects {
		s := sums[p.Name]

		fmt.Fprintf(&b, "\n## %s\n\n%s\n\n", repoName(p.Repo), p.Description)

		writeTable(&b, "Kind", "Findings", s.Kinds)

		if len(s.Reasons) > 0 {
			b.WriteString("\n")
			writeTable(&b, "Fix withheld because", "Findings", s.Reasons)
		}

		if s.NoFix > 0 {
			fmt.Fprintf(&b, "\n%d findings have no fix and no reason.\n", s.NoFix)
		}

		fmt.Fprintf(&b, "\nAfter the fixes, in %s:\n\n- `go build` %s.\n- `go vet` %s.\n- Tests: %s.\n",
			plural(s.Modules, "module"), statusCounts(s.Build, s.Modules), statusCounts(s.Vet, s.Modules),
			cmp.Or(s.Tests, "not run"))
	}

	return b.String(), nil
}

// writeTable writes counts as a table, the largest first.
func writeTable(b *strings.Builder, key, value string, counts map[string]int) {
	fmt.Fprintf(b, "| %s | %s |\n| --- | ---: |\n", key, value)

	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}

	slices.SortFunc(keys, func(a, b string) int {
		return cmp.Or(counts[b]-counts[a], strings.Compare(a, b))
	})

	for _, k := range keys {
		fmt.Fprintf(b, "| `%s` | %d |\n", k, counts[k])
	}
}

// statusCounts describes how many of n modules have each status.
func statusCounts(counts map[string]int, n int) string {
	if counts["passes"] == n {
		return "passes in every module"
	}

	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s in %d", k, counts[k])
	}

	return strings.Join(parts, ", ")
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}

	return fmt.Sprintf("%d %ss", n, noun)
}

// anchor is the fragment GitHub links a heading by.
func anchor(heading string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 'a' - 'A'
		case r == ' ':
			return '-'
		}

		return -1
	}, heading)
}
