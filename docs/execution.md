---
title: Execution
description: Acceptance execution and session evidence
---

# Execution

Execution is deliberately less clever than the spec. It runs explicit commands,
records evidence, and projects current state back into the Markdown file.

```text
draft -> harden -> approve -> build -> review -> complete
```

## Build

```bash
scafld build <task-id>
```

`build` loads the approved or active spec, marks it active while work is
executed, runs every executable acceptance criterion, appends criterion evidence
to the session ledger, and projects criterion/phase state back into the spec.

If all criteria pass, the task moves to `review` and the allowed follow-up is:

```bash
scafld review <task-id>
```

If any criterion fails, the task becomes `blocked`; use `status` and the
recorded diagnostics to decide whether to repair, rerun build, fail, or cancel.

## exec

```bash
scafld exec <task-id>
```

`exec` uses the same execution path as `build`. It exists as a narrower command
name for wrappers that want to distinguish "execute acceptance" from the broader
build lifecycle verb.

## Evidence Flow

For each criterion, scafld records:

- command
- exit code
- matcher result
- short output snippet
- criterion id
- phase id

The session ledger is written before the spec projection. That ordering is what
lets scafld rebuild projected state from evidence if a write fails halfway.

## Handoffs

```bash
scafld handoff <task-id>
```

The current Go handoff is a compact model-facing summary of title, status, and
allowed next command. Handoff is transport only; it is never read back to compute
state.

## Process Safety

The process runner supports command stdin, timeout, idle timeout, process-group
termination, and diagnostic capture. Long-running provider review uses the same
runner surface as acceptance execution.
