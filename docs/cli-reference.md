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
    "status": "active",
    "phase": "phase1",
    "passed": 0,
    "failed": 0,
    "next": "scafld handoff add-auth"
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
prompt files, creates spec/run directories, and installs managed core assets
under `.scafld/core/`.

`init` is deterministic. It does not ask an agent to infer project policy.

## config

```bash
scafld config [--root PATH] [--json]
```

Scans the workspace in read-only mode and writes
`.scafld/config.proposed.yaml`. The proposal contains cited evidence,
agent instructions, suggested invariant IDs, discovered validation commands,
and open questions.

`config` does not mutate `.scafld/config.yaml`. The operator or agent must
open the cited sources and copy only verified runtime policy into the real
config. Agent guidance belongs in `AGENTS.md`, `CLAUDE.md`, `.claude/rules`, or
project prompts rather than unsupported config fields.

## update

```bash
scafld update [--root PATH] [--json]
```

Refreshes managed `.scafld/core/` files from the bundled runtime. It also
refreshes `.scafld/prompts/*` copies that are still known defaults, while
skipping customized project prompts. It refreshes root agent docs and renders
generated `.scafld/config.yaml` into the current strict runtime shape. Specs,
runs, reviews, and local config are preserved.

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

With `--mark-passed`, it verifies the latest round's harden checks and
`Grounded in` citations, closes the round, and sets `harden_status: passed`.
Missing checks, failed checks, unresolved questions, and unresolved citations
fail closed and leave the round in progress.

Accepted citation shapes are `Grounded in: spec_gap:<field>`,
`Grounded in: code:<path>:<line>`, and `Grounded in: archive:<task-id>`.
Code citations must use an existing workspace-relative path and a real line
number.

Required checks are `Path audit`, `Command audit`, `Scope/migration audit`,
`Acceptance timing audit`, `Rollback/repair audit`, and `Design challenge`.
Each check must record `Result: passed` or `Result: not_applicable` plus
evidence. `Questions: none` is valid only after those checks have evidence.

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

Records approval in the session ledger, then moves the draft spec to
`.scafld/specs/approved/`. Approval is explicit operator action; it is not
implied by hardening.

## build

```bash
scafld build <task-id> [--json]
```

Runs the governed implementation loop. From `approved`, it activates the task,
captures the workspace baseline, opens the first phase, and points the agent at
`scafld handoff <task-id>`. It does not run future acceptance before the phase
has been implemented.

From `active` or `blocked`, `build` records evidence for the current phase. If
the phase passes, it opens the next phase. If the final phase and global
acceptance pass, it moves the task to `review`. Drafts, terminal specs, and
already-ready review specs are rejected.

Acceptance commands inherit the process environment plus `execution` overrides
from `.scafld/config.yaml` and `.scafld/config.local.yaml`. Use that config for
repo-wide toolchain setup such as rbenv shims instead of relying on interactive
shell startup.

Phase acceptance runs in order. If a phase blocks, later phase commands are not
run and the next command becomes `scafld handoff <task-id>` so the repair agent
gets the failed criterion, command, and evidence instead of a vague blocked
status.

## review

```bash
scafld review <task-id> [--provider auto|codex|claude|command|local] [--provider-command CMD] [--provider-binary PATH] [--model MODEL] [--review-scope PATH[,PATH...]] [--print-context] [--human-reviewed --reason TEXT] [--json]
```

`review` is the adversarial completion gate. Defaults come from
`.scafld/config.yaml` under `review.external`. Fresh workspaces use
`provider: auto`, which selects an installed external challenger (`codex`, then
`claude`). If neither is available, the command fails and tells the operator to
install a provider, use `--provider command`, or explicitly choose
`--provider local` for development smoke tests. Local verdicts cannot satisfy
`complete`.

Provider modes:

- `auto`: choose an installed external challenger.
- `codex`: run Codex in read-only ephemeral mode.
- `claude`: run Claude with read-only tools and the `submit_review` MCP tool.
- `command`: run a custom reviewer command; requires `--provider-command`.
- `local`: deterministic pass-through provider for development and tests only;
  its verdict cannot satisfy `complete`.
