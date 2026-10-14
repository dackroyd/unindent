package unindent

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// defaultWrappingMin is the default of the if-wrapping-rest-min-statements flag. Below it, most blocks are short
// filters inside loops, where the inverted "if" adds as many lines as the nesting it removes.
const defaultWrappingMin minStatements = 3

// minStatements is a flag holding a number of statements, at least 1: with none, an "if" that only exits would be
// reported again once fixed.
type minStatements int

func (m *minStatements) String() string {
	return strconv.Itoa(int(*m))
}

func (m *minStatements) Set(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil {
		return err
	}

	if n < 1 {
		return fmt.Errorf("must be at least 1, not %d", n)
	}

	*m = minStatements(n)

	return nil
}

// wrapping is the body of a function or loop that an "if" wraps the rest of, and how to leave it early.
type wrapping struct {
	// exit leaves the body: "return" for a function, and "continue" for a loop.
	exit token.Token
	// label is the label of the loop, if it has one.
	label *ast.Ident
}

// wrappedRest reports whether stmt, an "if" without an "else", is the last statement of the body of a function without
// results or of a loop, with at least as many statements to move out of its block as the check reports. Falling off the
// end of that body then does what the returned wrapping's exit does, so the inverted condition can exit early.
func (c *checker) wrappedRest(stmt *ast.IfStmt, stack []ast.Node) (wrapping, bool) {
	if len(stack) < 3 {
		return wrapping{}, false
	}

	body, ok := stack[len(stack)-2].(*ast.BlockStmt)
	if !ok || body.List[len(body.List)-1] != stmt {
		return wrapping{}, false
	}

	var w wrapping

	switch n := stack[len(stack)-3].(type) {
	case *ast.FuncDecl:
		// A function with results cannot end in an "if" without an "else", which is not a terminating statement.
		if n.Body != body || n.Type.Results.NumFields() > 0 {
			return wrapping{}, false
		}

		w.exit = token.RETURN
	case *ast.FuncLit:
		if n.Body != body || n.Type.Results.NumFields() > 0 {
			return wrapping{}, false
		}

		w.exit = token.RETURN
	case *ast.ForStmt, *ast.RangeStmt:
		if endsSearch(stmt.Body) {
			return wrapping{}, false
		}

		w.exit = token.CONTINUE

		if len(stack) >= 4 {
			if l, ok := stack[len(stack)-4].(*ast.LabeledStmt); ok {
				w.label = l.Label
			}
		}
	default:
		return wrapping{}, false
	}

	n := len(stmt.Body.List)
	if w.redundantExit(stmt.Body) != nil {
		n--
	}

	return w, n >= c.wrappingMin
}

// endsSearch reports whether block, at the end of a loop body, ends in a "return", "break" or "goto": the loop looks
// for one element and acts on it once. Inverted, the guard would make the block look like it runs for every element,
// with the exit at the end of the loop body.
func endsSearch(block *ast.BlockStmt) bool {
	if len(block.List) == 0 {
		return false
	}

	switch s := block.List[len(block.List)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return s.Tok == token.BREAK || s.Tok == token.GOTO
	}

	return false
}

// redundantExit returns the last statement of block when it is the exit of w, which falling off the end of the body
// does once the block follows the "if": a "return" without results in a function, or a "continue" to the loop.
func (w wrapping) redundantExit(block *ast.BlockStmt) ast.Stmt {
	if len(block.List) == 0 {
		return nil
	}

	switch s := block.List[len(block.List)-1].(type) {
	case *ast.ReturnStmt:
		if w.exit == token.RETURN && len(s.Results) == 0 {
			return s
		}
	case *ast.BranchStmt:
		toLoop := s.Label == nil || w.label != nil && s.Label.Name == w.label.Name
		if w.exit == token.CONTINUE && s.Tok == token.CONTINUE && toLoop {
			return s
		}
	}

	return nil
}

// reportWrappingRest reports stmt, an "if" without an "else" that wraps the rest of the function or loop body w
// describes. The fix inverts the condition to leave the body early, and moves the block after the "if", unless the fix
// would not compile.
func (c *checker) reportWrappingRest(stmt *ast.IfStmt, stack []ast.Node, w wrapping) {
	pass, info := c.pass, c.pass.TypesInfo
	_, enclosing := chainLinks(stack)
	hoists := invertHoistsInit(info, stmt)
	redundant := w.redundantExit(stmt.Body)

	var (
		args  []any
		msg   strings.Builder
		edits []analysis.TextEdit
	)

	n := len(stmt.Body.List)
	if redundant != nil {
		n--
	}

	if w.exit == token.RETURN {
		msg.WriteString(`If wrapping the rest of the function: the function ends in an "if" without an "else", whose block holds %d statements. `)
	} else {
		msg.WriteString(`If wrapping the rest of the loop: the loop body ends in an "if" without an "else", whose block holds %d statements. `)
	}

	args = append(args, n)

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

	msg.WriteString(`, leaving "if %s { %s }" followed by the block of statements`)
	args = append(args, c.formatNegation(stmt.Cond, neg, negEdits), w.exit)

	if redundant != nil {
		msg.WriteString(` without its final %q`)
		args = append(args, mustFormatNode(redundant))
	}

	// The block stays where it is, with only its braces replaced, so fixes inside it still apply. A blank line separates
	// it from the "if".
	edits = append(edits, analysis.TextEdit{
		Pos:     stmt.Body.Lbrace + 1,
		End:     stmt.Body.Lbrace + 1,
		NewText: []byte("\n" + w.exit.String() + "\n}\n\n"),
	})

	if redundant != nil {
		edits = append(edits, lineRemoval(pass, redundant.Pos(), redundant.End()))
	}

	// The line break after the closing brace goes too, as the block already ends in one.
	edits = append(edits, lineRemoval(pass, stmt.Body.Rbrace, stmt.Body.End()))

	c.report(finding{
		stmt:      stmt,
		message:   fmt.Sprintf(msg.String(), args...),
		fix:       "Invert the condition to leave early",
		edits:     edits,
		conflicts: c.invertConflicts(info, stmt, enclosing, hoists),
		copies:    hoists,
	})
}

// lineRemoval returns the edit that removes the source between pos and end. When nothing else is on its line, the line
// goes too, with the blank lines before it, so no blank line is left at the end of the block.
func lineRemoval(pass *analysis.Pass, pos, end token.Pos) analysis.TextEdit {
	edit := analysis.TextEdit{Pos: pos, End: end}

	tf := pass.Fset.File(pos)
	if tf == nil {
		return edit
	}

	if after, ok := source(pass, end, end+1); !ok || after != "\n" {
		return edit
	}

	line := tf.Line(pos)
	start := tf.LineStart(line)

	if before, ok := source(pass, start, pos); !ok || strings.TrimSpace(before) != "" {
		return edit
	}

	for ; line > 1; line-- {
		text, ok := source(pass, tf.LineStart(line-1), start)
		if !ok || strings.TrimSpace(text) != "" {
			break
		}

		start = tf.LineStart(line - 1)
	}

	return analysis.TextEdit{Pos: start, End: end + 1}
}
