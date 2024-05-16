package unindent

func NestSwitchReturns(s string, x int) int {
	if s == "" {
		switch {
		case x > 0:
			return x
		default:
			return 0
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
		l := len(s)
		switch {
		case l > 5:
			return x
		default:
			return l
		}
	}
}

func NestSwitchIncomplete(s string, x int) int {
	if s == "" {
		switch {
		case x > 0:
			return x
		}
	} else {
		l := len(s)
		switch {
		case l > 5:
			return x
		}
	}

	return 0
}

func NestSwitchAssign(s string, x int) int {
	var value int

	if s == "" {
		switch {
		case x > 0:
			value = x
		default:
			return 0
		}
	} else {
		l := len(s)
		switch {
		case l > 5:
			return x
		default:
			return l
		}
	}

	return value
}

func NestTypeswitchReturns(v any, x int) int {
	if x > 0 {
		switch t := v.(type) {
		case string:
			return -len(t)
		case int:
			return x * 2
		default:
			return 0
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
		switch t := v.(type) {
		case string:
			return len(t)
		case int:
			return t
		default:
			return 1
		}
	}
}

func NestTypeswitchIncomplete(v any, x int) int {
	if x > 0 {
		switch t := v.(type) {
		case string:
			return -len(t)
		case int:
			return x * 2
		}
	} else {
		switch t := v.(type) {
		case string:
			return len(t)
		case int:
			return t
		}
	}

	return 1
}

func NestTypeswitchAssign(v any, x int) int {
	var value int

	if x > 0 {
		switch t := v.(type) {
		case string:
			value = -len(t)
		case int:
			return x * 2
		default:
			return 0
		}
	} else {
		switch t := v.(type) {
		case string:
			return len(t)
		case int:
			return t
		default:
			return 1
		}
	}

	return value
}
