# bug queue (BUG-NNN)

| ID | grade | title | blocked-by |
|---|---|---|---|
| BUG-003 | S3 | internal/router/router_test.go:17 swallows w.Write error via _,_= without a justifying comment (STANDARDS); follows precedent of BUG-002 (003 review-r1) |  |
| BUG-004 | S3 | internal/server/server.go:35-38 /healthz swallows w.Write error via _,_= without a justifying comment (STANDARDS) (004 review-r1) |  |
| BUG-005 | S3 | 004 A4 router.New failure branch untested; cfg.Validate rejects bad upstreams first so the independent router-build error path has no test (004 review-r1) |  |
| BUG-006 | S3 | internal/session/session.go:104 LoadKey treats HLID_SESSION_KEY="" (empty) as unset and falls through to key_file; ticket says 'if set' (presence), so an explicitly-empty env should error not silently prefer the file (006 review-r1) |  |
