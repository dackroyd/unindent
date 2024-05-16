package unindent

func ContainsBreak(s []int, x int) (attempts int, present bool) {
	for _, v := range s {
		if v == x {
			present = true
			break
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
			attempts++
		}
	}

	return attempts, present
}

func ContainsContinue(s []int, x int) (attempts int, present bool) {
	for _, v := range s {
		if v != x {
			attempts++
			continue
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
			return attempts, true
		}
	}

	return attempts, false
}

func CountingContains(s []int, x int) (smaller, larger int, present bool) {
	for _, v := range s {
		if v > x {
			larger++
			continue
		} else if v < x { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else", leaving "if v < x { ... }"$`
			smaller++
			continue
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
			present = true
		}
	}

	return smaller, larger, present
}
