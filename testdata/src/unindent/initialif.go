package unindent

func InitByIfOnly(x int) int {
	if y := 1; x > 1 {
		return y
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
		return x
	}
}

func InitByIfAndElse(x int) int {
	if y := 1; x > 1 {
		return y
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "y := 1" before the "if", and remove the "else" wrapping the block of statements$`
		return x * y
	}
}

func InitByIfAndElseMulti(x int) int {
	if y := 1; x > 1 {
		return y
	} else if x < 10 { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "y := 1" before the "if", and remove the "else", leaving "if x < 10 { ... }"$`
		return y - x
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
		return x * y
	}
}

func InitMultiByIfAndElse(x int) int {
	if y, z := 1, 2; x > 1 {
		return y
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "y, z := 1, 2" before the "if", and remove the "else" wrapping the block of statements$`
		return x * y * z
	}
}
