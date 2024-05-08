package unindent

import "strconv"

func FizBuzz(x int) string {
	fizz := "fizz"
	buzz := "buzz"

	if x%3 == 0 && x%5 == 0 {
		return fizz + buzz
	} else if x%3 == 0 { // want `^Unnecessary "else": preceding conditions always end in a "return". Remove the "else", leaving "if x%3 == 0 { ... }"$`
		return fizz
	} else if x%5 == 0 { // want `^Unnecessary "else": preceding conditions always end in a "return". Remove the "else", leaving "if x%5 == 0 { ... }"$`
		return buzz
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return". Remove the "else {}" wrapping the block of statements$`
		return strconv.Itoa(x)
	}
}
