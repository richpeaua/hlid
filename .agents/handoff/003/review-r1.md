## Independent Review — Ticket 003 (path-prefix router), Round 1

**Gate (run myself):** `bash scripts/gate.sh` → gofmt clean, `go vet` clean, golangci-lint skipped (not installed — script-defined fallback), `go build ./...` clean, `go test -race ./...` → **PASS** (all packages, including `internal/router` 4/4 tests green, confirmed individually with `-v`).

**Scope check:** `git show --stat` on the ticket's commit (`0381312`) touches only `internal/router/router.go`, `internal/router/router_test.go`, and the ticket's own status line — matches `scope_files` exactly. No drive-by edits.

### Per-acceptance table

| # | Criterion | Evidence | Result |
|---|---|---|---|
| A1 | Longest-prefix match dispatches to correct upstream (`/`, `/app/`; assert each) | `TestNew_LongestPrefixDispatchesCorrectUpstream` (router_test.go:25-57): `/app/x`→app, `/other`→root, both asserted via distinct upstream response bodies | **PASS** |
| A2 | No match → 404 | `TestServeHTTP_NoMatchReturns404` (router_test.go:60-78): only `/app/` route, request `/other` → `http.StatusNotFound` asserted | **PASS** |
| A3 | Match independent of route input order | `TestNew_MatchIndependentOfRouteOrder` (router_test.go:82-123): 3 routes, 5 seeded shuffles, same dispatch outcome each time; `New`'s `sort.Slice` by len desc then lexicographic (router.go:37-42) makes this deterministic | **PASS** |
| A4 | `New` propagates `proxy.New` error for bad upstream | `TestNew_PropagatesProxyError` (router_test.go:126-133): `"not-an-absolute-url"` → non-nil error; router.go:30-33 wraps and returns immediately | **PASS** |

### Interface conformance
`New(routes []config.Route) (*Router, error)`, unexported `Router` struct, `ServeHTTP(w, r)` all match the contract verbatim. Doc comments on exported identifiers (`New`, `Router`, `ServeHTTP`) start with the identifier name per STANDARDS. Errors wrapped with `%w` and context (`"router: route %q: %w"`). No panics, no swallowed errors in library code.

### Standards / invariant audit
- Prefix-match semantics (`strings.HasPrefix`, no segment-boundary requirement) match DESIGN.md's stated behavior verbatim ("`/app/` beats `/` for `/app/x`") — not a defect, it's the specified design.
- Sort key (length, then lexicographic) is a total order given `config.Validate` dedupes paths upstream, so `sort.Slice`'s non-stability is immaterial — no latent tie-break bug.
- Router is read-only after construction (`entries` never mutated in `ServeHTTP`) — safe under `-race`, confirmed by the race-enabled gate run.
- Package/dir naming, test naming (`TestXxx`, table-driven subtests via `t.Run`) conform.

### BLOCKING findings
None.

### NON-BLOCKING findings
- `router_test.go:17` — `_, _ = w.Write([]byte(body))` swallows the write error with no justifying comment, which STANDARDS.md phrases as a blanket rule ("Do not use `_ =` to swallow one without a comment justifying it"). This exact pattern already exists, previously reviewed/merged, at `internal/proxy/proxy_test.go:24`, so this ticket didn't introduce the convention — it followed existing precedent. `[bug:S3]` — cosmetic, zero blast radius (test-only helper, error is never actionable in an `httptest` writer), consistent with an already-accepted codebase pattern.

Nothing else surfaced: reviewed proxy/config dependency boundaries (correct layering per DESIGN.md), determinism, error-wrapping, and test-file scope — all conform.

VERDICT: APPROVE