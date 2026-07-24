# Contract — 004 — server assembly + healthz

ticket: issues/004-server-assembly-healthz.md
design_refs: DESIGN.md (components: server; request lifecycle)
scope_files:
  - internal/server/server.go
  - internal/server/server_test.go

## Interface (verbatim from ticket)
```
package server
// New builds the top-level http.Server from cfg: a mux that serves GET /healthz -> 200 "ok"
// and delegates everything else to a router.Router built from cfg.Routes. Applies the base
// middleware chain (request logging placeholder + panic recovery). Sets Addr from cfg.Listen
// and sane ReadHeaderTimeout/IdleTimeout.
func New(cfg *config.Config) (*http.Server, error)
```

## Acceptance
- [ ] A1: `GET /healthz` -> 200 "ok" (via `httptest.NewServer(srv.Handler)`), and is not routed to an upstream
- [ ] A2: A non-health path is dispatched through the router to the matching upstream
- [ ] A3: A panicking test handler is recovered as `500`
- [ ] A4: `New` errors when `cfg` is invalid; `Addr`/timeouts are set on success

## Constraints
See .agents/STANDARDS.md for the binding coding/determinism/purity rules.

## Handoff log
Round records accumulate beside this file: implement-rN.md (implementer record) and review-rN.md (reviewer findings, ending in `VERDICT: APPROVE|REQUEST-CHANGES`).
