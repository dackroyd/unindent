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
