Already confirmed clean via the full `gate.sh` run (gofmt step passed with no output). That's sufficient.

## Review: issues/004-server-assembly-healthz.md (round 1)

**Independent gate result:** `bash scripts/gate.sh` → **PASS** (fresh, `go clean -testcache` first — not relying on cache). gofmt clean, `go vet` clean, golangci-lint not installed locally (script's documented conditional skip, matches STANDARDS.md's "if installed"), build clean, `go test -race ./...` all green including `internal/server`. Re-ran `internal/server` tests verbose+race independently — all 5 pass for real (not skipped/weakened).

**Scope check:** `git diff afab0e9..HEAD` touches only `internal/server/server.go`, `internal/server/server_test.go` (source) — exactly the contract's `scope_files`. No go.mod/go.sum changes, no drive-by edits elsewhere. (The `issues/004-*.md` status-line churn is workflow metadata, not source, and outside scope_files by design — not a scope violation.)

### Per-acceptance table

| # | Criterion | Result | Evidence |
|---|---|---|---|
| A1 | `GET /healthz` → 200 "ok", not proxied | PASS | `TestHealthzNotProxied` (server_test.go:22-58): asserts 200, body "ok", `upstreamHit==false`. Verified by real test run. |
| A2 | Non-health path dispatched through router to matching upstream | PASS | `TestNonHealthPathDispatchedToRouter` (server_test.go:61-87): `/app/foo` → `upstreamHit==true`. |
| A3 | Panicking handler recovered as 500 | PASS | `TestPanicRecovered` (server_test.go:91-111) unit-tests `recoverPanic` directly with a panicking handler; asserts no panic escapes and `rec.Code==500`. Middleware is wired into `New`'s chain (`server.go:41`: `chain(mux, logging, recoverPanic)`), so this genuinely protects the assembled handler, not just an isolated unit. |
| A4 | `New` errors on invalid cfg; `Addr`/timeouts set on success | PASS | `TestNewInvalidConfigErrors` (empty cfg → error) + `TestNewSetsAddrAndTimeouts` (Addr==cfg.Listen, ReadHeaderTimeout>0, IdleTimeout>0 and ≥1s). |

### Interface conformance
- Signature matches verbatim: `func New(cfg *config.Config) (*http.Server, error)` (server.go:24).
- `cfg.Validate()` checked first, wrapped with `%w` (server.go:25-27); `router.New(cfg.Routes)` called and wrapped (server.go:29-32) — matches `router.New(routes []config.Route) (*Router, error)`'s real signature.
- `/healthz` mounted via `mux.HandleFunc("GET /healthz", ...)`, catch-all via `mux.Handle("/", rt)` — matches "mux... delegates everything else to router.Router."
- Middleware type `func(http.Handler) http.Handler` matches STANDARDS.md's HTTP composition rule.
- `chain()`'s reverse-iteration wrapping is correctly documented and correctly makes `logging` outermost, `recoverPanic` innermost (still wraps `mux` and everything downstream, including the router) — verified by reading the loop, not just trusting the comment.

### Standards audit
- Errors: last-value `error`, wrapped with `fmt.Errorf(...%w...)` — conforms.
- No `panic` in library code; only `recover`, used correctly for fail-closed 500.
- No secrets logged — `logging` middleware logs only method+path.
- Naming: no stutter (`server.New`, not `server.NewServer`); exported `New` has doc comment starting with `New`.
- Test files/`TestXxx` naming conforms; package `server` (white-box) used deliberately to reach unexported `recoverPanic`, documented in-comment — reasonable, not a standards violation (STANDARDS doesn't mandate `_test` external package).

### BLOCKING findings
None.

### NON-BLOCKING findings

1. **`internal/server/server.go:35-38`** — `/healthz` writes `WriteHeader` then the body but ignores the `Write` error (`_, _ = w.Write(...)`) with no comment justifying the swallow, while STANDARDS.md says "Do not use `_ =` to swallow one without a comment justifying it." Trivial (write to an already-committed 200 response, essentially unrecoverable) but technically not compliant with the letter of the rule. `[bug:S3]`

2. **A4's "or the router can't be built" branch is untested.** Since `cfg.Validate()` already rejects malformed upstream URLs, there's no test exercising `router.New` returning an error independently of `Validate` (e.g., a config that passes `Validate` but somehow fails `router.New` — currently no such path exists, so this may be moot given the current `Validate`/`router.New` overlap, but the ticket text explicitly calls out both error sources and only one is covered). `[bug:S3]`

**What else I checked and found clean:** middleware ordering/doc-comment accuracy, panic-recovery integration into the full handler chain (not just isolated), Addr/timeout wiring, config `Validate()` semantics used by `New`, absence of stray scope creep, gofmt/vet/build/race-test cleanliness (verified independently, cache-cleared).

VERDICT: APPROVE