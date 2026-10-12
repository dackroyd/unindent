# Contributing

Bug reports, false positives and suggestions are welcome as [issues](https://github.com/dackroyd/unindent/issues). A
false positive report is most useful with the smallest code that reproduces it.

## Setup

Go 1.26 or later. [`.tool-versions`](.tool-versions) pins the version used for development, for
[asdf](https://asdf-vm.com/) or [mise](https://mise.jdx.dev/) users.

## Checks

CI runs these on every pull request and on `main`. Run them before pushing:

```sh
go test -race ./...
go vet ./...
gofmt -l .
go mod tidy -diff
golangci-lint run
```

Every check runs on both Go releases the Go team supports, Go 1.26 and 1.27, and the tests also run with Go 1.27 on
arm64 Linux and Apple silicon macOS.

The workflows are hardened against supply chain attacks: actions are pinned to commit SHAs with the version in a
comment, the token is read-only, checkouts keep no credentials, and scripts read values from the environment rather than
`${{ }}` expressions. The "Workflows" job runs [actionlint](https://github.com/rhysd/actionlint) and
[zizmor](https://docs.zizmor.sh) (pedantic) to keep it that way. Run them when you change a workflow:

```sh
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
zizmor --persona=pedantic .github
```

Dependabot proposes updates to the actions and Go modules weekly, once a release is a week old.

Pull requests from forks get a read-only token and no secrets, and only run once the maintainer approves them. This
needs the repository setting Settings → Actions → General → "Approval for running fork pull request workflows" →
"Require approval for all external contributors".

## Tests

The analyzer is tested with [`analysistest`](https://pkg.go.dev/golang.org/x/tools/go/analysis/analysistest) against the
files in [`testdata/src/unindent`](testdata/src/unindent):

- A `// want` comment on a line asserts the message reported there, as a regular expression. A line without one must not
    be reported.
- The `.golden` file next to each test file holds the source after the suggested fixes are applied.

Add a case for every change in behaviour: one that is reported, and one alongside it that is not.

## Checking against real code

Tests cover the cases we thought of. CI also runs `unindent` over pinned versions of real Go codebases, the default set
in [`corpus/projects.go`](corpus/projects.go): the standard library and nine well-known projects with few dependencies,
chosen so that together they produce every kind of finding and every reason a fix is withheld. It checks them with each
Go release the Go team supports, pinned to the patch releases go1.26.9 and go1.27.0, applies the fixes, then checks that
each project still builds, passes `go vet` and its tests, and keeps its comments. The standard library is reported on
but not fixed. The results are recorded in `corpus/testdata/<set>/<release>/`, and CI fails when they change, so a pull
request that changes what is reported or how it is fixed must update them:

- `findings.txt`: every report, its kind, and whether it has a fix or why not.
- `fixes.txt`: what the fixes change, whether each module still builds and passes `go vet`, recorded apart so a vet
    failure before the fixes can't hide a build they break, and whether the fixes break tests.
- `comparison.txt`: how the reports overlap with revive's, recorded in `revive.txt`. The comparison runs on go1.27.0
    only.

Run this from the `corpus` directory to check a change and record its results, then review the changes to those files:

```sh
go run . check -update
```

This checks the default set with every release. To check with one, pass it with `-go`, as in `-go go1.27.0`. The first
run downloads the projects, about 170 MB, into your user cache directory, and their dependencies into the module cache.
A check with one release takes about 4 minutes on 4 cores, 2 GB of disk and 3 GB of memory. The standard library is
analysed one top-level directory at a time, so the memory needed grows with its largest directory rather than with the
whole library. The check is a separate Go module, so none of this affects projects that import `unindent`.

The results are the same on any host. `unindent`, `go build` and `go vet` target linux/amd64 without cgo wherever the
check runs, as build constraints would otherwise select other files. Tests run natively, as they may need cgo or run
what they build, and the results only say whether the fixes broke them: tests that already fail on a host are logged
instead. Packages with tests that fail only after the fixes run up to twice more, and the tests count as broken only if
they fail every time; those that pass on a rerun are logged as flaky. CI checks the default set on linux/amd64 with both
releases, and with go1.27.0 on arm64 Linux and Apple silicon macOS, to confirm the results match and test the fixes on
other platforms.

### The large set

The large set holds golang/tools, grpc-go, terraform, hugo and prometheus. Their results back
[`docs/projects.md`](docs/projects.md) and the [comparison with revive](docs/comparison.md), so they are checked with
the latest release only. Check and record them with:

```sh
go run . check -set large -update
```

This takes up to half an hour, about 3.5 GB of memory and about 50 GB of disk, mostly for the build cache.
`-clean-cache` empties the build cache once each project's builds are checked, before the fixes and after them, which
brings the disk needed down to about 31 GB, the most that building and vetting hugo takes, but makes the check about a
fifth slower. `-update` also regenerates `docs/projects.md` from the results, as does `go run . docs` on its own, and
the corpus check's tests fail if the page no longer matches them, so don't edit it by hand.

The "Corpus (large)" workflow checks the large set every week, and can be run by hand from the Actions tab. It fails
when the results drift from those recorded, for example after a change whose effect the default set didn't show, or a
new Go release. Re-record them with the command above in a pull request of their own, after checking the changes are
intended.

### Adding a project

Add it to a set in `projects.go`, with its commit, the commit's date, a description and the packages whose tests to run,
then record revive's reports and the results:

```sh
go run . revive -set default
go run . check -update
```

A project in the default set should be well known and permissively licensed, have few dependencies, as the check's
memory and disk grow with them, a `go.mod` that supports the oldest Go release checked, and tests that take seconds.
Prefer projects that build and pass `go vet` before the fixes, and check what they add: the coverage of finding kinds
and withheld reasons is in `findings.txt`. Larger projects belong in the large set.

### CI

CI caches what the check needs. The caches are saved on `main` only, where pull requests can restore them, so pull
requests add none. The checkouts are one cache that every job shares, and `go run . fetch` downloads them. The modules
are cached for each release as downloaded, `$(go env GOMODCACHE)/cache/download`, rather than extracted. The build
cache, about 1.1 GB for each release, is cached for each runner's OS and architecture, as their compilers differ. All
are keyed by `projects.go`, and a change to the projects restores the last build cache saved, so that only what changed
is built again. Together they stay well under the repository's 10 GB cache limit.

CI runs the corpus jobs on every push to `main`, and on pull requests that change something the results depend on: the
analyzer and CLI (Go files other than tests and `testdata`), the root `go.mod`, `go.sum` or `go.work`, anything in
`corpus/`, `docs/projects.md`, or the CI workflow. The "Corpus" job passes when every corpus job passed or was skipped,
so it is the check to require, rather than the jobs for each release.

## Documentation

Update the documentation in the same change as the behaviour it describes:

- [`README.md`](README.md): the problem, the quick start, usage and limitations.
- [`docs/checks.md`](docs/checks.md): what each check reports and how it is fixed.
- [`docs/comparison.md`](docs/comparison.md): coverage compared with other linters.
- [`docs/projects.md`](docs/projects.md): generated from the large set's results, so record them rather than edit it.
- [`CHANGELOG.md`](CHANGELOG.md): an entry under "Unreleased" for every user-facing change.

## Commits and pull requests

One pull request makes one change, in a single commit. Amend the commit to address review feedback.

The subject starts with a [gitmoji](https://gitmoji.dev/), then states the outcome in the imperative, in sentence case,
without a trailing period. The body explains why the change is needed.

```text
🐛 Keep comments when removing an else

The fix deleted everything between "else {" and the first statement in
the block, so a comment at the top of the block was lost.
```
