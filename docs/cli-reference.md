---
title: CLI Reference
description: Default agent surface plus advanced operator tools
---

# CLI Reference

Default help teaches the agent-facing surface:

```bash
scafld init
scafld plan <task-id>
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

Use `scafld --help --advanced` for the full operator surface.

When the workspace includes them, these wrappers resolve the current handoff
before the external agent acts:

```bash
scripts/scafld-codex-build.sh <task-id>
scripts/scafld-codex-review.sh <task-id>
scripts/scafld-claude-build.sh <task-id>
scripts/scafld-claude-review.sh <task-id>
```

## JSON Mode

Automation-relevant commands support `--json` and emit one stable envelope:

```json
{
  "ok": true,
  "command": "build",
  "task_id": "add-auth",
  "warnings": [],
  "state": {},
  "result": {},
  "error": null
}
```

## plan

```bash
scafld plan <task-id> [-t TITLE] [-s SIZE] [-r RISK] [--json]
```

Creates the draft spec and immediately opens its harden round.

If the draft already exists, `plan` reopens harden instead of failing.

## approve

```bash
scafld approve <task-id> [--json]
```

Moves a validated draft to approved and records the approval in session.

## build

```bash
scafld build <task-id> [--json]
```

Wrapper behavior:

- approved spec: activates the task and immediately runs execution
- active spec: runs the next execution pass
- default execution scope: current open phase only
- if that phase passes, `build` emits the next phase handoff and stops there
- later phases do not run until another explicit `build` or `exec`

Important JSON fields:

- `state.action == "start_exec"`
- `result.executed_phase`
- `result.initial_handoff`
- `result.next_action`
- `result.current_handoff`
- `result.block_reason`

`result.next_action` is the canonical next step. `result.current_handoff`
describes the handoff the agent should read next when one is available.

## exec

```bash
scafld exec <task-id> [--phase PHASE] [--resume] [--json]
```

Default behavior matches `build`: without `--phase`, scafld resolves the
current open phase and executes only that phase's criteria.

Important JSON fields:

- `state.executed_phase`
- `result.executed_phase`
- `result.criteria`
- `result.next_handoff`
- `result.next_action`

## status

```bash
scafld status <task-id> [--json]
```

`status` is the control tower surface.

Important JSON fields:

- `result.next_action`
- `result.current_handoff`
- `result.block_reason`
- `result.review_gate`
- `result.lifecycle_flags.active_done_open`
- `result.supersession`

If a wrapper needs to know what to do next, it should start with `status --json`
instead of reconstructing lifecycle state manually.

## review

```bash
scafld review <task-id> [--runner external|local|manual] [--provider auto|codex|claude] [--model MODEL] [--json]
```

Default text-mode behavior:

- run automated passes
- append or refresh the latest review round
- resolve the configured review runner
- if `external`, execute a fresh external challenger, parse its prose review
  body, and let scafld write the completed review round only after validation
- if `local` or `manual`, emit the challenger prompt and leave the round
  `in_progress`

`--json` stays machine-facing snapshot mode: it returns the review prompt,
handoff paths, required sections, and resolved runner/provider/model metadata
without spawning the external reviewer.

External provider calls fail rather than hang when `review.external.timeout_seconds`
is reached. Invalid external output also fails the review command; it leaves the
latest round in progress and does not suggest `scafld complete`.

For `provider: auto`, `review.external.fallback_policy` controls Codex to Claude
fallback: `warn` allows it with a visible warning, `allow` allows it while
recording the weaker isolation in provenance, and `disable` requires Codex.

Important JSON fields:

- `handoff_file`
- `handoff_json_file`
- `handoff_role`
- `handoff_gate`
- `review_runner`
- `current_handoff`
- `next_action`

## complete

```bash
scafld complete <task-id> [--json]
scafld complete <task-id> --human-reviewed --reason "manual audit"
```

Archives only after the review gate passes, or after the audited human override
path is explicitly confirmed after a completed challenger review round exists.

## handoff

```bash
scafld handoff <task-id> [--phase PHASE | --recovery CRITERION | --review] [--json]
```

Defaults with no flags:

- current active phase handoff while work is in progress
- `phase1` when no phase is active yet
- archived review handoff after completion

Important JSON fields:

- `role`
- `gate`
- `handoff_file`
- `handoff_json_file`

See `docs/integrations.md` for the wrapper behavior and provider boundary.

## report

```bash
scafld report [--runtime-only] [--json]
```

Headlines:

- `first_attempt_pass_rate`
- `recovery_convergence_rate`
- `challenge_override_rate`

Use `--runtime-only` to limit the cohort to tasks with runtime session data.

`report` also includes review-signal counts such as completed challenger rounds,
grounded findings, and clean reviews with explicit attack evidence.

The triage section highlights stale active specs whose phases are all complete
but which still need review/completion or cancellation. Specs cancelled with
supersession metadata are reported as `old-task -> replacement-task`.

## list

```bash
scafld list [filter]
```

Useful filters:

- `active`
- `stale-active` for active specs with all phases complete
- `superseded` for archived specs retired in favor of another spec
- `archive`

## Advanced Commands

The operator surface remains available behind `--help --advanced`:

```bash
scafld harden
scafld validate
scafld branch
scafld sync
scafld audit
scafld diff
scafld fail
scafld cancel <task-id> [--reason TEXT] [--superseded-by TASK]
scafld summary
scafld checks
scafld pr-body
```
