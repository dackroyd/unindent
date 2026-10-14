# golangci-lint

`unindent` runs in golangci-lint as a [module plugin](https://golangci-lint.run/docs/plugins/module-plugins/):
`golangci-lint custom` builds a golangci-lint that includes it, from the package
`github.com/dackroyd/unindent/gclplugin`. golangci-lint itself does not have to adopt it. Tested with golangci-lint
v2.14.0.

## Building

`.custom-gcl.yaml`, next to `.golangci.yaml`, names the golangci-lint release and the `unindent` release to build:

```yaml
version: v2.14.0
name: custom-gcl
plugins:
  - module: github.com/dackroyd/unindent
    import: github.com/dackroyd/unindent/gclplugin
    version: v0.1.0
```

`golangci-lint custom` writes `./custom-gcl`, which takes the same commands and flags as `golangci-lint`. It needs `git`
and Go. [golangci-lint-action](https://github.com/golangci/golangci-lint-action) runs it by itself when it finds
`.custom-gcl.yaml` in its working directory, and then lints with the result.

Where a `.custom-gcl.yaml` already builds golangci-lint with other plugins, add `unindent` as one more entry in
`plugins`:

```yaml
version: v2.14.0
name: custom-gcl
plugins:
  - module: example.com/linters
    import: example.com/linters/plugin
    version: v1.4.0
  - module: github.com/dackroyd/unindent
    import: github.com/dackroyd/unindent/gclplugin
    version: v0.1.0
```

The plugins' requirements are then resolved together, as described in [Compatibility](#compatibility).

To try a checkout of `unindent` instead of a release, replace `version` with its path, such as `path: ../unindent`.

## Settings

The linter is named `unindent`. Its settings are the command's flags, without the leading `-`:

```yaml
version: "2"

linters:
  enable:
    - unindent
  settings:
    custom:
      unindent:
        type: module
        description: Identifies unnecessarily indented code
        original-url: https://github.com/dackroyd/unindent
        settings:
          else-after-exit: true
          else-if-after-exit: true
          exit-in-else: true
          if-wrapping-rest: true
          if-wrapping-rest-min-statements: 3
          if-alone-in-if: true
          if-alone-in-if-max-operands: 2
```

These are the defaults, so `settings` can be left out.

| Setting                                                 | Value                      | Default |
| ------------------------------------------------------- | -------------------------- | ------- |
| [`else-after-exit`](checks.md#else-after-an-exit)       | `true` or `false`          | `true`  |
| [`else-if-after-exit`](checks.md#else-if-after-an-exit) | `true` or `false`          | `true`  |
| [`exit-in-else`](checks.md#exit-in-an-else)             | `true` or `false`          | `true`  |
| [`if-wrapping-rest`](checks.md#if-wrapping-the-rest)    | `true` or `false`          | `true`  |
| `if-wrapping-rest-min-statements`                       | a whole number, at least 1 | 3       |
| [`if-alone-in-if`](checks.md#if-alone-in-an-if)         | `true` or `false`          | `true`  |
| `if-alone-in-if-max-operands`                           | a whole number, at least 2 | 2       |

An unknown setting, or a value of the wrong type or out of range, stops golangci-lint before it lints anything, and an
unknown one lists the settings:

```text
Error: build linters: plugin(unindent): newPlugin setting "exit-in-else" must be true or false, not 0
```

golangci-lint reads the configuration before the plugin sees it, which has two effects: setting names are not case
sensitive, and a setting whose value is `null` is left at its default.

The checks need type information, as they find the calls that never return in other packages too, so golangci-lint loads
the packages with types, as it does for `staticcheck` and `govet`, and shares that work with them.

## Fixes

`./custom-gcl run --fix` applies the same fixes as `unindent -fix`. Where two linters' fixes in a file overlap,
golangci-lint applies only one linter's there, and warns which it skipped:
`Changes related to "…" are skipped for the file … due to conflicts with "…"`. Running it again applies the rest.

## How the reports differ from the command's

golangci-lint filters what it reports, with these defaults:

- Findings in generated files are left out (`linters.exclusions.generated`). The `unindent` command reports them.
- At most 3 findings with the same message are shown (`issues.max-same-issues`), and at most 50 for each linter
    (`issues.max-issues-per-linter`). `--fix` still fixes every finding. Set both to 0 to see them all.
- `//nolint:unindent` on the reported line suppresses a finding, and its fix.

Test files are checked by both, unless `run.tests` is `false`, as `-test=false` turns them off for the command.

## Compatibility

`golangci-lint custom` builds golangci-lint with the dependencies of golangci-lint and of every plugin together, at the
newest version any of them requires. As `unindent` may require a newer `golang.org/x/tools` than the golangci-lint
release does, the other linters and plugins in that build run with it too, and `unindent` runs with any newer version
they require. Each `unindent` release is tested with golangci-lint v2.14.0 and no other plugins; other releases may not
build with it, or may run their own linters incorrectly with the newer dependencies. With other plugins, a CI run of the
binary built from your `.custom-gcl.yaml` is the check that they work together.

The linter name, the import path and the settings are part of `unindent`'s API from v1.0.0, as the flags are.

## If golangci-lint adopts unindent

Once a golangci-lint release includes `unindent`, upgrade to it and remove the `unindent` entry from `.custom-gcl.yaml`
in the same change, and move the settings from `linters.settings.custom.unindent.settings` to
`linters.settings.unindent`. The setting names stay the same. Without other plugins, `.custom-gcl.yaml` can go, and the
stock golangci-lint replaces `./custom-gcl`.
