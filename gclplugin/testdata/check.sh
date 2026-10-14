#!/usr/bin/env bash
# Checks the golangci-lint plugin end to end on the sample module: the findings with the default and with changed
# settings, and the result of --fix, against the golden files beside this script. Run it from anywhere with
# golangci-lint v2.14.0 on PATH, which builds the plugin from this checkout with sample/.custom-gcl.yaml, or with
# CUSTOM_GCL naming a golangci-lint already built that way. UPDATE=1 rewrites the golden files.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

gcl=${CUSTOM_GCL:-}
if [ -z "$gcl" ]; then
  (cd "$here/sample" && golangci-lint custom --destination "$work/bin" --name custom-gcl)
  gcl=$work/bin/custom-gcl
fi

cp -R "$here/sample" "$work/sample"
cd "$work/sample"

# golangci-lint exits with status 1 when it reports an issue.
lint() {
  "$gcl" run --output.text.print-issued-lines=false --output.text.colors=false --show-stats=false \
    --max-issues-per-linter=0 --max-same-issues=0 "$@" ./... || [ $? = 1 ]
}

check() {
  local golden=$here/$1 got=$2
  if [ "${UPDATE:-}" = 1 ]; then
    cp "$got" "$golden"
  elif ! diff -u "$golden" "$got"; then
    echo "$1 differs; rerun with UPDATE=1 if the change is intended" >&2
    exit 1
  fi
}

lint > "$work/default.txt"
check default.golden "$work/default.txt"
lint -c settings.golangci.yaml > "$work/settings.txt"
check settings.golden "$work/settings.txt"

lint --fix > /dev/null
check sample.go.golden sample.go
lint > "$work/fixed.txt"
if [ -s "$work/fixed.txt" ]; then
  echo "the fixed sample is reported again:" >&2
  cat "$work/fixed.txt" >&2
  exit 1
fi
