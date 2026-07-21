# warp build workflow

> Shipped as a `warp` default. Projects may override it, but keep the loop shape: the scheduler,
> the ticket loop, the slice checkpoints, and the two follow-up queues are what make concurrent
> work collision-free and reviewable.

A ticket-driven, multi-session loop. Small units, verified end-to-end, reviewed at
milestones. Optimized so any single session carries minimal context (one ticket).

## Ticket lifecycle

Each ticket in `issues/` moves through states, tracked by a `Status:` line the `warp` tools
maintain (absence of the line == `todo`):

    todo -> wip -> review -> done   (or -> blocked, with a note)

A ticket is **grabbable** when: its `Status` is `todo` AND every ticket in its
`Blocked by` list is `done`. The orchestrator runs grabbable tickets **concurrently** as a
**disjoint frontier**: a set of grabbable tickets in the current slice whose `scope_files`
don't overlap, so their branches never collide.

## Two levels of branch + merge

Work integrates on a per-slice branch, and `main` moves once per slice:

- **Ticket -> `slice/N`**: each ticket is its own branch + PR, targeting the slice
  integration branch `slice/N` (not `main`). Squash-merged via GitHub **auto-merge** once the
  gate check is green. Many per slice.
- **`slice/N` -> `main`**: one PR per slice, opened at the E2E checkpoint, merged (merge
  commit, preserving the per-ticket squashes) through `main`'s **merge queue**. One per slice.

GitHub enforces safety (branch protection + the required gate check + auto-merge / queue);
there is **no** orchestrator-side merge queue. The gate stays the local top-of-loop check so
PRs only go out green. The gate is whatever command the project configures; `warp` invokes it
via config and never hardcodes it.

## The scheduler loop (orchestrator)

The orchestrator **schedules**; it does not drive per-ticket steps. Per slice:

1. **Open the slice branch** - with `N = warp slice current`, run `warp slice branch N`
   to create `slice/N` off fresh `main`.
2. **Compute the disjoint frontier** - `warp frontier N` prints it deterministically:
   grabbable tickets (Status `todo`, all `Blocked by` `done` **on `origin/slice/N`**) whose
   `scope_files` don't overlap, ascending. It reads Status from the slice branch, not the main
   worktree (where every slice ticket still shows `todo`). The frontier is **single-slice by
   construction**, and `warp frontier` **refuses any slice but `warp slice current`** (so a
   later slice can't be grabbed before this one's E2E checkpoint lands) as well as a dirty main
   worktree. Run the command; do not reason the frontier in context. (`warp frontier N -v`
   explains each in/out.)
3. **Fan out** - spawn one ticket-supervisor per frontier ticket **in a single message**,
   `run_in_background: true` + `isolation: "worktree"` (each runs its loop in an isolated git
   worktree, so concurrent tickets never collide; the supervisor only drives `warp`, so a
   cheap/fast model suffices). Track with `warp slice status N`.

   **Concurrent fan-out is collision-free only under three conditions.** The frontier already
   guarantees (a); the supervisor prompt and the orchestrator must enforce (b) and (c):
   - **(a) `scope_files` strictly disjoint across the wave.** `warp frontier` guarantees this.
     Two supervisors sharing a file collide on `slice/N`; never spawn them together.
   - **(b) Each supervisor hard-resets its isolation worktree to `origin/slice/N` before the
     contract step.** An `isolation: "worktree"` checkout can start on a stale `main`-era commit,
     so the supervisor must `warp git reset-to origin/slice/N` (or equivalent) first - otherwise
     its diff spuriously reverts already-landed siblings. State this in the spawn prompt.
   - **(c) The orchestrator leaves the main worktree untouched while the wave runs.** No git ops
     on the shared checkout until the wave returns; all work stays in the isolated worktrees.
     Then run `warp git clean-tree` (step 4) before the next spawn.
   Given (a)-(c), prefer disjoint concurrent waves for wall-clock. Fall back to sequential only
   when scopes genuinely overlap.
