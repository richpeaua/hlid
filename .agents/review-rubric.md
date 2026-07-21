# Review finding rubric

Every **non-blocking** review finding carries a **type** and a **grade**. Blocking findings are
not graded - they are fixed in the `remediate` pass. The grade is **advisory**: it does not gate
merge or slice close; it lets checkpoint triage (`workflow.md`) sort and pull the highest-impact
findings first. Both reviewers (step-reviewer, e2e-reviewer) apply this rubric; `warp followup`
stamps the grade onto the queue ticket and sorts by it.

## Tag format (the reviewer emits this in the findings report)

    [bug:S1] | [bug:S2] | [bug:S3]                              -> bug queue      (issues/bugs)
    [out-of-scope:slice-high] | :slice-medium | :slice-low      -> unplanned queue (issues/unplanned)

Example: `[bug:S1] parser.py:64 — token consumed before the offset is committed, so replay skips it …`

## `[bug:*]` - criticality of a real defect

Anchored to the project's core invariants - whatever the project's `DESIGN.md` / `STANDARDS.md`
declare as binding. **Any violation of a stated invariant is S1 by definition**, even if the
current tests pass - the fault is latent and load-bearing.

| grade | meaning |
|---|---|
| **S1 critical** | Breaks a core invariant defined by the project's DESIGN/STANDARDS, or causes data loss / wrong results on a common path. |
| **S2 major** | Wrong behavior on a real but narrower path, or an untested load-bearing branch. No invariant break, but a caller will hit it. |
| **S3 minor** | Edge-case, cosmetic, or low-blast-radius defect. Correct on all common paths. |

The set of "core invariants" is project-defined - see the project's `DESIGN.md` and
`STANDARDS.md`, and the reviewer's Standing checklist in `roles/reviewer.md`. The rubric is
invariant-agnostic: what counts as S1 is a break of whatever the project holds binding.

## `[out-of-scope:slice-*]` - effect on the rest of the slice

Measures how tightly the finding couples to the slice's remaining or adjacent work - i.e. what it
costs the slice to leave it unaddressed. Drives whether triage pulls it **now**, **this
milestone**, or **defers** it.

| grade | meaning | triage routing |
|---|---|---|
| **slice-high** | Touches a shared seam or interface that other tickets in this slice build on; leaving it means later tickets inherit debt or rework. | Pull into the **current** slice now. |
| **slice-medium** | Localized hardening that aligns with slice intent but nothing else depends on. | Schedule **within the milestone**. |
| **slice-low** | Global or future concern with no bearing on this slice. | **Defer** to the backlog. |

## How the grade flows (deterministic)

1. Reviewer emits the graded tag in `.agents/handoff/NNN/review-rN.md`.
2. Orchestrator files it: `warp followup file <bug|unplanned> <grade> <slug> <blocked-by> "<title>"`
   - the grade is validated against the queue (S* for bug, slice-* for unplanned) and written as a
   `Grade:` line on the queue ticket + a column in the queue index.
3. At each checkpoint, `warp followup list` shows both queues **sorted by grade** (S1 / slice-high
   first), so triage pulls the top of each queue first.
4. `warp followup pull` promotes the chosen ticket into `issues/`.

No auto-scheduling: the grade orders the work; the orchestrator (and human) decide what to pull.
