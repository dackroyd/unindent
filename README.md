# Unindent

[![CI](https://github.com/dackroyd/unindent/actions/workflows/ci.yaml/badge.svg)](https://github.com/dackroyd/unindent/actions/workflows/ci.yaml)

`unindent` is a linter for Go code to help avoid unnecessarily indented code blocks. These may appear in various cases,
commonly via `else` blocks which follow an `if` which always ends in a `return`, `break`, or `continue`. In these
situations, the `else` is not needed, and its contents be included at a lower level of indentation.

https://go.dev/wiki/CodeReviewComments#indent-error-flow

> Try to keep the normal code path at a minimal indentation, and indent the error handling, dealing with it first. This
> improves the readability of the code by permitting visually scanning the normal path quickly. For instance, don’t
> write:
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

The workflows are hardened against supply chain attacks: actions are pinned to commit SHAs, the token is read-only, and
checkouts keep no credentials. A "Workflows" job runs [actionlint](https://github.com/rhysd/actionlint) and
[zizmor](https://docs.zizmor.sh) (pedantic) so changes keep these practices, and Dependabot proposes updates weekly,
once a release is a week old. To run the same checks:

```sh
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
zizmor --persona=pedantic .github
```

Pull requests from forks only run once the maintainer approves them, with the repository setting Settings → Actions →
General → "Approval for running fork pull request workflows" → "Require approval for all external contributors".

### Corpus

Tests cover the cases we thought of. CI also runs `unindent` over pinned versions of real Go codebases, the default set
in [`corpus/projects.go`](corpus/projects.go): the standard library and nine well-known projects with few dependencies,
chosen so that together they produce every kind of finding and every reason a fix is withheld. It checks them with each
Go release the Go team supports, pinned to the patch releases go1.26.9 and go1.27.0, applies the fixes, then checks that
each project still builds, passes `go vet` and its tests, and keeps its comments. The standard library is reported on
but not fixed. The results are recorded in `corpus/testdata/<set>/<release>/`, so a pull request that changes what is
reported or how it is fixed shows the change in those files:

- `findings.txt`: every report, its kind, and whether it has a fix or why not.
- `fixes.txt`: what the fixes change, whether each module still builds and passes `go vet`, recorded apart so a vet
    failure before the fixes can't hide a build they break, and whether the fixes break tests.
- `comparison.txt`: how the reports overlap with revive's, recorded in `revive.txt`. The comparison runs on go1.27.0
    only.

To check a change and record its results, run this from the `corpus` directory:

```sh
go run . check -update
```

This checks the default set with every release. To check with one, pass it with `-go`, as in `-go go1.27.0`. The first
run downloads the projects, about 170 MB, into your user cache directory, and their dependencies into the module cache.
A check with one release takes about 4 minutes on 4 cores, 2 GB of disk and 3 GB of memory. The standard library is
analysed one top-level directory at a time, so the memory needed grows with its largest directory rather than with the
whole library. The check is a separate Go module, so none of this affects projects that import `unindent`.

The large set holds golang/tools, grpc-go, terraform, hugo and prometheus. It is checked with the latest release only,
and is run with `-set large`:

```sh
go run . check -set large -update
```

It takes up to half an hour, about 3.5 GB of memory and about 50 GB of disk, mostly for the build cache. `-clean-cache`
empties the build cache once each project's builds are checked, before the fixes and after them, which brings the disk
needed down to about 31 GB, the most that building and vetting hugo takes, but makes the check about a fifth slower.
`-update` also regenerates [`docs/projects.md`](docs/projects.md) from the results, as does `go run . docs` on its own,
and the corpus check's tests fail if the page no longer matches them. The "Corpus (large)" workflow checks the large set
weekly, and can be run by hand from the Actions tab. It fails when the results drift from those recorded, for example
after a change that CI's default set didn't show, so they are re-recorded deliberately.

After changing a set's projects, record revive's reports with `go run . revive -set <set>`, then the results.

The results are the same on any host: `unindent`, `go build` and `go vet` target linux/amd64 without cgo wherever the
check runs, as build constraints would otherwise select other files. Tests run natively, as they may need cgo or run
what they build, and the results only say whether the fixes broke them. Tests that already fail on a host are logged.
Packages with tests that fail only after the fixes run up to twice more, and the tests count as broken only if they fail
every time; those that pass on a rerun are logged as flaky. CI checks the default set on linux/amd64 with both releases,
and with go1.27.0 on arm64 Linux and Apple silicon macOS, to confirm the results match and test the fixes on other
platforms.

CI caches what the check needs, saved on `main` only, where pull requests can restore it, so pull requests add no
caches. The checkouts are one cache that every job shares, and `go run . fetch` downloads them. The modules are cached
per release as downloaded, `$(go env GOMODCACHE)/cache/download`, rather than extracted. The build cache, about 1.1 GB
for each release, is cached for each runner's OS and architecture, as their compilers differ. All are keyed by
`projects.go`, and a change to the projects restores the last build cache saved, so that only what changed is built
again.

CI runs the corpus jobs on every push to `main`, and on pull requests that change something the results depend on: the
analyzer and CLI (Go files other than tests and `testdata`), the root `go.mod`, `go.sum` or `go.work`, anything in
`corpus/`, `docs/projects.md`, or the CI workflow. The "Corpus" job passes when every corpus job passed or was skipped,
so it is the check to require, rather than the jobs for each release.
