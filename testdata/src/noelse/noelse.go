package noelse

func g(x int) int { return x }

func use(int) {}

func ElseNotReported(x int) int {
	if x < 0 {
		return -x
	} else {
		x++
	}

	return x
}

func ElseIf(x int) int {
	if x < 0 {
		return -x
	} else if x > 10 { // want `^Unnecessary "else": .* Remove the "else", leaving "if x > 10 { ... }"$`
		return 10
	}

	return x
}

func ChainEndNotReported(x int) int {
	if x < 0 {
		return -x
	} else if x > 10 { // want `^Unnecessary "else": .* Remove the "else", leaving "if x > 10 { ... }"$`
		return 10
	} else {
		x++
	}

	return x
}

func ChainHoistsInitialiser(x int) int {
	if v := g(x); v < 0 {
		return -x
	} else if v > 10 { // want `^Unnecessary "else": .* Move variable declaration "v := g\(x\)" before the "if", and remove the "else", leaving "if v > 10 { ... }"$`
		return 10
	} else {
		use(v)
	}

	return x
}

func ElseBlockNotMoved(x int) int {
	if x > 5 {
		return 5
	} else {
		y := x * 2
		use(y)
	}

	if y := g(x); y < 0 {
		return -x
	} else if y > 10 { // want `^Unnecessary "else": .* Move variable declaration "y := g\(x\)" before the "if", and remove the "else", leaving "if y > 10 { ... }"$`
		return 10
	}

	return x
}

func HoistedInitialiserStillTracked(x int) int {
	if y := g(x); y < 0 {
		return -x
	} else if y > 10 { // want `^Unnecessary "else": .* Move variable declaration "y := g\(x\)" before the "if", and remove the "else", leaving "if y > 10 { ... }"$`
		return 10
	}

	if y := g(x); y > 5 {
		return 5
	} else if y > 1 { // want `^Unnecessary "else": .* Not fixed automatically: "y" is also moved into the enclosing block by an earlier fix$`
		return 1
	}

	return x
}
