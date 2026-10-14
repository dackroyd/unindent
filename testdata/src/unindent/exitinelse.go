package unindent

import (
	"errors"
	"testing"
)

func use(...any) {}

func load() (int, error) { return 0, nil }

type count int

type pair struct{ ok bool }

func (p pair) valid() bool { return p.ok }

func ExitInElse(v int, err error) error {
	if err == nil {
		use(v)
	} else { // want `^Exit in "else": the "else" block always ends in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends, and the "if" block does not. Invert the condition, leaving "if err != nil { ... }" followed by the block of statements$`
		return err
	}

	return nil
}

func NotLastStatement(v int, err error) int {
	if err == nil {
		v++
		use(v)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if err != nil { ... }" followed by the block of statements$`
		return 0
	}

	v *= 2

	return v
}

func EmptyBody(ok bool) int {
	if ok {
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !ok { ... }"$`
		return 0
	}

	return 1
}

func CommentOnlyBody(ok bool) int {
	if ok {
		// Carry on.
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !ok { ... }"$`
		return 0
	}

	return 1
}

func Negated(ok bool) int {
	if !ok {
		use(ok)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if ok { ... }" followed by the block of statements$`
		return 0
	}

	return 1
}

func NegatedParens(a, b bool) int {
	if !(a && b) {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if a && b { ... }" followed by the block of statements$`
		return 0
	}

	return 1
}

func Logical(a, b bool) int {
	if a && b {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !a \|\| !b { ... }" followed by the block of statements$`
		return 0
	}

	if a || !b {
		use(b)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !a && b { ... }" followed by the block of statements$`
		return 1
	}

	return 2
}

// DeMorgan negates "&&" and "||" by De Morgan's laws when every operand negates cleanly, and parenthesises an "||" that
// becomes an operand of "&&". Parentheses in the source stay.
func DeMorgan(a, b, c bool, x, y int, p pair, ps []pair, m map[string]bool, f func() bool, s string) int {
	if x == y && p.ok && ps[0].ok && m["k"] && f() && s < "z" {
		use(x)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if x != y \|\| !p.ok \|\| !ps\[0\].ok \|\| !m\["k"\] \|\| !f\(\) \|\| s >= "z" { ... }" followed by the block of statements$`
		return 0
	}

	if a && (b || c) {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !a \|\| \(!b && !c\) { ... }" followed by the block of statements$`
		return 1
	}

	if a || b && c {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !a && \(!b \|\| !c\) { ... }" followed by the block of statements$`
		return 2
	}

	if a && b || c && x > 0 {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if \(!a \|\| !b\) && \(!c \|\| x <= 0\) { ... }" followed by the block of statements$`
		return 3
	}

	if !a && x != y {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if a \|\| x == y { ... }" followed by the block of statements$`
		return 4
	}

	if a || !(b || c) {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !a && \(b \|\| c\) { ... }" followed by the block of statements$`
		return 5
	}

	if a && !(b || c) {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !a \|\| b \|\| c { ... }" followed by the block of statements$`
		return 6
	}

	if a && !(b && c) {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !a \|\| \(b && c\) { ... }" followed by the block of statements$`
		return 7
	}

	if (a) && (x < y) {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if \(!a\) \|\| \(x >= y\) { ... }" followed by the block of statements$`
		return 8
	}

	if a &&
		x > y {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !a \|\| x <= y { ... }" followed by the block of statements$`
		return 9
	}

	return 10
}

const (
	minExp = -10
	maxExp = 10
)

