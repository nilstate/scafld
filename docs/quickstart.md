---
title: Quickstart
description: From zero to first reviewed task
---

# Quickstart

## Initialize a Workspace

```bash
cd your-project
scafld init
```

`init` creates `.scafld/` and installs the managed core bundle under
`.scafld/core/`. Project prompt overrides are optional; add files under
`.scafld/prompts/` only when you need to customize the built-in prompts.

For a new repository, generate a grounded config proposal before the first
serious spec:

```bash
scafld config
```

Review `.scafld/config.proposed.yaml`, follow its `agent_instructions`, and
copy only verified runtime policy into `.scafld/config.yaml`. Put agent-facing
guidance in `AGENTS.md`, `CLAUDE.md`, `.claude/rules`, or project prompts
instead of inventing config fields.

## Create a Draft Spec

```bash
scafld plan add-auth --title "Add JWT authentication" --size medium --risk high --command "npm test"
```

This writes `.scafld/specs/drafts/add-auth.md`. Open the file and tighten the
human-readable contract: objective, scope, context, phase changes, and
acceptance criteria.

## Harden Before Approval

```bash
scafld harden add-auth
```

The command enters HARDEN MODE and records a hardening round in the spec. Use
the printed prompt to attack the plan before execution:

- whether paths and commands are real
- whether scope, migration, and cutover claims are honest
- whether acceptance criteria run at the right phase
- whether rollback or repair is realistic
- why the plan exists, whether it solves the underlying problem, and whether it
  is a short-sighted bandaid, future bloat, or the wrong abstraction

When the answers are worked into the spec:

```bash
scafld harden add-auth --mark-passed
```

Hardening is not ceremony. It is the step that keeps the agent from discovering
the definition of done while already changing code.

## Approve and Build

```bash
scafld validate add-auth
scafld approve add-auth
scafld build add-auth
```

`build` activates approved work, opens the current phase, and projects the
current state back into the spec. The first build does not run future
acceptance. Read the handoff, implement the phase, then run
`scafld build add-auth` again to record evidence and advance.

## Run Adversarial Review

Use an external challenger for real work:

```bash
scafld review add-auth --provider codex
scafld review add-auth --provider claude
scafld review add-auth --provider gemini
scafld review add-auth --provider command --provider-command "./reviewer"
scafld review add-auth --print-context
```

With no provider flag, scafld uses `--provider auto`. If scafld can infer the
current host agent, it prefers the other installed challenger, can use Gemini as
another external challenger, and fails closed when only the host provider is
available. If no external challenger is installed, review fails closed. Use
`--provider local` only for development smoke tests; local verdicts cannot
satisfy `complete`.

`--print-context` shows the exact deterministic brief before you spend a review
run.

## Complete

```bash
scafld complete add-auth
```

`complete` archives only work with a passing `codex`, `claude`, `gemini`,
`command`, or audited human review in the session. If review returns a blocking finding,
repair the work, rerun `build` as needed, then rerun `review`.

## Inspect State

```bash
scafld status add-auth
scafld list
scafld report
```

The lifecycle stays deliberately small:

```text
draft -> harden -> approved -> active -> review -> completed
```

## Next Steps

- [Lifecycle](lifecycle) -- states and transitions
- [Spec Schema](spec-schema) -- Markdown spec grammar
- [CLI Reference](cli-reference) -- command and flag surface
- [Review](review) -- adversarial review gate
