package unindent

func ElseRedeclaresEarlierName(ok bool) int {
	x := 1

	if ok {
		return x
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements. Not fixed automatically: "x" is already declared in the enclosing block$`
		x := 2
		_ = x
	}

	return x
}

func ElseRedeclaresResult(ok bool) (err error) {
	if ok {
		return nil
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements. Not fixed automatically: "err" is already declared in the enclosing block$`
		err := errorf("failed")
		return err
	}
}

func ElseRedeclaresLaterName(ok bool) int {
	if ok {
		return 0
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements. Not fixed automatically: "y" is already declared in the enclosing block$`
		y := 1
		_ = y
	}

	y := 2

	return y
}

func ElseHidesOuterUsedAfter(xs []int) int {
	total := 0

	for _, x := range xs {
		if x < 0 {
			continue
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements. Not fixed automatically: "total" would hide the "total" used after the "if"$`
			total := x * 2
			_ = total
		}

		total += x
	}

	return total
}

func ElseHidesBuiltinUsedAfter(xs []int) int {
	for i := range xs {
		if i == 0 {
			continue
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements. Not fixed automatically: "len" would hide the "len" used after the "if"$`
			len := i
			_ = len
		}

		_ = len(xs)
	}

	return 0
}

func ElseHidesOuterNotUsedAfter(xs []int) int {
	total := 0

	for _, x := range xs {
		if x < 0 {
			continue
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
			total := x * 2
			_ = total
		}
	}

	return total
}

func ElseNameRedeclaredInNestedScopeAfter(xs []int) int {
	for _, x := range xs {
		if x < 0 {
			continue
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
			v := x
			_ = v
		}

		for _, v := range xs {
			_ = v
		}
	}

	return 0
}

type errorString string

func (e errorString) Error() string { return string(e) }

func errorf(s string) error { return errorString(s) }
