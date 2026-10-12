package unindent

var errStop = errorf("stop")

func parse() (int, error) { return 0, nil }

func lookup() (int, bool) { return 0, true }

func InitRedeclaresParam(err error) error {
	if v, err := parse(); err != nil {
		return err
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "v, err := parse\(\)" before the "if", and remove the "else" wrapping the block of statements. Not fixed automatically: "err" is already declared in the enclosing block$`
		_ = v
	}

	return err
}

func InitRedeclaresResult() (err error) {
	if err := parse2(); err == errStop {
		return nil
	} else if err != nil { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "err := parse2\(\)" before the "if", and remove the "else", leaving "if err != nil { ... }". Not fixed automatically: "err" is already declared in the enclosing block$`
		return err
	}

	return nil
}

func InitHidesOuterUsedAfter(xs []int) int {
	n := 0

	for _, x := range xs {
		if n := x * 2; n > 10 {
			continue
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "n := x \* 2" before the "if", and remove the "else" wrapping the block of statements. Not fixed automatically: "n" would hide the "n" used after the "if"$`
			_ = n
		}

		n++
	}

	return n
}

func InitRedeclaredByLaterLink(m map[int]any, k int) (int, bool) {
	if v, ok := m[k]; !ok {
		return 0, false
	} else if v, ok := v.(int); !ok { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "v, ok := m\[k\]" before the "if", and remove the "else", leaving "if !ok { ... }"$`
		return 0, false
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "v, ok := v.\(int\)" before the "if", and remove the "else" wrapping the block of statements. Not fixed automatically: "ok" is also declared by an earlier "if" initialiser; "v" is also declared by an earlier "if" initialiser$`
		return v, true
	}
}

func EarlierLinkNotFixed(err error) (int, error) {
	if v, err := parse(); err != nil {
		return 0, err
	} else if v < 0 { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "v, err := parse\(\)" before the "if", and remove the "else", leaving "if v < 0 { ... }". Not fixed automatically: "err" is already declared in the enclosing block$`
		return 0, nil
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements. Not fixed automatically: an earlier "else" in the chain is not fixed automatically$`
		return v, nil
	}
}

func ElseRedeclaresHoisted() int {
	if v, ok := lookup(); !ok {
		return 0
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "v, ok := lookup\(\)" before the "if", and remove the "else" wrapping the block of statements. Not fixed automatically: "v" is also declared by an "if" initialiser$`
		v := v * 2
		return v
	}
}

func InitHoistedSafely(m map[int]int, k int) int {
	if v, ok := m[k]; !ok {
		return 0
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "v, ok := m\[k\]" before the "if", and remove the "else" wrapping the block of statements$`
		return v
	}
}

func parse2() error { return nil }

func NestedHoistMovedByOuterFix(ok bool) (err error) {
	defer func() { _ = err }()

	if ok {
		return nil
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements. Not fixed automatically: "err" is already declared in the enclosing block$`
		if err := parse2(); err == errStop {
			return nil
		} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "err := parse2\(\)" before the "if", and remove the "else" wrapping the block of statements$`
			return err
		}
	}
}

func SiblingsHoistSameName(a, b any) (string, string) {
	var first, second string

	if s, ok := a.(string); !ok {
		return "", ""
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "s, ok := a.\(string\)" before the "if", and remove the "else" wrapping the block of statements$`
		first = s
	}

	if s, ok := b.(string); !ok {
		return "", ""
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "s, ok := b.\(string\)" before the "if", and remove the "else" wrapping the block of statements. Not fixed automatically: "ok" is also moved into the enclosing block by an earlier fix; "s" is also moved into the enclosing block by an earlier fix$`
		second = s
	}

	return first, second
}

func SiblingElseDeclaresHoistedName(a, b any) string {
	if a == nil {
		return ""
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements$`
		s := "a"
		_ = s
	}

	if s, ok := b.(string); !ok {
		return ""
	} else { // want `^Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Move variable declaration "s, ok := b.\(string\)" before the "if", and remove the "else" wrapping the block of statements. Not fixed automatically: "s" is also moved into the enclosing block by an earlier fix$`
		return s
	}
}