// VariableFirst writes a reversed ordering comparison with a constant on the left only variable first. Equality, and
// comparisons with constants on both sides or the right, keep their order.
func VariableFirst(d byte, e, x int, s string, f float64, a bool) int {
	if '0' <= d && d <= '9' {
		use(d)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if d < '0' \|\| d > '9' { ... }" followed by the block of statements$`
		return 0
	}

	if minExp <= e && e <= maxExp {
		use(e)
	} else { // want `leaving "if e < minExp \|\| e > maxExp { ... }"`
		return 1
	}

	if 0 < len(s) {
		use(s)
	} else { // want `leaving "if len\(s\) <= 0 { ... }"`
		return 2
	}

	if a && 0 < x+1 {
		use(x)
	} else { // want `leaving "if !a \|\| x\+1 <= 0 { ... }"`
		return 3
	}

	if 0 == x {
		use(x)
	} else { // want `leaving "if 0 != x { ... }"`
		return 4
	}

	if 0 < f {
		use(f)
	} else { // want `leaving "if !\(0 < f\) { ... }"`
		return 5
	}

	if 0 < /* min */ x {
		use(x)
	} else { // want `leaving "if 0 >= x { ... }"`
		return 6
	}

	if minExp < maxExp-x {
		use(x)
	} else { // want `leaving "if maxExp-x <= minExp { ... }"`
		return 7
	}

	return 8
}

// DeMorganWrapped keeps "!(...)" around the whole condition when an operand does not negate cleanly: a floating point
// or type parameter ordering, or an operand that is not an identifier, selector, call or index expression, at any depth.
func DeMorganWrapped[T int | float64](a, b bool, x, y float64, t, u T, i, j int, ptr *bool, v any) int {
	if a && x < y {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !\(a && x < y\) { ... }" followed by the block of statements$`
		return 0
	}

	if a || t >= u {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !\(a \|\| t >= u\) { ... }" followed by the block of statements$`
		return 1
	}

	if a && (b || x > 0) {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !\(a && \(b \|\| x > 0\)\) { ... }" followed by the block of statements$`
		return 2
	}

	if a && *ptr {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !\(a && \*ptr\) { ... }" followed by the block of statements$`
		return 3
	}

	if b || v.(bool) {
		use(b)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !\(b \|\| v.\(bool\)\) { ... }" followed by the block of statements$`
		return 4
	}

	if a && x == y && i < j {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !a \|\| x != y \|\| i >= j { ... }" followed by the block of statements$`
		return 5
	}

	return 6
}

func Operands(p pair, ps []pair, m map[string]bool, f func() bool, b *bool, x any) int {
	if p.ok {
		use(p)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !p.ok { ... }" followed by the block of statements$`
		return 0
	}

	if p.valid() {
		use(p)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !p.valid\(\) { ... }" followed by the block of statements$`
		return 1
	}

	if ps[0].ok {
		use(ps)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !ps\[0\].ok { ... }" followed by the block of statements$`
		return 2
	}

	if m["k"] {
		use(m)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !m\["k"\] { ... }" followed by the block of statements$`
		return 3
	}

	if f() {
		use(f)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !f\(\) { ... }" followed by the block of statements$`
		return 4
	}

	if *b {
		use(b)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !\*b { ... }" followed by the block of statements$`
		return 5
	}

	if x.(bool) {
		use(x)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !x.\(bool\) { ... }" followed by the block of statements$`
		return 6
	}

	return 7
}

func IntegerComparisons(x, y int, n count) int {
	if x < y {
		use(x)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if x >= y { ... }" followed by the block of statements$`
		return 0
	}

	if x <= y {
		use(x)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if x > y { ... }" followed by the block of statements$`
		return 1
	}

	if x > 10 {
		use(x)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if x <= 10 { ... }" followed by the block of statements$`
		return 2
	}

	if n >= 3 {
		use(n)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if n < 3 { ... }" followed by the block of statements$`
		return 3
	}

	if x+1 != y*2 {
		use(x)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if x\+1 == y\*2 { ... }" followed by the block of statements$`
		return 4
	}

	return 5
}

func StringComparison(s, t string) int {
	if s < t {
		use(s)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if s >= t { ... }" followed by the block of statements$`
		return 0
	}

	return 1
}

