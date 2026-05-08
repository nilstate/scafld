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

- `workspace_baseline`
- `approval`
- `build`
- `criterion`
- `phase`
- `review`
- `review_override`
- `complete`
- `fail`
- `cancel`

Criterion and phase state is replayed into `criterion_states` and
`phase_blocks`. Review entries store the verdict in `status`, the accepted
finding payload in `output`, and the provider in `provider`.
Human-reviewed overrides record a `review_override` entry with the operator
reason before the passing `review` entry with provider `human`.

`workspace_baseline` is captured at approval before task execution starts. It
records dirty workspace fingerprints that review later uses to ignore unchanged
pre-existing dirt and identify new task or scope-drift changes.

If a command writes evidence and then fails before updating the spec, the
session remains the source for reconciliation.

Minimal session excerpt:

```json
{
  "task_id": "add-cache",
  "entries": [
    {
      "type": "workspace_baseline",
      "status": "captured",
      "output": " M hash README.md"
    },
    {
      "type": "criterion",
      "criterion_id": "ac1",
      "phase_id": "phase1",
      "status": "pass",
      "command": "go test ./internal/cache",
      "exit_code": 0
    },
    {
      "type": "review",
      "status": "fail",
      "provider": "codex",
      "output": "[{\"id\":\"cache-tenant-leak\",\"severity\":\"blocking\",\"summary\":\"internal/cache/store.go:88 invalidation keys omit tenant id.\"}]"
    }
  ]
}
```

## Review Findings

Accepted review findings are promoted into the normal workflow:

- `scafld review` prints the findings and next repair command
- `scafld status` repeats the latest verdict and findings
- `scafld handoff` includes findings for the next repair agent
- the spec projects findings under `## Review`
- the session stores the accepted finding payload

Diagnostics remain useful for raw provider output and timeout analysis, but a
repair agent should not have to discover normal findings there.

Review providers return one final ReviewPacket:

```json
{
  "verdict": "fail",
  "findings": [
    {
      "id": "cache-tenant-leak",
      "severity": "blocking",
      "summary": "internal/cache/store.go:88 invalidation keys omit tenant id."
    }
  ]
}
```

`scafld status --json` then exposes the accepted state without requiring an
agent to scrape diagnostics:

```json
{
  "task_id": "add-cache",
  "status": "review",
  "title": "Add Cache",
  "next": "scafld handoff add-cache",
  "session_ok": true,
  "review": {
    "verdict": "fail",
    "findings": [
      {
        "id": "cache-tenant-leak",
        "severity": "blocking",
        "summary": "internal/cache/store.go:88 invalidation keys omit tenant id."
      }
    ]
  }
}
```

## Handoff

`scafld handoff <task-id>` renders model-facing context to stdout from the
current spec and session. Blocked handoffs include failed or pending acceptance
criteria with commands and reasons. Review handoffs include the latest accepted
review findings. Handoff is transport only: scafld does not persist handoff
JSON, and it does not use handoff text as state.

## Retention

On `complete`, `fail`, or `cancel`, the spec moves to:

```text
.scafld/specs/archive/{YYYY-MM}/{task-id}.md
```

Run ledgers and diagnostics remain under `.scafld/runs/{task-id}/` for audit.

## Reports

`scafld report --json` is derived from the same sessions:

```json
{
  "total": 12,
  "by_status": {
    "draft": 2,
    "review": 1,
    "completed": 9
  },
  "metrics": {
    "first_attempt_pass_rate": 0.67,
    "first_attempt_passes": 8,
    "first_attempt_total": 12,
    "recovery_convergence_rate": 0.75,
    "recovered_tasks": 3,
    "recovery_total": 4,
    "challenge_override_rate": 0,
    "challenge_overrides": 0,
    "review_challenge_total": 2,
    "workspace_baseline_coverage": 1
  }
}
```
