package unindent

import (
	"cmp"
	"go/ast"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// finding is a report waiting for the rest of the package to be checked.
type finding struct {
	stmt      *ast.IfStmt
	message   string
	fix       string
	edits     []analysis.TextEdit
	conflicts []string
	// copies says that the fix rewrites source text that other fixes may edit, such as an initialiser it moves before
	// the "if" or an else block it moves before the body, so it can only be applied once they are.
	copies bool
}

func (c *checker) report(f finding) {
	c.found = append(c.found, f)
}

// flush reports the findings. A fix that copies source text is withheld when another fix edits that text, innermost
// first, so the fixes of one pass never overlap; the next pass can apply it.
func (c *checker) flush() {
	var copying []int

	for i, f := range c.found {
		if f.copies && len(f.conflicts) == 0 {
			copying = append(copying, i)
		}
	}

	slices.SortStableFunc(copying, func(a, b int) int {
		return cmp.Compare(c.found[a].stmt.End()-c.found[a].stmt.Pos(), c.found[b].stmt.End()-c.found[b].stmt.Pos())
	})

	pending := map[int]bool{}
	for _, i := range copying {
		pending[i] = true
	}

	for _, i := range copying {
		delete(pending, i)

		if c.overlapsFix(i, pending) {
			c.found[i].conflicts = append(c.found[i].conflicts, `another fix inside the "if" statement has to be applied first`)
		}
	}

	for _, f := range c.found {
		diag := analysis.Diagnostic{Pos: f.stmt.Else.Pos(), Message: f.message}

		if len(f.conflicts) > 0 {
			diag.Message += ". Not fixed automatically: " + strings.Join(f.conflicts, "; ")
		} else {
			diag.SuggestedFixes = []analysis.SuggestedFix{{Message: f.fix, TextEdits: f.edits}}
		}

		c.pass.Report(diag)
	}

	c.found = nil
}

// overlapsFix reports whether an edit of the i'th finding's fix overlaps one of another fix that is offered, other than
// the pending ones around it, which are checked against it in turn. An insertion overlaps an edit that starts or ends
// where it is, as their order would be ambiguous.
func (c *checker) overlapsFix(i int, pending map[int]bool) bool {
	for j, f := range c.found {
		if j == i || pending[j] || len(f.conflicts) > 0 {
			continue
		}

		for _, e := range f.edits {
			for _, m := range c.found[i].edits {
				inserts := e.Pos == e.End || m.Pos == m.End
				if e.Pos < m.End && m.Pos < e.End || inserts && e.Pos <= m.End && m.Pos <= e.End {
					return true
				}
			}
		}
	}

	return false
}
