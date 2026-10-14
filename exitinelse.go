package unindent

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// elseBlockExits reports whether stmt's else is a block that always exits. When its body does not, inverting the
// condition lets the else block exit first and the body follow the "if". Callers check the body, which they need to
// know about anyway.
func (c *checker) elseBlockExits(stmt *ast.IfStmt) bool {
	els, ok := stmt.Else.(*ast.BlockStmt)

	return ok && c.blockAlwaysReturns(els)
}

// invertHoistsInit reports whether the fix that inverts stmt moves its initialiser before the "if", as the body, which
// moves out of the "if", uses what it declares. Otherwise the initialiser stays.
func invertHoistsInit(info *types.Info, stmt *ast.IfStmt) bool {
	scope := info.Scopes[stmt]
	if stmt.Init == nil || scope == nil || scope.Len() == 0 {
		return false
	}

	var used bool

	ast.Inspect(stmt.Body, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if obj := info.Uses[ident]; obj != nil && obj.Parent() == scope {
				used = true
			}
		}

		return !used
	})

	return used
}

// reportExitInElse reports the else of stmt, which always exits after a body that does not. The fix inverts the
// condition, moves the else block into the "if" and the body after it, with a fix unless the fix would not compile or
// would change behaviour.
func (c *checker) reportExitInElse(stmt *ast.IfStmt, stack []ast.Node) {
	pass, info := c.pass, c.pass.TypesInfo
	els, _ := stmt.Else.(*ast.BlockStmt)
	_, enclosing := chainLinks(stack)
	hoists := invertHoistsInit(info, stmt)

	var (
		args      []any
		msg       strings.Builder
		conflicts []string
		edits     []analysis.TextEdit
	)

	for _, call := range c.buildDependentExits(els) {
		conflicts = append(conflicts, fmt.Sprintf("%q never returns only in some builds", call))
	}

	conflicts = append(conflicts, c.invertConflicts(info, stmt, enclosing, hoists)...)

	msg.WriteString(`Exit in "else": the "else" block always ends in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends, and the "if" block does not. `)

	if hoists {
		in := nodeSource(pass, stmt.Init)
		edits = append(edits, analysis.TextEdit{
			Pos:     stmt.Pos(),
			End:     stmt.Init.End(),
			NewText: []byte(in + "\nif "),
		})

		msg.WriteString(`Move variable declaration %q before the "if", and invert the condition`)
		args = append(args, in)
	} else {
		msg.WriteString(`Invert the condition`)
	}

	neg, negEdits := c.negation(stmt.Cond)
	edits = append(edits, negEdits...)

	msg.WriteString(`, leaving "if %s { ... }"`)
	args = append(args, c.formatNegation(stmt.Cond, neg, negEdits))

	if len(stmt.Body.List) > 0 {
		msg.WriteString(` followed by the block of statements`)
	}

	// The else block moves into the "if", and the body stays where it is, so fixes inside the body still apply.
	exit := commentsBetween(pass, stmt.Body.Rbrace, els.Lbrace) + blockSource(pass, els)

	if stmt.Body.Lbrace+1 == stmt.Body.Rbrace {
		edits = append(edits, analysis.TextEdit{
			Pos:     stmt.Body.Lbrace + 1,
			End:     els.End(),
			NewText: []byte(exit + "}"),
		})
	} else {
		// The line break after the else block goes too, as the body already ends in one before its closing brace.
		end := els.End()
		if next, ok := source(pass, end, end+1); ok && next == "\n" {
			end++
		}

		edits = append(edits,
			analysis.TextEdit{Pos: stmt.Body.Lbrace + 1, End: stmt.Body.Lbrace + 1, NewText: []byte(exit + "}\n\n")},
			analysis.TextEdit{Pos: stmt.Body.Rbrace, End: end},
		)
	}

	c.report(finding{
		stmt:      stmt,
		message:   fmt.Sprintf(msg.String(), args...),
		fix:       "Invert the condition to exit first",
		edits:     edits,
		conflicts: conflicts,
		copies:    true,
	})
}