4. **Collect + refill** - as each supervisor returns its record (harness notifies on
   completion), **file its NON-BLOCKING findings** into the queue matching the reviewer's tag,
   carrying the finding's grade (`warp followup file bug <S1|S2|S3> …` / `warp followup file
   unplanned <slice-high|slice-medium|slice-low> …`, per `review-rubric.md`; never drop one),
   then commit them to `main` with **`warp followup commit "<title>"`** - this pathspec-scopes
   the commit to the queue dirs only, so a supervisor's ticket files (which can leak into the
   orchestrator's main worktree via worktree-isolation writeback) can't ride onto `main`.
   **Before spawning the next supervisor, run `warp git clean-tree`** - it restores/removes any
   leaked ticket files from the main worktree (queue dirs preserved) so they aren't copied into
   the new supervisor's isolated worktree. Then re-run `warp frontier N` and spawn the
   newly-unblocked tickets; dependent tickets branch off the current `slice/N` tip, so they pick
   up landed blockers.
5. **Close the slice** - once `warp slice status N` shows the E2E checkpoint (and any trailing
   milestone REVIEW) `done` on `slice/N`, **compact context** (the slice's records are settled
   and nothing is in flight - the cheapest seam to shed accumulated supervisor records), then
   `warp slice pr N "<subject>"` opens the single `slice/N -> main` PR and enables auto-merge;
   the merge queue rebases + re-gates + merges it as a unit.
6. **Reclaim after the slice lands** - once the `slice/N -> main` PR has merged, sweep the
   litter the concurrent waves leave behind so the next slice starts from a clean checkout:
   - **Background shells** - stop every still-running background dispatch/watch/tail shell from
     this slice's waves (they linger as "running" UI entries after their process has exited).
     Nothing should be in flight at a closed slice; a live shell here is a leak, not work.
   - **Merged worktrees + branches** - remove every isolated worktree checkout, prune, then
     delete the merged throwaway ticket branches (all squash-landed into `slice/N`, so they're
     safe). Leave `slice/*` and `followups-*` branches alone.
   - Verify the worktree list shows only the main checkout and the tree is clean before opening
     the next slice's integration branch.

## The ticket loop (each supervisor, one ticket)

Each ticket-supervisor owns ONE ticket in its own worktree, driving the same `warp` commands the
orchestrator used to - but targeting `slice/N`, and stopping at the verdict. The implementer and
step-reviewer never touch `main` and communicate only through the durable handoff record in
`.agents/handoff/NNN/` (see `handoff-README.md`).

1. **Contract** - `warp contract NNN` branches off the ticket's `slice/N` tip, stamps
   `Status: wip`, scaffolds the shared spec `.agents/handoff/NNN/contract.md`
   (interface + acceptance `A1..An` + `scope_files` + any project constraints), committed on the
   branch. All git/gh goes through `warp git` (`start|commit|push|pr|done|merge|block`).
2. **Implement (TDD)** - `warp dispatch implement NNN` launches the implementer: named tests
   **red** -> minimum code **green** -> refactor under a green gate -> `warp git commit` (no
   push). Record teed to `implement-r1.md`. If underspecified it STOPs via `warp git block`.
3. **PR** - `warp git pr NNN "imperative subject"` pushes and opens the PR **into `slice/N`**,
   stamping `Status: review`.
4. **Review** - always call `warp dispatch review NNN`; it **auto-routes** the flavor from the
   `## Slices` list (via `warp slice role-for`), so the supervisor never tracks the checkpoint
   list. For an ordinary ticket that runs the step-reviewer (read-only): runs the gate, verifies
   each acceptance item for **this one ticket**, ends with `VERDICT: APPROVE|REQUEST-CHANGES`.
   **At an E2E checkpoint ticket** the same call upgrades to the e2e-reviewer - verifies the E2E
   acceptance then audits the **entire slice** (present in the worktree, branched off the slice
   tip). (A milestone REVIEW ticket is not a dispatch role; `warp dispatch review` on one refuses
   and points to the standalone `design-review` agent.) Findings teed to `review-rN.md`,
   committed + pushed by dispatch (the reviewer writes nothing) so they ride into the PR. Same
   record file, same VERDICT contract, same remediate loop.
5. **Remediate / pass** - the supervisor reads the `VERDICT`:
   - `REQUEST-CHANGES` and round < 3 -> `warp dispatch remediate NNN` (fixes every BLOCKING
     finding from the latest `review-rN.md` -> `implement-r{n+1}.md`), back to step 4. The cap is
     enforced mechanically: `warp dispatch remediate` **refuses** a 4th attempt (after 3 review
     rounds) and directs you to block.
   - Still `REQUEST-CHANGES` after round 3 -> `warp git block NNN "<open findings>"`, STOP.
6. **Enqueue** - on `VERDICT: APPROVE`, `warp git done NNN "<PR url>"` (stamps `Status: done` on
   the branch, pushed so it rides in via the PR) then `warp git merge NNN` enables auto-merge of
   the ticket PR into `slice/N`. GitHub lands it when the gate is green. The supervisor reports
   its record (ticket, PR, VERDICT, tagged non-blocking findings, and the loop's
   token/cost/time metrics via `warp metrics NNN`) back to the orchestrator and STOPs. It never
   merges `main` or opens the slice PR.

Each dispatch (`implement`/`remediate`/`review`) records its token/cost/time metrics to a
persistent sink, tagged with repo + slice + ticket, so the store is visible from any worktree and
persists across runs. `warp metrics [NNN]` summarizes one ticket or all.

## Slice checkpoints (hard gates)

The backlog is organized into slices; each ends in an **E2E checkpoint** ticket. **Do not spawn
any of the next slice's tickets until the current slice's E2E checkpoint is `done`.** This
guarantees a working end-to-end spine at every step. The E2E checkpoint ticket runs the ticket
loop like any other, except its review is a **slice-wide** e2e-review rather than the per-ticket
step review - see ticket-loop step 4. When it is `done` on `slice/N`, the orchestrator closes the
slice with the single `slice/N -> main` PR (`warp slice pr N`).

Milestone **REVIEW** tickets (where the backlog declares them) must be `done` before the following
slice begins; these run the standalone `design-review` skill/agent against the milestone diff (the
`slice/N -> main` PR), distinct from the two dispatched reviewers.

**Triage the follow-up queues at every checkpoint.** Before starting the next slice (at each E2E
checkpoint and milestone REVIEW gate), the orchestrator reviews the bug and unplanned queues -
`warp followup list` prints both **sorted by grade** (`S1` / `slice-high` first, per
`review-rubric.md`) - and `pull`s the tickets it schedules into the main board (`warp followup
pull <BUG-NNN|UNP-NNN> <slice>`), highest-grade first: an `S1` bug or a `slice-high` finding is a
pull-now candidate, `slice-low` defers. So hardening and known bugs are scheduled deliberately
rather than jumping the slice plan or being forgotten.

## Follow-up queues

Non-blocking review findings do not enter the planned backlog directly; they land in one of two
queues, classified AND graded by the reviewer (`review-rubric.md`), and are pulled into `issues/`
only when triaged:
- `issues/bugs/` (`BUG-NNN`) - real defects, non-blocking for the PR they were found in; graded `S1|S2|S3`.
- `issues/unplanned/` (`UNP-NNN`) - out-of-scope / hardening beyond the ticket; graded `slice-high|slice-medium|slice-low`.

See each queue's `README.md`; manage with `warp followup` (`file` / `list` / `pull`).

## Roles

Orchestrator (primary session, **scheduler**) spawns ticket-supervisor subagents (one per
frontier ticket, background + worktree); each supervisor dispatches the implementer and the two
reviewers - the step-reviewer (per-ticket) and the e2e-reviewer (slice-wide). Full charters:
`roles/orchestrator.md`, `roles/ticket-supervisor.md`, `roles/implementer.md`,
`roles/reviewer.md` (shared by both reviewers; see its **Review scopes**). Binding
project rules: `STANDARDS.md`.
