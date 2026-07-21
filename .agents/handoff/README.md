# Handoff records - the implement <-> review contract

One directory per ticket, `.agents/handoff/NNN/`, committed on the ticket branch so the whole
exchange rides into the PR and stays in history. It is the durable, structured channel between the
implementer and the reviewer, brokered by the supervisor.

## Files
- **`contract.md`** - the shared spec, scaffolded from `issues/NNN-*.md` by `warp contract`:
  interface (verbatim), an `A1..An` acceptance checklist, the `scope_files`, design refs, and any
  project-declared constraints. **Both agents read it**, so they work against one agreed spec
  instead of drifting.
- **`implement-rN.md`** - the implementer's record for round N: branch, commit, files, and
  per-acceptance evidence. Written by `warp dispatch implement|remediate` (tees the agent's
  stdout).
- **`review-rN.md`** - the reviewer's findings for round N: independent gate result, a
  per-acceptance pass/fail table, BLOCKING and NON-BLOCKING findings, and a final line
  `VERDICT: APPROVE` or `VERDICT: REQUEST-CHANGES`. The reviewer is **read-only on source**;
  `warp dispatch` (orchestrator context) writes and commits the findings.

## Loop (each supervisor, one ticket; max 3 rounds, then escalate)
```
warp contract NNN                     # branch off slice/N + shared contract
warp dispatch implement NNN           # -> implement-r1.md
warp git pr NNN "<subject>"           # open PR into slice/N
warp dispatch review NNN              # -> review-r1.md, ends with VERDICT:
  VERDICT: APPROVE          -> warp git done NNN <pr-url> ; warp git merge NNN   # enqueue auto-merge into slice/N
  VERDICT: REQUEST-CHANGES  -> warp dispatch remediate NNN  # -> implement-r{n+1}.md, back to review
  still REQUEST-CHANGES after round 3 -> warp git block NNN "<open findings>"    # human
```
(At an E2E checkpoint ticket, `warp dispatch review NNN` auto-routes to the e2e-review flavor -
read from `## Slices` via `warp slice role-for`; the caller always just says `review`.)
Round N is derived from the count of existing `implement-r*.md` / `review-r*.md`, so records never
clobber each other.
