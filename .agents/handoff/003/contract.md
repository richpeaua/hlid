# Contract — 003 — path-prefix router

ticket: issues/003-path-prefix-router.md
design_refs: DESIGN.md (request lifecycle step 1; components: router)
scope_files:
  - internal/router/router.go
  - internal/router/router_test.go

## Interface (verbatim from ticket)
```
package router
// New builds a Router from routes, constructing one proxy handler per route via proxy.New.
// Returns an error if any route's upstream is invalid (propagated from proxy.New).
func New(routes []config.Route) (*Router, error)
// Router dispatches requests to the proxy of the longest-prefix-matching route.
type Router struct { /* unexported */ }
// ServeHTTP implements http.Handler: longest prefix wins; 404 if no route matches.
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

## Acceptance
- [ ] A1: Longest-prefix match dispatches to the correct upstream (two routes `/` and `/app/`; assert each) 
- [ ] A2: No match returns `404`
- [ ] A3: Match result is independent of the order routes are supplied (shuffle input; same outcome)
- [ ] A4: `New` propagates a `proxy.New` error for a bad upstream

## Constraints
See .agents/STANDARDS.md for the binding coding/determinism/purity rules.

## Handoff log
Round records accumulate beside this file: implement-rN.md (implementer record) and review-rN.md (reviewer findings, ending in `VERDICT: APPROVE|REQUEST-CHANGES`).
