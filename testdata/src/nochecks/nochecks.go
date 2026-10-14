package nochecks

func Chain(x int) int {
	if x < 0 {
		return -x
	} else if x > 10 {
		return 10
	} else {
		x++
	}

	return x
}

func ExitInElse(x int) int {
	if x > 0 {
		x++
	} else {
		return 0
	}

	return x
}

func WrappingRest(ok bool) {
	if ok {
		x := 1
		x++
		x++
		x++
		x++
	}
}
