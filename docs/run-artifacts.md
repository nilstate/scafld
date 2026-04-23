---
title: Run Artifacts
description: spec, session, and handoff responsibilities
---

# Run Artifacts

The model is intentionally small:

- `spec`: what must be true
- `session`: what happened
- `handoff`: transport for the next voice

## Hard Rules

- `spec` never stores runtime state
- `handoff` is output for the model and harness; never read it back for state
- `session` is the only durable run-state source
- recovery is a handoff gate plus counters in session
- telemetry is a view of session, not a separate artifact
- v1 makes zero spec schema changes

## Layout

```text
.ai/
  runs/
    {task-id}/
      handoffs/
        executor-phase-phase1.md
        executor-phase-phase1.json
        executor-recovery-ac1_1-1.md
        executor-recovery-ac1_1-1.json
        challenger-review.md
        challenger-review.json
      diagnostics/
        ac1_1-attempt1.txt
      session.json
    archive/{YYYY-MM}/{task-id}/
```

## Handoff

Each handoff is a sibling pair:

- `*.md` for the model
- `*.json` for the harness

Stable payload fields include:

- `schema_version`
- `role`
- `gate`
- `kind`
- `task_id`
- `selector`
- `generated_at`
- `model_profile`
- `template`
- `session_ref`

Current role×gate handoffs:

- `executor × phase`
- `executor × recovery`
- `challenger × review`

## Session

`session.json` is the durable ledger.

It records:

- typed entries
- attempts
- recovery counters
- criterion states
- phase summaries
- optional usage data

Important typed entries in v1:

- `approval`
- `attempt`
- `phase_summary`
- `challenge_verdict`
- `human_override`

## Retention

Superseded handoffs stay inside the run dir for debugging.

On `complete`, `fail`, or `cancel`, scafld archives the whole run dir into:

```text
.ai/runs/archive/{YYYY-MM}/{task-id}/
```
