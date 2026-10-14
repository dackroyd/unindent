package alonemax3

func use(...any) {}

// Parentheses go around an "||" that becomes an operand of "&&", once the operand limit allows three.
func Parentheses(a, b, c bool, x, y int) {
	if a { // want `leaving "if a && \(b \|\| c\) { ... }"`
		if b || c {
			use()
		}
	}

	if a || b { // want `leaving "if \(a \|\| b\) && c { ... }"`
		if c {
			use()
		}
	}

	if a && b { // want `leaving "if a && b && x > y { ... }"`
		if x > y {
			use()
		}
	}

	if a || // want `leaving "if \(a \|\| b\) && c { ... }"`
		b {
		if c {
			use()
		}
	}

	if a || b {
		if c || x > y {
			use()
		}
	}

	use()
}

// Chain combines three levels in one "if", and the fourth on its own.
func Chain(a, b, c, d bool) {
	if a { // want `leaving "if a && b && c { ... }"`
		if b {
			if c {
				if d {
					use()
				}
			}
		}
	}

	if a { // want `leaving "if a && \(b \|\| c\) { ... }"`
		if b || c {
			if d {
				use()
			}
		}
	}

	use()
}
