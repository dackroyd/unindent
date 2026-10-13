# Checks

Each check reports a construct that indents code without need, explains the change in the message, and offers a fix that
`unindent -fix` applies.

A branch **exits** when every path through it ends in a `return`, `break` or `continue`. Paths are followed into nested
statements: an `if` with an `else` exits when both branches exit, and a `switch` exits when it has a `default` and every
case exits. Inside a `switch`, an unlabeled `break` only leaves the `switch`, so it is not an exit; a `continue`, or a
`break` to a label outside the `if`, is.

## Else after an exit

Reports an `else` that follows a branch that exits. The `else` is unnecessary: its block runs only when the branch did
not, so it can follow the `if` at the outer level.

```go
if err != nil {
	return err
} else { // Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else" wrapping the block of statements
	process(v)
}
```

Fixed:

```go
if err != nil {
	return err
}

process(v)
```

The exit can be nested, as long as every path exits:

```go
if s == "" {
	switch {
	case x > 0:
		return x
	default:
		return 0
	}
} else { // reported
	return len(s)
}
```

### Initialisers

When the `else` block uses a variable that the `if` statement's initialiser declares, the fix moves the initialiser
before the `if` so the variable stays in scope. It moves as written, comments included:

```go
if y := f(); x > 1 {
	return y
} else { // ... Move variable declaration "y := f()" before the "if", and remove the "else" wrapping the block of statements
	return x * y
}
```

Fixed:

```go
y := f()
if x > 1 {
	return y
}

return x * y
```

## Else if after an exit

Reports an `else if` that follows a branch that exits, and every branch before it in the chain exits too. The `else` is
unnecessary, so the condition starts a new `if`. The rest of the chain is checked the same way.

```go
if x > y {
	return x
} else if x == y { // Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". Remove the "else", leaving "if x == y { ... }"
	return 0
} else { // reported: the branch before also exits
	return y
}
```

Fixed:

```go
if x > y {
	return x
}

if x == y {
	return 0
}

return y
```

An `else if` after a branch that does not exit is not reported: removing its `else` would change which branches run.

## When no fix is offered

A finding is still reported, without a fix, when applying the fix would not compile or would change behaviour. The
message ends with "Not fixed automatically:" and the reason, so the change can be made by hand, usually after renaming a
variable.

- **A name declared in the `else` block is already declared in the enclosing block.** Moving the declaration would
    redeclare it, or turn `:=` into an assignment to the existing variable.
- **A name declared in the `else` block would hide a variable used after the `if`.** Once moved, the later uses would
    refer to the new variable instead.
- **An initialiser that has to move declares a name that is already declared in the enclosing block**, for example a
    named result `err`, or that would hide a variable used after the `if`.
- **Two fixes move the same name into the enclosing block.** Two initialisers in the same chain, or in consecutive `if`
    statements, declare the same name, or an `else` block redeclares a name from a moved initialiser.
- **An earlier link in the chain is not fixed.** The rest of the chain uses what that link's initialiser declares, so it
    cannot move out from under it.

```go
func load() (err error) {
	if err := open(); err == io.EOF {
		return nil
	} else if err != nil { // ... Not fixed automatically: "err" is already declared in the enclosing block
		return err
	}

	return nil
}
```

```go
for _, x := range xs {
	if x < 0 {
		continue
	} else { // ... Not fixed automatically: "total" would hide the "total" used after the "if"
		total := x * 2
		log(total)
	}

	total += x
}
```
