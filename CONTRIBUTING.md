# Contributing

Bug reports, false positives and suggestions are welcome as [issues](https://github.com/dackroyd/unindent/issues). A
false positive report is most useful with the smallest code that reproduces it.

A new check starts as an issue. [`docs/check-policy.md`](docs/check-policy.md) lists the rules every check follows, so a
proposal only discusses what is new about it, and what the issue and the pull request need.

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
golangci-lint custom && ./custom-gcl run
```

[`.golangci.yaml`](.golangci.yaml) enables `unindent` on its own code, so the lint uses the golangci-lint that
`golangci-lint custom` builds with this checkout from [`.custom-gcl.yaml`](.custom-gcl.yaml), as the "Lint" job does.
Plain `golangci-lint run` stops with `plugin "unindent" not found`.

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

### The golangci-lint plugin

[`gclplugin/gclplugin_test.go`](gclplugin/gclplugin_test.go) tests how the plugin's settings set the flags.
[`gclplugin/testdata/check.sh`](gclplugin/testdata/check.sh) checks the plugin end to end: it builds golangci-lint with
it, lints the module in [`gclplugin/testdata/sample`](gclplugin/testdata/sample) with the default and with changed
settings, applies `--fix`, and compares the results with the `.golden` files beside it. The "Plugin" job runs it with
both Go releases. Run it with golangci-lint v2.14.0 on `PATH`, and with `UPDATE=1` to rewrite the golden files after an
intended change:

```sh
gclplugin/testdata/check.sh
```

## Benchmarks

The benchmarks in [`bench_test.go`](bench_test.go) analyze generated code shaped to expose costs that grow faster than
the code, and a few standard library packages. For a change that may affect speed, compare at least 8 runs before and
after with [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat):

```sh
go test -run '^$' -bench . -benchmem -count 10 > new.txt
benchstat old.txt new.txt
```

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

The "Corpus (large)" workflow checks the large set every week, and is run by hand from the Actions tab before each
release. It fails when the results drift from those recorded, for example after a change whose effect the default set
didn't show. Re-record them with the command above in a pull request of their own, after checking the changes are
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
analyzer and CLI (Go files other than tests, `testdata` and the golangci-lint plugin in `gclplugin/`), the root
`go.mod`, `go.sum` or `go.work`, anything in `corpus/`, `docs/projects.md`, or the CI workflow. The "Corpus" job passes
when every corpus job passed or was skipped, so it is the check to require, rather than the jobs for each release.

## Documentation

Update the documentation in the same change as the behaviour it describes:

- [`README.md`](README.md): the problem, the quick start, usage and limitations.
- [`docs/checks.md`](docs/checks.md): what each check reports and how it is fixed.
- [`docs/check-policy.md`](docs/check-policy.md): the rules every check follows, and what adding one needs.
- [`docs/comparison.md`](docs/comparison.md): coverage compared with other linters.
- [`docs/golangci-lint.md`](docs/golangci-lint.md): the golangci-lint plugin, its settings and how its reports differ.
- [`docs/projects.md`](docs/projects.md): generated from the large set's results, so record them rather than edit it.
- A changelog fragment for every change that users notice, as [Changelog](#changelog) describes.

## Changelog

Changes that users notice are recorded as [changie](https://changie.dev) fragments in
[`.changes/unreleased`](.changes/unreleased), one per entry, and batched into [`CHANGELOG.md`](CHANGELOG.md) when a
version is released. Add one in the same pull request as the change, from the repository root:

```sh
go tool -modfile=tools/go.mod changie new
```

It asks for the kind of change, one of Keep a Changelog's: Added, Changed, Deprecated, Removed, Fixed or Security. Then
it asks for the entry, written for people who use `unindent`: what changed and, for a fix, what went wrong before. The
kinds choose the next version: Fixed and Security bump the patch version, and the others the minor version until v1.0.0.

Changes that users don't notice, such as CI, the corpus check, tests and documentation, need no fragment. The
"Changelog" check fails a pull request that changes the analyzer, the CLI or the golangci-lint plugin (Go files other
than tests, in the root, `cmd/` or `gclplugin/`) without adding a fragment. Label the pull request `no changelog` when
such a change is internal, a refactoring for example.

changie and [GoReleaser](https://goreleaser.com) are pinned with `tool` directives in the nested [`tools`](tools/go.mod)
module, not the root `go.mod`, as the root module's requirements join the module graph of every module that imports
`unindent`. `-modfile=tools/go.mod` runs them from the repository root, where their configuration is. The first run
builds them.

## Commits and pull requests

One pull request makes one change, in a single commit. Amend the commit to address review feedback.

The subject starts with a [gitmoji](https://gitmoji.dev/), then states the outcome in the imperative, in sentence case,
without a trailing period. The body explains why the change is needed.

```text
🐛 Keep comments when removing an else

The fix deleted everything between "else {" and the first statement in
the block, so a comment at the top of the block was lost.
```

## Releasing

Releases are made by hand, when there is something worth releasing. First run the "Corpus (large)" workflow from the
Actions tab on `main`, and wait for it to pass: CI only checks the default set, so this confirms the large set's results
still hold. If it fails, re-record them as [The large set](#the-large-set) describes before releasing. Then release in
two steps:

1. **Prepare.** Run the "Prepare release" workflow from the Actions tab. It batches the fragments into
    `.changes/vX.Y.Z.md`, choosing the version from their kinds unless you enter one, regenerates `CHANGELOG.md`, and
    pushes the branch `release/vX.Y.Z`. Open a pull request for the branch yourself, as CI only runs on pull requests
    that a person opens, and review the release notes. Edit them in the pull request if needed, then merge it.
2. **Publish.** The merge starts the "Release" workflow. It reruns CI, then waits for your approval in the `release`
    environment. Once approved, it tags the merge commit `vX.Y.Z`, builds the binaries for Linux, macOS and Windows on
    amd64 and arm64 with GoReleaser, publishes the GitHub release with the version's notes, attests the build
    provenance of the archives and checksums, and asks the Go module proxy for the new version, so that
    `go install …@latest` and pkg.go.dev find it.

The first release is v0.1.0: the prepare workflow uses it when no version has been released yet. Only the repository
root is tagged, as Go modules require. The nested `corpus` and `tools` modules are never tagged.

To try the release configuration locally, without publishing anything:

```sh
go tool -modfile=tools/go.mod changie batch auto --dry-run
go tool -modfile=tools/go.mod goreleaser check
go tool -modfile=tools/go.mod goreleaser release --snapshot --clean
```

The snapshot is written to `dist/`, which git ignores.

### Repository settings

The release process relies on these settings, made once by the maintainer:

- **Rulesets.** Import each file in [`.github/rulesets`](.github/rulesets) with Settings → Rules → Rulesets → New
    ruleset → Import a ruleset, or `gh api -X POST repos/dackroyd/unindent/rulesets --input <file>`.
    [`main.json`](.github/rulesets/main.json) requires a pull request with the CI and "Changelog" checks passing before
    merging to `main`, and blocks force pushes and deletion. [`tags.json`](.github/rulesets/tags.json) only lets
    administrators, maintainers and GitHub Actions, for the release workflow, create, move or delete `v*` tags. Update
    the required checks in `main.json` when a job's name changes. The corpus jobs are required through the "Corpus" job,
    which passes when they pass or are skipped, so changing their matrix needs no update.
- **The `release` environment.** Settings → Environments → New environment, named `release`, with yourself as a required
    reviewer, and its deployment branches limited to `main`.
- **Fork pull requests.** Settings → Actions → General → "Approval for running fork pull request workflows" → "Require
    approval for all external contributors".
- **The `no changelog` label.** Create it under Issues → Labels.
