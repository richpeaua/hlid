# bug queue (BUG-NNN)

| ID | grade | title | blocked-by |
|---|---|---|---|
| BUG-003 | S3 | internal/router/router_test.go:17 swallows w.Write error via _,_= without a justifying comment (STANDARDS); follows precedent of BUG-002 (003 review-r1) |  |
| BUG-004 | S3 | internal/server/server.go:35-38 /healthz swallows w.Write error via _,_= without a justifying comment (STANDARDS) (004 review-r1) |  |
| BUG-005 | S3 | 004 A4 router.New failure branch untested; cfg.Validate rejects bad upstreams first so the independent router-build error path has no test (004 review-r1) |  |
