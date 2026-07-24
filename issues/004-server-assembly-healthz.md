# 004 — server assembly + healthz

Status: done - https://github.com/richpeaua/hlid/pull/6
Parent: DESIGN.md (components: server; request lifecycle) | Slice 1 | Type: AFK

## What to build
Assemble the router, a base middleware chain, and a health endpoint into a ready `*http.Server`.

    package server

    // New builds the top-level http.Server from cfg: a mux that serves GET /healthz -> 200 "ok"
    // and delegates everything else to a router.Router built from cfg.Routes. Applies the base
    // middleware chain (request logging placeholder + panic recovery). Sets Addr from cfg.Listen
    // and sane ReadHeaderTimeout/IdleTimeout.
    func New(cfg *config.Config) (*http.Server, error)

Middleware are `func(http.Handler) http.Handler`; provide at least a `recover` middleware that
turns a handler panic into `500` (fail closed, no stack to the client).

## Behavior (no decisions)
- `GET /healthz` returns `200` with body `ok`, and is NOT proxied.
- All other paths are handled by the router (longest-prefix proxy, else 404).
- A panic in a downstream handler is recovered as `500` (does not crash the server).
- `New` returns an error if `cfg.Validate()` fails or the router can't be built.
- `Addr` == `cfg.Listen`; `ReadHeaderTimeout` and `IdleTimeout` are set (non-zero).

## Acceptance
- [ ] `GET /healthz` -> 200 "ok" (via `httptest.NewServer(srv.Handler)`), and is not routed to an upstream
- [ ] A non-health path is dispatched through the router to the matching upstream
- [ ] A panicking test handler is recovered as `500`
- [ ] `New` errors when `cfg` is invalid; `Addr`/timeouts are set on success

## Scope files
- internal/server/server.go
- internal/server/server_test.go

## Blocked by
- issues/003-path-prefix-router.md
