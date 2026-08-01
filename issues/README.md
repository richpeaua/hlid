# Hlid — Issue Tracker

Atomic, zero-cognition tickets. Each is scoped for one focused session; every architectural
decision is pre-made in the ticket or in `DESIGN.md`, so execution is mechanical.
Binding rules: `.agents/STANDARDS.md`. Design authority: `DESIGN.md`.

**Follow-up queues:** non-blocking review findings go to `bugs/` (`BUG-NNN`) or `unplanned/`
(`UNP-NNN`) and are `pull`ed into this board only when triaged at a checkpoint.

**Conventions**
- Grab the lowest-numbered ticket whose `Blocked by` are all done (`warp frontier N`).
- Do not make architectural decisions in a ticket; if one is genuinely underspecified, stop and
  `warp git block`.
- `E2E` tickets are slice integration checkpoints — they must pass before the next slice starts.

## Slices

**Slice 1 — Proxy spine (config-driven reverse proxy, no auth):** 001–005 → E2E 005
**Slice 2 — OIDC authentication (sessions + login/callback):** 006–010 → E2E 010
**Slice 3 — Authorization policy (per-route allow rules):** 011–014 → E2E 014
**Slice 4 — Identity injection (signed upstream identity + header stripping):** 015–018 → E2E 018
**Slice 5 — Hardening (TLS, logging/metrics, graceful shutdown, reload):** 019–023 → E2E 023

## Index

| # | Ticket | Slice | Blocked by |
|---|---|---|---|
| 001 | config schema + loader | 1 | — |
| 002 | reverse-proxy handler | 1 | — |
| 003 | path-prefix router | 1 | 001, 002 |
| 004 | server assembly + healthz | 1 | 003 |
| 005 | E2E: request routes through Hlid to a test upstream | 1 | 004 |
| 006 | session cookie store (AES-256-GCM) | 2 | — |
| 007 | config schema: OIDC provider + session | 2 | — |
| 008 | OIDC auth handlers (login + callback) | 2 | 006, 007 |
| 009 | session middleware + server wiring | 2 | 008 |
| 010 | E2E: OIDC login flow through Hlid | 2 | 009 |
