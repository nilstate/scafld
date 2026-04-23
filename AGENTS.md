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

Those commands sit on top of the stable internal lifecycle:

```text
new -> harden -> approve -> start -> exec -> review -> complete
```

## Agent-Facing Commands

Use these by default:

```bash
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

Prompt ownership is simple:

- `.ai/prompts/*.md` is the active template layer
- `.ai/scafld/prompts/*.md` is the managed reset copy

## Handoffs

Current role×gate handoffs:

- `executor × phase`
- `executor × recovery`
- `challenger × review`

Hard rules:

- read the generated handoff before acting
- never read a handoff back to compute state
- read `session.json` when you need durable state

Defaults:

- `scafld handoff <task-id>` returns the active phase handoff
- if no phase is active yet, it returns `phase1`
- after completion, it returns the archived review handoff

## Execution

`build` has two modes:

- approved spec: starts the task and immediately runs validation to the next handoff or block
- active spec: runs `exec --resume` and emits the next executor or recovery handoff

During execution:

- stay inside the declared files, invariants, and acceptance criteria
- use recovery handoffs when validation fails
- let phase summaries replace old chatter

## Review

`review` is the hero gate.

- it runs automated checks first
- it emits a `challenger × review` handoff
- the challenger writes findings into `.ai/reviews/{task-id}.md`
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
- `.ai/config.yaml`

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
