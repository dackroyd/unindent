# Unindent

[![CI](https://github.com/dackroyd/unindent/actions/workflows/ci.yaml/badge.svg)](https://github.com/dackroyd/unindent/actions/workflows/ci.yaml)

`unindent` is a linter for Go code to help avoid unnecessarily indented code blocks. These may appear in various cases,
commonly via `else` blocks which follow an `if` which always ends in a `return`, `break`, or `continue`. In these
situations, the `else` is not needed, and its contents be included at a lower level of indentation.

https://go.dev/wiki/CodeReviewComments#indent-error-flow

> Try to keep the normal code path at a minimal indentation, and indent the error handling, dealing with it first. This
> improves the readability of the code by permitting visually scanning the normal path quickly. For instance, don’t write:
>
> ```
> if err != nil {
>     // error handling
> } else {
>     // normal code
> }
> ```
>
> Instead, write:
>
> ```
> if err != nil {
>     // error handling
>     return // or continue, etc.
> }
> // normal code
> ```

While this calls out errors specifically, it does not only apply to error cases.

## Requirements

Go 1.26 or later.

## Development

CI runs these checks on every pull request and on `main`. Run them locally before pushing:

```sh
go test -race ./...
go vet ./...
gofmt -l .
go mod tidy -diff
golangci-lint run
```

Every check runs on both Go releases the Go team supports, Go 1.26 and 1.27, and the tests also run with Go 1.27 on
arm64 Linux and Apple silicon macOS.

The workflows are hardened against supply chain attacks: actions are pinned to commit SHAs, the token is read-only,
and checkouts keep no credentials. A "Workflows" job runs [actionlint](https://github.com/rhysd/actionlint) and
[zizmor](https://docs.zizmor.sh) (pedantic) so changes keep these practices, and Dependabot proposes updates weekly,
once a release is a week old. To run the same checks:

```sh
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
zizmor --persona=pedantic .github
```

Pull requests from forks only run once the maintainer approves them, with the repository setting Settings → Actions →
General → "Approval for running fork pull request workflows" → "Require approval for all external contributors".
