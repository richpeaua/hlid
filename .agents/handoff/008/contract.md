# Contract — 008 — OIDC auth handlers (login + callback)

ticket: issues/008-oidc-auth-handlers.md
design_refs: DESIGN.md (components: auth; Auth & session decisions)
scope_files:
  - internal/auth/auth.go
  - internal/auth/auth_test.go

## Interface (verbatim from ticket)
```
package auth
// Authenticator drives the OIDC authorization-code flow for one provider.
type Authenticator struct { /* verifier *oidc.IDTokenVerifier; oauth2 oauth2.Config; store *session.Store; sess config.Session */ }
// New builds an Authenticator: OIDC discovery on prov.Issuer, an ID-token verifier bound to
// prov.ClientID, an oauth2.Config (secret read from os.Getenv(prov.ClientSecretEnv)), and the
// session store. ctx is the discovery context.
func New(ctx context.Context, prov config.Provider, sess config.Session, store *session.Store) (*Authenticator, error)
// Login starts the flow: sets a short-lived pre-auth cookie binding random state+nonce, then
// 302-redirects to the provider's authorization endpoint (AuthCodeURL with state + nonce).
func (a *Authenticator) Login(w http.ResponseWriter, r *http.Request)
// Callback completes the flow: verifies state against the pre-auth cookie, exchanges the code,
// verifies the ID token + nonce, extracts sub/email, seals a session.Identity (Expiry = now+ttl)
// into the session cookie, clears the pre-auth cookie, and 302-redirects to the original path
// (from the pre-auth cookie, default "/").
func (a *Authenticator) Callback(w http.ResponseWriter, r *http.Request)
```

## Acceptance
- [ ] A1: `New` errors on a missing client secret env and on unreachable discovery; succeeds against a fake OIDC discovery server
- [ ] A2: `Login` sets the pre-auth cookie and returns `302` to an authorization URL carrying the expected `client_id`, `redirect_uri`, `scope`, and the cookie's `state`+`nonce` (parse the Location)
- [ ] A3: `Callback` with a valid code + matching state + valid ID token (nonce match) sets a session cookie that `session.Store.Open` decrypts to the expected `Identity`, and `302`s to the saved path
- [ ] A4: `Callback` fails closed (no session cookie, non-2xx/302-to-upstream) on: missing pre-auth cookie, state mismatch, exchange failure, bad ID token, nonce mismatch (subtests)
- [ ] A5: Deterministic `Expiry` via an injected clock; a table-driven fake provider (issuer + JWKS + token endpoint) backs the tests with no real network

## Constraints
See .agents/STANDARDS.md for the binding coding/determinism/purity rules.

## Handoff log
Round records accumulate beside this file: implement-rN.md (implementer record) and review-rN.md (reviewer findings, ending in `VERDICT: APPROVE|REQUEST-CHANGES`).
