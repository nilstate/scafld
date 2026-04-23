---
title: CLI Reference
description: Default agent surface plus advanced compatibility commands
---

# CLI Reference

Default help teaches the agent-facing surface:

```bash
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

Use `scafld --help --advanced` for the full compatibility surface.

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

- approved spec: runs `start`, then immediately runs `exec --resume`
- active spec: runs `exec --resume`

When JSON mode starts approved work, inspect:

- `state.action == "start_exec"`
- `result.initial_handoff`
- `result.exec.next_action`

## review

```bash
scafld review <task-id> [--json]
```

Runs automated passes, appends a review round, and emits the challenger handoff.

Important JSON fields:

- `handoff_file`
- `handoff_json_file`
- `handoff_role`
- `handoff_gate`

## complete

```bash
scafld complete <task-id> [--json]
scafld complete <task-id> --human-reviewed --reason "manual audit"
```

Archives only after the review gate passes, or after the audited human override
path is explicitly confirmed.

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
- `kind`
- `handoff_file`
- `handoff_json_file`

## report

```bash
scafld report [--json]
```

Headlines:

- `first_attempt_pass_rate`
- `recovery_convergence_rate`
- `challenge_override_rate`

## Advanced Commands

The legacy surface remains available for scripts and operators:

```bash
scafld init
scafld new
scafld start
scafld harden
scafld exec
scafld validate
scafld branch
scafld sync
scafld audit
scafld diff
scafld fail
scafld cancel
scafld summary
scafld checks
scafld pr-body
```
