# Comparison with other linters

Several linters report part of the community guidance on indentation. This page lists what each one covers, and measures
how their reports overlap with `unindent`'s on real code.

## Coverage

| Construct                                                                                  | `unindent` | revive                                  | gocritic        | nestif                   |
| ------------------------------------------------------------------------------------------ | ---------- | --------------------------------------- | --------------- | ------------------------ |
| `else` after a branch ending in `return`, `break` or `continue`                            | yes        | `indent-error-flow`, `superfluous-else` | —               | —                        |
| `else` after a branch ending in `panic`, `os.Exit` or `log.Fatal`                          | yes        | `superfluous-else`                      | —               | —                        |
| `else` after a branch ending in `t.Fatal`, `t.Skip` or a helper that never returns         | yes        | —                                       | —               | —                        |
| `else` after a branch ending in `goto`                                                     | yes        | `superfluous-else`                      | —               | —                        |
| `else` after a branch ending in a `for` loop or `select` that never ends                   | yes        | —                                       | —               | —                        |
| `else` after a branch that exits through a nested `if`/`else` or `switch`                  | yes        | —                                       | —               | —                        |
| `else if` after a branch that exits                                                        | yes        | —                                       | —               | —                        |
| `if` inside a `select` case                                                                | yes        | —                                       | —               | —                        |
| `if c { ... } else { return }`, inverted to exit first                                     | yes        | `early-return`                          | —               | —                        |
| The same, with an `else` ending in `t.Fatal`, a helper that never returns or a nested exit | yes        | —                                       | —               | —                        |
| `if` wrapping the rest of a loop body                                                      | —          | —                                       | `nestingReduce` | —                        |
| Deeply nested `if` statements                                                              | —          | `max-control-nesting`                   | —               | yes, by complexity score |
| Suggested fix                                                                              | yes        | —                                       | —               | —                        |

revive and gocritic only look at the last statement of a branch. nestif reports how complex a nested `if` is, not what
to change.

## Measured overlap

`unindent` and revive (v1.15.0, via golangci-lint v2.13.1, with `indent-error-flow`, `superfluous-else` and
`early-return`) ran over the five codebases of the corpus check's [large set](../CONTRIBUTING.md#the-large-set),
described in [Projects](projects.md). Generated files and `testdata` directories are excluded. A report counts once per
`if` statement.

The corpus check produces these numbers, and every change that alters them updates
[`corpus/testdata/large/go1.27.0/comparison.txt`](../corpus/testdata/large/go1.27.0/comparison.txt).

| Codebase                                                          | Version               |
| ----------------------------------------------------------------- | --------------------- |
| [golang/tools](https://github.com/golang/tools)                   | `16517d9`, 2026-10-10 |
| [grpc/grpc-go](https://github.com/grpc/grpc-go)                   | `8c59ee6`, 2026-10-10 |
| [hashicorp/terraform](https://github.com/hashicorp/terraform)     | `d8e73fc`, 2026-10-09 |
| [gohugoio/hugo](https://github.com/gohugoio/hugo)                 | `c9f6e4d`, 2026-10-11 |
| [prometheus/prometheus](https://github.com/prometheus/prometheus) | `fffc15f`, 2026-10-11 |

|                             | `if` statements |
| --------------------------- | --------------- |
| Reported by `unindent`      | 734             |
| Reported by revive          | 306             |
| Reported by both            | 300             |
| Reported by `unindent` only | 434             |
| Reported by revive only     | 6               |

Of the 434 that only `unindent` reported, 340 are `else if` chains, 59 follow a branch ending in a call that never
returns, 19 are exits in an `else` block that ends in such a call, 9 are inside `select` cases, 4 exit through a nested
`if`/`else` or `switch`, 1 is an exit in an `else` block through a nested `if` or `switch`, 1 follows a `for` loop that
never ends and 1 is another nested form.

The 6 that only revive reported are all `early-return` inversions. 5 are of an `else if`, whose body can't move out of
its chain. 4 of those have an empty body, holding only a comment that explains the case, after a branch that doesn't
exit. The other follows a branch that exits, so `unindent` reports the `else if` instead, and the inversion once the
chain is split. The other tests that an error is not `nil`, so inverting it would nest the happy path under
`err == nil`, which `unindent` never suggests.

## Using them together

`unindent` reports all but those 6 of revive's `indent-error-flow`, `superfluous-else` and `early-return` findings, so
revive's three rules can be turned off where `unindent` runs. Where both run they ask for the same change, so the only
cost is the same `if` reported twice.
