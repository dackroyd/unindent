# Check policy

Every check in `unindent` follows these rules unless its section in [checks.md](checks.md) says otherwise and why. A
proposal for a new check only needs to discuss what is new about it; the rest is settled here. Each rule gives its
reason, and links to the section of [checks.md](checks.md) that describes it where there is one.

## Scope

- **A check removes indentation, or the cause of it, that Go's guidance calls unnecessary.** That is the purpose of the
    linter.
- **Never the reverse.** No check splits a combined condition or adds nesting; code written flatter is left alone. A fix
    that increased nesting would undo a style the author chose. ([Limitations](checks.md#limitations))
- **A fix never increases the maximum indentation of the code it changes**, and the fixed code is never reported again,
    by any check: one run reaches the fixed point, which `TestFixedNotReported` checks. Otherwise `-fix` would
    ping-pong.
- **Code that would read worse fixed stays out of scope, as a documented limitation.** A search loop's final `if` acts
    once, and `if !found { continue }` would make it look like it runs for every element; a refactor is not a mechanical
    fix. ([Limitations](checks.md#limitations))
- **Test files and guards are reported like other code.** Tests follow the same guidance, and error and comma-ok guards
    are the commonest real cases.
- **Constant conditions are not reported** (`if debug`, `if false`): they are switches, not logic.

## Flags and settings

- **On by default, with a boolean flag that turns it off, in positive kebab case named after the check's heading in
    checks.md, without the article** (`-exit-in-else` for "Exit in an else"). It reads naturally as `=false`, and each
    check adds its own flag rather than growing a list. ([Checks](checks.md#checks))
- **A check's settings sit beside its flag, prefixed with it** (`-if-wrapping-rest-min-statements`). The name says which
    check it tunes.
- **Settings are `flag.Value`s that reject values the check can't use**, such as a minimum below 1, with
    `must be at least N, not M`. A bad setting fails at start-up rather than silently doing nothing.
- **Flags belong to the analyzer `NewAnalyzer` returns**, never to package state, and are the golangci-lint settings
    with no change. One mechanism serves the command, `go vet` and golangci-lint.
- **Flag combinations are tested by unit tests, not corpus runs.** The corpus records the defaults only.
- **Limits are structural, not widths**: counts of statements or operands, never columns or lines. Go's guidance is to
    break lines by structure, and gofmt never wraps a condition.
- **Defaults are sized by volume as well as correctness**: the check runs beside other linters, so the default is where
    the result is clearly better, not where it is merely correct.
- **Flags, setting names and the golangci-lint plugin settings become a compatibility promise at v1.0.0.** Before then,
    a minor release may rename or remove them, with a changelog entry; from v1.0.0 they change only in a new major
    version.

## Reports and fixes

- **Report once, at the construct the fix changes**: the `else`, the `if`, or the head of a chain, never each link.
    Moving one link out of a chain would change what runs. ([Exit in an else](checks.md#exit-in-an-else))
- **The message names the construct, says why it is unnecessary, and shows what the fix leaves**, built from the edited
    source (`leaving "if d < '0' || d > '9' { ... }"`). The reader sees the result without applying it.
- **Withhold the fix, never the report, when the fix would not compile or would change behaviour**, with the reason
    after "Not fixed automatically:". A wrong fix is worse than a manual one.
    ([When no fix is offered](checks.md#when-no-fix-is-offered))
- **The fixes of one pass never overlap**: a fix that copies source text is withheld, innermost first, while another fix
    edits that text, and the next run applies it. Drivers skip overlapping fixes, and golangci-lint skips all of a
    linter's fixes in the file.
- **Fixes copy source text rather than reformatting it**, and insert or delete around a block instead of rewriting it,
    so comments survive and fixes inside the block still apply in the same run.
- **Comments move with what they describe; where no placement is right, the check doesn't report.** A misplaced comment
    misleads more than nesting does.
- **An initialiser moves before the `if` only when what follows uses what it declares** (by object, not name), with the
    shared conflict reasons: redeclaration, hiding a later use, two fixes moving one name, a `goto` over a moved
    declaration. ([Initialisers](checks.md#initialisers), [When no fix is offered](checks.md#when-no-fix-is-offered))
- **Where two checks would edit the same token, one yields**, with a unit test, and the next run does the rest.

## Conditions

- **Negate as written, editing only what must change**: drop a leading `!`, swap `==` and `!=`, reverse `<`, `<=`, `>`,
    `>=` only for integers and strings, as NaN makes `!(x < y)` differ from `x >= y` for floats.
    ([Exit in an else](checks.md#exit-in-an-else))
- **`&&` and `||` swap by De Morgan's laws when every operand negates cleanly**; otherwise the whole condition is
    wrapped in `!(…)`, never half of it. Parentheses only where precedence needs them.
- **A reversed comparison reads variable first**: `0 < n` negates to `n <= 0`, and `'0' <= d && d <= '9'` to
    `d < '0' || d > '9'`. A comparison with a comment inside keeps its order.
- **Never nest the happy path under `err == nil`**: a condition that can only hold with a non-nil error is not inverted
    when the `else` holds more than its exit. `if err == nil { break }` is an early exit; a nested success path is not.
    ([Exit in an else](checks.md#exit-in-an-else))
- **Calls with side effects may be combined with `&&`**: short-circuiting is ordinary Go and evaluates them exactly when
    they ran before.

## Exits

- **An exit is a terminating statement of the Go specification, `break`, `continue`, or a call that never returns**
    (`panic`, the `testing.TB` methods that stop a test, and what `ctrlflow` finds, across packages). One definition for
    every check. ([Checks](checks.md#checks))
- **A label after an exit ends the scan**, as a `goto` may reach it; missing a report is safe, a wrong fix is not.
- **A call that never returns only in some builds is an exit without a fix**, as the code after it may run in another
    build. ([Checks](checks.md#checks))

## Proposing and adding a check

Open an issue before writing a check, with:

- **How often it fires** at each candidate threshold, on the corpus projects (see [CONTRIBUTING](../CONTRIBUTING.md)).
- **Real before and after examples from the corpus**, including cases that read worse and what is left out.
- **The comparison** with revive, gocritic and staticcheck: what they already report of it, and what only they report.
- **The Go style guidance** behind the default and any limits.

The pull request then needs:

- unit tests for each case reported and each case not reported, and for each combination of its flags;
- a hand check of at least 30 of its fixes across projects, or all of them when there are fewer;
- the corpus check passing on both supported Go releases, with its recorded results updated (`go run . check -update` in
    `corpus`, as [CONTRIBUTING](../CONTRIBUTING.md) describes);
- benchmarks compared against the base branch;
- the docs: its section in [checks.md](checks.md), its row in the README and [comparison.md](comparison.md), and a
    changelog fragment.

A bug found on the way is fixed in its own pull request first, and so is a speed-up that also helps existing code.
Commit messages and pull requests describe corpus effects approximately and point to [projects.md](projects.md) rather
than quoting exact counts, which change with every later check.
