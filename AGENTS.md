# scafld - Agent Guide

Canonical guide for agents working in a scafld-managed repo.

Read this before doing work.

## Core Model

- `spec` is the reviewed contract
- `session` is the durable run ledger
- `handoff` is transport: immutable `*.md + *.json`, tagged by `role × gate`

Do not treat the raw spec as the prompt.

## Identity

scafld builds long-running AI coding work under adversarial review.

The agent may execute autonomously inside the contract, but it does not get to
close the task unchallenged. The review gate is the quality boundary.

## Lifecycle

```text
plan -> approve -> build -> review -> complete
```

`init` stays public because seeding a repo is part of the core workflow.

## Agent-Facing Commands

Use these by default:

```bash
scafld init
scafld plan <task-id> [-t title] [-s size] [-r risk]
scafld approve <task-id>
scafld build <task-id>
scafld review <task-id>
scafld complete <task-id>
scafld status <task-id>
scafld list
scafld report
scafld handoff <task-id>
scafld update
```

Use `scafld handoff` when an external harness needs the current handoff without
moving the lifecycle.

Use `scafld review <task-id>` as the default challenger entrypoint. Use the
wrapper scripts when you explicitly want the provider-specific handoff adapter:

- `.scafld/core/scripts/scafld-codex-build.sh <task-id>` resolves the current scafld handoff
  and pipes it to Codex before the model acts
- `.scafld/core/scripts/scafld-codex-review.sh <task-id>` is the optional codex handoff adapter for challenger review
- `.scafld/core/scripts/scafld-claude-build.sh <task-id>` does the same for Claude Code
- `.scafld/core/scripts/scafld-claude-review.sh <task-id>` is the optional claude handoff adapter for challenger review

Prompt ownership is simple:

- `.scafld/prompts/*.md` is the active template layer
- `.scafld/core/prompts/*.md` is the managed reset copy

## Handoffs

Current role×gate handoffs:

- `executor × phase`
- `executor × recovery`
- `executor × review_repair`
- `challenger × review`

Hard rules:

- read the generated handoff before acting
- never read a handoff back to compute state
- read `session.json` when you need durable state

Defaults:

- `scafld handoff <task-id>` returns the active phase handoff
- if no phase is active yet, it returns `phase1`
- after completion, it returns the archived review handoff
- after a structured external review finds issues, read
  `.scafld/runs/{task-id}/handoffs/executor-review-repair.md` before repairing; it
  is packet-derived review context, not lifecycle state

## Execution

`build` has two modes:

- approved spec: starts the task and immediately runs validation to the next handoff or block
- active spec: runs the next execution pass and emits the next executor or recovery handoff

`status` is the control tower:

- trust `result.next_action`
- trust `result.current_handoff`
- do not reconstruct lifecycle state manually when status already tells you what happens next

During execution:

- stay inside the declared files, invariants, and acceptance criteria
- use recovery handoffs when validation fails
- let phase summaries replace old chatter

## Review

`review` is the hero gate.

- it runs automated checks first
- it defaults to a fresh external challenger when configured for `external`
- it still exposes a `challenger × review` handoff for explicit `local` or `manual` review modes
- the challenger writes findings into `.scafld/reviews/{task-id}.md`
- `complete` closes only if the gate passes, or a human uses the audited override path

The review stance is adversarial:

- find defects
- cite file and line
- separate blocking and non-blocking findings
- do not rewrite code from the review handoff

## Metrics

The report surface that matters:

- `first_attempt_pass_rate`
- `recovery_convergence_rate`
- `challenge_override_rate`

These measure session outcomes. They do not prove an external harness actually
consumed the handoff.

## Invariants

Project-specific invariants live in:

- `CONVENTIONS.md`
- `.scafld/config.yaml`

Typical examples:

- preserve layer boundaries
- do not introduce hardcoded secrets
- do not add runtime fallbacks without approval
- keep public APIs stable unless the spec explicitly allows breakage

## Spec Management

Always use the CLI for lifecycle moves. Never hand-edit spec status or move spec
files between directories manually.

## Review Override

Human override is exceptional and audited:

```bash
scafld complete <task-id> --human-reviewed --reason "manual audit"
```

It exists for blocked review gates, not as a routine shortcut.
It is only available after a completed challenger review round exists.
