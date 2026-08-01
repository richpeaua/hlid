Gate passes independently. I've read all scope files and the accumulated slice-2 modules (config, session, auth, middleware, server, main, e2e). Here's my review.

---

# E2E Review — Ticket 010 (Slice 2 checkpoint), Round 1

## Independent gate result
`bash scripts/gate.sh` → **PASS** (gofmt clean; `go vet` clean; golangci-lint not installed/skipped; `go build ./...` ok; `go test -race ./...` ok — all packages green, e2e included). Ran it myself; not trusting the implementer claim.

## Per-acceptance verification

| # | Acceptance | Verdict | Evidence |
|---|---|---|---|
| A1 | unauth protected req → `302 /auth/login`; `/healthz` → `200 ok` | **PASS (w/ deviation)** | `TestE2E_UnauthenticatedRedirectsToLogin` (auth_test.go:249): navigation GET `/app/protected` → 302, upstream hits 0, `/healthz`=200 "ok". Note: Location is the **provider authorize endpoint**, not literally `/auth/login` — middleware calls `Login` directly (middleware.go:34), collapsing the ticket's 2-hop description. Matches DESIGN "starts login (302)". See NB2. |
| A2 | full auth-code flow → session cookie → same request reaches upstream, body intact | **PASS** | `TestE2E_FullAuthCodeFlowReachesUpstream` (auth_test.go:305): drives real discovery/JWKS/token/verify against a TLS fake provider (no internals mocked), gets `hlid_session`, replays POST body → upstream echoes `POST:/app/protected:original request body`, hits==1. |
| A3 | tampered/missing session cookie denied, never hits upstream | **PASS** | `TestE2E_DeniedSessionNeverReachesUpstream` (auth_test.go:403): table {missing, tampered} non-navigation → 401, upstream hits 0. Fail-closed confirmed via real `store.Open` rejection (session.go:76). |
| A4 | `go build ./cmd/hlid` ok; binary `log.Fatal` non-zero on invalid `-config` | **PASS** | `TestE2E_AuthMainBuildsAndFailsFastOnInvalidConfig` (auth_test.go:445): builds binary, asserts non-zero exit on (a) missing config file and (b) unreachable OIDC discovery. main.go:16-28 `log.Fatalf` on load/validate/build. |

**E2E spine genuinely exercised:** yes. The test assembles the real `server.NewWithContext` handler → router → `auth.Middleware` → `proxy` → upstream, and runs real OIDC discovery, JWKS fetch, code exchange, and ID-token verification against a TLS `httptest` provider. The only injection is an `*http.Client` (via `oidc.ClientContext` + `BaseContext`) that trusts the fake provider's cert — a legitimate transport seam, not a mock of Hlid internals.

## BLOCKING findings
None. All acceptance items pass, gate is green for real reasons, and no stated invariant (fail-closed, never-forward-unauthenticated, no-secret-logging, deterministic/race-free) is broken.

## NON-BLOCKING findings

**NB1 — `[bug:S2]` middleware.go:34 (× auth.go:113,200) — interactive login loses the original destination.**
The middleware starts login by calling `a.Login(w, r)` with the *original* protected request, but `Login` derives the post-login return path only from `returnPath(r)`, which reads the `redirect_to` **query param** (auth.go:200-206). A middleware-initiated request to `/app/protected` carries no `redirect_to`, so `pre.Path` defaults to `/` and `Callback` 302s the user to `/` (auth.go:194), not back to `/app/protected`. Failure scenario: a user navigates to `/app/reports`, logs in, and lands on `/` — every interactive login drops the requested destination. The E2E doesn't catch it because it manually *replays the original request* (matching the ticket's step-4 model) and never asserts the callback's `Location`. Cross-ticket seam (009 middleware ↔ 008 login/callback); wrong behavior on the common interactive path, no invariant break → S2. Fix would have the middleware thread the original path into the flow (e.g. `redirect_to`).

**NB2 — `[out-of-scope:slice-low]` middleware.go:34 — enforced flow bypasses the `/auth/login` hop.**
The ticket Behavior describes `protected → 302 /auth/login → 302 provider`; the implementation collapses this to a single 302 straight to the provider, so the mounted `GET /auth/login` endpoint (middleware.go:50) is effectively unused by the enforced path. This is correct per DESIGN ("starts login (302)") and arguably better (one fewer hop), but it deviates from A1's literal wording and leaves a dead endpoint. Defer/observe.

**NB3 — `[out-of-scope:slice-low]` config.go:145 — `redirect_url` validation accepts `http`, but DESIGN/field doc say https.**
`Provider.validate` allows `http` or `https` for `redirect_url`, while DESIGN's schema and the field comment (config.go:35) specify an absolute **https** callback URL. The relaxation is what lets the e2e use an `http` httptest callback; harmless now, but a production config could silently configure a plaintext callback. Localized; nothing in-slice depends on it. Defer.

## Notes on what else I checked (no finding)
- Pre-auth cookie is unauthenticated base64 JSON (auth.go:48-52) — acceptable: it's a double-submit CSRF token confined to the caller's own browser, `SameSite=Lax` + `state`/`nonce` compared on callback (auth.go:135,159). Documented and defensible.
- `/healthz` and `/auth/` are mounted directly on the server mux (server.go:55,78), so they never pass through the auth middleware; the middleware's own exempt lists (middleware.go:16-19) are belt-and-suspenders, harmless.
- Session cookie `MaxAge` and `Identity.Expiry` are consistent (auth.go:173,189); `store.Open` fails closed on tamper/short/wrong-key (session.go:76-98); expiry re-checked in middleware (middleware.go:74).
- `main.go` stays thin and `log.Fatal`s on every startup error incl. discovery; `-race` clean.

VERDICT: APPROVE