// invertConflicts describes why the fix that inverts stmt cannot move its body, and its initialiser when it hoists it,
// into the block enclosing it.
func (c *checker) invertConflicts(info *types.Info, stmt *ast.IfStmt, enclosing ast.Node, hoists bool) []string {
	ifScope, bodyScope := info.Scopes[stmt], info.Scopes[stmt.Body]
	if enclosing == nil || ifScope == nil || bodyScope == nil {
		return nil
	}

	outer := ifScope.Parent()
	earlier := c.movedBefore(info, enclosing, stmt)

	var (
		conflicts []string
		initNames []string
		vars      []movedVar
	)

	if hoists {
		initNames = ifScope.Names()

		for _, name := range initNames {
			vars = append(vars, movedVar{name, ifScope.Lookup(name).Pos()})
		}
	}

	bodyNames := c.movedNames(info, stmt.Body)
	bodyVars := movedVars(bodyScope, stmt.Body, bodyNames)

	for i, name := range slices.Concat(initNames, bodyNames) {
		switch {
		case i >= len(initNames) && slices.Contains(initNames, name):
			conflicts = append(conflicts, fmt.Sprintf("%q is also declared by the \"if\" initialiser", name))
		case outer.Lookup(name) != nil:
			conflicts = append(conflicts, fmt.Sprintf("%q is already declared in the enclosing block", name))
		case slices.Contains(earlier, name):
			conflicts = append(conflicts, fmt.Sprintf("%q is also moved into the enclosing block by an earlier fix", name))
		case usedAfter(info, enclosing, stmt.End(), name, outer):
			conflicts = append(conflicts, fmt.Sprintf("%q would hide the %q used after the \"if\"", name, name))
		}
	}

	conflicts = append(conflicts, gotoConflicts(enclosing, stmt, slices.Concat(vars, bodyVars))...)

	return append(conflicts, elseGotoConflicts(stmt, enclosing, bodyVars, conflicts)...)
}

// elseGotoConflicts describes the gotos in stmt's else block that would jump over a declaration of the body once the
// fix moves the else block before it: a goto to a label in enclosing after the "if". Those before the "if" or in the
// body are described by gotoConflicts, and skipped here.
func elseGotoConflicts(stmt *ast.IfStmt, enclosing ast.Node, vars []movedVar, reported []string) []string {
	labels := labelsAfter(enclosing, stmt.End())
	if stmt.Else == nil || len(labels) == 0 || len(vars) == 0 {
		return nil
	}

	var conflicts []string

	ast.Inspect(stmt.Else, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			if t.Tok != token.GOTO || !slices.Contains(labels, t.Label.Name) {
				return true
			}

			for _, v := range vars {
				conflict := fmt.Sprintf("\"goto %s\" would jump over the declaration of %q", t.Label.Name, v.name)
				if !slices.Contains(reported, conflict) && !slices.Contains(conflicts, conflict) {
					conflicts = append(conflicts, conflict)
				}
			}
		}

		return true
	})

	return conflicts
}

// negation returns cond negated, and the edits that negate it in the source, which keep the rest of it as written. A
// negation is removed, "==" and "!=" swap, and an ordering comparison of integers or strings is reversed. Floating
// point comparisons are not, as "!(a < b)" differs from "a >= b" when either is NaN. "&&" and "||" swap by De Morgan's
// laws when each operand negates that way or is an identifier, selector, call or index expression, which gets a "!".
// Other conditions are wrapped in "!(...)" unless they are an operand already.
func (c *checker) negation(cond ast.Expr) (ast.Expr, []analysis.TextEdit) {
	switch e := cond.(type) {
	case *ast.UnaryExpr:
		if e.Op != token.NOT {
			break
		}

		if p, ok := e.X.(*ast.ParenExpr); ok {
			return p.X, []analysis.TextEdit{{Pos: e.OpPos, End: p.Lparen + 1}, {Pos: p.Rparen, End: p.End()}}
		}

		return e.X, []analysis.TextEdit{{Pos: e.OpPos, End: e.X.Pos()}}
	case *ast.BinaryExpr:
		if neg, edits, ok := c.negatedComparisonEdits(e); ok {
			return neg, edits
		}

		if neg, edits, ok := c.deMorgan(e, token.ILLEGAL); ok {
			return neg, mergeInsertions(edits)
		}

		return &ast.UnaryExpr{Op: token.NOT, X: &ast.ParenExpr{X: e}}, []analysis.TextEdit{
			{Pos: e.Pos(), End: e.Pos(), NewText: []byte("!(")},
			{Pos: e.End(), End: e.End(), NewText: []byte(")")},
		}
	}

	return &ast.UnaryExpr{Op: token.NOT, X: cond}, []analysis.TextEdit{
		{Pos: cond.Pos(), End: cond.Pos(), NewText: []byte("!")},
	}
}

