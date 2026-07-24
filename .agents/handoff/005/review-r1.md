All four E2E tests pass on a fresh `-race -count=1` run. Slice audit complete.

---

# E2E Review — 005 E2E Proxy Spine (Slice 1 checkpoint, round 1)

## Independent gate result
`scripts/gate.sh` run locally: **PASS** — gofmt clean, `go vet` clean, `go build ./...` OK, `go test -race ./...` OK (all 5 packages). golangci-lint not installed (gate skips, as designed). E2E suite re-run fresh (`-count=1`): 4/4 PASS.

## Per-acceptance verification

| # | Acceptance | Verdict | Evidence |
|---|---|---|---|
| A1 | Two `httptest.Server` upstreams; temp config `/app/`→A, `/`→B; build via `server.New`; `/app/x`→A, `/y`→B, bodies intact | **PASS** | `test/e2e/proxy_test.go:64-100`; asserts body `"app:/app/x"`/`"root:/y"` + 200; handler built via `config.Load`+`server.New` (same path as main) |
| A2 | `/healthz` → 200 "ok" | **PASS** | `proxy_test.go:103-130`; asserts status 200 and body `"ok"` |
| A3 | Unrouted path → 404 | **PASS** | `proxy_test.go:135-158`; config with only `/app/` (no `/` catch-all), `GET /nope` → 404 — genuinely exercises router miss path |
| A4 | `cmd/hlid` builds + fails fast on invalid `-config` | **PASS** | `proxy_test.go:162-192`; real `go build ./cmd/hlid`, runs binary against missing config, asserts non-zero `*exec.ExitError` + output mentions "config" |

E2E genuinely exercises the spine: real TCP via `httptest.NewServer`, real upstreams, no internals mocked; drives config→server→router→proxy→upstream and the binary itself.

## BLOCKING findings
None.

## NON-BLOCKING findings

- `[out-of-scope:slice-medium]` `internal/router/router.go:50` — routing is raw `strings.HasPrefix`, not path-segment-aware. A non-trailing-slash route (config permits `"/foo"`, validated only to start with `/`) matches `/foobar`. Design examples all use trailing slashes so no current mis-route, but the routing seam is what Slice 3 policy binds per-route allow rules to; segment-boundary matching should be settled before authz builds on it. Not a defect vs the stated "longest path-prefix match" contract.

- `[out-of-scope:slice-low]` `internal/config/config.go:37` + `cmd/hlid/main.go:16-23` + `internal/server/server.go:25` — config validated three times (`Load` internally, `main` again, `server.New` again). Harmless redundancy; no behavior impact.

- `[out-of-scope:slice-low]` `internal/config/config.go:65-71` vs `internal/proxy/proxy.go:19-21` — upstream URL absolute-http(s) validation duplicated in two packages. DRY cleanup only; both agree today.

## What was checked
All 4 accumulated `internal/` modules (config, router, proxy, server) + `cmd/hlid` + e2e read in full. Verified: fail-closed behavior (`recoverPanic`→500, router→404), no swallowed errors, no secrets logged, proxy `Host` rewrite, middleware order (logging outermost), no `t.Skip`/weakened tests, Go 1.26 mux method-scoped `GET /healthz` coexists with `/` catch-all. Inbound `X-Hlid-*` stripping is absent but correctly deferred to Slice 4 per DESIGN — not a slice-1 finding.

VERDICT: APPROVE