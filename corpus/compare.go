package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
)

// ifStmt is an if statement that a tool reports on: one with an else, or one wrapping the rest of a function or loop.
type ifStmt struct {
	If       position
	stmt     *ast.IfStmt
	inSelect bool
	link     bool
	// wraps is "function" or "loop" when the if has no else and ends the body of one.
	wraps string
}

type fileIfs struct {
	// byIf holds every if statement, and byElse those with an else.
	byIf, byElse map[position]*ifStmt
}

// reported returns the if statement that a finding at pos with message is about: the one whose else is at pos, or for an
// if wrapping the rest of a function or loop or holding only another if, which has no else, the one at pos.
func (f *fileIfs) reported(pos position, message string) *ifStmt {
	switch {
	case strings.HasPrefix(message, ifWrappingRestMessage):
		if is := f.byIf[pos]; is != nil && is.wraps != "" {
			return is
		}

		return nil
	case strings.HasPrefix(message, ifAloneInIfMessage):
		if is := f.byIf[pos]; is != nil && holdsOnlyIf(is.stmt) {
			return is
		}

		return nil
	}

	return f.byElse[pos]
}

// holdsOnlyIf reports whether s has no else and its block holds only another if.
func holdsOnlyIf(s *ast.IfStmt) bool {
	if s.Else != nil || len(s.Body.List) != 1 {
		return false
	}

	_, ok := s.Body.List[0].(*ast.IfStmt)

	return ok
}

// wrappedBody describes the body that s ends, when s has no else and is the last statement of a function's or loop's
// body, the parent and grandparent in stack.
func wrappedBody(s *ast.IfStmt, stack []ast.Node) string {
	if s.Else != nil || len(stack) < 2 {
		return ""
	}

	body, ok := stack[len(stack)-1].(*ast.BlockStmt)
	if !ok || body.List[len(body.List)-1] != s {
		return ""
	}

	switch stack[len(stack)-2].(type) {
	case *ast.FuncDecl, *ast.FuncLit:
		return "function"
	case *ast.ForStmt, *ast.RangeStmt:
		return "loop"
	}

	return ""
}

// ifIndex finds the if statement a report is about: unindent and revive's indent-error-flow and superfluous-else
// report at the start of its else branch, and early-return at its if.
type ifIndex struct {
	dirs  map[string]string
	files map[string]*fileIfs
}

func (x *ifIndex) file(proj, file string) *fileIfs {
	key := proj + " " + file
	if f, ok := x.files[key]; ok {
		return f
	}

	ifs := &fileIfs{byIf: map[position]*ifStmt{}, byElse: map[position]*ifStmt{}}
	x.files[key] = ifs

	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, filepath.Join(x.dirs[proj], file), nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return ifs
	}

	pos := func(p token.Pos) position {
		pp := fset.Position(p)
		return position{Project: proj, File: file, Line: pp.Line, Col: pp.Column}
	}

	var stack []ast.Node

	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}

		if s, ok := n.(*ast.IfStmt); ok && s.Else == nil {
			ifs.byIf[pos(s.If)] = &ifStmt{If: pos(s.If), stmt: s, wraps: wrappedBody(s, stack)}
		} else if ok {
			// The head of an if/else chain decides whether its links are in a select case.
			var child ast.Node = s

			i := len(stack) - 1
			for ; i >= 0; i-- {
				parent, ok := stack[i].(*ast.IfStmt)
				if !ok || parent.Else != child {
					break
				}

				child = parent
			}

			var inSelect bool
			if i >= 0 {
				_, inSelect = stack[i].(*ast.CommClause)
			}

			is := &ifStmt{If: pos(s.If), stmt: s, inSelect: inSelect, link: child != s}
			ifs.byIf[is.If] = is
			ifs.byElse[pos(s.Else.Pos())] = is
		}

		stack = append(stack, n)

		return true
	})

	return ifs
}

type comparisonRow struct {
	project                                     string
	unindent, revive, both, unindentOnly, rOnly int
}

