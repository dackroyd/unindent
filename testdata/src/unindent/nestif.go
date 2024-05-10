package unindent

import "strconv"

func NestIf(s string, x int) int {
	if s == "" {
		if x > 0 {
			return x
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return". Remove the "else" wrapping the block of statements$`
			return 0
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return". Remove the "else" wrapping the block of statements$`
		if l := len(s); l > 5 {
			return x
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return". Move variable declaration "l := len\(s\)" before the "if", and remove the "else" wrapping the block of statements$`
			return l
		}
	}
}

func NestIfDeep(s string, x int) int {
	if s == "" {
		if x > 0 {
			if l := len(s); l > 5 {
				return x
			} else { // want `^Unnecessary "else": preceding conditions always end in a "return". Move variable declaration "l := len\(s\)" before the "if", and remove the "else" wrapping the block of statements$`
				return l
			}
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return". Remove the "else" wrapping the block of statements$`
			return 0
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return". Remove the "else" wrapping the block of statements$`
		return 1
	}
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
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return". Remove the "else" wrapping the block of statements$`
			return buzz
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return". Remove the "else" wrapping the block of statements$`
		if x%5 == 0 {
			return buzz
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return". Remove the "else" wrapping the block of statements$`
			return strconv.Itoa(x)
		}
	}
}
