package unindent

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// defaultAloneMaxOperands is the default of the if-alone-in-if-max-operands flag: two simple conditions combine, and a
// condition that already holds "&&" or "||" keeps its nesting, so the fix never adds parentheses around an "||".
const defaultAloneMaxOperands maxOperands = 2

// maxOperands is a flag holding a number of operands, at least 2, as two combined conditions have at least two.
type maxOperands int

func (m *maxOperands) String() string {
	return strconv.Itoa(int(*m))
}

func (m *maxOperands) Set(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil {
		return err
	}

	if n < 2 {
		return fmt.Errorf("must be at least 2, not %d", n)
	}

	*m = maxOperands(n)

	return nil
}

// reportIfAloneInIf reports the "if" statements whose block holds only another "if" that can join it, from stmt down,
// when stmt starts such a chain: "if a { if b { ... } }" becomes "if a && b { ... }". The chain is split into groups
// from the top, each combining as many levels as the operand limit allows, so one run fixes them all. When another
// check reports stmt, as an "if" wrapping the rest of a function or loop or an "else if" after an exit, its fix edits
// the same lines, so the chain is grouped from the next level.
func (c *checker) reportIfAloneInIf(stmt *ast.IfStmt, stack []ast.Node, parentInfo nodeInfo) {
	if c.joinsParent(stmt, stack) {
		return
	}

	levels := []*ast.IfStmt{stmt}
	for inner := c.aloneIn(stmt); inner != nil; inner = c.aloneIn(inner) {
		levels = append(levels, inner)
	}

	if c.reportedByOthers(stmt, stack, parentInfo) {
		levels = levels[1:]
	}

	for start := 0; start < len(levels)-1; {
		end := c.groupEnd(levels, start)
		if end > start {
			c.reportGroup(levels[start : end+1])
		}

		start = end + 1
	}
}

// joinsParent reports whether stmt is the "if" alone in the block of an "if" it can join, which starts its chain.
func (c *checker) joinsParent(stmt *ast.IfStmt, stack []ast.Node) bool {
	if len(stack) < 3 {
		return false
	}

	parent, ok := stack[len(stack)-3].(*ast.IfStmt)

	return ok && parent.Body == stack[len(stack)-2] && c.aloneIn(parent) == stmt
}

// reportedByOthers reports whether another check reports stmt with a fix that edits its "{" or the start of its line:
// if-wrapping-rest, which can report an "if" holding one statement when its threshold is 1, or the "else if" check,
// when stmt is a link after a branch that exits.
func (c *checker) reportedByOthers(stmt *ast.IfStmt, stack []ast.Node, parentInfo nodeInfo) bool {
	if c.checks.ifWrappingRest {
		if _, ok := c.wrappedRest(stmt, stack); ok {
			return true
		}
	}

	parent, ok := stack[len(stack)-2].(*ast.IfStmt)

	return ok && parent.Else == stmt && parentInfo.Returns && c.checks.elseIfAfterExit
}

// aloneIn returns the "if" that is the only statement of outer's block, when the two can be combined as written:
// neither has an "else", the inner one has no initialiser or label, the outer condition is not a constant, their
// types can be joined by "&&", and no comment between them would be lost or describe half of the combined condition.
// A comment after the inner "if" describes the outer case once the inner one is done, which the combined block is not.
func (c *checker) aloneIn(outer *ast.IfStmt) *ast.IfStmt {
	if outer.Else != nil || len(outer.Body.List) != 1 {
		return nil
	}

	inner, ok := outer.Body.List[0].(*ast.IfStmt)
	if !ok || inner.Else != nil || inner.Init != nil {
		return nil
	}

	info := c.pass.TypesInfo
	if info.Types[outer.Cond].Value != nil {
		return nil
	}

	if _, ok := joinedType(info.TypeOf(outer.Cond), info.TypeOf(inner.Cond)); !ok {
		return nil
	}

	for _, comment := range c.comments(outer.Cond.End(), inner.Cond.Pos()) {
		if !c.onBraceLine(outer, comment) {
			return nil
		}
	}

	if len(c.comments(inner.End(), outer.Body.Rbrace)) > 0 {
		return nil
	}

	return inner
}

// groupEnd returns the index of the last of levels, from start, that combine into one condition: as many as keep the
// operands within the limit and the types joinable, with at most one comment after the "{" of their lines, which moves
// to the "{" that stays.
func (c *checker) groupEnd(levels []*ast.IfStmt, start int) int {
	info := c.pass.TypesInfo
	first := levels[start]
	n := operands(first.Cond)
	t := info.TypeOf(first.Cond)
	comments := len(c.braceComments(first))

	end := start
	for ; end+1 < len(levels); end++ {
		next := levels[end+1]

		joined, ok := joinedType(t, info.TypeOf(next.Cond))
		if !ok || n+operands(next.Cond) > int(c.aloneMax) || comments+len(c.braceComments(next)) > 1 {
			break
		}

		n += operands(next.Cond)
		t = joined
		comments += len(c.braceComments(next))
	}

	return end
}

