## Independent Step Review — Ticket 007 (round 1)

**Branch:** `ticket-007-config-provider-session` · **Diff reviewed:** commit `eb58a28` ("Add OIDC provider and session config schema") — the only commit modifying source; earlier commits in the branch are contract/handoff records and ticket-006 (already merged separately).

### Independent gate result
Ran `bash scripts/gate.sh` myself (fresh, `go clean -testcache` first, then full gate): **PASS**.
```
gofmt: clean · go vet: clean · golangci-lint: not installed (skipped) · go build: ok
go test -race ./...: ok (config, proxy, router, server, session, e2e all green)
```
Also ran `go test -race -v ./internal/config/...` directly — every subtest listed in the acceptance criteria executed and passed (not just a package-level "ok").

### Scope audit
Commit `eb58a28` touches exactly:
- `internal/config/config.go` (contract scope_files ✓)
- `internal/config/config_test.go` (contract scope_files ✓)
- `issues/007-config-provider-session.md` (status-line bump only — standard, not a scope violation)

No drive-by edits elsewhere. Diff to `config.go` is purely additive (new types/fields/methods); the pre-existing `Validate`/`Load` logic for `listen`/`routes` is untouched byte-for-byte aside from gofmt realignment of the `Config` struct tags.

### Per-acceptance table

| # | Acceptance | Verdict | Evidence |
|---|---|---|---|
| A1 | `Load` parses full config (routes+session+provider, TTL `8h`) and auth-less config; both pass `Validate` | **PASS** | `TestLoad_ParsesSessionAndProvider` subtests `full_config_with_session_and_provider`, `auth-less_config_with_routes_only` — both green, field values asserted, `Validate()` checked nil |
| A2 | `session` present → rejects empty cookie_name/key_file/non-positive ttl (1 subtest each); omitted → accepted | **PASS** | `TestValidate_Session`: 5 subtests (`session_omitted`, `valid_session`, `empty_cookie_name`, `empty_key_file`, `non-positive_ttl`) all green, `config.go:116-127` |
| A3 | `provider` present → rejects non-https/relative issuer, empty client_id/client_secret_env, bad redirect_url, scopes lacking openid; omitted → accepted | **PASS** | `TestValidate_Provider`: 8 subtests including separate `relative_issuer` and `non-https_issuer` cases, `config.go:130-160` |
| A4 | Omitted `scopes` (provider present) defaults to `[openid, email, profile]` | **PASS** | `TestLoad_ProviderDefaultScopes`; default applied in `Load` before `Validate` (`config.go:59-61`), defensive copy via `append([]string(nil), defaultScopes...)` |
| A5 | Existing Slice-1 route/listen validation unchanged and still rejects prior bad inputs | **PASS** | `TestValidate`, `TestLoad_ParsesValidYAML`, `TestLoad_Errors` present unmodified (diff shows 0 deletions in these); all green |

### Interface conformance
`Session`, `Provider` structs and `Config.Session`/`Config.Provider` fields match the contract's verbatim interface exactly (field names, yaml tags, types, `*Session`/`*Provider` pointer optionality). `Config.Validate()` calls both sub-validators only when non-nil, matching "validated only when present."

### Invariant check (DESIGN.md / ticket)
- "Secret never read/stored in config" — confirmed: `ClientSecretEnv` is only a name, never dereferenced. ✓
- "Half-set auth config (session xor provider) rejected" (DESIGN.md Auth & session decisions) — correctly **not** implemented here; ticket 007 explicitly assigns that check to ticket 009 ("Auth is made mandatory at server assembly, not here"). Not a gap in this ticket's scope.
- Fail-closed / no panics / errors as last value / `%w` wrapping only where wrapping an underlying error — all followed per STANDARDS.md.

### BLOCKING findings
None.

### NON-BLOCKING findings

1. **[bug:S3]** `internal/config/config.go:35` — the `Provider.RedirectURL` field's doc comment reads `// absolute https callback URL`, but `provider.validate()` (config.go:144-147) accepts either `http` or `https` (matching the ticket's own Behavior section, which says "http(s)"). The comment (copied verbatim from the ticket's Interface block, which itself uses shorthand) is inconsistent with the actual runtime-enforced rule and could mislead a future reader into believing http redirect URLs are rejected. No test or invariant is affected — implementation is behaviorally correct per the contract's Behavior section, this is a doc/code drift only.

Nothing else surfaced: reviewed all 5 acceptance items against code+tests, scope discipline, interface conformance, naming (no stutter, doc comments start with identifier name), error handling/wrapping, and the fail-closed/no-secret-read invariants. Gate run independently and fresh, not trusted from a claim.

VERDICT: APPROVE