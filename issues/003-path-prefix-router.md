# 003 — path-prefix router

Status: todo
Parent: DESIGN.md (request lifecycle step 1; components: router) | Slice 1 | Type: AFK

## What to build
A router that matches a request to a route by longest path prefix and dispatches to that route's
proxy handler.

    package router

    // New builds a Router from routes, constructing one proxy handler per route via proxy.New.
    // Returns an error if any route's upstream is invalid (propagated from proxy.New).
    func New(routes []config.Route) (*Router, error)

    // Router dispatches requests to the proxy of the longest-prefix-matching route.
    type Router struct { /* unexported */ }

    // ServeHTTP implements http.Handler: longest prefix wins; 404 if no route matches.
    func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request)

## Behavior (no decisions)
- Longest matching `Route.Path` prefix wins (e.g. `/app/` beats `/` for `/app/x`).
- No matching prefix -> `404 Not Found`.
- Matching order is deterministic regardless of input route order (sort by path length desc, then
  lexicographically, at construction).
- `New` returns the first `proxy.New` error encountered.

## Acceptance
- [ ] Longest-prefix match dispatches to the correct upstream (two routes `/` and `/app/`; assert each) 
- [ ] No match returns `404`
- [ ] Match result is independent of the order routes are supplied (shuffle input; same outcome)
- [ ] `New` propagates a `proxy.New` error for a bad upstream

## Scope files
- internal/router/router.go
- internal/router/router_test.go

## Blocked by
- issues/001-config-schema-loader.md
- issues/002-reverse-proxy-handler.md
