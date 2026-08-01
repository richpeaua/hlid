# BUG-011 — middleware.go:34 calls Login on the original request, but Login derives return path only from redirect_to query param (auth.go:200); middleware-initiated login has none, so post-login Callback redirects to / not the requested path. Thread original path into the flow (010 e2e-review)
Queue: bug follow-up | Type: AFK | Grade: S2
Blocked by:  | Slice: <assigned on pull>

Source: non-blocking finding from a review (fill in the source PR/ticket).

## What to build
<what to change and why>

## Behavior (no decisions)
- <observable behavior; no new decisions>

## Acceptance
- [ ] <criterion>

## Blocked by

