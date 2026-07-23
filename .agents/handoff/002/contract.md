# Contract — 002 — reverse-proxy handler

ticket: issues/002-reverse-proxy-handler.md
design_refs: DESIGN.md (components: proxy)
scope_files:
  - internal/proxy/proxy.go
  - internal/proxy/proxy_test.go

## Interface (verbatim from ticket)
```
package proxy
// New returns an http.Handler that reverse-proxies every request to the given upstream base
// URL. Returns an error if upstream is not a valid absolute http/https URL.
func New(upstream string) (http.Handler, error)
```

## Acceptance
- [ ] A1: `New` forwards method+path+query to an `httptest.Server` upstream and returns its response (test)
- [ ] A2: Unreachable upstream returns `502` (point at a closed port; assert status)
- [ ] A3: `New` errors on empty and relative upstream URLs (subtests)

## Constraints
See .agents/STANDARDS.md for the binding coding/determinism/purity rules.

## Handoff log
Round records accumulate beside this file: implement-rN.md (implementer record) and review-rN.md (reviewer findings, ending in `VERDICT: APPROVE|REQUEST-CHANGES`).
