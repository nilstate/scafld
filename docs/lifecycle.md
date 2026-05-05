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
| `active` | `active/` | Acceptance criteria are being executed. |
| `blocked` | `active/` | Execution found a blocking failure. |
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
scafld review add-auth
scafld complete add-auth
```

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
- `failed`

`scafld harden <task-id>` appends a harden round while the spec is still a
draft. The active prompt asks the agent to record grounded questions and work
the answers back into the spec. `scafld harden <task-id> --mark-passed`
verifies citations, closes the latest round, and records that the draft
survived hardening. Unresolved citations fail closed and leave the round open.

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
