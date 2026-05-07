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

`build` loads approved work, or previously active/blocked/review work that is
being repaired. It marks the task active while work is executed, runs executable
acceptance criteria, appends criterion evidence to the session ledger, and
projects criterion/phase state back into the spec.

Acceptance commands run in a non-login shell with inherited environment plus
the `execution` overrides from `.scafld/config.yaml` and
`.scafld/config.local.yaml`. Declare repo-wide toolchain setup there:

```yaml
execution:
  path_prepend:
    - "$HOME/.rbenv/shims"
    - "$HOME/.rbenv/bin"
  env:
    BUNDLE_GEMFILE: "api/Gemfile"
```

This makes validation deterministic. If a command needs rbenv, asdf, pnpm, or
another shim, the dependency is visible in the workspace contract instead of
hidden in a developer's interactive shell startup.

Approval captures the workspace baseline before task execution starts. Review
uses that baseline later to separate task changes from unrelated pre-existing
dirty state.

Build is phase-sequenced. scafld runs a phase's acceptance criteria, records
that evidence, and stops immediately if the phase blocks. Later phases do not
get passing evidence while an earlier phase is failed or pending.

If all criteria pass, the task moves to `review` and the allowed follow-up is:

```bash
scafld review <task-id>
```

If any criterion is missing evidence, fails, or cannot be evaluated, the task
becomes `blocked`; use `scafld handoff <task-id>` to get the failed or pending
criteria, commands, and reasons for the repair agent.

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

The handoff is a compact model-facing summary of title, status, allowed next
command, blocked acceptance evidence, and latest review findings when present.
Handoff is transport only; it is never read back to compute state.

## Process Safety

The process runner supports command stdin, timeout, idle timeout, process-group
termination, and diagnostic capture. Long-running provider review uses the same
runner surface as acceptance execution.
