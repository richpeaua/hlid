# 005 — E2E: request routes through Hlid to a test upstream

Status: todo
Parent: DESIGN.md (Slice 1 checkpoint; request lifecycle) | Slice 1 | Type: AFK (E2E checkpoint)

## What to build
The binary entrypoint plus a black-box end-to-end test proving the whole Slice 1 spine works:
config -> server -> router -> proxy -> upstream, and `/healthz`.

    // cmd/hlid/main.go
    package main
    // main: parse a -config flag (default "hlid.yaml"), config.Load + Validate, server.New,
    // then srv.ListenAndServe(). Fatal (log.Fatal) on any startup error.
    func main()

    // test/e2e/proxy_test.go — black-box integration test (no internals mocked).

## Behavior (no decisions)
- Starting from a real config file, a request to Hlid on a configured route reaches the intended
  upstream and the upstream's response is returned to the client verbatim.
- Longest-prefix routing holds end-to-end (two upstreams on `/` and `/app/`).
- `GET /healthz` returns `200 ok` end-to-end.
- An unknown path returns `404`.
- `main` exits non-zero (log.Fatal) on a missing/invalid config.

## Acceptance
- [ ] E2E: two `httptest.Server` upstreams; a temp config routes `/app/`->A and `/`->B; build the
      Hlid handler via `server.New` and assert requests to `/app/x` hit A and `/y` hit B with bodies intact
- [ ] E2E: `/healthz` -> 200 "ok"
- [ ] E2E: an unrouted path -> 404
- [ ] `cmd/hlid` builds (`go build ./cmd/hlid`) and fails fast on an invalid `-config`

## Scope files
- cmd/hlid/main.go
- test/e2e/proxy_test.go

## Blocked by
- issues/004-server-assembly-healthz.md
