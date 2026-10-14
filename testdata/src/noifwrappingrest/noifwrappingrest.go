package noifwrappingrest

func use(...any) {}

func WrappingNotReported(ok bool) {
	if ok {
		use(1)
		use(2)
		use(3)
		use(4)
		use(5)
	}
}

// ExitInElse is fixed, as no fix is offered inside its else block.
func ExitInElse(ok bool) error {
	if ok {
		use(1)
	} else { // want `^Exit in "else": .* Invert the condition, leaving "if !ok { ... }" followed by the block of statements$`
		go func() {
			if ok {
				use(1)
				use(2)
				use(3)
				use(4)
				use(5)
			}
		}()

		return nil
	}

	return nil
}
