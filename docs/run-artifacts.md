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
.scafld/
  runs/
    {task-id}/
      handoffs/
        executor-phase-phase1.md
        executor-phase-phase1.json
        executor-recovery-ac1_1-1.md
        executor-recovery-ac1_1-1.json
        challenger-review.md
        challenger-review.json
        executor-review-repair.md
        executor-review-repair.json
      review-packets/
        review-1.json
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
- `task_id`
- `selector`
- `generated_at`
- `model_profile`
- `template`
- `session_ref`

Current role×gate handoffs:

- `executor × phase`
- `executor × recovery`
- `executor × review_repair`
- `challenger × review`

`executor-review-repair.md` is rendered from the latest external ReviewPacket.
It is the repair brief for the next implementation agent and includes checked
surfaces, finding evidence, suggested fixes, tests to add, and spec-update
suggestions. Its JSON sibling carries the schema, task, review round, packet
path, and finding counts for tooling.

## Review Packet

External challenger output is normalized into:

```text
.scafld/runs/{task-id}/review-packets/review-N.json
```

The packet artifact is the structured provider content captured before the
markdown review projection. It carries pass results, checked surfaces, findings,
evidence, repair guidance, tests, and spec-update suggestions. It does not carry
provider provenance; scafld records provider, model, session, timing, isolation,
hashes, diagnostics, and artifact references in review metadata and session
entries.

The accepted packet is promoted into the normal workflow. Findings are recorded
on the session review entry, projected into the spec `## Review` section, shown
by `scafld status`, and included by `scafld handoff` for the next repair agent.
Diagnostics are only the fallback surface for provider transport failures,
invalid packets, timeouts, and other unaccepted output.

## Session

`session.json` is the durable ledger.

It records:

- typed entries
- attempts
- recovery counters
- criterion states
- phase summaries
- optional usage data

Important typed entries:

- `approval`
- `criterion`
- `phase`
- `review`
- `complete`
- `fail`
- `cancel`

Review entries store the verdict in `status`, a concise summary in `reason`,
and the accepted findings payload in `output`. Replayed criterion and phase
state lives in `criterion_states` and `phase_blocks`; the Markdown spec is
rendered from this evidence instead of being trusted as the source of state.

## Retention

Superseded handoffs stay inside the run dir for debugging.

On `complete`, `fail`, or `cancel`, scafld archives the whole run dir into:

```text
.scafld/runs/archive/{YYYY-MM}/{task-id}/
```
