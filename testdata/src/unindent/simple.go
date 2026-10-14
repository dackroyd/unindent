package unindent

import "strconv"

func Empty() {}

func EmptyElse(x, y int) int {
	if x > y {
		return x
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		// Continue
	}

	return y
}

func Ident(x int) int {
	return x
}

func IfEmpty(x, y int) int {
	if x > y {
		// Continue
	}

	return y
}

func MaxOk(x, y int) int {
	if x > y {
		return x
	}

	return y
}

func MaxReturnWithElse(x, y int) int {
	if x > y {
		return x
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		return y
	}
}

func MaxReturnMulti(x, y int) int {
	if x > y {
		return x
	} else if x == y { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else", leaving "if x == y { ... }"$`
		return x
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		return y
	}
}

func MaxAssign(x, y int) int {
	var max int

	if x > y {
		max = x
	} else {
		max = y
	}

	return max
}

func MaxAssignReturn(x, y int) int {
	var max int

	if x > y {
		max = x
	} else { // want `^Exit in "else": the "else" block always ends in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends, and the "if" block does not. Invert the condition, leaving "if x <= y { ... }" followed by the block of statements$`
		// Note: only the "else" returns, so the condition can be inverted to return first
		return y
	}

	return max
}

func MaxAssignReturnMulti(x, y int) int {
	var max int

	if x > y {
		max = x
	} else if x == y {
		return x
	} else {
		// Note: "else" required, as the first branch doesn't return
		return y
	}

	return max
}

func MaxReturnAssign(x, y int) int {
	var max int

	if x > y {
		return x
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		max = y
	}

	return max
}

func MaxUpdateIf(x, y int) int {
	max := x

	if x < y {
		max = y
	}

	return max
}

func Sign(x int) int {
	sign := 1
	if x > 0 {
		return sign
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		return -sign
	}
}

func SignWithInit(x int) int {
	if sign := 1; x > 0 {
		return sign
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Move variable declaration "sign := 1" before the "if", and remove the "else" wrapping the block of statements$`
		return -sign
	}
}

func FizBuzz(x int) string {
	fizz := "fizz"
	buzz := "buzz"

	if x%3 == 0 && x%5 == 0 {
		return fizz + buzz
	} else if x%3 == 0 { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else", leaving "if x%3 == 0 { ... }"$`
		return fizz
	} else if x%5 == 0 { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else", leaving "if x%5 == 0 { ... }"$`
		return buzz
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		return strconv.Itoa(x)
	}
}

func Contains(s []int, x int) (attempts int, present bool) {
	for _, v := range s {
		if v == x {
			return attempts, true
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
			attempts++
		}
	}

	return attempts, false
}
