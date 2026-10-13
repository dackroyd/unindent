# Checks

Each check reports a construct that indents code without need, explains the change in the message, and offers a fix that
`unindent -fix` applies.

A branch **exits** when every path through it ends in a `return`, `break`, `continue` or a call that never returns.
Paths are followed into nested statements: an `if` with an `else` exits when both branches exit, and a `switch` exits
when it has a `default`, every case exits, and no unlabeled `break` in its cases leaves it early, since that `break`
only leaves the `switch`. A `break` that leaves a loop, `switch` or `select` nested in a case doesn't count. A
`continue`, or a `break` to a label outside the `if`, is an exit. Statements after an exit only run when a `goto` jumps
to a label among them, so a branch with a label after its last exit doesn't exit: the path from the label carries on to
the end of the branch.

Calls that never return are:

- `panic`.
- The methods of `testing.T`, `B`, `F` and `TB` that stop the test: `Fatal`, `Fatalf`, `FailNow`, `Skip`, `Skipf` and
    `SkipNow`.
- Any function that the [`ctrlflow`](https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/ctrlflow) analysis finds
    cannot return, such as `os.Exit`, `log.Fatal` and `runtime.Goexit`, and functions in any package that always call
    one of them.

Some functions never return only in some builds. A function declared in a file with build constraints, a `//go:build`
line or a name such as `stub_linux.go`, may have a counterpart: a declaration of the same function in another file of
the package that the build leaves out. When it does, the function may return in another build, as may a function that
never returns only because it calls one like it. A call to either still counts as an exit, but the fix is withheld, as
[when no fix is offered](#when-no-fix-is-offered) describes. This is a heuristic: files left out of the build are not
type checked, so a counterpart counts even when it never returns either. A function without a counterpart is the same in
every build that can call it.

## Else after an exit

Reports an `else` that follows a branch that exits. The `else` is unnecessary: its block runs only when the branch did
not, so it can follow the `if` at the outer level.

```go
if err != nil {
	return err
} else { // Unnecessary "else": preceding conditions always end in a "return", "break", "continue" or a call that never returns. Remove the "else" wrapping the block of statements
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
} else if x == y { // Unnecessary "else": preceding conditions always end in a "return", "break", "continue" or a call that never returns. Remove the "else", leaving "if x == y { ... }"
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
- **The branch exits through a call that never returns only in some builds.** The function, or one it calls, is declared
    in a file with build constraints and again in a file that the build leaves out, so in another build it may return,
    and the `else` block would then run after it. The runtime's `racerelease`, for example, throws in builds without
    `-race`, where `race0.go` declares it as a stub, and returns in builds with it, where `race.go` declares it. The
    reason names the call: `"racerelease" never returns only in some builds`.

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

A finding without a fix can be fixed by hand, or, when the `else` is wanted, as it is for a call that returns in other
builds, suppressed. Where golangci-lint runs `unindent` as a custom linter named `unindent`, add `//nolint:unindent` to
the line the finding is reported on, the `else`.
