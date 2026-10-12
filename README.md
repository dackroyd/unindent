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
} else { // Unnecessary "else": preceding conditions always end in a "return", "break" or "continue". ...
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

- **Branches that exit in every path.** An `if` whose body ends in an `if`/`else` or `switch` that returns in every case
    still makes its `else` unnecessary. revive does not follow those paths.
- **`else if` chains.** When a branch exits, the `else if` after it can start a new `if`. revive does not report chains.
- **`select` cases.** revive skips `if` statements directly inside a `select` case.
- **Fixes.** Every report comes with a fix that `-fix` applies.

On five large Go codebases, `unindent` reported 263 cases that revive's rules did not. revive reported 105 that
`unindent` does not yet, so run both for now. [Comparison with other linters](docs/comparison.md) has the details, and
[Projects](docs/projects.md) what `unindent` finds in each codebase and what its fixes do.

## Quick start

```sh
go install github.com/dackroyd/unindent/cmd/unindent@latest
unindent ./...
```

Requires Go 1.26 or later.

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

The analyzer is also available as a Go package, `unindent.NewAnalyzer()`, for use with any
[`go/analysis`](https://pkg.go.dev/golang.org/x/tools/go/analysis) driver.

## Checks

| Check                                                         | Reports                                                              |
| ------------------------------------------------------------- | -------------------------------------------------------------------- |
| [Else after an exit](docs/checks.md#else-after-an-exit)       | An `else` after a branch that always returns, breaks or continues    |
| [Else if after an exit](docs/checks.md#else-if-after-an-exit) | An `else if` after a branch that always returns, breaks or continues |

## Limitations

- Only `return`, `break` and `continue` count as exits. A branch that ends in `panic`, `os.Exit`, `log.Fatal` or
    `t.Fatal` is not recognised.
- A `break` inside a `switch` or `select` is treated as leaving the `if`, though it only leaves the `switch`, so the fix
    can change behaviour.
- The fix moves the `else` block's declarations into the enclosing block without checking for names already declared
    there. The result may not compile, or may shadow a variable used after the `if`.
- Moving an `if` statement's initialiser before the `if` can shadow a variable used after it.
- Comments between `else {` and the first statement in the block are removed by the fix.

Review fixes before committing them.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Changes are listed in the [changelog](CHANGELOG.md).

## License

[MIT](LICENSE)