// FloatComparisons keeps ordering comparisons of floating point numbers whole, as "!(a < b)" is true when a or b is
// NaN, but "a >= b" is false. "==" and "!=" are each other's negation, NaN or not.
func FloatComparisons(a, b float64, c float32, i int) int {
	if a < b {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !\(a < b\) { ... }" followed by the block of statements$`
		return 0
	}

	if float64(i) >= a {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !\(float64\(i\) >= a\) { ... }" followed by the block of statements$`
		return 1
	}

	if c > 0.5 {
		use(c)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !\(c > 0.5\) { ... }" followed by the block of statements$`
		return 2
	}

	if a == b {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if a != b { ... }" followed by the block of statements$`
		return 3
	}

	return 4
}

func TypeParameterComparison[T int | float64](a, b T) int {
	if a < b {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !\(a < b\) { ... }" followed by the block of statements$`
		return 0
	}

	return 1
}

func ElseExits(t *testing.T, xs []int) {
	for _, x := range xs {
		if x > 0 {
			use(x)
		} else { // want `^Exit in "else": .* Invert the condition, leaving "if x <= 0 { ... }" followed by the block of statements$`
			continue
		}

		if x < 10 {
			use(x)
		} else { // want `^Exit in "else": .* Invert the condition, leaving "if x >= 10 { ... }" followed by the block of statements$`
			break
		}

		if x != 5 {
			use(x)
		} else { // want `^Exit in "else": .* Invert the condition, leaving "if x == 5 { ... }" followed by the block of statements$`
			t.Fatal("five")
		}

		if x != 6 {
			use(x)
		} else { // want `^Exit in "else": .* Invert the condition, leaving "if x == 6 { ... }" followed by the block of statements. Not fixed automatically: another fix inside the "if" statement has to be applied first$`
			if x > 3 {
				panic("big")
			} else { // want `^Unnecessary "else": .* Remove the "else" wrapping the block of statements$`
				return
			}
		}
	}
}

func NestedExitInElse(a, b bool) int {
	if a {
		if b {
			use(b)
		} else { // want `^Exit in "else": .* Invert the condition, leaving "if !b { ... }" followed by the block of statements$`
			return 0
		}

		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !a { ... }" followed by the block of statements$`
		return 1
	}

	return 2
}

// FixInsideElse withholds the outer fix, which copies the else block, while a fix inside the else block applies.
func FixInsideElse(a, b bool) int {
	if a {
		use(a)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !a { ... }" followed by the block of statements. Not fixed automatically: another fix inside the "if" statement has to be applied first$`
		if b {
			use(b)
		} else { // want `^Exit in "else": .* Invert the condition, leaving "if !b { ... }" followed by the block of statements$`
			return 0
		}

		return 1
	}

	return 2
}

func NotReported(a, b bool) int {
	if a {
		use(a)
	} else {
		use(b)
	}

	if a {
		return 0
	} else { // want `^Unnecessary "else": .* Remove the "else" wrapping the block of statements$`
		return 1
	}
}

type failure struct{}

func (*failure) Error() string { return "failure" }

// ErrorSet is not reported where inverting would nest the happy path, an else block holding more than its exit, under a
// condition that needs an error to be nil.
func ErrorSet(t *testing.T, err error, f *failure, a bool) int {
	if err != nil {
		use(err)
	} else {
		use(a)

		return 0
	}

	if nil != err {
		use(err)
	} else {
		use(a)

		return 1
	}

	if !(err == nil) {
		use(err)
	} else {
		use(a)

		return 2
	}

	if (err != nil) || a {
		use(err)
	} else {
		use(a)

		return 3
	}

	if a || err != nil {
		use(err)
	} else {
		use(a)

		return 4
	}

	if !(a && err == nil) {
		use(err)
	} else {
		use(a)

		return 5
	}

	if f != nil {
		use(f)
	} else {
		use(a)

		return 6
	}

	if _, err := load(); err != nil {
		use(err)
	} else {
		use(a)
		t.Fatal("expected an error")
	}

	return 7
}

