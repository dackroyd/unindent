package noexitinelse

func use(...any) {}

func ExitInElseNotReported(v int, err error) error {
	if err == nil {
		use(v)
	} else {
		return err
	}

	return nil
}

func ElseAfterExit(x int) int {
	if x < 0 {
		return -x
	} else { // want `^Unnecessary "else": .* Remove the "else" wrapping the block of statements$`
		x++
	}

	return x
}

// OuterElseMoves fixes the outer else, as the inner if keeps its body when the check is off, so nothing moves y into
// the else block.
func OuterElseMoves(x int) int {
	y := 0

	if x > 10 {
		return 1
	} else { // want `^Unnecessary "else": .* Remove the "else" wrapping the block of statements$`
		if x > 5 {
			y := 2
			use(y)
		} else {
			return 3
		}
	}

	return y
}
