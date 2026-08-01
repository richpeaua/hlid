# 008 — OIDC auth handlers (login + callback)

Status: done - https://github.com/richpeaua/hlid/pull/15

Parent: DESIGN.md (components: auth; Auth & session decisions) | Slice 2 | Type: AFK

## What to build
The OIDC authorization-code flow: an `Authenticator` built from provider config via discovery,
a login handler that redirects to the provider, and a callback handler that exchanges the code,
verifies the ID token, and seals a session via the ticket-006 store.

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

Use `github.com/coreos/go-oidc/v3/oidc` + `golang.org/x/oauth2` (`go get` both; commit
go.mod/go.sum). Inject a clock for `Expiry` (`func() time.Time`, defaulting to `time.Now`) so
tests are deterministic. Scopes come from `prov.Scopes`.

## Behavior (no decisions)
- `New` errors if discovery fails or `os.Getenv(prov.ClientSecretEnv)` is empty (fail closed).
- `Login` writes a `state` (random, CSRF) + `nonce` (random, replay) + the pre-auth original
  path into an httponly, secure, short-TTL pre-auth cookie, and redirects (`302`) to a URL whose
  `state`/`nonce`/`client_id`/`redirect_uri`/`scope` match the config.
- `Callback` fails closed (`400`/`401`, never proxies) when: the pre-auth cookie is missing, the
  `state` query param ≠ the cookie's state, the token exchange fails, ID-token verification fails,
  or the token `nonce` ≠ the cookie's nonce.
- On success, `Callback` sets the session cookie (name/TTL from `config.Session`, httponly+secure)
  to a `store.Seal(Identity{sub,email,now+ttl})`, deletes the pre-auth cookie, and `302`s to the
  saved original path.
- Never logs the code, token, secret, or cookie values.

## Acceptance
- [ ] `New` errors on a missing client secret env and on unreachable discovery; succeeds against a fake OIDC discovery server
- [ ] `Login` sets the pre-auth cookie and returns `302` to an authorization URL carrying the expected `client_id`, `redirect_uri`, `scope`, and the cookie's `state`+`nonce` (parse the Location)
- [ ] `Callback` with a valid code + matching state + valid ID token (nonce match) sets a session cookie that `session.Store.Open` decrypts to the expected `Identity`, and `302`s to the saved path
- [ ] `Callback` fails closed (no session cookie, non-2xx/302-to-upstream) on: missing pre-auth cookie, state mismatch, exchange failure, bad ID token, nonce mismatch (subtests)
- [ ] Deterministic `Expiry` via an injected clock; a table-driven fake provider (issuer + JWKS + token endpoint) backs the tests with no real network

## Scope files
- internal/auth/auth.go
- internal/auth/auth_test.go

## Blocked by
- issues/006-session-cookie-store.md
- issues/007-config-provider-session.md
