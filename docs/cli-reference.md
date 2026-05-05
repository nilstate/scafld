---
title: CLI Reference
description: Current scafld command surface
---

# CLI Reference

scafld is intentionally small. The binary teaches the same command surface to
humans, agents, wrappers, and package launchers:

```bash
scafld init
scafld plan <task-id>
scafld harden <task-id>
scafld validate <task-id>
scafld approve <task-id>
scafld build <task-id>
scafld exec <task-id>
scafld review <task-id>
scafld complete <task-id>
scafld fail <task-id>
scafld cancel <task-id>
scafld status <task-id>
scafld list
scafld report
scafld handoff <task-id>
scafld update
```

Global flags:

- `--root PATH`: operate on a specific workspace root.
- `--json`: emit a stable JSON envelope when the command supports it.
- `--version`: print the binary version.

## JSON Mode

Automation-relevant commands emit one envelope:

```json
{
  "ok": true,
  "command": "build",
  "result": {
    "task_id": "add-auth",
    "status": "review",
    "passed": 1,
    "failed": 0
  }
}
```

Failures use the same shape with `ok: false` and an `error` object carrying
`code`, `message`, and `exit_code`.

The envelope and every command `result` use `snake_case` JSON keys. The
Markdown spec schema, session ledger, and CLI automation output therefore share
one public casing convention.

Exit codes:

- `0`: success
- `1`: generic runtime failure
- `2`: invalid command or flag
- `3`: validation or acceptance failure
- `4`: review gate failure
- `5`: cancelled context
- `6`: workspace discovery or bootstrap failure

## init

```bash
scafld init [--root PATH] [--json]
```

Bootstraps `.scafld/` in the workspace. It installs project-owned config and
prompt files, creates spec/run/review directories, and installs managed core
assets under `.scafld/core/`.

## update

```bash
scafld update [--root PATH] [--json]
```

Refreshes managed `.scafld/core/` files from the bundled runtime. It preserves
project-owned files such as `.scafld/config.yaml`, `.scafld/config.local.yaml`,
`.scafld/prompts/*`, specs, runs, and reviews.

## plan

```bash
scafld plan <task-id> [--title TITLE] [--summary TEXT] [--size SIZE] [--risk RISK] [--command CMD] [--json]
```

Creates `.scafld/specs/drafts/<task-id>.md`. `--command` seeds the first
executable acceptance criterion. Existing drafts are not overwritten.

## harden

```bash
scafld harden <task-id> [--mark-passed] [--json]
```

Hardening is the pre-build adversarial pass. It attacks the draft before
approval: product goal, authority, ownership boundaries, halfway failure
repair, hidden cutovers, testable invariants, golden examples, and recovery
commands.

Without flags, `harden` appends a round, sets `harden_status: in_progress`, and
prints the active prompt from `.scafld/prompts/harden.md`, falling back to
`.scafld/core/prompts/harden.md` and then the built-in prompt.

With `--mark-passed`, it warning-checks the latest round's `Grounded in`
citations, closes the round, and sets `harden_status: passed`.

## validate

```bash
scafld validate <task-id> [--json]
```

Parses the Markdown spec into the normalized model and rejects malformed
lifecycle state, phase identity, harden state, duplicate criteria, or
non-executable acceptance criteria.

## approve

```bash
scafld approve <task-id> [--json]
```

Moves a draft spec to approved and records the approval in the session ledger.
Approval is explicit operator action; it is not implied by hardening.

## build

```bash
scafld build <task-id> [--json]
```

Activates approved work and runs executable acceptance criteria. Evidence is
written to the session first and projected back into the Markdown spec.

## exec

```bash
scafld exec <task-id> [--json]
```

Runs the execution path against the current task. It uses the same acceptance
model and evidence-writing discipline as `build`.

## review

```bash
scafld review <task-id> [--provider auto|codex|claude|command|local] [--provider-command CMD] [--provider-binary PATH] [--model MODEL] [--json]
```

`review` is the adversarial completion gate. Defaults come from
`.scafld/config.yaml` under `review.external`. Fresh workspaces use
`provider: auto`, which selects an installed external challenger (`codex`, then
`claude`). If neither is available, the command fails and tells the operator to
install a provider, use `--provider command`, or explicitly choose
`--provider local` for development smoke tests.

Provider modes:

- `auto`: choose an installed external challenger.
- `codex`: run Codex in read-only ephemeral mode.
- `claude`: run Claude with read-only tools and structured JSON output.
- `command`: run a custom reviewer command; requires `--provider-command`.
- `local`: deterministic pass-through provider for development and tests only.

Provider-specific model defaults come from
`review.external.codex.model` and `review.external.claude.model`. `--provider`,
`--provider-command`, `--provider-binary`, and `--model` override config for one
invocation.

The provider returns a ReviewPacket. scafld validates it, rejects workspace
mutation, writes the review event to session, and projects the verdict back into
the spec. `complete` will not archive the task unless the review verdict is
`pass`.

On review failure, the text output prints the findings and next repair command.
The same findings appear in `scafld status`, `scafld handoff`, the session
review entry, and the spec `## Review` section.

## complete

```bash
scafld complete <task-id> [--json]
```

Archives completed work only after the review gate has passed.

## fail

```bash
scafld fail <task-id> [--reason TEXT] [--json]
```

Archives a task as failed and records the reason in session.

## cancel

```bash
scafld cancel <task-id> [--reason TEXT] [--json]
```

Archives a task as cancelled and records the reason in session.

## status

```bash
scafld status <task-id> [--json]
```

Shows lifecycle status and the next allowed follow-up command.

## list

```bash
scafld list [--json]
```

Lists all known specs by task id, status, and title.

## report

```bash
scafld report [--json]
```

Aggregates workspace spec counts and runtime evidence. This is the reporting
surface that will grow as session-derived metrics deepen.

## handoff

```bash
scafld handoff <task-id>
```

Renders model-facing context from the current spec state. Handoffs are
transport, not source of truth; scafld computes state from the spec and session.