// formatNegation returns cond as the edits that negate it leave it, formatted on one line. Where the file cannot be
// read, it returns neg formatted.
func (c *checker) formatNegation(cond, neg ast.Expr, edits []analysis.TextEdit) string {
	src, ok := source(c.pass, cond.Pos(), cond.End())
	if !ok {
		return mustFormatNode(neg)
	}

	edits = slices.SortedStableFunc(slices.Values(edits), func(a, b analysis.TextEdit) int { return cmp.Compare(a.Pos, b.Pos) })

	var b strings.Builder

	last := cond.Pos()
	for _, e := range edits {
		b.WriteString(src[last-cond.Pos() : e.Pos-cond.Pos()])
		b.Write(e.NewText)
		last = e.End
	}

	b.WriteString(src[last-cond.Pos():])

	x, err := parser.ParseExpr(b.String())
	if err != nil {
		return mustFormatNode(neg)
	}

	return mustFormatNode(x)
}

// deMorgan returns e, an "&&" or "||", negated by De Morgan's laws, with the edits that negate it, if every operand
// negates without wrapping it in "!(...)". The negation is an operand of outer, or of nothing when outer is
// token.ILLEGAL. An "&&" that becomes an "||" as an operand of "&&" is parenthesised.
func (c *checker) deMorgan(e *ast.BinaryExpr, outer token.Token) (ast.Expr, []analysis.TextEdit, bool) {
	var op token.Token

	switch e.Op {
	case token.LAND:
		op = token.LOR
	case token.LOR:
		op = token.LAND
	default:
		return nil, nil, false
	}

	var edits []analysis.TextEdit

	parens := op == token.LOR && outer == token.LAND
	if parens {
		edits = append(edits, analysis.TextEdit{Pos: e.Pos(), End: e.Pos(), NewText: []byte("(")})
	}

	x, xEdits, ok := c.negatedOperand(e.X, op)
	if !ok {
		return nil, nil, false
	}

	y, yEdits, ok := c.negatedOperand(e.Y, op)
	if !ok {
		return nil, nil, false
	}

	edits = append(edits, xEdits...)
	edits = append(edits, operatorEdit(e, op))
	edits = append(edits, yEdits...)

	var neg ast.Expr = &ast.BinaryExpr{X: x, OpPos: e.OpPos, Op: op, Y: y}

	if parens {
		edits = append(edits, analysis.TextEdit{Pos: e.End(), End: e.End(), NewText: []byte(")")})
		neg = &ast.ParenExpr{X: neg}
	}

	return neg, edits, true
}

// negatedOperand returns x, an operand of an "&&" or "||" that becomes an operand of op by De Morgan's laws, negated
// without wrapping it in "!(...)", with the edits that negate it, if it can be. Parentheses in the source stay, except
// around the operand of a removed "!" that needs none: "!(a && b)" becomes "a && b" as an operand of "&&", and stays
// "(a && b)" as an operand of "||".
func (c *checker) negatedOperand(x ast.Expr, op token.Token) (ast.Expr, []analysis.TextEdit, bool) {
	switch e := x.(type) {
	case *ast.ParenExpr:
		neg, edits, ok := c.negatedOperand(e.X, token.ILLEGAL)
		if !ok {
			return nil, nil, false
		}

		return &ast.ParenExpr{X: neg}, edits, true
	case *ast.UnaryExpr:
		if e.Op != token.NOT {
			return nil, nil, false
		}

		p, ok := e.X.(*ast.ParenExpr)
		if !ok {
			return e.X, []analysis.TextEdit{{Pos: e.OpPos, End: e.X.Pos()}}, true
		}

		if b, ok := p.X.(*ast.BinaryExpr); ok && op != token.ILLEGAL && (b.Op == token.LAND || b.Op == token.LOR) && b.Op != op {
			return p, []analysis.TextEdit{{Pos: e.OpPos, End: p.Pos()}}, true
		}

		return p.X, []analysis.TextEdit{{Pos: e.OpPos, End: p.Lparen + 1}, {Pos: p.Rparen, End: p.End()}}, true
	case *ast.BinaryExpr:
		if neg, edits, ok := c.negatedComparisonEdits(e); ok {
			return neg, edits, true
		}

		return c.deMorgan(e, op)
	case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr:
		return &ast.UnaryExpr{Op: token.NOT, X: x}, []analysis.TextEdit{{Pos: x.Pos(), End: x.Pos(), NewText: []byte("!")}}, true
	}

	return nil, nil, false
}

