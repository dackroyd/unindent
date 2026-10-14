# Checks

Each check reports a construct that indents code without need, explains the change in the message, and offers a fix that
`unindent -fix` applies. Every check is on by default, and its flag turns it off: `-else-after-exit=false`,
`-else-if-after-exit=false`, `-exit-in-else=false` or `-if-wrapping-rest=false`.

A branch **exits** when every path through it ends in a `return`, `break`, `continue`, `goto` or a call that never
returns, or in a loop or `select` that never ends. These are the Go specification's
[terminating statements](https://go.dev/ref/spec#Terminating_statements), plus `break`, `continue` and calls that never
return, which leave the `if` as surely. Paths are followed into nested statements:

- An `if` with an `else` exits when both branches exit.
- A `switch` exits when it has a `default` and every case exits or ends in a `fallthrough`.
- A `for` loop exits when it has no condition and no `range` clause.
- A `select` exits when every case exits. A `select {}` blocks forever, so it exits too.
- A labelled statement exits when the statement it labels does.

A `break` that refers to a `for`, `switch` or `select` in the branch, unlabelled in its body or to its label from
anywhere inside it, leaves that statement instead, so the statement doesn't exit. A `break` that leaves a loop, `switch`
or `select` nested inside it doesn't count. A `continue`, or a `break` to a label outside the `if`, is an exit.
Statements after an exit only run when a `goto` jumps to a label among them, so a label after the exit ends the branch's
last path there: the branch exits only if what follows the label does.

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

Flag: `-else-after-exit`.

Reports an `else` that follows a branch that exits. The `else` is unnecessary: its block runs only when the branch did
not, so it can follow the `if` at the outer level.

```go
if err != nil {
	return err
} else { // Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else" wrapping the block of statements
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

Flag: `-else-if-after-exit`.

Reports an `else if` that follows a branch that exits, and every branch before it in the chain exits too. The `else` is
unnecessary, so the condition starts a new `if`. The rest of the chain is checked the same way.

```go
if x > y {
	return x
} else if x == y { // Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends. Remove the "else", leaving "if x == y { ... }"
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

The two `else` checks are independent. With this one off, the `else` at the end of such a chain is still reported, and
its fix moves the block out of the chain, leaving the `else if` links as they are. With
[Else after an exit](#else-after-an-exit) off, the `else if` links are still reported, and the final `else` stays.

## Exit in an else

Flag: `-exit-in-else`.

Reports an `else` block that exits after a body that does not. Inverting the condition lets the `else` block exit first,
and the body, which only ran when the `else` block did not, follows the `if` at the outer level. This is revive's
`early-return`, with exits followed through nested statements and calls as for the other checks.

```go
v, err := load()
if err == nil {
	process(v)
} else { // Exit in "else": the "else" block always ends in a "return", "break", "continue", "goto", a call that never returns or a loop or select that never ends, and the "if" block does not. Invert the condition, leaving "if err != nil { ... }" followed by the block of statements
	return err
}

return nil
```

Fixed:

```go
v, err := load()
if err != nil {
	return err
}

process(v)

return nil
```

Moving the body out keeps behaviour wherever the `if` is: the body flowed into the statements after the `if`, and still
does. It doesn't need to be the last statement of its block, and the body can be empty: `if ok {} else { return }`
becomes `if !ok { return }`.

The condition is negated so it reads naturally, keeping the rest of it as written:

- A negation is removed: `!ok` becomes `ok`, and `!(a && b)` becomes `a && b`.
- `==` and `!=` swap.
- `<`, `<=`, `>` and `>=` are reversed when both operands are integers or strings, so `n > 0` becomes `n <= 0`. A
    constant on the left only moves to the right, so the comparison reads variable first: `0 < n` becomes `n <= 0`.
    Floating point comparisons are wrapped instead, `!(x < y)`, as `x >= y` is false when either is NaN, while
    `!(x < y)` is true. So are comparisons of type parameters, which may be floating point.
- `&&` and `||` swap by De Morgan's laws when every operand negates in one of these ways or is an identifier, selector,
    call or index expression, which gets a `!`: `ok && n > 0` becomes `!ok || n <= 0`, `a && (b || c)` becomes
    `!a || (!b && !c)`, and the range check `'0' <= c && c <= '9'` becomes `c < '0' || c > '9'`. An `&&` that becomes an
    `||` inside an `&&` is parenthesised, so `a || b && c` becomes `!a && (!b || !c)`. When an operand, at any depth,
    would need wrapping, such as a floating point comparison, the whole condition is wrapped instead: `!(ok && x < y)`.
- Anything else gets a `!`: `!ok`, `!s.Valid()`, `!*p`.

An `if` whose condition only holds while an error is not `nil` is not reported when its `else` block holds more than its
exit, as inverting it would nest the happy path under `err == nil`:

```go
if err != nil {
	log.Print(err)
} else { // not reported: the fix would leave "if err == nil { ... }" followed by the logging
	process(v)
	return nil
}
```

That covers `err != nil`, `nil != err`, `!(err == nil)` and an `||` with such an operand, such as `err != nil || retry`,
for any value whose type implements `error`. `err != nil && retry` is still reported, as its `else` block also runs with
an error when `retry` is false. An `else` block that only exits is reported, as its inverted `if` is an early exit on
success rather than a nested happy path: in a retry loop, `if err != nil { wait() } else { break }` becomes
`if err == nil { break }` followed by `wait()`, and the test idiom `if err != nil { check(err) } else { t.Fatal(…) }`
becomes `if err == nil { t.Fatal(…) }`.

Only an `if` that starts a chain is reported. The body of an `else if` would run after the branches before it once it
moved out of the chain. When those branches exit, [Else if after an exit](#else-if-after-an-exit) reports the `else if`
first, and once it starts its own `if`, the next run reports its `else`.

When the body uses a variable that the initialiser declares, the fix moves the initialiser before the `if`. Otherwise
the initialiser stays:

```go
if v, err := load(); err == nil {
	process(v)
} else { // ... Move variable declaration "v, err := load()" before the "if", and invert the condition, leaving "if err != nil { ... }" followed by the block of statements
	return err
}
```

Fixed:

```go
v, err := load()
if err != nil {
	return err
}

process(v)
```

The fix moves the `else` block, comments included, into the `if`. When a fix inside that block, or in the condition, is
offered too, this one is withheld until it is applied, so the fixes of one run never overlap: run `unindent -fix` again
to apply it.

The check is independent of the other two. With it off, an `else` block that exits after a body that does not is not
reported, and the other checks' fixes no longer allow for the body it would move. With
[Else if after an exit](#else-if-after-an-exit) off, the `else` of an `else if` is still not reported, as the chain
stays.

## If wrapping the rest

Flags: `-if-wrapping-rest`, and `-if-wrapping-rest-min-statements`, 3 by default.

Reports an `if` without an `else` that is the last statement of a function or loop body, when its block holds at least 3
statements. Nothing runs after the block, so the inverted condition can leave the function with `return`, or go on to
the next iteration with `continue`, and the block follows the `if` at the outer level.

```go
for _, item := range items {
	if item.Ready() { // If wrapping the rest of the loop: the loop body ends in an "if" without an "else", whose block holds 3 statements. Invert the condition, leaving "if !item.Ready() { continue }" followed by the block of statements
		v := item.Value()
		store(v)
		count++
	}
}
```

Fixed:

```go
for _, item := range items {
	if !item.Ready() {
		continue
	}

	v := item.Value()
	store(v)
	count++
}
```

Falling off the end of the body does what the new exit does, so behaviour is the same:

- In a function, only one without results can end in an `if` without an `else`, as a function with results needs a
    terminating statement, so a bare `return` is always right. A function literal is checked against its own signature,
    wherever it is. An `if` that returns a value before a final `return` has the `else` implied, which this check leaves
    alone.
- In a `for` or `range` loop, the `if` must be the last statement of the loop's own body, so the unlabelled `continue`
    goes on to the next iteration of that loop, running its post statement, as falling off the end did. Labelled loops
    are reported too.
- `defer` statements keep their order and still run when the function returns. Labels and `goto` statements in the block
    move with it and keep their targets, and nothing follows the `if` for a `goto` to jump over.

An `if` at the end of a `switch` or `select` case, or of a plain block, is not reported: leaving them early takes a
`break`, which reads less clearly than the nesting it removes. Nor is a labelled `if`, or, in a loop, an `if` whose
block ends in a `return`, `break` or `goto`, whatever its size. Such a loop searches for one element and acts on it
once, and the inverted `if` would hide that: `if !found { continue }` makes the block look like it runs for every
element, with the one-off exit at the end of the loop body.

The threshold counts the statements directly in the block, however long they are: a nested loop or `switch` counts once,
and a statement split over several lines counts once. A final `return` in a function, or `continue` to the loop, does
not count, as falling off the end of the body does the same: the fix removes it. Set `-if-wrapping-rest-min-statements`
to report shorter blocks, down to 1, or only longer ones: 5 matches gocritic's `nestingReduce`. Below 3, most reports
are short filters inside loops, where the new `if` adds as many lines as the nesting it removes.

The condition is negated as for [Exit in an else](#exit-in-an-else). Unlike that check, this one reports a block under
`if err != nil`: the fix leaves `if err == nil { return }`, which holds only the exit, and the error handling follows,
so no happy path ends up under `err == nil`. When the block uses a variable that the initialiser declares, the fix moves
the initialiser before the `if`; otherwise it stays, as only the condition uses it:

```go
func handle(k string) {
	if v, ok := lookup(k); ok { // ... Move variable declaration "v, ok := lookup(k)" before the "if", and invert the condition, leaving "if !ok { return }" followed by the block of statements
		// ...
	}
}
```

Fixed:

```go
func handle(k string) {
	v, ok := lookup(k)
	if !ok {
		return
	}

	// ...
}
```

The fix only replaces the block's braces, so fixes inside it apply in the same run, and comments stay where they are. A
comment after the `{` moves with the block, as it describes the case the block handles. The fixed code is reported by no
check, so the checks never undo each other's fixes. When the block ends in another `if` wrapping the rest, as in
`if a { …; if b { … } }`, that `if` ends the function once the outer one is fixed, and the next run reports it.

The check is independent of the others. With it off, an `if` wrapping the rest of a function or loop is not reported,
and the other checks' fixes are offered as if it were not there.

## Limitations

- A happy path nested under `if err == nil` without an `else`, `if err == nil { process(v); return nil }` followed by
    the error handling, is not reported. Turning it around would move the statements after the `if` into it, which is a
    refactor rather than a change of indentation. When the `else` holds the exit instead,
    `if err == nil { process(v) } else { return err }`, [Exit in an else](#exit-in-an-else) reports it.
- A search loop, whose last `if` acts on the element it finds and leaves the loop with a `return`, `break` or `goto`, is
    not reported by [If wrapping the rest](#if-wrapping-the-rest), however long the block is. Splitting the search from
    the action, finding the element in the loop and acting on it after, is a refactor rather than a mechanical fix.

## When no fix is offered

A finding is still reported, without a fix, when applying the fix would not compile or would change behaviour. The
message ends with "Not fixed automatically:" and the reason, so the change can be made by hand, usually after renaming a
variable.

- **A name declared in the `else` block is already declared in the enclosing block.** Moving the declaration would
    redeclare it, or turn `:=` into an assignment to the existing variable. For an exit in an `else` or an `if` wrapping
    the rest, the same applies to the names the body declares, as the body is what moves. A function's parameters are
    declared in its body's block.
- **A name declared in the `else` block would hide a variable used after the `if`.** Once moved, the later uses would
    refer to the new variable instead.
- **An initialiser that has to move declares a name that is already declared in the enclosing block**, for example a
    named result `err`, or that would hide a variable used after the `if`.
- **Two fixes move the same name into the enclosing block.** Two initialisers in the same chain, or in consecutive `if`
    statements, declare the same name, or an `else` block, or for an exit in an `else` or an `if` wrapping the rest the
    body, redeclares a name from a moved initialiser.
- **An earlier link in the chain is not fixed.** The rest of the chain uses what that link's initialiser declares, so it
    cannot move out from under it.
- **An initialiser in the chain stays, as the Else if after an exit check is off.** The final `else` uses a variable
    declared by the initialiser of an `if` in the chain, which can only move before the chain once every `else if` is
    removed: `"v" is declared by an initialiser that stays in the chain, as the else-if-after-exit check is off`.
- **The branch exits through a call that never returns only in some builds.** The function, or one it calls, is declared
    in a file with build constraints and again in a file that the build leaves out, so in another build it may return,
    and the `else` block would then run after it. The runtime's `racerelease`, for example, throws in builds without
    `-race`, where `race0.go` declares it as a stub, and returns in builds with it, where `race.go` declares it. The
    reason names the call: `"racerelease" never returns only in some builds`. For an exit in an `else`, the same goes
    for the `else` block: in a build where the call returns, the body would run after it.
- **A `goto` would jump over a moved declaration.** A `goto` before the `if`, or inside it, to a label after the `if`
    would jump over a variable that the fix moves out of the `else` block or the initialiser, which Go does not allow:
    `"goto fail" would jump over the declaration of "v"`. For an exit in an `else`, the variables are those the body
    declares, and a `goto` in the `else` block counts too, as the block moves before the body. Moving a block cannot
    break a `goto` to a label inside it: such a `goto` must be inside the block too, and the fix keeps the block's
    statements in order with nothing between them.
- **Another fix inside the `if` statement has to be applied first.** An initialiser that has to move holds another fix,
    such as an `else` in a function literal, or so does the `else` block or condition of an exit in an `else`, for
    example an `if` wrapping the rest of a function literal in the `else` block: `unindent -fix` applies that one, and a
    second run applies this one.

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
