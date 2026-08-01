# 007 — config schema: OIDC provider + session

Parent: DESIGN.md (Config schema; Auth & session decisions) | Slice 2 | Type: AFK

## What to build
Grow the config schema with the OIDC `provider` block and the `session` block, plus validation.
Extends the existing `config` package only; no other package changes.

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

Add to `Config.Validate()`. `TTL` uses yaml.v3's native `time.Duration` decoding.

## Behavior (no decisions)
- `Validate` requires a non-nil `session` and `provider` (auth is now part of the spine).
- Session rules: `CookieName` non-empty; `KeyFile` non-empty; `TTL > 0`.
- Provider rules: `Issuer` a parseable absolute `https` URL; `ClientID`, `ClientSecretEnv`,
  `RedirectURL` non-empty; `RedirectURL` a parseable absolute `http(s)` URL; `Scopes` contains
  `"openid"` (defaulting to `[openid, email, profile]` when omitted).
- The secret is NOT read here and never appears in config — only the env var name is stored.
- All existing Slice-1 validation (listen, routes, upstreams) is preserved unchanged.

## Acceptance
- [ ] `Load` parses a full config (routes + session + provider, TTL like `8h`) into `*Config` (temp-file test)
- [ ] `Validate` rejects: missing session, missing provider, empty cookie_name, empty key_file, non-positive ttl (one subtest each)
- [ ] `Validate` rejects: non-https/relative issuer, empty client_id, empty client_secret_env, bad redirect_url, scopes lacking "openid" (one subtest each)
- [ ] Omitted `scopes` defaults to `[openid, email, profile]`; a valid full config passes `Validate`
- [ ] Existing Slice-1 route/listen validation still passes and still rejects its prior bad inputs

## Scope files
- internal/config/config.go
- internal/config/config_test.go

## Blocked by
None — edits only the config package; disjoint from 006.
