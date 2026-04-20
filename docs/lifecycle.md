---
title: Lifecycle
description: Spec states, transitions, and the filesystem state machine
---

# Lifecycle

Every scafld spec moves through a defined lifecycle. The filesystem is the state machine; the directory a spec lives in determines its status. No database, no lock files, no hidden state.

## States

| Status | Directory | Description |
|--------|-----------|-------------|
| `draft` | `drafts/` | Spec is being written. Not yet reviewed. |
| `under_review` | `drafts/` | Spec is ready for human review. Status field updated, file stays in drafts. |
| `approved` | `approved/` | Spec has been reviewed and accepted. Ready for execution. |
| `in_progress` | `active/` | An agent is actively working against this spec. |
| `review` | `active/` | Work is complete, undergoing automated and manual review. |
| `completed` | `archive/YYYY-MM/` | All acceptance criteria passed. Archived with review artifact. |
| `failed` | `archive/YYYY-MM/` | Work did not meet acceptance criteria. Archived with failure record. |
| `cancelled` | `archive/YYYY-MM/` | Spec was abandoned before completion. |

## Transitions

```
draft ──→ under_review ──→ approved ──→ in_progress ──→ review ──→ completed
                                                          │          │
                                                          ├──→ failed
                                                          │
                                                    cancelled
```

Each transition is triggered by a CLI command:

```bash
scafld approve add-auth    # drafts/ → approved/
scafld start add-auth      # approved/ → active/
scafld complete add-auth   # active/ → archive/YYYY-MM/
scafld fail add-auth       # active/ → archive/YYYY-MM/
scafld cancel add-auth     # any → archive/YYYY-MM/
```

## Filesystem as state machine

The directory structure enforces the lifecycle mechanically. A spec in `approved/` cannot be executed until `scafld start` moves it to `active/`. A spec in `active/` cannot be archived until it passes review or is explicitly failed.

This design is deliberate. The filesystem is auditable, diffable, and requires no runtime process. You can inspect the state of every spec with `ls`:

```bash
ls .ai/specs/drafts/      # what's being planned
ls .ai/specs/approved/     # what's ready to execute
ls .ai/specs/active/       # what's in flight
ls .ai/specs/archive/      # what's done
```

## Archive structure

Completed, failed, and cancelled specs are archived by month:

```
archive/
  2026-04/
    add-auth.yaml
    refactor-db.yaml
  2026-03/
    upgrade-deps.yaml
```

The archive preserves the full spec including the review artifact, self-evaluation, and any deviation records. It's a complete audit trail of every task the project has executed.

## Concurrent specs

Multiple specs can be active simultaneously. Each spec tracks its own state independently. If two specs touch overlapping files, the scope audit will flag the conflict.

## Status queries

```bash
# List all specs by status
scafld list
scafld list active
scafld list draft

# Detailed status for a specific spec
scafld status add-auth
scafld status add-auth --json
```

## Lifecycle discipline

The lifecycle is intentionally rigid. You can't skip states. You can't move a draft directly to active. This friction is the point; it forces the planning-before-execution discipline that makes specs useful.

If a spec needs changes after approval, cancel it and create a new one. Specs are cheap. Sloppy execution against a stale spec is expensive.

## Harden status

Separate from the lifecycle `status` field, every spec may carry a `harden_status` field recording whether the operator has interrogated the draft with `scafld harden`. Values: `not_run`, `in_progress`, `passed`. Missing field is treated as equivalent to `not_run`.

`scafld approve` does **not** consult `harden_status`. Hardening is optional and operator-driven; you run it when you want to stress-test a draft and skip it for trivial or well-understood specs. The field exists for audit and reporting (`scafld report` shows adoption), not as a gate.

Round semantics: `scafld harden <id>` appends a round and sets `harden_status: in_progress`. `scafld harden <id> --mark-passed` closes the latest round and sets `passed`. Re-running on a passed spec resets to `in_progress` and appends a new round; prior rounds are preserved as audit trail.
