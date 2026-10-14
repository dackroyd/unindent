# Projects

<!-- Generated from corpus/testdata/large/go1.27.0 by "go run . docs" in corpus/. Do not edit. -->

What `unindent` reports on large Go codebases, and what its fixes do to them, with go1.27.0. The corpus check runs it
over each codebase at a pinned commit for linux/amd64, skipping generated files and `testdata` directories. It
applies the fixes, and checks that the code still builds, passes `go vet` and its tests, and keeps its comments.

| Project | Commit | Findings | Fixed | Fix withheld | Files changed | Comments lost |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| [golang/tools](#golangtools) | [`16517d9`](https://github.com/golang/tools/commit/16517d951552611e7bd358c48ee1051fd0e57b0c), 2026-10-10 | 452 | 431 | 21 | 239 | 0 |
| [grpc/grpc-go](#grpcgrpc-go) | [`8c59ee6`](https://github.com/grpc/grpc-go/commit/8c59ee6356bf92d0a5819b0cdbd3118876783028), 2026-10-10 | 115 | 89 | 26 | 63 | 0 |
| [hashicorp/terraform](#hashicorpterraform) | [`d8e73fc`](https://github.com/hashicorp/terraform/commit/d8e73fcbd9bb9d7c7ab38242eaccef5bfe88e270), 2026-10-09 | 446 | 421 | 25 | 216 | 0 |
| [gohugoio/hugo](#gohugoiohugo) | [`c9f6e4d`](https://github.com/gohugoio/hugo/commit/c9f6e4d4abc3645cb46824c870e4e1b4c098b765), 2026-10-11 | 139 | 133 | 6 | 81 | 0 |
| [prometheus/prometheus](#prometheusprometheus) | [`fffc15f`](https://github.com/prometheus/prometheus/commit/fffc15f5b5a6c3ed50282d307e96db0e5f25bcb2), 2026-10-11 | 116 | 111 | 5 | 70 | 0 |
| Total | | 1268 | 1185 | 83 | 669 | 0 |

A finding's kind is the kind of `else` and the last statement of the branch before it, for an exit in an `else`, the
last statement of the `else` block, and for an `if` wrapping the rest of a function or loop, which of the two it
ends. A fix is withheld when it would not compile or would change behaviour; the reasons quote the names involved,
shown here as "…".

## golang/tools

Go's analysis and refactoring tools, and the gopls language server.

| Kind | Findings |
| --- | ---: |
| `if wrapping the rest of a loop` | 164 |
| `if wrapping the rest of a function` | 84 |
| `else after return` | 76 |
| `else if after return` | 56 |
| `else if after continue` | 15 |
| `else if after call` | 14 |
| `exit in else ending in return` | 10 |
| `else after call` | 9 |
| `else if after break` | 7 |
| `else after continue` | 5 |
| `exit in else ending in break` | 4 |
| `exit in else ending in call` | 4 |
| `else after break` | 1 |
| `else after goto` | 1 |
| `else after switch` | 1 |
| `exit in else ending in continue` | 1 |

| Fix withheld because | Findings |
| --- | ---: |
| `"…" is already declared in the enclosing block` | 20 |
| `"…" is also moved into the enclosing block by an earlier fix` | 1 |

After the fixes, in 3 modules:

- `go build` passes in every module.
- `go vet` passes in every module.
- Tests: not broken by the fixes.

## grpc/grpc-go

The Go implementation of gRPC.

| Kind | Findings |
| --- | ---: |
| `else if after call` | 24 |
| `exit in else ending in call` | 20 |
| `if wrapping the rest of a function` | 19 |
| `if wrapping the rest of a loop` | 19 |
| `else if after return` | 17 |
| `else after call` | 6 |
| `else if after continue` | 4 |
| `else if after break` | 3 |
| `exit in else ending in return` | 2 |
| `else after continue` | 1 |

| Fix withheld because | Findings |
| --- | ---: |
| `"…" is also moved into the enclosing block by an earlier fix` | 13 |
| `"…" is already declared in the enclosing block` | 11 |
| `"…" is also declared by an earlier "if" initialiser` | 1 |
| `an earlier "else" in the chain is not fixed automatically` | 1 |

After the fixes, in 10 modules:

- `go build` passes in every module.
- `go vet` passes in every module.
- Tests: not run.

## hashicorp/terraform

An infrastructure as code tool.

| Kind | Findings |
| --- | ---: |
| `else if after return` | 100 |
| `else after return` | 82 |
| `if wrapping the rest of a loop` | 70 |
| `else after call` | 52 |
| `else if after call` | 41 |
| `exit in else ending in return` | 34 |
| `if wrapping the rest of a function` | 29 |
| `exit in else ending in call` | 13 |
| `else after continue` | 7 |
| `else if after break` | 5 |
| `else after nested if` | 2 |
| `else after switch` | 2 |
| `else if after nested if` | 2 |
| `exit in else ending in break` | 2 |
| `else after break` | 1 |
| `else if after continue` | 1 |
| `else if after switch` | 1 |
| `exit in else ending in continue` | 1 |
| `exit in else ending in nested if` | 1 |

| Fix withheld because | Findings |
| --- | ---: |
| `"…" is already declared in the enclosing block` | 10 |
| `"…" is also moved into the enclosing block by an earlier fix` | 9 |
| `an earlier "else" in the chain is not fixed automatically` | 4 |
| `"…" is also declared by an earlier "if" initialiser` | 1 |
| `another fix inside the "if" statement has to be applied first` | 1 |

After the fixes, in 11 modules:

- `go build` passes in every module.
- `go vet` fails before and after the fixes in 1, passes in 10.
- Tests: not run.

## gohugoio/hugo

A static site generator.

| Kind | Findings |
| --- | ---: |
| `if wrapping the rest of a loop` | 34 |
| `else if after return` | 30 |
| `if wrapping the rest of a function` | 19 |
| `else after return` | 18 |
| `exit in else ending in return` | 12 |
| `else after continue` | 7 |
| `exit in else ending in call` | 6 |
| `else if after break` | 4 |
| `else after break` | 3 |
| `else if after call` | 2 |
| `else if after continue` | 2 |
| `exit in else ending in continue` | 2 |

| Fix withheld because | Findings |
| --- | ---: |
| `"…" is already declared in the enclosing block` | 6 |
| `"…" is also moved into the enclosing block by an earlier fix` | 1 |

After the fixes, in 4 modules:

- `go build` fails before and after the fixes in 1, passes in 3.
- `go vet` fails before and after the fixes in 3, passes in 1.
- Tests: not run.

## prometheus/prometheus

A monitoring system and time series database.

| Kind | Findings |
| --- | ---: |
| `if wrapping the rest of a loop` | 57 |
| `if wrapping the rest of a function` | 39 |
| `else if after return` | 10 |
| `else after switch` | 2 |
| `else if after continue` | 2 |
| `else if after goto` | 2 |
| `exit in else ending in return` | 2 |
| `else after for` | 1 |
| `else if after call` | 1 |

| Fix withheld because | Findings |
| --- | ---: |
| `"…" is already declared in the enclosing block` | 4 |
| `"…" is also declared by the "if" initialiser` | 1 |

After the fixes, in 5 modules:

- `go build` passes in every module.
- `go vet` fails before and after the fixes in 2, passes in 3.
- Tests: not run.