// compare counts the if statements each tool reports in projects, and why those reported by only one are.
func compare(projects []project, dirs map[string]string, findings []finding, revive []reviveReport) string {
	x := &ifIndex{dirs: dirs, files: map[string]*fileIfs{}}

	unindentIfs := map[position]*ifStmt{}
	unindentKinds := map[position]string{}
	reviveRules := map[position][]string{}
	reviveIfs := map[position]*ifStmt{}

	var unmatched []string

	for _, f := range findings {
		is := x.file(f.Pos.Project, f.Pos.File).reported(f.Pos, kindMessage(f.Kind))
		if is == nil {
			unmatched = append(unmatched, f.Pos.String()+" unindent")
			continue
		}

		unindentIfs[is.If] = is
		unindentKinds[is.If] = f.Kind
	}

	for _, rep := range revive {
		ifs := x.file(rep.Pos.Project, rep.Pos.File)

		is := ifs.byElse[rep.Pos]
		if rep.Rule == "early-return" {
			is = ifs.byIf[rep.Pos]
		}

		if is == nil {
			unmatched = append(unmatched, rep.Pos.String()+" "+rep.Rule)
			continue
		}

		reviveRules[is.If] = append(reviveRules[is.If], rep.Rule)
		reviveIfs[is.If] = is
	}

	rows := map[string]*comparisonRow{}
	row := func(proj string) *comparisonRow {
		if rows[proj] == nil {
			rows[proj] = &comparisonRow{project: proj}
		}

		return rows[proj]
	}

	unindentOnly, reviveOnly := map[string]int{}, map[string]int{}

	for p, is := range unindentIfs {
		row(p.Project).unindent++

		if _, ok := reviveIfs[p]; ok {
			row(p.Project).both++
			continue
		}

		row(p.Project).unindentOnly++
		unindentOnly[unindentReason(is, unindentKinds[p])]++
	}

	for p, is := range reviveIfs {
		row(p.Project).revive++

		if _, ok := unindentIfs[p]; ok {
			continue
		}

		row(p.Project).rOnly++
		reviveOnly[reviveReason(is, reviveRules[p])]++
	}

	var b strings.Builder

	b.WriteString("# The if statements unindent and revive report on the corpus. Generated by \"go run . check -update\".\n\n")

	var table strings.Builder

	table.WriteString("project\tunindent\trevive\tboth\tunindent only\trevive only\n")

	var total comparisonRow

	for _, p := range projects {
		r := row(p.Name)
		fmt.Fprintf(&table, "%s\t%d\t%d\t%d\t%d\t%d\n", r.project, r.unindent, r.revive, r.both, r.unindentOnly, r.rOnly)

		total.unindent += r.unindent
		total.revive += r.revive
		total.both += r.both
		total.unindentOnly += r.unindentOnly
		total.rOnly += r.rOnly
	}

	fmt.Fprintf(&table, "total\t%d\t%d\t%d\t%d\t%d\n", total.unindent, total.revive, total.both, total.unindentOnly,
		total.rOnly)

	// Writing to a strings.Builder cannot fail.
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	_, _ = w.Write([]byte(table.String()))
	_ = w.Flush()

	writeCounts(&b, "Reported by unindent only", unindentOnly)
	writeCounts(&b, "Reported by revive only", reviveOnly)

	if len(unmatched) > 0 {
		b.WriteString("\nReports not on an if statement with an else:\n")

		for _, u := range unmatched {
			fmt.Fprintf(&b, "  %s\n", u)
		}
	}

	return b.String()
}

func writeCounts(b *strings.Builder, title string, counts map[string]int) {
	fmt.Fprintf(b, "\n%s:\n", title)

	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}

	slices.SortFunc(keys, func(a, b string) int {
		if counts[a] != counts[b] {
			return counts[b] - counts[a]
		}

		return strings.Compare(a, b)
	})

	for _, k := range keys {
		fmt.Fprintf(b, "  %s: %d\n", k, counts[k])
	}
}

// unindentReason describes what revive misses about an if statement only unindent reports, as a finding of kind. For
// an exit in an else, revive's early-return looks at how the else block ends rather than the body. revive has no rule
// for an if wrapping the rest of a function or loop, or for an if alone in an if, so its kind is the reason.
func unindentReason(is *ifStmt, kind string) string {
	if strings.HasPrefix(kind, ifWrappingRestKind) || kind == ifAloneInIfKind {
		return kind
	}

	if is.inSelect {
		return "in a select case"
	}

	branch, prefix := is.stmt.Body, ""

	if els, ok := is.stmt.Else.(*ast.BlockStmt); ok && strings.HasPrefix(kind, exitInElseKind) {
		branch, prefix = els, exitInElseKind+": "
	} else if _, ok := is.stmt.Else.(*ast.IfStmt); ok {
		return "else if chain"
	}

	switch lastStmt(branch).(type) {
	case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
		return prefix + "exits through a nested if, switch or type switch"
	case *ast.ExprStmt:
		return prefix + "ends in a call"
	case *ast.ForStmt, *ast.SelectStmt:
		return prefix + "ends in a for or select that never ends"
	}

	return prefix + "other"
}

// reviveReason describes what unindent misses about an if statement only revive reports.
func reviveReason(is *ifStmt, rules []string) string {
	if !slices.ContainsFunc(rules, func(r string) bool { return r != "early-return" }) {
		if is.link {
			return "early-return on an else if"
		}

		return "early-return"
	}

	switch last := lastStmt(is.stmt.Body).(type) {
	case *ast.ExprStmt:
		return "ends in a call"
	case *ast.BranchStmt:
		return "ends in " + last.Tok.String()
	}

	return "other"
}

// lastStmt returns the last statement of b, without its labels.
func lastStmt(b *ast.BlockStmt) ast.Stmt {
	if len(b.List) == 0 {
		return nil
	}

	last := b.List[len(b.List)-1]
	for {
		labeled, ok := last.(*ast.LabeledStmt)
		if !ok {
			return last
		}

		last = labeled.Stmt
	}
}
