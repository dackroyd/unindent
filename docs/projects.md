# Projects

<!-- Generated from corpus/testdata/large/go1.27.0 by "go run . docs" in corpus/. Do not edit. -->

What `unindent` reports on large Go codebases, and what its fixes do to them, with go1.27.0. The corpus check runs it
over each codebase at a pinned commit for linux/amd64, skipping generated files and `testdata` directories. It
applies the fixes, and checks that the code still builds, passes `go vet` and its tests, and keeps its comments.

| Project | Commit | Findings | Fixed | Fix withheld | Files changed | Comments lost |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| [golang/tools](#golangtools) | [`16517d9`](https://github.com/golang/tools/commit/16517d951552611e7bd358c48ee1051fd0e57b0c), 2026-10-10 | 160 | 160 | 0 | 91 | 38 |
| [grpc/grpc-go](#grpcgrpc-go) | [`8c59ee6`](https://github.com/grpc/grpc-go/commit/8c59ee6356bf92d0a5819b0cdbd3118876783028), 2026-10-10 | 25 | 25 | 0 | 22 | 0 |
| [hashicorp/terraform](#hashicorpterraform) | [`d8e73fc`](https://github.com/hashicorp/terraform/commit/d8e73fcbd9bb9d7c7ab38242eaccef5bfe88e270), 2026-10-09 | 203 | 203 | 0 | 111 | 37 |
| [gohugoio/hugo](#gohugoiohugo) | [`c9f6e4d`](https://github.com/gohugoio/hugo/commit/c9f6e4d4abc3645cb46824c870e4e1b4c098b765), 2026-10-11 | 64 | 64 | 0 | 34 | 1 |
| [prometheus/prometheus](#prometheusprometheus) | [`fffc15f`](https://github.com/prometheus/prometheus/commit/fffc15f5b5a6c3ed50282d307e96db0e5f25bcb2), 2026-10-11 | 12 | 12 | 0 | 8 | 0 |
| Total | | 464 | 464 | 0 | 266 | 76 |

A finding's kind is the kind of `else` and the last statement of the branch before it. A fix is withheld when
it would not compile or would change behaviour; the reasons quote the names involved, shown here as "…".

## golang/tools

Go's analysis and refactoring tools, and the gopls language server.

| Kind | Findings |
| --- | ---: |
| `else after return` | 76 |
| `else if after return` | 56 |
| `else if after continue` | 15 |
| `else if after break` | 7 |
| `else after continue` | 5 |
| `else after break` | 1 |

After the fixes, in 3 modules:

- `go build` FAILS AFTER THE FIXES in 1, passes in 2.
- `go vet` FAILS AFTER THE FIXES in 1, passes in 2.
- Tests: not broken by the fixes.

## grpc/grpc-go

The Go implementation of gRPC.

| Kind | Findings |
| --- | ---: |
| `else if after return` | 17 |
| `else if after continue` | 4 |
| `else if after break` | 3 |
| `else after continue` | 1 |

After the fixes, in 10 modules:

- `go build` FAILS AFTER THE FIXES in 7, passes in 3.
- `go vet` FAILS AFTER THE FIXES in 8, passes in 2.
- Tests: not run.

## hashicorp/terraform

An infrastructure as code tool.

| Kind | Findings |
| --- | ---: |
| `else if after return` | 100 |
| `else after return` | 82 |
| `else after continue` | 7 |
| `else if after break` | 5 |
| `else after nested if` | 2 |
| `else after switch` | 2 |
| `else if after nested if` | 2 |
| `else after break` | 1 |
| `else if after continue` | 1 |
| `else if after switch` | 1 |

After the fixes, in 11 modules:

- `go build` FAILS AFTER THE FIXES in 1, passes in 10.
- `go vet` FAILS AFTER THE FIXES in 1, fails before and after the fixes in 1, passes in 9.
- Tests: not run.

## gohugoio/hugo

A static site generator.

| Kind | Findings |
| --- | ---: |
| `else if after return` | 30 |
| `else after return` | 18 |
| `else after continue` | 7 |
| `else if after break` | 4 |
| `else after break` | 3 |
| `else if after continue` | 2 |

After the fixes, in 4 modules:

- `go build` fails before and after the fixes in 1, passes in 3.
- `go vet` fails before and after the fixes in 3, passes in 1.
- Tests: not run.

## prometheus/prometheus

A monitoring system and time series database.

| Kind | Findings |
| --- | ---: |
| `else if after return` | 10 |
| `else if after continue` | 2 |

After the fixes, in 5 modules:

- `go build` passes in every module.
- `go vet` fails before and after the fixes in 2, passes in 3.
- Tests: not run.
