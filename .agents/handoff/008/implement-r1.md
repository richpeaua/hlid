I'll stop retrying `warp git commit` since it needs your interactive approval and isn't going through. Implementation is done and gate is green — waiting on you to approve the commit command (or tell me to run plain `git commit` instead).

**Status: ready to commit, blocked on approval for `warp git commit 008 "Implement OIDC login and callback handlers"`**

- branch: `ticket-008-oidc-auth-handlers`
- commit hash: none yet (pending approval)
- files touched: `internal/auth/auth.go`, `internal/auth/auth_test.go` (new); `go.mod`, `go.sum` (deps: go-oidc/v3, go-jose/v4, x/oauth2 per ticket instruction); `issues/008-oidc-auth-handlers.md` (Status: wip, pre-existing from earlier dispatch)
- per-acceptance evidence:
  - A1: `TestNew` — subtests `missing_client_secret_env`, `unreachable_discovery`, `succeeds_against_fake_discovery_server`
  - A2: `TestLogin` — asserts 302, pre-auth cookie httponly+secure, Location `client_id`/`redirect_uri`/`scope` match config and `state`/`nonce` match cookie
  - A3: `TestCallbackSuccess` — valid code/state/ID-token → session cookie decrypts via `session.Store.Open` to expected `Identity`, 302 to saved path, pre-auth cookie cleared
  - A4: `TestCallbackFailsClosed` — subtests `missing_pre-auth_cookie`, `state_mismatch`, `exchange_failure`, `bad_id_token_(signature)`, `nonce_mismatch`, all asserting 4xx + no session cookie
  - A5: injected `a.now` clock verified against `Identity.Expiry`; `fakeProvider` (discovery + JWKS + token endpoint via `httptest`) backs all tests, no real network
- gate result: `bash scripts/gate.sh` → PASS (gofmt clean, go vet clean, build OK, `go test -race -count=1 ./...` all packages OK)

Let me know when to retry the commit, or if you'd like me to fall back to plain `git commit`.