// operatorEdit returns the edit that replaces the operator of e with op.
func operatorEdit(e *ast.BinaryExpr, op token.Token) analysis.TextEdit {
	return analysis.TextEdit{Pos: e.OpPos, End: e.OpPos + token.Pos(len(e.Op.String())), NewText: []byte(op.String())}
}

// mergeInsertions sorts edits and joins each insertion with the edits that start where it is, in order, so that no two
// edits start at the same position, where the order they apply in would be ambiguous.
func mergeInsertions(edits []analysis.TextEdit) []analysis.TextEdit {
	slices.SortStableFunc(edits, func(a, b analysis.TextEdit) int { return cmp.Compare(a.Pos, b.Pos) })

	var merged []analysis.TextEdit

	for _, e := range edits {
		if n := len(merged); n > 0 && merged[n-1].Pos == e.Pos && merged[n-1].Pos == merged[n-1].End {
			merged[n-1].End = e.End
			merged[n-1].NewText = append(slices.Clip(merged[n-1].NewText), e.NewText...)

			continue
		}

		merged = append(merged, e)
	}

	return merged
}

// negatedComparisonEdits returns the comparison e negated, with the edits that negate it, if its operator can be
// swapped. A reversed ordering comparison with a constant on the left only is written variable first, so the range
// check "'0' <= d && d <= '9'" negates to "d < '0' || d > '9'".
func (c *checker) negatedComparisonEdits(e *ast.BinaryExpr) (ast.Expr, []analysis.TextEdit, bool) {
	op, ok := c.negatedComparison(e)
	if !ok {
		return nil, nil, false
	}

	neg := *e
	neg.Op = op

	mirrored, ok := mirroredOrder[op]
	if !ok || !c.constant(e.X) || c.constant(e.Y) || c.hasComments(e.Pos(), e.End()) {
		return &neg, []analysis.TextEdit{operatorEdit(e, op)}, true
	}

	x, okX := source(c.pass, e.X.Pos(), e.X.End())
	before, okB := source(c.pass, e.X.End(), e.OpPos)
	after, okA := source(c.pass, e.OpPos+token.Pos(len(e.Op.String())), e.Y.Pos())
	y, okY := source(c.pass, e.Y.Pos(), e.Y.End())

	if !okX || !okB || !okA || !okY {
		return &neg, []analysis.TextEdit{operatorEdit(e, op)}, true
	}

	swapped := &ast.BinaryExpr{X: e.Y, OpPos: e.OpPos, Op: mirrored, Y: e.X}
	text := y + before + mirrored.String() + after + x

	return swapped, []analysis.TextEdit{{Pos: e.Pos(), End: e.End(), NewText: []byte(text)}}, true
}

// mirroredOrder holds, for each ordering operator, the one that gives the same result with the operands swapped.
var mirroredOrder = map[token.Token]token.Token{
	token.LSS: token.GTR,
	token.GTR: token.LSS,
	token.LEQ: token.GEQ,
	token.GEQ: token.LEQ,
}

// constant reports whether x has a constant value.
func (c *checker) constant(x ast.Expr) bool {
	return c.pass.TypesInfo.Types[x].Value != nil
}

// hasComments reports whether a comment lies between pos and end.
func (c *checker) hasComments(pos, end token.Pos) bool {
	for _, f := range c.pass.Files {
		if pos < f.FileStart || pos >= f.FileEnd {
			continue
		}

		for _, group := range f.Comments {
			if group.Pos() < end && group.End() > pos {
				return true
			}
		}
	}

	return false
}

// negatedComparison returns the operator that negates the comparison e, if it is one that can be swapped.
func (c *checker) negatedComparison(e *ast.BinaryExpr) (token.Token, bool) {
	switch e.Op {
	case token.EQL:
		return token.NEQ, true
	case token.NEQ:
		return token.EQL, true
	}

	ordered := map[token.Token]token.Token{
		token.LSS: token.GEQ,
		token.GEQ: token.LSS,
		token.GTR: token.LEQ,
		token.LEQ: token.GTR,
	}

	op, ok := ordered[e.Op]
	if !ok || !c.totallyOrdered(e.X) || !c.totallyOrdered(e.Y) {
		return 0, false
	}

	return op, true
}