// ErrorTested is reported where the inverted condition also holds with an error that is not nil, or tests no error.
func ErrorTested(err error, p *int, x any, a bool) int {
	if err != nil && a {
		use(err)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if err == nil \|\| !a { ... }" followed by the block of statements$`
		return 0
	}

	if !(err != nil) {
		use(err)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if err != nil { ... }" followed by the block of statements$`
		return 1
	}

	if p != nil {
		use(p)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if p == nil { ... }" followed by the block of statements$`
		return 2
	}

	if x != nil {
		use(x)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if x == nil { ... }" followed by the block of statements$`
		return 3
	}

	return 4
}

// ErrorSetExitOnly is reported when the else block holds only its exit: the inverted "if" is an early exit on success
// rather than a nested happy path.
func ErrorSetExitOnly(t *testing.T, try func() error) {
	for {
		if err := try(); err != nil {
			use(err)
		} else { // want `^Exit in "else": .* Move variable declaration "err := try\(\)" before the "if", and invert the condition, leaving "if err == nil { ... }" followed by the block of statements$`
			break
		}
	}

	if err := try(); err != nil {
		use(err)
	} else { // want `^Exit in "else": .* Move variable declaration "err := try\(\)" before the "if", and invert the condition, leaving "if err == nil { ... }" followed by the block of statements$`
		t.Fatal("expected an error")
	}
}

// Chain is only reported for its else if, as the body of an "else if" cannot move out of the chain: it would run after
// the branches before it. Once the else if starts its own "if", the next run reports its else.
func Chain(a, b bool) int {
	if a {
		use(a)
	} else if b {
		use(b)
	} else {
		return 0
	}

	if a {
		return 1
	} else if b { // want `^Unnecessary "else": .* Remove the "else", leaving "if b { ... }"$`
		use(b)
	} else {
		return 2
	}

	return 3
}

func ElseIfInElse(a, b bool) int {
	if a {
		use(a)
	} else if b {
		return 0
	} else {
		return 1
	}

	return 2
}

func InitialiserStays(x int) int {
	if v, err := load(); err == nil {
		use(x)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if err != nil { ... }" followed by the block of statements$`
		return v
	}

	return x
}

func InitialiserHoisted() error {
	if v, err := load(); err == nil {
		use(v)
	} else { // want `^Exit in "else": .* Move variable declaration "v, err := load\(\)" before the "if", and invert the condition, leaving "if err != nil { ... }" followed by the block of statements$`
		return err
	}

	return nil
}

func InitialiserAlreadyDeclared() (err error) {
	if v, err := load(); err == nil {
		use(v)
	} else { // want `^Exit in "else": .* Move variable declaration "v, err := load\(\)" before the "if", and invert the condition, leaving "if err != nil { ... }" followed by the block of statements. Not fixed automatically: "err" is already declared in the enclosing block$`
		return err
	}

	return nil
}

func InitialiserHidesLaterUse(v int) int {
	for range 3 {
		if v, err := load(); err == nil {
			use(v)
		} else { // want `^Exit in "else": .* Not fixed automatically: "v" would hide the "v" used after the "if"$`
			return 0
		}

		v++
	}

	return v
}

func BodyRedeclaresInitialiser() int {
	if v, err := load(); err == nil {
		use(v)
		err := errors.New("shadow")
		use(err)
	} else { // want `^Exit in "else": .* Not fixed automatically: "err" is also declared by the "if" initialiser$`
		return 0
	}

	return 1
}

func BodyAlreadyDeclared(ok bool) int {
	y := 1

	for range 3 {
		y := 2
		if ok {
			y := 3
			use(y)
		} else { // want `^Exit in "else": .* Not fixed automatically: "y" is already declared in the enclosing block$`
			continue
		}

		use(y)
	}

	return y
}

