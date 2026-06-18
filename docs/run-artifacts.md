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
finding payload in `output`, the accepted `review_packet`, the
`canonical_response_sha256`, the provider in `provider`, provider provenance in
`provider_model` and `provider_session` when available, and the reviewed
workspace seal in `reviewed_head`, `reviewed_dirty`, and `reviewed_diff`.
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
      "canonical_response_sha256": "d6552ecb8f6b...",
      "provider_model": "gpt-5.5",
      "provider_session": "session-123",
      "reviewed_head": "a1b2c3d4",
      "reviewed_dirty": "true",
      "reviewed_diff": "9f8e7d6c5b4a...",
      "output": "{\"verdict\":\"fail\",\"mode\":\"discover\",\"summary\":\"Review found one open completion blocker.\",\"findings\":[{\"id\":\"cache-tenant-leak\",\"severity\":\"high\",\"blocks_completion\":true,\"location\":{\"path\":\"internal/cache/store.go\",\"line\":88},\"evidence\":\"invalidation keys omit tenant id\",\"impact\":\"cross-tenant cache state can leak\",\"validation\":\"go test ./internal/cache\",\"summary\":\"tenant id omitted from cache key\"}],\"attack_log\":[{\"target\":\"cache invalidation\",\"attack\":\"trace tenant key construction\",\"result\":\"finding\"}],\"budget\":{\"actual_findings\":1,\"actual_attack_angles\":1}}"
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
- the session stores the accepted review dossier

Diagnostics remain useful for raw provider output and timeout analysis, but a
repair agent should not have to discover normal findings there.

Review providers return one final ReviewDossier:

```json
{
  "verdict": "fail",
  "mode": "discover",
  "summary": "Review found one open completion blocker.",
  "findings": [
    {
      "id": "cache-tenant-leak",
      "severity": "high",
      "blocks_completion": true,
      "location": {"path": "internal/cache/store.go", "line": 88},
      "evidence": "invalidation keys omit tenant id",
      "impact": "cross-tenant cache state can leak",
      "validation": "go test ./internal/cache",
      "summary": "tenant id omitted from cache key"
    }
  ],
  "attack_log": [
    {"target": "cache invalidation", "attack": "trace tenant key construction", "result": "finding"}
  ],
  "budget": {"actual_findings": 1, "actual_attack_angles": 1}
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
    "mode": "discover",
    "summary": "Review found one open completion blocker.",
    "open_blockers": 1,
    "findings": [
      {
        "id": "cache-tenant-leak",
        "severity": "high",
        "blocks_completion": true,
        "summary": "tenant id omitted from cache key"
      }
    ]
  }
}
```

## Completion Authority

Archived completed tasks are immutable. `scafld build` and `scafld review`
will not reopen them; create a new task for follow-up work.

For completed tasks, scafld derives a terminal completion authority from the
session ledger:

- latest passing external review before `complete`, with sealed review packet,
  matching canonical hash, and reviewed workspace state
- audited human-reviewed override before `complete`
- integrity error when a completed ledger has no valid terminal authority

Historical failed reviews remain in the ledger. They are evidence, not current
state, once a later passing review or audited human-reviewed override has
authorized completion.

`scafld status --json` exposes this under `completion_authority`:

```json
{
  "task_id": "add-cache",
  "status": "completed",
  "completion_authority": {
    "status": "valid",
    "kind": "review",
    "provider": "codex",
    "verdict": "pass",
    "review_event": "review-2",
    "complete_event": "complete-1",
    "summary": "Review passed with no open completion blockers."
  }
}
```

If archive state and ledger evidence disagree, the authority is invalid:

```json
{
  "task_id": "add-cache",
  "status": "completed",
  "completion_authority": {
    "status": "invalid",
    "kind": "invalid",
    "reason": "latest review gate has not passed",
    "actual": "latest review verdict fail"
  }
}
```

## Handoff

`scafld handoff <task-id>` renders model-facing context to stdout from the
current spec and session. Blocked handoffs include failed or pending acceptance
criteria with commands and reasons. Review handoffs include the latest accepted
review findings. Completed handoffs include the terminal completion authority so
old failed reviews are not mistaken for current blockers. Handoff is transport
only: scafld does not persist handoff JSON, and it does not use handoff text as
state.

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
    "review_dossier_coverage": 1,
    "review_dossier_total": 10,
    "review_findings_total": 14,
    "review_open_blockers_total": 3,
    "review_attack_angles_total": 42,
    "review_mode_distribution": {
      "discover": 7,
      "verify": 3
    },
    "workspace_baseline_coverage": 1
  }
}
```
