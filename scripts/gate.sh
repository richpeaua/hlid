#!/usr/bin/env bash
# Hlid acceptance gate. The single top-of-loop check every ticket must pass before its PR goes
# out and every reviewer runs independently (warp references this via [gate].command).
# Steps: gofmt (formatting) -> go vet (suspicious constructs) -> golangci-lint (if installed)
#        -> go build (compiles) -> go test (race-enabled). Fails fast on the first red step.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== gofmt =="
unformatted="$(gofmt -l . 2>/dev/null | grep -v '^vendor/' || true)"
if [ -n "$unformatted" ]; then
  echo "unformatted files (run: gofmt -w .):" >&2
  echo "$unformatted" >&2
  exit 1
fi

# No packages yet (fresh bootstrap): the go tools error on "./..." matching nothing, so the gate
# is vacuously green until the first ticket lands a package.
if [ -z "$(go list ./... 2>/dev/null)" ]; then
  echo "== go: no packages yet (bootstrap) — skipping vet/build/test =="
  echo "gate: PASS"
  exit 0
fi

echo "== go vet =="
go vet ./...

if command -v golangci-lint >/dev/null 2>&1; then
  echo "== golangci-lint =="
  golangci-lint run ./...
else
  echo "== golangci-lint (skipped: not installed) =="
fi

echo "== go build =="
go build ./...

echo "== go test =="
go test -race ./...

echo "gate: PASS"
