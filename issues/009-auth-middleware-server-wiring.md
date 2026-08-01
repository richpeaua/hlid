# 009 — session middleware + server wiring

Parent: DESIGN.md (request lifecycle steps 2-3; Auth & session decisions) | Slice 2 | Type: AFK

## What to build
The auth middleware that gates proxied routes on a valid session, plus wiring the `/auth/login`
and `/auth/callback` endpoints and the middleware into `server.New`. Two disjoint files: a new
middleware file in `auth`, and the existing `server.go`.

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

`server.New` gains auth: build the `session.Store` (LoadKey), `auth.New`, mount `/auth/` to
`a.Handler()`, and wrap the router with `a.Middleware`. `server.New` keeps its signature
`New(cfg *config.Config) (*http.Server, error)`; it now needs a context for discovery — add a
`NewWithContext(ctx, cfg)` and have `New` call it with `context.Background()`.

## Behavior (no decisions)
- A request with no/expired/invalid session cookie to a proxied route does NOT reach the upstream:
  a browser navigation (`Accept: text/html` or `Sec-Fetch-Mode: navigate`) gets `302` to
  `/auth/login`; otherwise `401`. Fail closed.
- A request whose session cookie `Open`s to an unexpired `Identity` reaches the upstream, and the
  `Identity` is retrievable via `IdentityFrom`.
- An expired `Identity` (Expiry ≤ now) is treated as unauthenticated.
- `/healthz` (200 ok) and `/auth/login` `/auth/callback` are reachable without a session.
- Expiry checks use an injected clock so tests are deterministic.

## Acceptance
- [ ] `Middleware`: a valid unexpired session cookie passes through to the wrapped handler and `IdentityFrom` returns the Identity
- [ ] `Middleware`: missing/invalid/expired cookie yields `302 /auth/login` for a navigation request and `401` for a non-navigation request, and never calls next (subtests)
- [ ] `/healthz` and `/auth/login` are reachable with no session cookie (via the assembled `server.New` handler)
- [ ] `server.New` builds the full chain (session store + auth + middleware) from a valid config and errors when auth setup fails (e.g. missing session key); existing Slice-1 server behavior (healthz, routing, panic recovery) still holds
- [ ] Deterministic expiry via injected clock; tests use a fake provider/store, no real network

## Scope files
- internal/auth/middleware.go
- internal/auth/middleware_test.go
- internal/server/server.go
- internal/server/server_test.go

## Blocked by
- issues/008-oidc-auth-handlers.md
