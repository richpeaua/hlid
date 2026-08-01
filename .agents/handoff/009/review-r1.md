Already confirmed via the fresh gate run and `go test -race -count=1 ./...` above (all green). Compiling the final report now.

---

# Review — Ticket 009 (auth middleware + server wiring), round 1

## Independent gate result
Ran `bash scripts/gate.sh` myself (not trusting any claim): `gofmt` clean, `go vet` clean, `golangci-lint` not installed (skip, matches gate's own conditional), `go build ./...` clean, `go test -race ./...` → **all packages PASS**. Re-ran `go test -race -count=1 ./...` (bypassing cache) to confirm a genuine pass, not a stale cache hit — all 7 packages green. Also ran the new/relevant tests individually with `-v`: all subtests pass.

**Gate: PASS (independently verified, not cached).**

## Per-acceptance table

| # | Acceptance | Verdict | Evidence |
|---|---|---|---|
| A1 | Valid unexpired cookie passes through, `IdentityFrom` returns Identity | **PASS** | `TestMiddlewareValidSessionPassesThrough` (middleware_test.go:43-77): asserts `calledNext`, `IdentityFrom` ok+equal, 200. `middleware.go:32-44` implements exactly this path. |
| A2 | Missing/invalid/expired cookie → 302 (nav) / 401 (non-nav), next never called | **PASS** | `TestMiddlewareDeniesUnauthenticated` (middleware_test.go:81-155), 3 cookie cases × 2 header cases = 6 subtests, all pass; `next` asserted never-called via `t.Fatal` in the handler itself. `identityFromRequest` (middleware.go:63-79) covers all three failure modes (no cookie, decrypt failure, expiry). |
| A3 | Via `server.New` w/ auth: `/healthz`,`/auth/login` reachable, unauth proxied request denied, never hits upstream | **PASS** | `TestNewWithAuthDeniesUnauthenticatedProxiedRequest` (server_test.go:218-282), 3 subtests, real `httptest.Server` round trip through the assembled handler chain; `upstreamHit` flag asserted false. |
| A4 | Auth-less `server.New`: Slice-1 behavior unchanged | **PASS** | Diffed `4607037` (pre-009) vs `6bf22e0`: `TestHealthzNotProxied`, `TestNonHealthPathDispatchedToRouter`, `TestPanicRecovered`, `TestNewInvalidConfigErrors`, `TestNewSetsAddrAndTimeouts` are byte-for-byte unchanged, only new tests appended. All still pass. |
| A5 | `server.New` errors on missing session key and on half-set session/provider | **PASS** | `TestNewAuthSetupErrors` (server_test.go:290-329), 3 subtests: missing key, session-only, provider-only — all assert non-nil error. Implemented at `server.go:45-47` (half-set) and `server.go:63-66` (`LoadKey` failure). |
| A6 | Deterministic expiry via injected clock; fakes only, no real network | **PASS** | `middleware_test.go` injects `now func() time.Time` fixed clock throughout. `server_test.go`'s `fakeOIDCDiscovery` (httptest TLS server) + `oidc.ClientContext` binds discovery to the fake client — no real network egress. In-memory `session.NewStore` in both. |

## Interface conformance
`Middleware`, `Handler`, `IdentityFrom` signatures and doc comments match the contract verbatim (middleware.go:25, 48, 56). `server.New`/`NewWithContext` signatures match the ticket's spec (`New` unchanged, delegates to new `NewWithContext(ctx, cfg)` with `context.Background()`).

## Scope discipline
`git show 6bf22e0 --stat` (the 009 wiring commit): touches exactly `internal/auth/middleware.go`, `internal/auth/middleware_test.go`, `internal/server/server.go`, `internal/server/server_test.go`, plus the ticket's own status line — all 4 contract `scope_files`, no drive-by edits. `test/e2e/` and `go.mod`/`go.sum` untouched by this commit. **Clean.**

## Standards audit
Checked against `.agents/STANDARDS.md`: fail-closed (never falls through to `next` on auth failure) ✓; `context.Context` first param on `NewWithContext` ✓; errors wrapped with `%w` and context ✓; no secret/cookie/token logging (`logging` middleware logs only method+path) ✓; exported doc comments start with the identifier ✓; table-driven subtests via `t.Run` ✓; no real network in unit tests ✓; `go test -race` passes ✓.

## BLOCKING findings
None.

## NON-BLOCKING findings

1. **`wantNav` struct field is declared but never read** — `internal/auth/middleware_test.go:95`. The `cases` table declares `wantNav int // status for a navigation request` but every subtest hardcodes `http.StatusFound`/`http.StatusUnauthorized` directly instead of reading `tc.wantNav`, and no test case ever sets it (always zero-value). Reads as if per-case expected codes are configurable; they aren't — a future case relying on `wantNav` to vary the expectation would silently get `0` and the assertion would just ignore it. Failure scenario: someone adds a case expecting a different nav status by setting `wantNav`, the test keeps asserting `http.StatusFound` unconditionally, and the intended check silently no-ops.
`[bug:S3]`

## What was checked beyond the above
Compared old vs new `server_test.go` line-by-line to confirm A4's "unchanged" claim rather than trusting the ticket's own annotation; verified `config.Validate()` doesn't duplicate/conflict with the half-set check that lives in `server.go` (design correctly keeps config-layer optionality separate from server-assembly rejection); checked ServeMux routing precedence to confirm `/healthz` and `/auth/` bypass `Middleware` entirely at the mux level (so `middleware.go`'s own `isExempt` is defense-in-depth for direct/unit-test invocation, not dead in the sense of unreachable-and-wrong — it's exercised directly by `TestMiddlewareExemptPathsPassThrough`); confirmed no golangci-lint available in this environment (consistent with the gate's own skip condition, not a reviewer shortcut).

VERDICT: APPROVE