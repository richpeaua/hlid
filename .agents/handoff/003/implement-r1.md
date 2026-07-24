Implementation is done and the gate is green, but `git add`/`git commit`/`warp` all require interactive approval that isn't coming through in this session — can't commit.

**Status: gate green, commit blocked on approval**

- branch: `ticket-003-path-prefix-router`
- commit hash: none yet (staging/commit denied)
- files touched (staged, unattempted commit): `internal/router/router.go`, `internal/router/router_test.go`
- gate: PASS (`bash scripts/gate.sh` — gofmt, vet, build, test -race all green; `router` pkg test 1.487s)

Per-acceptance evidence:
- A1: `TestNew_LongestPrefixDispatchesCorrectUpstream` (routes `/`, `/app/`; asserts both)
- A2: `TestServeHTTP_NoMatchReturns404`
- A3: `TestNew_MatchIndependentOfRouteOrder` (5 shuffles via seeded rand)
- A4: `TestNew_PropagatesProxyError` (bad upstream string)

Two issues need your action:
1. Run `warp git commit 003 "<subject>"` yourself — `git add`/`git commit`/`warp` are all denied in this sandboxed session.
2. Stray untracked files at `/Users/peaua/Dev/hlid/internal/router/router.go` + `router_test.go` (wrong path I wrote to before catching the worktree mismatch) — outside my sandbox now, delete manually if unwanted.