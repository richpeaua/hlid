# unplanned queue (UNP-NNN)

| ID | grade | title | blocked-by |
|---|---|---|---|
| UNP-002 | slice-medium | internal/router/router.go:50 uses raw strings.HasPrefix not segment-aware; '/foo' would match '/foobar'. Settle segment-boundary matching before Slice 3 policy binds to it (005 e2e-review) |  |
| UNP-003 | slice-low | config validated 3x (Load, main, server.New) - harmless redundancy (005 e2e-review) |  |
| UNP-004 | slice-low | upstream URL absolute-http(s) validation duplicated across config.go and proxy.go; DRY cleanup (005 e2e-review) |  |
| UNP-005 | slice-low | middleware.go:34 collapses protected->/auth/login->provider into one 302 straight to provider; mounted GET /auth/login is unused by the enforced path (dead endpoint). Correct per DESIGN, deviates from ticket wording (010 e2e-review) |  |
