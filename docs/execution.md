---
title: Execution
description: Executor handoffs, recovery, and session-led execution
---

# Execution

The lifecycle is unchanged:

```text
draft -> harden -> approve -> build -> review -> complete
```

The runtime model underneath it is:

- `spec`: reviewed contract
- `session`: durable run ledger
- `handoff`: generated transport for the next voice

Execution uses two handoff shapes:

- `executor × phase`
- `executor × recovery`

## Build

```bash
scafld build <task-id>
```

`build` is the agent-facing executor wrapper:

- approved spec: initializes `session.json`, emits the first phase handoff, and runs execution
- active spec: runs the next execution pass

That means a fresh `build` call from `approved` advances work to the next
handoff or block in one invocation instead of requiring a second `build`.

In JSON mode, treat these as the canonical executor signals:

- `result.next_action`
- `result.current_handoff`
- `result.block_reason`

`status --json` mirrors the same guidance without moving the lifecycle.

## Execution Passes

```bash
scafld build <task-id>
```

For each runnable criterion, execution:

1. runs the command
2. records a short audit-friendly result snippet back into the spec
3. appends the full attempt to `session.json`
4. writes full diagnostics into `.scafld/runs/{task-id}/diagnostics/`

The spec stays concise. Session carries the real run history.

## Recovery

Recovery is not a subsystem.

It is:

- a `recovery` gate on the handoff
- a counter in session
- a max-attempt policy in config

When a criterion fails inside the configured budget, scafld emits:

- `executor-recovery-{criterion}-{attempt}.md`
- `executor-recovery-{criterion}-{attempt}.json`

That handoff includes:

- failed criterion
- expected result
- diagnostic reference
- prior attempts for the same criterion
- current phase slice
- prior phase summary

Use `status --json` or `build --json` to discover whether recovery is pending.
Use `scafld handoff <task-id> --recovery <criterion>` when you need the current
recovery handoff without moving the lifecycle.

## Phase Summaries

At phase boundaries, scafld writes compact `phase_summary` entries into session.

Later executor handoffs read those summaries instead of replaying old tool
output. That is how long runs avoid linear context growth.

## Recovery Cap

`llm.recovery.max_attempts` is hard.

When the next failure would exceed the cap, execution:

- stops emitting recovery handoffs
- records `failed_exhausted` in session
- blocks the phase
- returns `human_required` in JSON output

## Honest Boundary

scafld can generate a better executor handoff. It cannot force an external
harness to use it. Session metrics measure outcomes, not handoff consumption.

When the workspace includes them, the wrapper scripts remain optional handoff
adapters for Codex and Claude Code:

- `.scafld/core/scripts/scafld-codex-build.sh <task-id>`
- `.scafld/core/scripts/scafld-claude-build.sh <task-id>`
- `.scafld/core/scripts/scafld-codex-review.sh <task-id>`
- `.scafld/core/scripts/scafld-claude-review.sh <task-id>`

`scafld review` itself is now the default challenger entrypoint.

Prompt ownership is also explicit:

- `.scafld/prompts/exec.md` and `.scafld/prompts/recovery.md` are the active template sources
- `.scafld/core/prompts/*.md` is the managed reset copy that `scafld update` refreshes
