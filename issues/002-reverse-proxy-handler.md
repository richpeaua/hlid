# 002 — reverse-proxy handler

Status: done - https://github.com/richpeaua/hlid/pull/1
Parent: DESIGN.md (components: proxy) | Slice 1 | Type: AFK

## What to build
A single-upstream reverse-proxy handler wrapping `net/http/httputil`.

    package proxy

    // New returns an http.Handler that reverse-proxies every request to the given upstream base
    // URL. Returns an error if upstream is not a valid absolute http/https URL.
    func New(upstream string) (http.Handler, error)

Implementation notes (no decisions): build on `httputil.NewSingleHostReverseProxy`, preserve the
inbound path/query, set the `Host` header to the upstream host, and set a non-nil `ErrorHandler`
that responds `502 Bad Gateway` (never leak the upstream error to the client body).

## Behavior (no decisions)
- A request to the handler is forwarded to the upstream with the same method, path, and query.
- The upstream's status and body are returned to the caller unchanged.
- An unreachable upstream yields `502` (via `ErrorHandler`), not a panic or a 500 stack.
- `New` returns an error for an empty or non-absolute upstream URL.

## Acceptance
- [ ] `New` forwards method+path+query to an `httptest.Server` upstream and returns its response (test)
- [ ] Unreachable upstream returns `502` (point at a closed port; assert status)
- [ ] `New` errors on empty and relative upstream URLs (subtests)

## Scope files
- internal/proxy/proxy.go
- internal/proxy/proxy_test.go

## Blocked by
None — can start immediately.
