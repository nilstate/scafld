# scafld Runtime

scafld builds long-running AI coding work under adversarial review.

## Core Model

- `spec`: reviewed contract
- `session`: durable run ledger
- `handoff`: generated transport for the next voice

The lifecycle stays the same internally, but the taught surface is smaller:

```text
plan -> approve -> build -> review -> complete
```

## Directory Layout

```text
.ai/
  config.yaml
  config.local.yaml
  prompts/
    plan.md
    exec.md
    recovery.md
    review.md
    harden.md
  runs/
    {task-id}/
      handoffs/
      diagnostics/
      session.json
    archive/{YYYY-MM}/{task-id}/
  reviews/
  schemas/
  specs/
```

Prompt ownership:

- `.ai/prompts/*` is the active template layer
- `.ai/scafld/prompts/*` is the managed reset copy

## Handoffs

Each handoff is a sibling pair:

- `*.md` for the model
- `*.json` for the harness

Current runtime handoffs:

- `executor-phase-*`
- `executor-recovery-*`
- `challenger-review`

The handoff is one-way. scafld emits it; the system observes outcomes through
the filesystem and criteria runs.

## Review

Challenge fires at `review` only in v1.

That means:

- one challenger handoff per task
- one completion gate that matters
- one attribution metric that stays honest: `challenge_override_rate`

## Metrics

`report` surfaces:

- `first_attempt_pass_rate`
- `recovery_convergence_rate`
- `challenge_override_rate`
