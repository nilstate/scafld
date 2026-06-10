---
title: Lifecycle
description: Spec states, transitions, and evidence flow
---

# Lifecycle

scafld keeps workflow state visible in the filesystem and durable in the
session ledger. Specs move through `.scafld/specs/`; runtime evidence lives
under `.scafld/runs/`.

## States

| Status | Directory | Description |
|--------|-----------|-------------|
| `draft` | `drafts/` | Spec is being written and hardened. |
| `approved` | `approved/` | Human accepted the contract. Ready to execute. |
| `active` | `active/` | A build phase is open or phase evidence is being recorded. |
| `blocked` | `active/` | Attempted build evidence found a blocking failure. |
| `review` | `active/` | Work reached the adversarial review gate. |
| `completed` | `archive/YYYY-MM/` | Review passed and work was archived. |
| `failed` | `archive/YYYY-MM/` | Work was explicitly failed. |
| `cancelled` | `archive/YYYY-MM/` | Work was abandoned before completion. |

Typical path:

```text
draft -> approved -> active -> review -> completed
```

Hardening is tracked separately with `harden_status` so it can be repeated and
audited without inventing another lifecycle directory.

## Commands

```bash
scafld plan add-auth
scafld harden add-auth
scafld harden add-auth --mark-passed
scafld validate add-auth
scafld approve add-auth
scafld build add-auth
# implement the opened phase, then repeat build until review
scafld build add-auth
scafld review add-auth
scafld complete add-auth
```

`scafld finalize add-auth` collapses the finish line into one call: it runs
acceptance against an immutable snapshot, runs the independent review, mints a
signed receipt, and records completion. Use the step-by-step
`review` and `complete` commands when hand-sequencing the lifecycle; use
`finalize` as the default completion authority. `scafld verify <receipt>
--target <commit-ish>` then replays the receipt as the CI merge wall. See
[CLI Reference](cli-reference.md) for both commands.

Failure paths:

```bash
scafld fail add-auth --reason "acceptance cannot be satisfied"
scafld cancel add-auth --reason "replaced by narrower task"
```

## Filesystem State

```text
.scafld/specs/
  drafts/
  approved/
  active/
  archive/
    2026-05/
```

The directory a spec lives in matches its lifecycle status. You should be able
to answer "what is in flight?" with:

```bash
ls .scafld/specs/active
```

## Evidence Ordering

When a command changes runtime state, scafld writes the session first and then
projects the current state back into the Markdown spec.

That gives scafld one authority rule:

- session is the durable evidence source
- spec is the readable contract plus current projection
- handoff is transport for the next model voice

If the spec projection and session ever disagree, reconciliation should rebuild
the projected state from session evidence.

## Hardening

`harden_status` values:

- `not_run`
- `in_progress`
- `passed`
- `needs_revision`
- `error`

`scafld harden <task-id>` appends a harden round while the spec is still a
draft. The active prompt asks the agent to record evidence-backed observations
for paths, commands, scope and migration claims, acceptance timing, rollback or
repair shape, and design quality. `scafld harden <task-id>
--mark-passed` verifies dimension coverage, anchors, and unresolved blocking
observations before recording that the draft survived hardening. Missing
dimensions, open blocking observations, and unresolved citations keep approval
blocked and leave the round open.

Approval remains explicit. Hardening makes the approval decision worth trusting.

## Review Gate

`scafld review` moves work to the review gate and writes the challenger verdict.
`scafld complete` refuses to archive until the latest review verdict is `pass`.

That separation is the core product stance: execution tries to finish the work;
adversarial review tries to break confidence in the work.

## Queries

```bash
scafld status add-auth
scafld status add-auth --json
scafld list
scafld list --json
scafld report
```

`status --json` is the right integration surface for wrappers. It reports the
current lifecycle state and the next allowed follow-up command without requiring
the wrapper to scrape Markdown.
