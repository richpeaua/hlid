# Contract — 009 — session middleware + server wiring

ticket: issues/009-auth-middleware-server-wiring.md
design_refs: DESIGN.md (request lifecycle steps 2-3; Auth & session decisions)
scope_files:
  - internal/auth/middleware.go
  - internal/auth/middleware_test.go
  - internal/server/server.go
  - internal/server/server_test.go

## Interface (verbatim from ticket)
```
package auth
// Middleware gates a handler on a valid, unexpired session cookie. On success it stores the
// Identity in the request context and calls next. On failure it starts login for a browser
// navigation (302 via Login) or returns 401 for a non-navigation request; it never calls next.
// Requests to exempt paths (/healthz, /auth/*) pass through untouched.
func (a *Authenticator) Middleware(next http.Handler) http.Handler
// Handler mounts the auth endpoints: GET /auth/login -> a.Login, GET /auth/callback -> a.Callback.
func (a *Authenticator) Handler() http.Handler
// IdentityFrom returns the Identity stored by Middleware, if any.
func IdentityFrom(ctx context.Context) (session.Identity, bool)
```

## Acceptance
- [ ] A1: `Middleware`: a valid unexpired session cookie passes through to the wrapped handler and `IdentityFrom` returns the Identity
- [ ] A2: `Middleware`: missing/invalid/expired cookie yields `302 /auth/login` for a navigation request and `401` for a non-navigation request, and never calls next (subtests)
- [ ] A3: Via `server.New` with an auth config: `/healthz` and `/auth/login` are reachable with no session cookie; an unauthenticated proxied request is denied (302/401), never reaching the upstream
- [ ] A4: Via `server.New` with an auth-LESS config: existing Slice-1 behavior holds (healthz, routing to upstreams, panic recovery) — the Slice-1 server tests still pass unchanged
- [ ] A5: `server.New` errors when auth setup fails (missing session key) and when exactly one of `session`/`provider` is set
- [ ] A6: Deterministic expiry via injected clock; tests use a fake provider/store, no real network

## Constraints
See .agents/STANDARDS.md for the binding coding/determinism/purity rules.

## Handoff log
Round records accumulate beside this file: implement-rN.md (implementer record) and review-rN.md (reviewer findings, ending in `VERDICT: APPROVE|REQUEST-CHANGES`).
