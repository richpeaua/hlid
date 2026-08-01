# Contract — 010 — E2E: OIDC login flow through Hlid

ticket: issues/010-e2e-oidc-login.md
design_refs: DESIGN.md (Slice 2 checkpoint; request lifecycle)
scope_files:
  - cmd/hlid/main.go
  - test/e2e/auth_test.go

## Interface (verbatim from ticket)
```
// test/e2e/auth_test.go — black-box: real server.New handler + a fake OIDC provider
//   (httptest server serving /.well-known/openid-configuration, JWKS, and the token endpoint,
//   signing ID tokens with a test key) + a test upstream.
```

## Acceptance
- [ ] A1: E2E: unauthenticated protected request → `302 /auth/login`; `/healthz` → `200 ok`
- [ ] A2: E2E: driving the full auth-code flow against a fake OIDC provider yields a session cookie, after which the same request reaches the test upstream with its body intact
- [ ] A3: E2E: a request with a tampered/missing session cookie is denied (never hits the upstream)
- [ ] A4: `go build ./cmd/hlid` succeeds and the binary fails fast (`log.Fatal`, non-zero) on an invalid `-config`

## Constraints
See .agents/STANDARDS.md for the binding coding/determinism/purity rules.

## Handoff log
Round records accumulate beside this file: implement-rN.md (implementer record) and review-rN.md (reviewer findings, ending in `VERDICT: APPROVE|REQUEST-CHANGES`).
