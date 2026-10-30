# Changelog

All notable changes are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
versions follow [Semantic Versioning](https://semver.org/).

## [0.2.0](https://github.com/dackroyd/unindent/releases/tag/v0.2.0) - 2026-10-30

### Added

- An `else` block that always exits after a body that does not is reported, with a fix that inverts the condition so
  the `else` block exits first and the body follows the `if`, as revive's `early-return` suggests. The condition is
  negated as written, swapping `==` and `!=`, reversing `<` or `>=` for integers and strings, variable first, but not
  floating point numbers, where NaN makes `!(x < y)` and `x >= y` differ, and applying De Morgan's laws to `&&` and
  `||` when every operand negates that way. An `if` whose condition needs an error that is not `nil`, such as `if err
  != nil { log.Print(err) } else { process(v); return }`, is not reported, so the happy path never ends up under `err
  == nil`; an `else` block that only exits, as in a retry loop's `else { break }`, is. The new check is on by default,
  and `-exit-in-else=false` turns it off.
- An `if` without an `else` that ends a function or loop body and holds at least 3 statements is reported, with a fix
  that inverts the condition to `return` or `continue` early, so the block follows the `if` at the outer level, as
  gocritic's `nestingReduce` suggests for loops. A final `return` or `continue` in the block, which the fix makes
  redundant, is removed and not counted. A loop whose block ends in a `return`, `break` or `goto`, a search for one
  element, is left alone. The new check is on by default; `-if-wrapping-rest=false` turns it off, and
  `-if-wrapping-rest-min-statements` sets the number of statements.
- An `if` whose block holds only another `if`, neither with an `else`, is reported, with a fix that combines the two
  conditions with `&&` into one `if`, so the inner block moves out one level. `-if-alone-in-if-max-operands`, 2 by
  default, limits the operands of `&&` and `||` in the combined condition, and a chain of such `if` statements is
  combined from the top in one run. An inner `if` with an initialiser or a label, a constant outer condition, a
  comment on its own line between the two and a comment after the inner `if` are left alone. The new check is on by
  default; `-if-alone-in-if=false` turns it off.
- A golangci-lint module plugin: `golangci-lint custom` builds golangci-lint with `unindent` from a `.custom-gcl.yaml`
  that imports `github.com/dackroyd/unindent/gclplugin`. The linter `unindent` takes the flags as its settings, such
  as `exit-in-else: false`, rejects unknown ones, and `golangci-lint run --fix` applies the same fixes as `unindent
  -fix`. Tested with golangci-lint v2.14.0; see `docs/golangci-lint.md`.

### Fixed

- A file with many fixes is checked faster and with less memory, as each source file is read once per package rather
  than once for every fix.

## [0.1.0](https://github.com/dackroyd/unindent/releases/tag/v0.1.0) - 2026-10-29

### Added

- Calls that never return count as exits: `panic`, `t.Fatal`, `t.Skip` and the other methods of `testing.TB` that stop
  the test, and functions that always call `os.Exit`, `log.Fatal` or `runtime.Goexit`, in any package. A call to a
  function that never returns only in some builds, as a file with other build constraints declares it again, is
  reported without a fix: the race detector stubs in the runtime, for example.
- Every terminating statement in the Go specification counts as an exit: `goto`, a `for` loop without a condition, a
  `select` whose cases all exit, `select {}`, a labelled statement that exits, and a `switch` case ending in
  `fallthrough`. A `for`, `switch` or `select` that a `break` refers to, by label too, does not count. A fix is
  withheld when a `goto` would jump over a declaration it moves.
- Each check can be turned off with a flag, `-else-after-exit=false` or `-else-if-after-exit=false`, from the command
  line, `go vet -vettool` or the analyzer's `Flags`. Both checks stay on by default.

### Fixed

- Builds with Go 1.23 and later. Go 1.26 is now the minimum version.
- An unlabeled `break` inside a `switch` no longer counts as leaving the `if`. The fix moved the `else` block out of
  the `if`, so it ran when the `switch` broke out.
- No fix is offered when the `else` block declares a name that is already declared in the enclosing block, or that
  would hide a variable used after the `if`. The fix produced code that did not compile, or that silently used the
  wrong variable. The finding is still reported, with the reason.
- No fix is offered when an initialiser that has to move before the `if` declares a name already in scope, one that
  would hide a variable used after the `if`, or one that an earlier fix also moves into the same block. These fixes
  failed to compile in grpc-go, gopls, terraform, esbuild and nats-server, among others.
- Comments between `else {` and the first statement of the block are kept by the fix. They were deleted with the
  `else`.
- A `switch` with an unlabeled `break` before the end of a case no longer counts as an exit. The `break` leaves the
  `switch`, so the fix moved the `else` block where it also ran after the `switch`.
- An `else if` chain whose initialisers move before the `if` no longer takes time that doubles with every link. A
  chain of 20 took 19 seconds to check, and now takes 7 milliseconds.
- Comments inside an initialiser that the fix moves before the `if` are kept. They were deleted, as the initialiser
  was rebuilt from its syntax tree.
- A branch with a label after the statement that ends it, such as a `return`, no longer counts as an exit. A `goto`
  can jump to the label and carry on to the end of the branch, so the fix moved the `else` block where it also ran
  after that path.
- A fix that moves an initialiser before the `if` is withheld when the initialiser holds another fix, such as an
  `else` in a function literal, and is offered on the next run. The two fixes overlapped, so `unindent -fix` skipped
  one and exited with an error.