// reportGroup reports levels, "if" statements each alone in the block of the one before, which combine into one. The fix
// joins their conditions with "&&", parenthesising an "||", and removes the closing braces of all but the first, so
// the last block, which stays where it is, follows the combined condition, and fixes inside it apply in the same run.
// A comment after a "{" that goes moves after the one that stays.
func (c *checker) reportGroup(levels []*ast.IfStmt) {
	pass := c.pass
	first, last := levels[0], levels[len(levels)-1]

	var (
		edits []analysis.TextEdit
		conds []string
		moved []string
	)

	if orTop(first.Cond) {
		edits = append(edits, analysis.TextEdit{Pos: first.Cond.Pos(), End: first.Cond.Pos(), NewText: []byte("(")})
	}

	for i, l := range levels {
		cond := nodeSource(pass, l.Cond)
		if orTop(l.Cond) {
			cond = "(" + cond + ")"
		}

		conds = append(conds, cond)

		if i == 0 {
			continue
		}

		prev := levels[i-1]

		join := " && "
		if orTop(prev.Cond) {
			join = ")" + join
		}

		if orTop(l.Cond) {
			join += "("
		}

		edits = append(edits,
			analysis.TextEdit{Pos: prev.Cond.End(), End: l.Cond.Pos(), NewText: []byte(join)},
			lineRemoval(pass, l.Body.Rbrace, l.Body.End()),
		)

		for _, comment := range c.braceComments(prev) {
			moved = append(moved, " "+comment.Text)
		}
	}

	if orTop(last.Cond) {
		edits = append(edits, analysis.TextEdit{Pos: last.Cond.End(), End: last.Cond.End(), NewText: []byte(")")})
	}

	if len(moved) > 0 {
		edits = append(edits, analysis.TextEdit{
			Pos:     last.Body.Lbrace + 1,
			End:     last.Body.Lbrace + 1,
			NewText: []byte(strings.Join(moved, "")),
		})
	}

	combined := strings.Join(conds, " && ")
	if x, err := parser.ParseExpr(combined); err == nil {
		combined = mustFormatNode(x)
	}

	c.report(finding{
		stmt: first,
		message: fmt.Sprintf(`If alone in an "if": the "if" block holds only another "if", and neither has an "else". `+
			`Combine the conditions, leaving "if %s { ... }"`, combined),
		fix:   `Combine the conditions with "&&"`,
		edits: edits,
	})
}

// braceComments returns the comments after the "{" of stmt's block on the same line.
func (c *checker) braceComments(stmt *ast.IfStmt) []*ast.Comment {
	tf := c.pass.Fset.File(stmt.Body.Lbrace)
	if tf == nil {
		return nil
	}

	line := tf.Line(stmt.Body.Lbrace)

	end := stmt.Body.Rbrace
	if line < tf.LineCount() {
		end = min(end, tf.LineStart(line+1))
	}

	var comments []*ast.Comment

	for _, comment := range c.comments(stmt.Body.Lbrace+1, end) {
		if c.onBraceLine(stmt, comment) {
			comments = append(comments, comment)
		}
	}

	return comments
}

// onBraceLine reports whether comment is after the "{" of stmt's block, on the same line.
func (c *checker) onBraceLine(stmt *ast.IfStmt, comment *ast.Comment) bool {
	fset := c.pass.Fset

	return comment.Pos() > stmt.Body.Lbrace && fset.Position(comment.Pos()).Line == fset.Position(stmt.Body.Lbrace).Line
}

// comments returns the comments between pos and end.
func (c *checker) comments(pos, end token.Pos) []*ast.Comment {
	var comments []*ast.Comment

	for _, f := range c.pass.Files {
		if pos < f.FileStart || pos >= f.FileEnd {
			continue
		}

		for _, group := range f.Comments {
			if group.End() <= pos || group.Pos() >= end {
				continue
			}

			for _, comment := range group.List {
				if comment.Pos() >= pos && comment.End() <= end {
					comments = append(comments, comment)
				}
			}
		}
	}

	return comments
}

// operands counts the operands of the "&&" and "||" operators in cond, through parentheses: 1 for a condition without
// either.
func operands(cond ast.Expr) int {
	if b, ok := ast.Unparen(cond).(*ast.BinaryExpr); ok && (b.Op == token.LAND || b.Op == token.LOR) {
		return operands(b.X) + operands(b.Y)
	}

	return 1
}

// orTop reports whether cond is an "||" outside parentheses, which binds less tightly than the "&&" joining it.
func orTop(cond ast.Expr) bool {
	b, ok := cond.(*ast.BinaryExpr)

	return ok && b.Op == token.LOR
}

// joinedType returns the type of "x && y" for operands of types x and y, if they can be joined: an untyped boolean
// takes the other operand's type, and otherwise the types must be identical, which two different named boolean types
// are not.
func joinedType(x, y types.Type) (types.Type, bool) {
	if x == nil || y == nil {
		return nil, false
	}

	untyped := func(t types.Type) bool {
		b, ok := t.(*types.Basic)
		return ok && b.Info()&types.IsUntyped != 0
	}

	switch {
	case untyped(x):
		return y, true
	case untyped(y), types.Identical(x, y):
		return x, true
	}

	return nil, false
}
