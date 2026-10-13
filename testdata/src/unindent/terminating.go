package unindent

func work() bool { return false }

func GotoExit(x int) int {
	if x < 0 {
		goto fail
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		x++
	}

	return x

fail:
	return -1
}

func GotoBackwards(x int) int {
retry:
	if x < 0 {
		x++
		goto retry
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		y := x * 2
		x = y
	}

	return x
}

func GotoOverMovedDeclaration(x int) int {
	if x < 0 {
		goto fail
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements. Not fixed automatically: "goto fail" would jump over the declaration of "y"$`
		y := x * 2
		x = y
	}

	return x

fail:
	return -1
}

func GotoBeforeChainOverMovedDeclaration(x int) int {
	if x > 10 {
		goto done
	}

	if x < 0 {
		return -1
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements. Not fixed automatically: "goto done" would jump over the declaration of "y"$`
		y := x * 2
		x = y
	}

done:
	return x
}

func GotoInElseOverMovedDeclaration(x int) int {
	if x < 0 {
		return -1
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements. Not fixed automatically: "goto done" would jump over the declaration of "y"$`
		if x > 10 {
			goto done
		}

		y := x * 2
		x = y
	}

done:
	return x
}

func GotoInElseAfterMovedDeclaration(x int) int {
	if x < 0 {
		return -1
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		y := x * 2
		if y > 10 {
			goto done
		}

		x = y
	}

done:
	return x
}

func GotoOverHoistedInitialiser(x int) int {
	if x > 10 {
		goto done
	}

	if y := x * 2; y < 0 {
		return -1
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Move variable declaration "y := x \* 2" before the "if", and remove the "else" wrapping the block of statements. Not fixed automatically: "goto done" would jump over the declaration of "y"$`
		x = y
	}

done:
	return x
}

func GotoAfterHoistedInitialiser(x int) int {
	if y := x * 2; y < 0 {
		goto done
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Move variable declaration "y := x \* 2" before the "if", and remove the "else" wrapping the block of statements$`
		x = y
	}

done:
	return x
}

func GotoOverMovedConstant(x int) int {
	if x < 0 {
		goto fail
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		const two = 2
		x *= two
	}

	return x

fail:
	return -1
}

func GotoToLabelInElse(x int) int {
	if x < 0 {
		return -1
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
	again:
		x--
		if x > 10 {
			goto again
		}
	}

	return x
}

func LabeledReturn(x int) int {
	if x < 0 {
		if x < -10 {
			goto out
		}

		x = -x
	out:
		return x
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		x++
	}

	return x
}

func ForeverLoop(x int) int {
	if x < 0 {
		for {
			work()
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		x++
	}

	return x
}

func ForeverLoopWithInitAndPost(x int) int {
	if x < 0 {
		for i := 0; ; i++ {
			x += i
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		x++
	}

	return x
}

func ForeverLoopWithNestedBreaks(x int, xs []int, ch chan int) int {
	if x < 0 {
		for {
			for range xs {
				break
			}

			switch {
			case work():
				break
			}

			select {
			case <-ch:
				break
			default:
			}

			func() {
				for {
					break
				}
			}()
		}
	} else if x > 10 { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else", leaving "if x > 10 { ... }"$`
		x--
	}

	return x
}

func ForeverLoopBreakingOuterLoop(xs []int) int {
	n := 0

outer:
	for _, x := range xs {
		if x < 0 {
			for {
				if work() {
					break outer
				}
			}
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
			n += x
		}
	}

	return n
}

func ForeverLoopContinuing(x int) int {
	if x < 0 {
	loop:
		for {
			if work() {
				continue loop
			}
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		x++
	}

	return x
}

func LoopWithCondition(x int) int {
	if x < 0 {
		for x < 10 {
			x++
		}
	} else {
		x--
	}

	return x
}

func LoopOverRange(xs []int) int {
	n := 0

	if len(xs) > 0 {
		for range xs {
			n++
		}
	} else {
		n--
	}

	return n
}

func LoopWithBreak(x int) int {
	if x < 0 {
		for {
			if work() {
				break
			}
		}
	} else {
		x++
	}

	return x
}

func LabeledLoopWithNestedBreak(x int, xs []int) int {
	if x < 0 {
	loop:
		for {
			for range xs {
				if work() {
					break loop
				}
			}
		}
	} else {
		x++
	}

	return x
}

func LoopLeftByGoto(x int) int {
	if x < 0 {
		for {
			if work() {
				goto out
			}
		}
	out:
	} else {
		x++
	}

	return x
}

func EmptySelect(x int) int {
	if x < 0 {
		select {}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		x++
	}

	return x
}

func SelectEveryClauseExits(xs []int, ch chan int) int {
	n := 0

	for _, x := range xs {
		if x < 0 {
			select {
			case v := <-ch:
				return v
			case ch <- x:
				continue
			default:
				panic("blocked")
			}
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
			n += x
		}
	}

	return n
}

func SelectClauseFallsThrough(x int, ch chan int) int {
	if x < 0 {
		select {
		case v := <-ch:
			return v
		default:
			x++
		}
	} else {
		x--
	}

	return x
}

func SelectWithBreak(x int, ch chan int) int {
	if x < 0 {
		select {
		case v := <-ch:
			if v > 0 {
				break
			}

			return v
		}
	} else {
		x--
	}

	return x
}

func LabeledSelectWithNestedBreak(x int, xs []int, ch chan int) int {
	if x < 0 {
	wait:
		select {
		case <-ch:
			for range xs {
				break wait
			}

			return 0
		}
	} else {
		x--
	}

	return x
}

func LabeledSwitchExits(x, mode int) int {
	if x < 0 {
	modes:
		switch mode {
		case 1:
			if work() {
				goto modes
			}

			return 1
		default:
			return 0
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		x--
	}

	return x
}

func LabeledSwitchWithBreak(x, mode int, xs []int) int {
	if x < 0 {
	modes:
		switch mode {
		case 1:
			for range xs {
				break modes
			}

			return 1
		default:
			return 0
		}
	} else {
		x--
	}

	return x
}

func SwitchFallthrough(x, mode int) int {
	if x < 0 {
		switch mode {
		case 1:
			x++
			fallthrough
		default:
			return x
		}
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements$`
		x--
	}

	return x
}

func SwitchFallthroughIntoFallingCase(x, mode int) int {
	if x < 0 {
		switch mode {
		case 1:
			fallthrough
		case 2:
			x++
		default:
			return x
		}
	} else {
		x--
	}

	return x
}
