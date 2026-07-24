# Contract — 005 — E2E: request routes through Hlid to a test upstream

ticket: issues/005-e2e-proxy-spine.md
design_refs: DESIGN.md (Slice 1 checkpoint; request lifecycle)
scope_files:
  - cmd/hlid/main.go
  - test/e2e/proxy_test.go

## Interface (verbatim from ticket)
```
// cmd/hlid/main.go
package main
// main: parse a -config flag (default "hlid.yaml"), config.Load + Validate, server.New,
// then srv.ListenAndServe(). Fatal (log.Fatal) on any startup error.
func main()
// test/e2e/proxy_test.go — black-box integration test (no internals mocked).
```

## Acceptance
- [ ] A1: E2E: two `httptest.Server` upstreams; a temp config routes `/app/`->A and `/`->B; build the
- [ ] A2: E2E: `/healthz` -> 200 "ok"
- [ ] A3: E2E: an unrouted path -> 404
- [ ] A4: `cmd/hlid` builds (`go build ./cmd/hlid`) and fails fast on an invalid `-config`

## Constraints
See .agents/STANDARDS.md for the binding coding/determinism/purity rules.

## Handoff log
Round records accumulate beside this file: implement-rN.md (implementer record) and review-rN.md (reviewer findings, ending in `VERDICT: APPROVE|REQUEST-CHANGES`).
