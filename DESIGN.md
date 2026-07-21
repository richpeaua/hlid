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

## Config schema (v0, grows per slice)
```yaml
listen: ":8443"
routes:
  - path: /app/          # longest-prefix match
    upstream: http://127.0.0.1:9001
  - path: /
    upstream: http://127.0.0.1:9000
# added in later slices:
# providers: [...]   session: {...}   policy: [...]
```

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