- `--human-reviewed`: record an audited operator review instead of invoking a
  provider. `--reason` is required and is stored in the session ledger.

Provider-specific model defaults come from
`review.external.codex.model` and `review.external.claude.model`. `--provider`,
`--provider-command`, `--provider-binary`, and `--model` override config for one
invocation.

`--print-context` prints the exact deterministic review-context packet without
invoking a provider. Use it when an agent needs to see why a reviewer is
under-informed or why a gate is likely to block before spending a model run.

scafld derives review scope from spec packages, impacted files, and phase
changes. Use `--review-scope` only when a dirty monorepo or workspace needs an
explicit path boundary:

```bash
scafld review email-contracts --review-scope api
scafld review email-contracts --review-scope api,cli/packages/mcp
```

The approval baseline is captured before task execution. Review compares the
current workspace to that baseline, reports task-scoped changes to the provider,
and includes changes outside declared scope as ambient workspace drift.
Unchanged baseline dirt and ambient drift are context, not findings by
themselves. Task-relevant files changed during review still fail closed;
unrelated workspace churn does not discard a valid review.

The provider returns a ReviewDossier. scafld validates it, rejects workspace
mutation in the review-relevant surface, writes the review event to session, and
projects the verdict back into the spec. A human-reviewed override writes a
`review_override` event before the passing review event. `complete` will not
archive the task unless the review verdict is `pass`.

On review failure, the text output prints the findings and next repair command.
The same findings appear in `scafld status`, `scafld handoff`, the session
review entry, and the spec `## Review` section.

## complete

```bash
scafld complete <task-id> [--json]
```

Archives completed work only after the latest session review event has a
`pass` verdict from `codex`, `claude`, `command`, or an audited human review.

## fail

```bash
scafld fail <task-id> [--reason TEXT] [--json]
```

Records the failure in session, then archives the spec.

## cancel

```bash
scafld cancel <task-id> [--reason TEXT] [--json]
```

Records the cancellation in session, then archives the spec.

## status

```bash
scafld status <task-id> [--json]
```

Shows lifecycle status, the next allowed follow-up command, and latest review
findings when present.

## list

```bash
scafld list [--json]
```

Lists all known specs by task id, status, and title.

## report

```bash
scafld report [--json]
```

Aggregates workspace spec counts and session-derived product metrics:

- `first_attempt_pass_rate`: tasks whose first completed build moved straight
  to review.
- `recovery_convergence_rate`: blocked first attempts that later recovered to
  review.
- `challenge_override_rate`: challenged tasks completed without a later
  passing review from `codex`, `claude`, or `command`. This should normally stay
  at `0`.
- `review_pass_rate`: accepted review verdicts over all review verdicts.
- `review_dossier_coverage`: review events that stored a valid ReviewDossier.
- `review_findings_total`: findings accepted across all valid dossiers.
- `review_open_blockers_total`: findings that still blocked completion when
  recorded.
- `review_attack_angles_total`: attack-log entries accepted across dossiers.
- `workspace_baseline_coverage`: sessions with an approval/build baseline.

Human output keeps the same numbers compact:

```text
total specs: 12
by status:
- review: 1
- completed: 9
metrics:
- first_attempt_pass_rate: 66.7% (8/12)
- recovery_convergence_rate: 75.0% (3/4)
- review_pass_rate: 80.0% (8/10)
- review_dossier_coverage: 100.0% (10/10)
- review_findings_total: 14
- review_open_blockers_total: 3
- review_attack_angles_total: 42
- review_mode_distribution:
  - discover: 7
  - verify: 3
- challenge_override_rate: 0.0% (0/2)
- workspace_baseline_coverage: 100.0% (12/12)
```

## handoff

```bash
scafld handoff <task-id>
```

Renders model-facing context from the current spec and session state. Handoffs
include failed or pending acceptance criteria while a task is blocked, and
latest review findings when present. They are transport, not source of truth.
