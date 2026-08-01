## Blocked — genuine scope/gate conflict in ticket 007

**What I did:** Implemented `internal/config/config.go` (Session/Provider types, Config.Session/Provider fields, Validate() session+provider checks, default-scopes) and wrote/updated `internal/config/config_test.go` per acceptance A1–A5. The `config` package itself is fully green:

```
ok  	.../internal/config	1.353s
```

**The blocker:** Ticket 007 explicitly scopes to `internal/config/{config,config_test}.go` only ("Extends the existing `config` package only; no other package changes") and requires `Validate` to reject any config with a nil `Session` or `Provider` (A2). But `internal/server.New` calls `cfg.Validate()` directly, and its tests (`internal/server/server_test.go`) plus `test/e2e/proxy_test.go` build/load `Config` values that have no `session`/`provider` block — those are Slice-1 fixtures outside 007's scope_files. With A2 correctly implemented, `bash scripts/gate.sh` now fails on the full-repo test run (`go test -race ./...`), even though every acceptance item in the contract is met:

```
--- FAIL: TestHealthzNotProxied ... session: must not be empty
--- FAIL: TestNonHealthPathDispatchedToRouter ... session: must not be empty
--- FAIL: TestNewSetsAddrAndTimeouts ... session: must not be empty
--- FAIL: TestE2E_RoutesToUpstream ... session: must not be empty
--- FAIL: TestE2E_Healthz ... session: must not be empty
--- FAIL: TestE2E_UnroutedPath ... session: must not be empty
```

Fixing those fixtures would require editing `internal/server/server_test.go` and `test/e2e/proxy_test.go`, both outside `scope_files` — a scope violation per my charter. Leaving them broken means the gate can never go green while strictly honoring scope. This is the "genuinely underspecified / conflicts with design" case the charter says to stop on, not guess.

**Question for the orchestrator:** should 007's scope include updating the Slice-1 test fixtures in `internal/server/server_test.go` and `test/e2e/proxy_test.go` (adding a valid session/provider block) so the full gate stays green, or is there a separate ticket that's supposed to land first/alongside to absorb that consumer-side update?

**Tooling note:** I could not execute `warp git block 007 "<question>"` (or any `warp ...` invocation, including `--help`) — every call returned "This command requires approval" with no way for me to grant it in this session. Plain `git` commands work fine. Someone with warp access needs to run:
```
warp git block 007 "Slice-1 fixtures in internal/server/server_test.go and test/e2e/proxy_test.go construct Config without session/provider; with A2 (Validate rejects nil session/provider) implemented, cfg.Validate()/server.New()/config.Load() calls in those out-of-scope files now fail, so go test -race ./... cannot go green without touching files outside 007's scope_files. Is updating those fixtures in-scope for 007, or handled by another ticket?"
```

**No commit made** — the gate is not green and I did not commit. Working tree has the implementation uncommitted on branch `ticket-007-config-provider-session`, ready to resume once the scope question is answered.

**Per-acceptance evidence (config package only, all passing):**
- A1: `TestLoad_ParsesValidYAML` (both subtests now assert Session/Provider parsing incl. `ttl: 8h`)
- A2: `TestValidate_Session` (subtests: valid config, missing session, missing provider, empty cookie_name, empty key_file, zero ttl, negative ttl)
- A3: `TestValidate_Provider` (subtests: valid config, http issuer, relative issuer, empty issuer, empty client_id, empty client_secret_env, empty/relative/bad redirect_url, scopes missing openid)
- A4: `TestValidate_ScopesDefault` (asserts default `[openid email profile]` applied and full config validates)
- A5: `TestValidate` (existing Slice-1 route/listen subtests, now carrying valid session/provider, all still pass/fail as before)

Files touched: `internal/config/config.go`, `internal/config/config_test.go` (both in scope_files).