func BodyHidesLaterUse(ok bool) int {
	y := 1

	for range 3 {
		if ok {
			y := 3
			use(y)
		} else { // want `^Exit in "else": .* Not fixed automatically: "y" would hide the "y" used after the "if"$`
			continue
		}

		y++
	}

	return y
}

func BodyMovedByEarlierFix(ok bool) int {
	if ok {
		return 0
	} else { // want `^Unnecessary "else": .* Remove the "else" wrapping the block of statements$`
		y := 1
		use(y)
	}

	if !ok {
		y := 2
		use(y)
	} else { // want `^Exit in "else": .* Not fixed automatically: "y" is also moved into the enclosing block by an earlier fix$`
		return 1
	}

	return 2
}

// OuterElseHoldsMove withholds the outer fix: the inner fix moves y into the else block, which the outer fix moves next
// to the y declared before it.
func OuterElseHoldsMove(x int) int {
	y := 0

	if x > 10 {
		return 1
	} else { // want `^Unnecessary "else": .* Not fixed automatically: "y" is already declared in the enclosing block$`
		if x > 5 {
			y := 2
			use(y)
		} else { // want `^Exit in "else": .* Invert the condition, leaving "if x <= 5 { ... }" followed by the block of statements$`
			return 3
		}
	}

	return y
}

func Comments(x int) int {
	// Before.
	if /* positive */ x > 0 { // Positive.
		// Body.
		use(x)
		// End of body.
	} else /* between */ { // want `^Exit in "else": .* Invert the condition, leaving "if x <= 0 { ... }" followed by the block of statements$`
		// Else.
		return 0 // Zero.
		// End of else.
	} // After.

	return x
}

func LineCommentBeforeBrace(x int) int {
	if x > 0 {
		use(x)
	} else // Brace on the next line.
	{      // want `^Exit in "else": .* Invert the condition, leaving "if x <= 0 { ... }" followed by the block of statements$`
		return 0
	}

	return x
}

func LabelInBody(xs []int) int {
	if len(xs) > 0 {
	loop:
		for _, x := range xs {
			if x == 0 {
				break loop
			}
		}
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if len\(xs\) <= 0 { ... }" followed by the block of statements$`
		return 0
	}

	return 1
}

func GotoOverBody(x int) int {
	if x > 5 {
		goto done
	}

	if x > 0 {
		y := x
		use(y)
	} else { // want `^Exit in "else": .* Not fixed automatically: "goto done" would jump over the declaration of "y"$`
		return 0
	}

done:
	return x
}

func GotoInElse(x int) int {
	if x > 0 {
		y := x
		use(y)
	} else { // want `^Exit in "else": .* Not fixed automatically: "goto done" would jump over the declaration of "y"$`
		goto done
	}

	return 1

done:
	return x
}

func GotoInElseWithoutDeclarations(x int) int {
	if x > 0 {
		use(x)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if x <= 0 { ... }" followed by the block of statements$`
		goto done
	}

	return 1

done:
	return x
}

func Labelled(x int) int {
outer:
	if x > 0 {
		use(x)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if x <= 0 { ... }" followed by the block of statements$`
		return 0
	}

	if x > 10 {
		goto outer
	}

	return x
}

func InCases(ch chan int, x int) int {
	switch x {
	case 1:
		if x > 0 {
			use(x)
		} else { // want `^Exit in "else": .* Invert the condition, leaving "if x <= 0 { ... }" followed by the block of statements$`
			break
		}

		x++
	}

	select {
	case v := <-ch:
		if v > 0 {
			use(v)
		} else { // want `^Exit in "else": .* Invert the condition, leaving "if v <= 0 { ... }" followed by the block of statements$`
			return 0
		}
	}

	return x
}

func InClosure() func(int) int {
	return func(x int) int {
		if x > 0 {
			x--
		} else { // want `^Exit in "else": .* Invert the condition, leaving "if x <= 0 { ... }" followed by the block of statements$`
			return 0
		}

		return x
	}
}
