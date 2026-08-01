# Contract — 007 — config schema: OIDC provider + session

ticket: issues/007-config-provider-session.md
design_refs: DESIGN.md (Config schema; Auth & session decisions)
scope_files:
  - internal/config/config.go
  - internal/config/config_test.go

## Interface (verbatim from ticket)
```
package config
// Session configures the encrypted session cookie.
type Session struct {
    CookieName string        `yaml:"cookie_name"` // e.g. "hlid_session"
    KeyFile    string        `yaml:"key_file"`    // path to base64 32-byte key (env HLID_SESSION_KEY overrides)
    TTL        time.Duration `yaml:"ttl"`         // session lifetime, e.g. 8h
}
// Provider configures the single upstream OIDC identity provider.
type Provider struct {
    Issuer          string   `yaml:"issuer"`            // OIDC discovery base URL (https)
    ClientID        string   `yaml:"client_id"`
    ClientSecretEnv string   `yaml:"client_secret_env"` // env var NAME holding the secret
    RedirectURL     string   `yaml:"redirect_url"`      // absolute https callback URL
    Scopes          []string `yaml:"scopes"`            // must include "openid"
}
// Config gains: Session *Session and Provider *Provider (both yaml:"session"/"provider").
```

## Acceptance
- [ ] A1: `Load` parses a full config (routes + session + provider, TTL like `8h`) into `*Config`, and also parses a valid auth-less config (routes only) — both pass `Validate` (temp-file subtests)
- [ ] A2: With `session` present, `Validate` rejects empty cookie_name, empty key_file, non-positive ttl (one subtest each); omitting `session` entirely is accepted
- [ ] A3: With `provider` present, `Validate` rejects non-https/relative issuer, empty client_id, empty client_secret_env, bad redirect_url, scopes lacking "openid" (one subtest each); omitting `provider` entirely is accepted
- [ ] A4: Omitted `scopes` (provider present) defaults to `[openid, email, profile]`
- [ ] A5: Existing Slice-1 route/listen validation still passes and still rejects its prior bad inputs

## Constraints
See .agents/STANDARDS.md for the binding coding/determinism/purity rules.

## Handoff log
Round records accumulate beside this file: implement-rN.md (implementer record) and review-rN.md (reviewer findings, ending in `VERDICT: APPROVE|REQUEST-CHANGES`).
