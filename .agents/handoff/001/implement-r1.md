Implementation is done and the gate is green, but I'm blocked from committing: every mutating git command (`warp git commit`, `git add`, even a single-file `git add go.mod`) is being denied with "This command requires approval" in this session, with no prompt I can resolve myself. I did not use `--no-verify` or any bypass. Stopping here per the git safety protocol rather than forcing it.

## Status

**Branch:** `ticket-001-config-schema-loader` (already checked out, not pushed)
**Commit:** none yet — blocked (see above)

**Files touched (all within contract `scope_files`, plus dependency files the ticket explicitly calls for):**
- `internal/config/config.go` (new)
- `internal/config/config_test.go` (new)
- `go.mod` (added `gopkg.in/yaml.v3 v3.0.1` via `go get`, per ticket text)
- `go.sum` (new, from `go get`)
- Untouched: `issues/001-config-schema-loader.md` has a pre-existing uncommitted `Status: wip` diff from the earlier contract step — not mine, left alone (outside `scope_files`).

**Per-acceptance evidence:**
- A1: `TestLoad_ParsesValidYAML` (table-driven, temp file, 2 cases) — PASS
- A2: `TestValidate` (table-driven subtests: valid config, empty listen, empty routes, non-slash path, empty path, relative upstream URL, bad upstream URL, unsupported-scheme upstream URL, duplicate paths) — PASS, 9/9 subtests
- A3: `TestLoad_Errors/missing_file` (wraps `os.ErrNotExist`, error contains path) and `TestLoad_Errors/malformed_YAML` (error contains path) — PASS

**Gate:** `scripts/gate.sh` → `gate: PASS` (gofmt clean, go vet clean, golangci-lint not installed/skipped, build OK, `go test -race ./...` green)

**Next step needed from you:** either grant the git-add/commit permission in this session so I can run `warp git commit 001 "Add config schema and YAML loader/validator"`, or run that commit yourself — the working tree is ready as-is.