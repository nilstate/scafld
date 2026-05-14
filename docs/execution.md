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

`build` is the governed implementation loop. From `approved`, it activates the
task, captures the workspace baseline, opens the first phase, and prints the
allowed handoff. It does not run acceptance for files that do not exist yet.

After the agent implements the opened phase, run `scafld build <task-id>`
again. That second call records evidence for the current phase, then either
blocks, opens the next phase, or moves the task to review after final
acceptance.

Acceptance commands run in a non-login shell. scafld does not read interactive
shell startup files. Instead it builds the command environment from checked-in
project evidence, then overlays `.scafld/config.yaml` and
`.scafld/config.local.yaml`.

When repo toolchain files declare language versions, scafld automatically
prepends common version-manager shims. `.tool-versions` enables asdf/mise
shims, `mise.toml` enables mise shims, and language-specific files such as
`.ruby-version`, `.python-version`, `.node-version`, `.go-version`, and
`.java-version` enable the matching common shim directory. Declare
project-specific overrides in config:

| Checked-in file | Auto-prepended shims |
| --- | --- |
| `.tool-versions` | `$HOME/.asdf/shims`, `$HOME/.local/share/mise/shims`, `$HOME/.mise/shims` |
| `mise.toml`, `.mise.toml` | `$HOME/.local/share/mise/shims`, `$HOME/.mise/shims` |
| `.ruby-version` | `$HOME/.rbenv/shims` |
| `.python-version` | `$HOME/.pyenv/shims` |
| `.node-version`, `.nvmrc` | `$HOME/.nodenv/shims` |
| `.go-version` | `$HOME/.goenv/shims` |
| `.java-version` | `$HOME/.jenv/shims` |

```yaml
execution:
  absolute_timeout_seconds: 300
  idle_timeout_seconds: 0
  path_prepend:
    - "$HOME/.rbenv/shims"
    - "$HOME/.rbenv/bin"
  env:
    BUNDLE_GEMFILE: "api/Gemfile"
```

This makes validation deterministic. If a command needs rbenv, asdf, pnpm, or
another shim, the dependency is visible in the workspace contract instead of
hidden in a developer's interactive shell startup. Explicit config paths are
placed before auto-detected shims.

Each acceptance command has an absolute timeout. The default is 300 seconds.
Raise `execution.absolute_timeout_seconds` for legitimate slow project tools
such as cold typechecks. `execution.idle_timeout_seconds` defaults to `0` and
only applies when explicitly set.

Approval captures the workspace baseline before task execution starts. Review
uses that baseline later to separate task changes from unrelated pre-existing
dirty state.

Build is phase-sequenced. scafld opens exactly one phase at a time. A phase is
`active` while the agent is implementing it, `completed` only after its
acceptance evidence passes, and `blocked` only after attempted evidence fails.
Later phases do not get evidence while an earlier phase is active or blocked.

The normal agent loop is:

```bash
scafld build <task-id>   # open current phase
scafld handoff <task-id> # read what to implement
# implement the phase
scafld build <task-id>   # record phase evidence and advance
```

If all criteria pass, the task moves to `review` and the allowed follow-up is:

```bash
scafld review <task-id>
```

If attempted evidence fails or cannot be evaluated, the task becomes
`blocked`; use `scafld handoff <task-id>` to get the failed criteria, commands,
and reasons for the repair agent. Pending future phase criteria are not
blockers.

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
