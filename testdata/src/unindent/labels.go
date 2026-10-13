package unindent

func LabelAfterReturn(x int) int {
	if x < 0 {
		if x < -10 {
			goto out
		}

		return -1
	out:
		x = -x
	} else {
		x++
	}

	return x
}

func EmptyLabelAfterReturn(x int) int {
	if x < 0 {
		if x < -10 {
			goto out
		}

		return -1
	out:
	} else {
		x++
	}

	return x
}

func LabelAfterReturnInCase(x, mode int) int {
	if x < 0 {
		switch mode {
		case 1:
			if x < -10 {
				goto out
			}

			return -1
		out:
			x = -x
		default:
			return 0
		}
	} else {
		x++
	}

	return x
}

func LabelAfterReturnBeforeElseIf(x int) int {
	if x < 0 {
		if x < -10 {
			goto out
		}

		return -1
	out:
		x = -x
	} else if x > 10 {
		return 10
	} else {
		x++
	}

	return x
}

func LabelBeforeReturn(x int) int {
	if x < 0 {
		if x < -10 {
			goto out
		}

		x = -x
	out:
		x--

		return x
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue" or a call that never returns. Remove the "else" wrapping the block of statements$`
		x++
	}

	return x
}
