package unindent

func SwitchBreakLeavesSwitch(xs []int, mode int) int {
	n := 0

	for _, x := range xs {
		if x > 0 {
			switch mode {
			case 1:
				break
			default:
				break
			}
		} else {
			n++
		}

		n += x
	}

	return n
}

func SwitchContinueLeavesIf(xs []int, mode int) int {
	n := 0

	for _, x := range xs {
		if x > 0 {
			switch mode {
			case 1:
				continue
			default:
				continue
			}
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue" or a call that never returns. Remove the "else" wrapping the block of statements$`
			n++
		}

		n += x
	}

	return n
}

func SwitchLabeledBreakLeavesIf(xs []int, mode int) int {
	n := 0

loop:
	for _, x := range xs {
		if x > 0 {
			switch mode {
			case 1:
				break loop
			default:
				break loop
			}
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue" or a call that never returns. Remove the "else" wrapping the block of statements$`
			n++
		}

		n += x
	}

	return n
}

func SwitchNestedIfBreakLeavesSwitch(xs []int, mode int) int {
	n := 0

	for _, x := range xs {
		if x > 0 {
			switch mode {
			case 1:
				if x > 1 {
					break
				} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue" or a call that never returns. Remove the "else" wrapping the block of statements$`
					return n
				}
			default:
				return n
			}
		} else {
			n++
		}

		n += x
	}

	return n
}

func BreakInCaseLeavesIf(x, mode int) int {
	n := 0

	switch mode {
	case 1:
		if x > 0 {
			break
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue" or a call that never returns. Remove the "else" wrapping the block of statements$`
			n++
		}

		n += x
	}

	return n
}

func SwitchEarlyBreakLeavesSwitch(xs []int, mode int) int {
	n := 0

	for _, x := range xs {
		if x > 0 {
			switch mode {
			case 1:
				if x > 1 {
					break
				}

				return n
			default:
				return n
			}
		} else {
			n++
		}

		n += x
	}

	return n
}

func TypeSwitchEarlyBreakLeavesSwitch(vs []any) int {
	n := 0

	for _, v := range vs {
		if v != nil {
			switch v.(type) {
			case int:
				for range n {
					break
				}

				if n > 1 {
					break
				}

				return n
			default:
				return n
			}
		} else {
			n++
		}

		n++
	}

	return n
}

func SwitchInnerLoopBreakLeavesIf(xs []int, mode int) int {
	n := 0

	for _, x := range xs {
		if x > 0 {
			switch mode {
			case 1:
				for range x {
					break
				}

				return n
			default:
				return n
			}
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue" or a call that never returns. Remove the "else" wrapping the block of statements$`
			n++
		}

		n += x
	}

	return n
}
