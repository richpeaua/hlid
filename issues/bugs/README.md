# bug queue (BUG-NNN)

| ID | grade | title | blocked-by |
|---|---|---|---|
| BUG-003 | S3 | internal/router/router_test.go:17 swallows w.Write error via _,_= without a justifying comment (STANDARDS); follows precedent of BUG-002 (003 review-r1) |  |
| BUG-004 | S3 | internal/server/server.go:35-38 /healthz swallows w.Write error via _,_= without a justifying comment (STANDARDS) (004 review-r1) |  |
| BUG-005 | S3 | 004 A4 router.New failure branch untested; cfg.Validate rejects bad upstreams first so the independent router-build error path has no test (004 review-r1) |  |
| BUG-006 | S3 | internal/session/session.go:104 LoadKey treats HLID_SESSION_KEY="" (empty) as unset and falls through to key_file; ticket says 'if set' (presence), so an explicitly-empty env should error not silently prefer the file (006 review-r1) |  |
| BUG-007 | S3 | internal/config/config.go:35 Provider.RedirectURL doc says 'absolute https callback URL' but provider.validate accepts http(s) per spec; doc/code drift, fix comment (007 review-r1) |  |
| BUG-008 | S2 | internal/auth/auth.go:194 Callback redirects to pre.Path from the pre-auth cookie without re-applying Login's same-origin guard (returnPath); asymmetric open-redirect defense, harden the read path (008 review-r1) |  |
| BUG-009 | S3 | internal/auth/auth_test.go:199 TestNew/unreachable_discovery dials real 127.0.0.1:1 instead of a closed httptest.Server; STANDARDS says no real network in unit tests (008 review-r1) |  |
| BUG-010 | S3 | internal/auth/middleware_test.go:95 table field wantNav is declared but never read; subtests hardcode expected status, so a future case setting wantNav would silently no-op (009 review-r1) |  |
