# Hlid Standards

Binding coding, testing, and git rules for Hlid. The reviewer enforces these; the implementer
follows them. Referenced by the contract and both reviewer flavors.

## Language & layout
- Go 1.26, standard module layout:
  - `cmd/hlid/` - the binary entrypoint (`main`), thin: parse flags, load config, run the server.
  - `internal/` - all application packages (config, proxy, router, server, auth, session, policy, ...).
    Not importable outside the module, which is the boundary we want for a security product.
  - `pkg/` - only for genuinely reusable, stable API (avoid until something earns it).
  - `test/e2e/` - black-box integration tests that exercise the assembled server.
- One package per directory; package name == directory name (lower-case, no underscores).

## Naming
- Exported identifiers: `PascalCase` with a doc comment starting with the identifier name.
- Unexported: `camelCase`. Acronyms keep case: `HTTP`, `URL`, `OIDC`, `ID`, `JWT` (e.g. `parseJWT`, `userID`).
- No stutter: `config.Config` not `config.ConfigStruct`; `proxy.New` not `proxy.NewProxy`.
- Test files `*_test.go`; test funcs `TestXxx`; prefer table-driven subtests via `t.Run`.

## Errors & control flow
- Return `error` as the last value; never `panic` in library code (only an unrecoverable
  `main`/init misconfiguration may `log.Fatal`).
- Wrap with context: `fmt.Errorf("load config %q: %w", path, err)`. Preserve the chain with `%w`.
- Check every error. Do not use `_ =` to swallow one without a comment justifying it.
- `context.Context` is the first parameter of any request-scoped or cancellable function.

## HTTP & security discipline
- All request handlers are `http.Handler` / `http.HandlerFunc`; compose via middleware
  (`func(http.Handler) http.Handler`).
- Never log secrets, tokens, cookies, or full `Authorization` headers.
- Fail closed: an auth/policy error denies the request (`403`/`401`), never falls through to the upstream.
- No `Date.now`-style nondeterminism smuggled into tests; inject clocks/IDs where behavior depends on them.

## Testing
- Every acceptance item is backed by a named test asserting it. Prefer `net/http/httptest` for
  handler and proxy tests; no real network in unit tests.
- `go test -race ./...` must pass (the gate runs it with `-race`).

## Scope & git
- Stay strictly within the contract's `scope_files`. No drive-by edits, no reformatting unrelated files.
- Do not make architectural decisions inside a ticket. If a ticket is genuinely underspecified or
  conflicts with `DESIGN.md`, STOP and `warp git block NNN "<precise question>"`.
- Commits are imperative-subject, small, and reference the ticket (`warp git commit` does this).
- All git/gh goes through `warp git`; never push to `main`.

## The gate
`scripts/gate.sh` = gofmt -> go vet -> golangci-lint (if installed) -> go build -> go test -race.
A ticket is not done until the gate is green.
