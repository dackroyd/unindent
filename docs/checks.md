# Checks

Each check reports a construct that indents code without need, explains the change in the message, and offers a fix that
`unindent -fix` applies.

A branch **exits** when every path through it ends in a `return`, `break` or `continue`. Paths are followed into nested
statements: an `if` with an `else` exits when both branches exit, and a `switch` exits when it has a `default` and every
case exits.

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
before the `if` so the variable stays in scope:

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
