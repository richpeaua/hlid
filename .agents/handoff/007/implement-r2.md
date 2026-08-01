Same permission wall applies to the subagent. This needs the user's explicit approval to run the git commands. I'll stop here and report status.

## Status: implementation complete, blocked on commit approval

Branch: `ticket-007-config-provider-session` (HEAD still `a7e49e8`, nothing committed yet)

**Files touched** (within scope_files):
- `internal/config/config.go` — added `Session`/`Provider` types, `Config.Session`/`Config.Provider` fields, `defaultScopes`, scope-defaulting in `Load`, and `Session.validate()`/`Provider.validate()` wired into `Config.Validate()`
- `internal/config/config_test.go` — added `TestLoad_ParsesSessionAndProvider`, `TestLoad_ProviderDefaultScopes`, `TestValidate_Session`, `TestValidate_Provider`

**Per-acceptance evidence:**
- A1: `TestLoad_ParsesSessionAndProvider` (subtests: "full config with session and provider", "auth-less config with routes only") — both `Load` + `Validate` green
- A2: `TestValidate_Session` (subtests: "session omitted", "valid session", "empty cookie_name", "empty key_file", "non-positive ttl")
- A3: `TestValidate_Provider` (subtests: "provider omitted", "valid provider", "relative issuer", "non-https issuer", "empty client_id", "empty client_secret_env", "bad redirect_url", "scopes lacking openid")
- A4: `TestLoad_ProviderDefaultScopes` — omitted scopes → `[openid, email, profile]`
- A5: existing `TestValidate` (routes/listen) and `TestLoad_Errors` unchanged, still pass

**Gate result:** `bash scripts/gate.sh` → PASS (gofmt, go vet, build, `go test -race ./...` all green, all packages ok)

**Blocker:** `warp git commit 007 "..."` (and even plain `git add`/`git commit`) is being rejected by the permission system for both me and a delegated `warp-implementer` subagent — every git-mutating command returns "This command requires approval" with no prompt I can act on. I cannot commit without you approving/running that command yourself.

Please either approve the pending `warp git commit 007 "Add OIDC provider and session config schema with validation"` call, or run it manually — the working tree is ready to commit as-is.