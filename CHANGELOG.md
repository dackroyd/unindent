# Changelog

All notable changes are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
versions follow [Semantic Versioning](https://semver.org/).

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
