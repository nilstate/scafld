---
title: Run Artifacts
description: spec, session, diagnostics, and handoff responsibilities
---

# Run Artifacts

The runtime model is intentionally small:

- `spec`: the readable contract plus current projection
- `session`: the durable evidence ledger
- `diagnostics`: raw process evidence for failures, timeouts, and transport
- `handoff`: generated stdout transport for the next model voice

## Hard Rules

- session is the durable run-state source
- spec state is projected from session evidence
- handoff is never read back for state
- diagnostics are not the primary surface for accepted findings
- the filesystem path must match lifecycle status

## Layout

```text
.scafld/
  specs/
    drafts/{task-id}.md
    approved/{task-id}.md
    active/{task-id}.md
    archive/YYYY-MM/{task-id}.md
  runs/
    {task-id}/
      diagnostics/
      session.json
```

`draft` specs live in `drafts/`; `approved` specs live in `approved/`;
`active`, `blocked`, and `review` specs live in `active/`; terminal specs move
to `archive/YYYY-MM/`.

## Session

`session.json` is the durable ledger.

It records typed events such as:

- `approval`
- `build`
- `criterion`
- `phase`
- `review`
- `complete`
- `fail`
- `cancel`

Criterion and phase state is replayed into `criterion_states` and
`phase_blocks`. Review entries store the verdict in `status`, the accepted
finding payload in `output`, and the provider in `provider`.

If a command writes evidence and then fails before updating the spec, the
session remains the source for reconciliation.

## Review Findings

Accepted review findings are promoted into the normal workflow:

- `scafld review` prints the findings and next repair command
- `scafld status` repeats the latest verdict and findings
- `scafld handoff` includes findings for the next repair agent
- the spec projects findings under `## Review`
- the session stores the accepted finding payload

Diagnostics remain useful for raw provider output and timeout analysis, but a
repair agent should not have to discover normal findings there.

## Handoff

`scafld handoff <task-id>` renders model-facing context to stdout from the
current spec and session. It is transport only. scafld does not persist handoff
JSON, and it does not use handoff text as state.

## Retention

On `complete`, `fail`, or `cancel`, the spec moves to:

```text
.scafld/specs/archive/{YYYY-MM}/{task-id}.md
```

Run ledgers and diagnostics remain under `.scafld/runs/{task-id}/` for audit.
