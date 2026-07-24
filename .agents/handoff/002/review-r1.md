# Independent Step Review — Ticket 002 (reverse-proxy handler)

## Gate result (independently run)
```
bash scripts/gate.sh
```
`gofmt` clean → `go vet` clean → `golangci-lint` skipped (not installed, per gate script's own fallback) → `go build` OK → `go test -race` OK → `gate: PASS`.

Re-ran tests forcing a fresh execution (bypassing cache) to confirm they aren't stale:
```
go test -race -count=1 -v ./internal/proxy/...
```
All 3 top-level tests + 2 subtests pass for real (observed the upstream-error log line fire on the 502 case, confirming the test actually exercises the closed-port path).

## Per-acceptance table

| # | Acceptance | Verdict | Evidence |
|---|---|---|---|
| A1 | `New` forwards method+path+query to an `httptest.Server` upstream, returns its response | **PASS** | `proxy_test.go:15-62` `TestNew_ForwardsRequest` — POSTs to `/some/path?foo=bar`, upstream captures method/path/query, asserts response status 201 and body round-trip. Ran green. |
| A2 | Unreachable upstream (closed port) → `502` | **PASS** | `proxy_test.go:66-94` `TestNew_UnreachableUpstreamReturns502` — binds+closes a listener for a guaranteed-refused port, asserts `http.StatusBadGateway`. Ran green; `ErrorHandler` (`proxy.go:29-32`) writes only the status header, no body — matches ticket's "never leak the upstream error to the client body." |
| A3 | `New` errors on empty and relative upstream URLs (subtests) | **PASS** | `proxy_test.go:98-118` `TestNew_InvalidUpstream`, table-driven with `t.Run` subtests `empty`/`relative`; asserts non-nil error and nil handler. Ran green. Validation logic (`proxy.go:19-21`) checks `IsAbs()`, scheme ∈ {http,https}, and non-empty `Host` — correctly rejects `""` (empty scheme → `IsAbs()` false) and `/just/a/path` (same). |

## Interface conformance
`package proxy`; `func New(upstream string) (http.Handler, error)` (`proxy.go:14`) — matches the contract verbatim. Returned value is `*httputil.ReverseProxy`, which satisfies `http.Handler`.

## Scope discipline
Commit `30d3ed1` touches exactly `internal/proxy/proxy.go`, `internal/proxy/proxy_test.go`, plus `issues/002-reverse-proxy-handler.md` (Status: `todo`→`wip`, standard `warp contract` bookkeeping per `workflow.md`, not a drive-by source edit). No files outside `scope_files` were modified. **Pass.**

## Standards audit
- Naming/doc comments, no stutter (`proxy.New` not `proxy.NewProxy`), package/dir match — conforms.
- Errors wrapped with `%w` where there's an underlying error (`proxy.go:17`); second validation error has no underlying cause, correctly not wrapped.
- No `panic` in library code — conforms.
- `ErrorHandler` non-nil, forwards to 502 without leaking upstream error text — conforms to DESIGN's fail-closed-adjacent intent for this slice.
- `go test -race` passes — conforms.
- One STANDARDS-letter nit, see non-blocking findings below.

## BLOCKING findings
None.

## NON-BLOCKING findings

`[bug:S3]` `internal/proxy/proxy_test.go:24` — `_, _ = w.Write([]byte(wantBody))` swallows the `Write` error with no justifying comment, which STANDARDS.md's "Check every error. Do not use `_ =` to swallow one without a comment justifying it" technically prohibits. In practice this is an idiomatic, extremely-low-risk pattern (writing to an in-process `httptest` handler; a real failure here would surface as a test assertion failure anyway), so grading it minor/cosmetic rather than load-bearing.

## What was checked beyond the above
Query/path preservation semantics of `httputil.NewSingleHostReverseProxy`'s default director, `Host`-header rewrite correctness, absence of any shared mutable state (race-clean), golangci-lint availability (confirmed genuinely absent from this environment, not silently disabled), and that the implementer's `implement-r1.md` block note (a tooling/permission issue, not a code issue) didn't leave any uncommitted or partial source changes — working tree is clean apart from the ticket's own `Status:` line.

VERDICT: APPROVE