# 010 — E2E: OIDC login flow through Hlid

Status: wip

Parent: DESIGN.md (Slice 2 checkpoint; request lifecycle) | Slice 2 | Type: AFK (E2E checkpoint)

## What to build
A black-box end-to-end test proving the Slice 2 auth spine: an unauthenticated request is
redirected to login, completes the OIDC authorization-code flow against a fake provider, and the
now-authenticated request reaches the upstream. Plus any `cmd/hlid/main.go` wiring the flow needs
(discovery happens at startup; surface errors via `log.Fatal`).

This checkpoint flips the running config to **auth-enforced**. The Slice-1 e2e
(`test/e2e/proxy_test.go`) builds an auth-LESS config and asserts unauthenticated requests reach
the upstream — still valid under ticket 009's active-when-configured wiring, so leave those cases
as the "no-auth passthrough" baseline OR fold them into the auth config by authenticating first.
The NEW behavior (redirect-then-reach) lives in `test/e2e/auth_test.go`.

    // test/e2e/auth_test.go — black-box: real server.New handler + a fake OIDC provider
    //   (httptest server serving /.well-known/openid-configuration, JWKS, and the token endpoint,
    //   signing ID tokens with a test key) + a test upstream.

`cmd/hlid/main.go` stays thin: parse `-config`, `config.Load`, `server.New` (which now performs
OIDC discovery), `ListenAndServe`; `log.Fatal` on any startup error.

## Behavior (no decisions)
- Start-to-finish over the assembled `server.New` handler and a fake provider:
  1. A request to a protected route with no session → `302` to `/auth/login`.
  2. `/auth/login` → `302` to the fake provider's authorization endpoint (state+nonce set).
  3. Simulating the provider redirect back to `/auth/callback?code=...&state=...` → the callback
     exchanges the code, verifies the ID token, sets the session cookie, and `302`s to the app.
  4. Replaying the original request WITH the session cookie → reaches the upstream, body intact.
- `/healthz` → `200 ok` end-to-end (unauthenticated).
- A protected-route request with a tampered/absent session cookie never reaches the upstream.
- `cmd/hlid` builds (`go build ./cmd/hlid`) and `log.Fatal`s on a missing/invalid `-config`.

## Acceptance
- [ ] E2E: unauthenticated protected request → `302 /auth/login`; `/healthz` → `200 ok`
- [ ] E2E: driving the full auth-code flow against a fake OIDC provider yields a session cookie, after which the same request reaches the test upstream with its body intact
- [ ] E2E: a request with a tampered/missing session cookie is denied (never hits the upstream)
- [ ] `go build ./cmd/hlid` succeeds and the binary fails fast (`log.Fatal`, non-zero) on an invalid `-config`

## Scope files
- test/e2e/auth_test.go
- cmd/hlid/main.go

## Blocked by
- issues/009-auth-middleware-server-wiring.md
