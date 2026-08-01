# BUG-006 — internal/session/session.go:104 LoadKey treats HLID_SESSION_KEY="" (empty) as unset and falls through to key_file; ticket says 'if set' (presence), so an explicitly-empty env should error not silently prefer the file (006 review-r1)
Queue: bug follow-up | Type: AFK | Grade: S3
Blocked by:  | Slice: <assigned on pull>

Source: non-blocking finding from a review (fill in the source PR/ticket).

## What to build
<what to change and why>

## Behavior (no decisions)
- <observable behavior; no new decisions>

## Acceptance
- [ ] <criterion>

## Blocked by

