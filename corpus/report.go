package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// notFixed precedes the reasons in the message of a finding reported without a fix.
const notFixed = "Not fixed automatically: "

// position is a place in a project's source, with File relative to the project.
type position struct {
	Project, File string
	Line, Col     int
}

func (p position) String() string {
	return fmt.Sprintf("%s %s:%d:%d", p.Project, p.File, p.Line, p.Col)
}

func (p position) compare(q position) int {
	if c := strings.Compare(p.File, q.File); c != 0 {
		return c
	}

	if p.Line != q.Line {
		return p.Line - q.Line
	}

	return p.Col - q.Col
}

type finding struct {
	Pos position
	// Kind says what the report is about, from the source: see findingKind.
	Kind    string
	Message string
	Fixed   bool
}

// report runs unindent over every module of p, and returns what it reports outside generated files.
func (r *runner) report(bin string, p project, dir string) ([]finding, error) {
	mods, err := modules(dir)
	if err != nil {
		return nil, err
	}

	runs, err := units(p, dir, mods)
	if err != nil {
		return nil, err
	}

	var findings []finding

	seen := map[position]bool{}
	generated := map[string]bool{}
	ifs := &ifIndex{dirs: map[string]string{p.Name: dir}, files: map[string]*fileIfs{}}

	for _, u := range runs {
		// unindent exits with a non-zero status when it reports anything, so only output that can't be read fails.
		out, runErr := run(r.targetCommand(filepath.Join(dir, u.Module), bin, "-json", u.Pattern))

		// A batch can match no packages for the target platform.
		if strings.TrimSpace(out) == "" {
			log.Printf("%s %s %s: skipped, as unindent reported nothing it could analyse: %v", p.Name, u.Module,
				u.Pattern, runErr)
			continue
		}

		var pkgs map[string]map[string]json.RawMessage
		if err := json.Unmarshal([]byte(out), &pkgs); err != nil {
			return nil, fmt.Errorf("%s %s %s: reading unindent's output: %w (%w)", p.Name, u.Module, u.Pattern, err,
				runErr)
		}

		for _, analyzers := range pkgs {
			var diags []struct {
				Posn           string            `json:"posn"`
				Message        string            `json:"message"`
				SuggestedFixes []json.RawMessage `json:"suggested_fixes"`
			}

			// A package that fails to load holds an error instead, as it does for golangci-lint and go vet.
			if json.Unmarshal(analyzers["unindent"], &diags) != nil {
				continue
			}

			for _, d := range diags {
				pos, err := parsePosn(p.Name, dir, d.Posn)
				if err != nil {
					return nil, err
				}

				if seen[pos] || isGenerated(generated, filepath.Join(dir, pos.File)) {
					continue
				}

				seen[pos] = true

				findings = append(findings, finding{
					Pos:     pos,
					Kind:    findingKind(ifs.file(p.Name, pos.File).byElse[pos]),
					Message: d.Message,
					Fixed:   len(d.SuggestedFixes) > 0,
				})
			}
		}
	}

	slices.SortFunc(findings, func(a, b finding) int { return a.Pos.compare(b.Pos) })

	return findings, nil
}

// parsePosn reads a "file:line:col" position, with file absolute, as a position in the project at dir.
func parsePosn(proj, dir, posn string) (position, error) {
	parts := strings.Split(posn, ":")
	if len(parts) < 3 {
		return position{}, fmt.Errorf("unexpected position %q", posn)
	}

	n := len(parts)

	line, err := strconv.Atoi(parts[n-2])
	if err != nil {
		return position{}, fmt.Errorf("unexpected position %q: %w", posn, err)
	}

	col, err := strconv.Atoi(parts[n-1])
	if err != nil {
		return position{}, fmt.Errorf("unexpected position %q: %w", posn, err)
	}

	return relPosition(proj, dir, strings.Join(parts[:n-2], ":"), line, col)
}

func relPosition(proj, dir, file string, line, col int) (position, error) {
	if real, err := filepath.EvalSymlinks(file); err == nil {
		file = real
	}

	rel, err := filepath.Rel(dir, file)
	if err != nil {
		return position{}, err
	}

	return position{Project: proj, File: filepath.ToSlash(rel), Line: line, Col: col}, nil
}

func isGenerated(cache map[string]bool, file string) bool {
	if g, ok := cache[file]; ok {
		return g
	}

	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.PackageClauseOnly|parser.ParseComments)
	cache[file] = err == nil && ast.IsGenerated(f)

	return cache[file]
}

// findingKind describes the if statement with an else that a finding is about: whether the else is an "else if", and
// how the branch before it ends, its last statement.
func findingKind(is *ifStmt) string {
	if is == nil {
		return "not on an else"
	}

	form := "else"
	if _, ok := is.stmt.Else.(*ast.IfStmt); ok {
		form = "else if"
	}

	var exit string

	switch last := lastStmt(is.stmt.Body).(type) {
	case *ast.ReturnStmt:
		exit = "return"
	case *ast.BranchStmt:
		exit = last.Tok.String()
	case *ast.ExprStmt:
		exit = "call"
	case *ast.IfStmt:
		exit = "nested if"
	case *ast.SwitchStmt, *ast.TypeSwitchStmt:
		exit = "switch"
	default:
		exit = "other"
	}

	return form + " after " + exit
}

// renderFindings lists one finding per line, with its kind and whether it is fixed, or why not.
func renderFindings(findings []finding) string {
	var b strings.Builder

	b.WriteString("# What unindent reports on the corpus, and whether it offers a fix. Generated by \"go run . check -update\".\n")

	for _, f := range findings {
		status := "fixed"

		if i := strings.Index(f.Message, notFixed); i >= 0 {
			status = "not fixed: " + f.Message[i+len(notFixed):]
		} else if !f.Fixed {
			status = "no fix"
		}

		fmt.Fprintf(&b, "%s %s: %s\n", f.Pos, f.Kind, status)
	}

	return b.String()
}
