# Step Review — Ticket 008 (OIDC auth handlers) — Round 1

## Independent gate result
Ran `bash scripts/gate.sh` myself (not trusting any claim): `gofmt` → `go vet` → `golangci-lint` (skipped, not installed) → `go build` → `go test -race ./...` → **`gate: PASS`**. Also re-ran `go test -race -count=1 ./internal/auth/... -v` (bypassing cache) — all 4 top-level tests / 9 subtests pass.

## Scope audit
Ticket commit `7cfdeaa` touches exactly: `internal/auth/auth.go`, `internal/auth/auth_test.go`, `go.mod`, `go.sum` (new deps `go-oidc`, `go-jose` (test-only), `x/oauth2`, per the ticket's explicit "go get both; commit go.mod/go.sum" instruction), and a one-line `Status: wip` marker in `issues/008-oidc-auth-handlers.md`. Matches contract `scope_files` — **PASS**, no drive-by edits.

## Interface conformance
`Authenticator` struct, `New`, `Login`, `Callback` signatures and doc comments match the contract verbatim. The unexported `now func() time.Time` field is the clock-injection the ticket text mandates ("inject a clock ... defaulting to time.Now"), not a deviation. Both `internal/session` (ticket 006) and `internal/config` (ticket 007) are consumed correctly (`session.Identity`, `session.Store.Seal/Open`, `config.Provider`, `config.Session`) — checked their actual definitions, not assumed.

## Per-acceptance table

| # | Criterion | Verdict | Evidence |
|---|---|---|---|
| A1 | `New` errors on missing client secret env and unreachable discovery; succeeds against fake discovery server | **PASS** | `TestNew` subtests, auth_test.go:180-226; independently re-run, all green |
| A2 | `Login` sets pre-auth cookie, 302s to auth URL with expected `client_id`/`redirect_uri`/`scope`/cookie's `state`+`nonce` | **PASS** | `TestLogin`, auth_test.go:231-286; asserts httponly+secure cookie, parses `Location`, checks all 5 query params against cookie state |
| A3 | Valid code+state+ID token(nonce match) → session cookie decrypts via `session.Store.Open` to expected `Identity`, 302 to saved path | **PASS** | `TestCallbackSuccess`, auth_test.go:318-382; decrypts with a fresh `Store` (not the authenticator's own) to prove the cookie is genuinely portable/correct, checks sub/email/Expiry and `Location` |
| A4 | Fails closed (no session cookie, non-2xx) on: missing pre-auth cookie, state mismatch, exchange failure, bad ID token, nonce mismatch | **PASS** | `TestCallbackFailsClosed` subtests, auth_test.go:387-461, all 5 named cases; shared `assertFailsClosed` checks 4xx AND absence of a (non-empty) session cookie on every branch |
| A5 | Deterministic `Expiry` via injected clock; table-driven fake provider (issuer+JWKS+token endpoint), no real network | **PASS with a caveat** | Fixed clock override at auth_test.go:324; `fakeProvider` serves discovery/JWKS/token via `httptest.Server` (auth_test.go:50-110) with RS256 signing incl. a wrong-signer key for negative tests. Caveat: one subtest performs a real (loopback) network dial — see non-blocking finding #2 |

## Security-relevant behavior verified directly
- `idToken.Nonce` comparison is manual (confirmed via `go doc oidc.IDToken`: the library explicitly does **not** verify nonce itself, "it's the user's responsibility") — the ticket's fail-closed nonce check is load-bearing and correctly implemented.
- No secrets/tokens/cookies logged anywhere in `auth.go` (grepped for `panic|log\.|fmt\.Print` — no matches).
- `returnPath()` rejects non-`/`-prefixed and protocol-relative (`//`) redirect targets on the `Login` write path (open-redirect guard) — see finding #1 for the asymmetry with `Callback`'s read path.

## BLOCKING findings
None.

## NON-BLOCKING findings

1. **`[bug:S2]`** `internal/auth/auth.go:194` — `Callback`'s final redirect uses `pre.Path` straight from the pre-auth cookie without re-applying the same-origin guard (`returnPath`'s leading-`/`-and-no-`//` check) that `Login` applies when it first writes that cookie. Today nothing in this ticket lets an attacker set/tamper the pre-auth cookie, so it's not exploitable through this code alone — but the "only same-origin relative paths are honored" rule is enforced on one side (write) and trusted unchecked on the other (read). A future write path to this cookie (or a cross-subdomain cookie-tossing vector) would turn this into an open redirect with no test currently guarding against it.
2. **`[bug:S3]`** `internal/auth/auth_test.go:199-217` (`TestNew/unreachable_discovery`) — dials a real socket at `https://127.0.0.1:1` to simulate an unreachable discovery endpoint, rather than spinning up an `httptest.Server` and closing it (which would deterministically refuse connections without depending on port 1 being unbound). STANDARDS.md's Testing section states a preference for `httptest` and "no real network in unit tests"; this is a real loopback syscall whose fast-refusal isn't guaranteed under every sandbox/CI network policy. It passed cleanly here and is low blast-radius (test-only, deterministic in virtually all real environments), so I'm grading it minor rather than a hard invariant break.

Everything else on the standing checklist (interface conformance, scope discipline, test integrity, HTTP/security discipline, boundaries) checked clean — the two items above are the only things I'd flag.

VERDICT: APPROVE