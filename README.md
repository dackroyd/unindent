# unindent

[![CI](https://github.com/dackroyd/unindent/actions/workflows/ci.yaml/badge.svg)](https://github.com/dackroyd/unindent/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/dackroyd/unindent.svg)](https://pkg.go.dev/github.com/dackroyd/unindent)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

`unindent` finds Go code that is nested deeper than it needs to be, and fixes it so the normal path runs down the left
edge of the function.

```go
v, err := load()
if err != nil {
	return err
} else { // Unnecessary "else": preceding conditions always end in a "return", "break", "continue", "goto", ...
	process(v)
}
```

`unindent -fix` rewrites it as:

```go
v, err := load()
if err != nil {
	return err
}

process(v)
```

## Why

The Go community's guidance on this is consistent: deal with the error or special case first, exit, and keep the normal
path unindented.

- [Go Code Review Comments: Indent Error Flow](https://go.dev/wiki/CodeReviewComments#indent-error-flow)
- [Effective Go: If](https://go.dev/doc/effective_go#if): "the unnecessary `else` is omitted"
- [Google Go Style Decisions: Indent error flow](https://google.github.io/styleguide/go/decisions#indent-error-flow)
- [Code Health: Reduce Nesting, Reduce Complexity](https://testing.googleblog.com/2017/06/code-health-reduce-nesting-reduce.html)
- [Maël Valais: Go happy line of sight](https://maelvls.dev/go-happy-line-of-sight/)

Existing linters enforce only part of it. revive's `indent-error-flow` and `superfluous-else` look at the last statement
of the `if` body only, gocritic's `nestingReduce` looks at loops only, and `nestif` reports complexity without saying
what to change. `unindent` fills the gaps:

- **Branches that exit in every path.** An `if` whose body ends in an `if`/`else` or `switch` that returns in every
    case, or in a loop or `select` that never ends, still makes its `else` unnecessary. revive does not follow those
    paths.
- **Calls that never return.** `t.Fatal`, `t.Skip` and your own helpers that always call `os.Exit`, `log.Fatal` or
    `panic` end a branch as surely as a `return`. revive only knows `panic`, `os.Exit` and the `log` package's `Fatal`
    and `Panic` functions.
- **`else if` chains.** When a branch exits, the `else if` after it can start a new `if`. revive does not report chains.
- **`select` cases.** revive skips `if` statements directly inside a `select` case.
- **Exits in an `else`.** Like revive's `early-return`, `unindent` reports an `else` block that exits after a body that
    does not, and inverts the condition so the `else` block exits first. It follows exits through nested statements and
    calls here too, doesn't turn `x < y` into `x >= y` for floating point numbers, where NaN makes them differ, and
    never nests the happy path under `err == nil`.
- **An `if` wrapping the rest of a function or loop.** When a function or loop body ends in an `if` holding 3 statements
    or more, other than a loop's search for one element, `unindent` inverts the condition to `return` or `continue`
    early, as gocritic's `nestingReduce` suggests for loops whose body is only that `if` and 5 statements or more. It
    reports functions and function literals as well, and an `if` after other statements.
- **An `if` alone in an `if`.** When an `if` block holds only another `if`, and neither has an `else`, `unindent`
    combines the two conditions with `&&`, up to a limit on their operands. revive, gocritic and staticcheck have no
    such rule.
- **Fixes.** Every report comes with a fix that `-fix` applies.

On five large Go codebases, `unindent` reported 1209 cases that revive's rules did not, and all but 6 of the 306 that
revive reported: 5 are an `else if` that `unindent` leaves alone or reports once its chain is split, and 1 would nest
the happy path under `err == nil`. [Comparison with other linters](docs/comparison.md) has the details, and
[Projects](docs/projects.md) what `unindent` finds in each codebase and what its fixes do.

## Quick start

```sh
go install github.com/dackroyd/unindent/cmd/unindent@latest
unindent ./...
```

Requires Go 1.26 or later. Each [release](https://github.com/dackroyd/unindent/releases) also has binaries for Linux,
macOS and Windows, with checksums and build provenance that `gh attestation verify <archive> --repo dackroyd/unindent`
checks.

## Usage

```sh
unindent ./...             # report
unindent -fix ./...        # apply the suggested fixes
unindent -fix -diff ./...  # show the fixes as a diff without applying them
unindent -json ./...       # report as JSON
go vet -vettool=$(command -v unindent) ./...
```

`unindent` exits with status 3 when it reports a finding, 1 when it cannot load the packages, and 0 otherwise. Test
files are checked too; pass `-test=false` to skip them.

Every check is on by default, and each has a flag that turns it off, named in the [table of checks](#checks), beside any
settings it has:

```sh
unindent -else-if-after-exit=false ./...
unindent -if-wrapping-rest-min-statements=5 ./...
unindent -if-alone-in-if-max-operands=3 ./...
go vet -vettool=$(command -v unindent) -else-if-after-exit=false ./...
```

The analyzer is also available as a Go package, `unindent.NewAnalyzer()`, for use with any
[`go/analysis`](https://pkg.go.dev/golang.org/x/tools/go/analysis) driver. Its flags are in the analyzer's `Flags`, for
example `a.Flags.Set("else-if-after-exit", "false")`, and belong to that analyzer alone.

## Checks

| Check                                                         | Reports                                                                                                                                                       | Flag                                                    |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| [Else after an exit](docs/checks.md#else-after-an-exit)       | An `else` after a branch that always exits                                                                                                                    | `-else-after-exit`                                      |
| [Else if after an exit](docs/checks.md#else-if-after-an-exit) | An `else if` after a branch that always exits                                                                                                                 | `-else-if-after-exit`                                   |
| [Exit in an else](docs/checks.md#exit-in-an-else)             | An `else` that always exits after a branch that does not, fixed by inverting the condition                                                                    | `-exit-in-else`                                         |
| [If wrapping the rest](docs/checks.md#if-wrapping-the-rest)   | An `if` without an `else` that ends a function or loop body and holds at least 3 statements, fixed by inverting the condition to `return` or `continue` early | `-if-wrapping-rest`, `-if-wrapping-rest-min-statements` |
| [If alone in an if](docs/checks.md#if-alone-in-an-if)         | An `if` whose block holds only another `if`, neither with an `else`, fixed by combining the conditions with `&&`, up to 2 operands                            | `-if-alone-in-if`, `-if-alone-in-if-max-operands`       |

## Limitations

- A call through a function value, or through an interface other than `testing.TB`, is not known to exit.
- A call that never returns only in some builds counts as an exit, but its fix is withheld, as it would change behaviour
    in a build where the call returns. It is found by a heuristic: the function, or one it calls, is declared in a file
    with build constraints and again in a file that the build leaves out.
- Finding helpers in other packages that never return means type-checking every dependency from source. Run on its own,
    `unindent` uses about 3.5 times the CPU time of a linter that only reads the packages it checks, and a standalone
    run on a project the size of prometheus can need about 7.5 GB of memory, because dependencies are loaded from
    source. `go vet -vettool` caches this work between runs, and golangci-lint shares it with linters such as
    `staticcheck` and `govet`'s `lostcancel`.

When a fix would not compile or would change behaviour, the finding is reported without one, with the reason. See
[when no fix is offered](docs/checks.md#when-no-fix-is-offered). Where golangci-lint runs `unindent` as a custom linter
named `unindent`, `//nolint:unindent` suppresses a finding that should stay as it is.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Released changes are listed in the [changelog](CHANGELOG.md), and those not yet
released in [`.changes/unreleased`](.changes/unreleased).

## License

[MIT](LICENSE)