// totallyOrdered reports whether x is an integer or a string, whose comparisons have no NaN to make "<" and ">="
// both false. Type parameters are not, as their type set may hold floating point types.
func (c *checker) totallyOrdered(x ast.Expr) bool {
	t := c.pass.TypesInfo.TypeOf(x)
	if t == nil {
		return false
	}

	b, ok := t.Underlying().(*types.Basic)

	return ok && b.Info()&(types.IsInteger|types.IsString) != 0
}

// nestsHappyPath reports whether inverting stmt would nest its else block, the happy path, under a condition that
// needs an error to be nil: its condition can only hold while an error is not nil, and the else block holds more than
// its exit. An else block that only exits becomes an early exit on success, "if err == nil { break }".
func (c *checker) nestsHappyPath(stmt *ast.IfStmt) bool {
	els, ok := stmt.Else.(*ast.BlockStmt)

	return ok && len(els.List) > 1 && c.errorSet(stmt.Cond)
}

// errorSet reports whether cond can only hold while an error is not nil, so that the else of an "if" testing it runs
// only on the happy path: "err != nil", "!(err == nil)" or a "||" with such an operand. "err != nil && ok" is not, as
// its else block also runs when ok is false.
func (c *checker) errorSet(cond ast.Expr) bool {
	switch e := ast.Unparen(cond).(type) {
	case *ast.UnaryExpr:
		return e.Op == token.NOT && c.errorUnset(e.X)
	case *ast.BinaryExpr:
		switch e.Op {
		case token.LOR:
			return c.errorSet(e.X) || c.errorSet(e.Y)
		case token.NEQ:
			return c.errorComparedWithNil(e)
		}
	}

	return false
}

// errorUnset reports whether cond can only hold while an error is nil, the negation of what errorSet tests.
func (c *checker) errorUnset(cond ast.Expr) bool {
	switch e := ast.Unparen(cond).(type) {
	case *ast.UnaryExpr:
		return e.Op == token.NOT && c.errorSet(e.X)
	case *ast.BinaryExpr:
		switch e.Op {
		case token.LAND:
			return c.errorUnset(e.X) || c.errorUnset(e.Y)
		case token.EQL:
			return c.errorComparedWithNil(e)
		}
	}

	return false
}

// errorComparedWithNil reports whether e compares a value whose type implements error with nil.
func (c *checker) errorComparedWithNil(e *ast.BinaryExpr) bool {
	info := c.pass.TypesInfo
	errorType := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

	for _, pair := range [][2]ast.Expr{{e.X, e.Y}, {e.Y, e.X}} {
		if !info.Types[pair[1]].IsNil() {
			continue
		}

		if t := info.TypeOf(pair[0]); t != nil && types.Implements(t, errorType) {
			return true
		}
	}

	return false
}

// blockSource returns the source between the braces of block, comments included. Where the file cannot be read, it
// returns its statements formatted, without their comments.
func blockSource(pass *analysis.Pass, block *ast.BlockStmt) string {
	if src, ok := source(pass, block.Lbrace+1, block.Rbrace); ok {
		return src
	}

	var b strings.Builder

	for _, s := range block.List {
		b.WriteString("\n" + mustFormatNode(s))
	}

	return b.String() + "\n"
}

// commentsBetween returns the comments between pos and end, each followed by the space or line break it needs, for
// a fix that removes the text between them.
func commentsBetween(pass *analysis.Pass, pos, end token.Pos) string {
	var b strings.Builder

	for _, f := range pass.Files {
		if pos < f.FileStart || pos >= f.FileEnd {
			continue
		}

		for _, group := range f.Comments {
			for _, comment := range group.List {
				if comment.Pos() < pos || comment.End() > end {
					continue
				}

				b.WriteString(comment.Text)

				if strings.HasPrefix(comment.Text, "//") {
					b.WriteString("\n")
				} else {
					b.WriteString(" ")
				}
			}
		}
	}

	return b.String()
}

// source returns the source text between pos and end, if the file can be read.
func source(pass *analysis.Pass, pos, end token.Pos) (string, bool) {
	tf := pass.Fset.File(pos)
	if tf == nil || pass.ReadFile == nil {
		return "", false
	}

	src, err := pass.ReadFile(tf.Name())
	if err != nil || len(src) != tf.Size() {
		return "", false
	}

	return string(src[tf.Offset(pos):tf.Offset(end)]), true
}
