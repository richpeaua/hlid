# Hlid — Design

**Hlid** is an identity-aware proxy (IAP): a reverse proxy that sits in front of internal HTTP
services, authenticates the caller, authorizes the request against per-route policy, and forwards
it to the upstream carrying a trusted identity. It replaces per-app auth with one enforcement point
at the edge (the BeyondCorp / Pomerium / oauth2-proxy model).

> "Hlíð" (Old Norse: a slope/gate-side) — a watched gate in front of the services.

## Goals
- One authenticating reverse proxy in front of many upstreams, config-driven.
- Standard OIDC/OAuth2 login; encrypted session cookies; no per-app auth code.
- Per-route authorization policy (who may reach which upstream).
- Forward a verifiable identity to upstreams (signed header / JWT), never the raw provider token.
- Fail closed, deterministic, race-free; observable.

## Non-goals (for now)
- Not a WAF, not an API gateway with rate-limiting/transforms, not a service mesh.
- No mTLS to upstreams in the first milestones (plain HTTP to trusted internal upstreams).
- No multi-tenant control plane; single static config file.

## Request lifecycle
```
client ──▶ [ server ]
             │  1. router: longest path-prefix match ──▶ route (or 404)
             │  2. session middleware: load/verify the session cookie
             │  3. auth: if unauthenticated ──▶ start OIDC login (302) / handle callback
             │  4. policy: is this identity allowed on this route?  no ──▶ 403
             │  5. identity headers: stamp verified identity onto the request
             ▼
          [ proxy ] ──▶ upstream ──▶ response ──▶ client
```
Slices build this spine outside-in: proxy first (steps 1+6), then auth (2-3), then policy (4),
then identity injection (5).

## Components (packages under `internal/`)
- **config** - load + validate the YAML config (listen addr, routes, providers, session, policy).
- **router** - longest-path-prefix match of a request to a route; 404 on no match.
- **proxy** - a per-route `httputil.ReverseProxy` to the route's upstream.
- **server** - assemble router + middleware chain + `/healthz` into an `*http.Server`.
- **session** (later) - encrypted, signed cookie session store; load/save identity.
- **auth** (later) - OIDC discovery, authorization-code login, callback, token verification.
- **policy** (later) - evaluate per-route allow rules against the session identity.
- **identity** (later) - mint the signed upstream identity header / JWT.

## Config schema (grows per slice)
```yaml
listen: ":8443"
routes:
  - path: /app/          # longest-prefix match
    upstream: http://127.0.0.1:9001
  - path: /
    upstream: http://127.0.0.1:9000

# Slice 2 — OIDC authentication:
session:
  cookie_name: hlid_session   # session cookie name
  key_file: /etc/hlid/session.key  # 32-byte AES-256 key, base64 in a file (env HLID_SESSION_KEY overrides)
  ttl: 8h                     # session lifetime
provider:                     # single OIDC provider (v0; a list may come later)
  issuer: https://accounts.example.com  # OIDC discovery base (must serve /.well-known/openid-configuration)
  client_id: hlid
  client_secret_env: HLID_CLIENT_SECRET # env var name holding the secret; never inline
  redirect_url: https://hlid.example.com/auth/callback
  scopes: [openid, email, profile]

# added in later slices:
# policy: [...]
```

## Auth & session decisions (Slice 2, binding)
- **OIDC**: `github.com/coreos/go-oidc/v3` + `golang.org/x/oauth2` handle discovery, JWKS
  rotation, and ID-token verification. We do not hand-roll JWKS/token verification.
- **Session cookie**: hand-rolled `internal/session` using **AES-256-GCM** authenticated
  encryption (confidentiality + integrity in one primitive) over the serialized identity. The
  32-byte key comes from env/file (never config-in-git), fail-closed on a missing/short key.
- **Login endpoints** live under a fixed reserved prefix `/auth/` (`/auth/login`,
  `/auth/callback`) that the router never proxies. Auth-code flow carries `state` (CSRF) and
  `nonce` (ID-token replay), both bound to the pre-auth cookie and verified on callback.
- **Auth middleware**: loads/verifies the session cookie; on an unauthenticated request it
  starts login (`302` for a browser navigation, else `401`) and never falls through to an
  upstream. `/healthz` and `/auth/*` are exempt.

## Security model
- Fail closed: any auth/session/policy error denies (401/403); it never forwards.
- Upstreams trust ONLY Hlid's stamped identity header (signed), never client-supplied identity
  headers - the proxy strips inbound `X-Hlid-*` before stamping.
- Secrets (cookie keys, client secrets) come from env/file, never the config-in-git or logs.

## Slice plan (E2E checkpoint per slice; the `## Slices` list in issues/README.md is authoritative)
1. **Proxy spine** - config + router + proxy + server; E2E: a request routes through Hlid to a
   test upstream and back. (No auth yet.)
2. **OIDC authentication** - session cookies + OIDC login/callback; E2E: an unauthenticated request
   is redirected to login and, once authenticated, reaches the upstream.
3. **Authorization policy** - per-route allow rules; E2E: an authenticated-but-unauthorized user
   gets 403; an allowed user passes.
4. **Identity injection** - signed identity header to upstream + inbound `X-Hlid-*` stripping;
   E2E: the upstream receives and verifies the stamped identity.
5. **Hardening** - TLS, structured logging/metrics, graceful shutdown, config reload.
