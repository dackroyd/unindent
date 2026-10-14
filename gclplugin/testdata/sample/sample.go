package sample

import (
	"errors"
	"strconv"

	"example.com/sample/fail"
)

func ElseAfterExit(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty")
	} else {
		return strconv.Atoi(s)
	}
}

func ExitInElse(d byte) int {
	v := 0
	if '0' <= d && d <= '9' {
		v = int(d - '0')
		v *= 2
	} else {
		fail.Exit("not a digit")
	}
	return v
}

func WrappingRest(xs []string) int {
	total := 0
	for _, x := range xs {
		if x != "" {
			n := len(x)
			total += n
			total *= 2
		}
	}
	return total
}

func AloneInIf(a, b bool, n int) int {
	if a {
		if b {
			n++
		}
	}
	if a && b {
		if n > 0 {
			n--
		}
	}
	return n
}

func Suppressed(s string) int {
	if s == "" {
		return 0
	} else { //nolint:unindent // Kept to show that nolint suppresses a finding.
		return len(s)
	}
}
