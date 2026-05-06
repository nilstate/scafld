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

`init` creates `.scafld/`, installs the managed core bundle under
`.scafld/core/`, and creates project-owned prompt files under
`.scafld/prompts/`.

For a new repository, generate a grounded config proposal before the first
serious spec:

```bash
scafld configure
```

Review `.scafld/config.proposed.yaml` and copy only verified invariant IDs or
local review defaults into `.scafld/config.yaml`.

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

- what the product goal really is
- what artifact is authoritative when facts overlap
- which files and modules are in or out of bounds
- what can fail halfway and how a human recovers
- what invariants and golden examples prove the shape

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

`build` activates approved work, runs acceptance criteria, writes evidence to
the session ledger, and projects the current state back into the spec.

## Run Adversarial Review

Use an external challenger for real work:

```bash
scafld review add-auth --provider codex
scafld review add-auth --provider claude
scafld review add-auth --provider command --provider-command "./reviewer"
```

With no provider flag, scafld uses `--provider auto` and selects an installed
external challenger. If none is installed, review fails closed. Use
`--provider local` only for development smoke tests; local verdicts cannot
satisfy `complete`.

## Complete

```bash
scafld complete add-auth
```

`complete` archives only work with a non-local passing review in the session. If
review returns a blocking finding, repair the work, rerun `build` as needed,
then rerun `review`.

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
