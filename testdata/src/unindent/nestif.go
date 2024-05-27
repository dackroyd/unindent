package unindent

import "strconv"

func NestIf(s string, x int) int {
	if s == "" {
		if x > 0 {
			return x
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
			return 0
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
		if l := len(s); l > 5 {
			return x
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "l := len\(s\)" before the "if", and remove the "else" wrapping the block of statements$`
			return l
		}
	}
}

func NestIfFallout(s string, x int) int {
	if s == "" {
		if x > 0 {
			return x
		}
	} else if x < 0 {
		if l := len(s); l > 5 {
			return l
		}
	} else {
		if l := len(s); l > 5 {
			return x
		}
	}

	return 0
}

func NestIfElseAssign(s string, x int) int {
	var y int

	if s == "" {
		if x > 0 {
			return x
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
			y = 1
		}
	} else if x < 0 {
		if l := len(s); l > 5 {
			return l
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
			y = 2
		}
	} else {
		if l := len(s); l > 5 {
			return x
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
			y = 3
		}
	}

	return y
}

func NestIfDeep(s string, x int) int {
	if s == "" {
		if x > 0 {
			if l := len(s); l > 5 {
				return x
			} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "l := len\(s\)" before the "if", and remove the "else" wrapping the block of statements$`
				return l
			}
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
			return 0
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
		return 1
	}
}

func NestIfElseAssignDeep(s string, x int) int {
	var y int

	if s == "" {
		if x > 0 {
			if l := len(s); l > 5 {
				return x
			} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "l := len\(s\)" before the "if", and remove the "else" wrapping the block of statements$`
				y = l
			}
		} else {
			return 0
		}
	} else {
		return 1
	}

	return y
}

func NestIfOk(s string, x int) int {
	if s == "" {
		if x > 0 {
			return x
		}

		return 0
	}

	l := len(s)
	if l > 5 {
		return x
	}

	return l
}

func FizBuzzNest(x int) string {
	fizz := "fizz"
	buzz := "buzz"

	if x%3 == 0 {
		if x%5 == 0 {
			return fizz + buzz
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
			return buzz
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
		if x%5 == 0 {
			return buzz
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
			return strconv.Itoa(x)
		}
	}
}
