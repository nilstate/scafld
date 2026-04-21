---
title: CLI Reference
description: Complete command documentation
---

# CLI Reference

## Command Guarantees

The CLI surface stays simple, but the implementation now relies on shared
internal layers for the pieces that tend to drift in long-lived tooling:

- workspace discovery and `--scan-root` traversal
- spec lookup and lifecycle moves between draft, approved, active, and archive
- human-facing errors and machine-facing output

That means commands such as `status`, `approve`, `start`, `complete`, and
`update` share one set of root-discovery and spec-transition rules instead of
reimplementing them independently.

## JSON Mode

Automation-relevant commands support `--json` and emit one stable envelope:

```json
{
  "ok": true,
  "command": "status",
  "task_id": "add-auth",
  "warnings": [],
  "state": {},
  "result": {},
  "error": null
}
```

- `ok` -- whether the command succeeded
- `command` -- the command that ran
- `task_id` -- present when the command targets one spec
- `warnings` -- non-fatal warnings that callers may surface
- `state` -- compact current state such as lifecycle status, file, round, or mode
- `result` -- command-specific payload
- `error` -- `null` on success, otherwise `{ code, message, details, next_action, exit_code }`

Human output remains the default. JSON mode exists so wrappers such as runx do
not need to scrape terminal prose.

## scafld init

Bootstrap the `.ai/` workspace structure in the current directory.

```bash
scafld init [--json]
```

Creates directories, templates, config files, prompts, and the JSON schema. Safe to re-run -- updates templates without overwriting existing specs.

Also installs the managed runtime bundle in `.ai/scafld/`, which is what `scafld update` refreshes later.

JSON mode returns created/skipped template actions, directory creation, bundle
sync details, detected config summary, and gitignore updates.

## scafld new

Create a new spec from the default template.

```bash
scafld new <task-id> [-t TITLE] [-s SIZE] [-r RISK] [--json]
```

| Flag | Description |
|------|-------------|
| `-t, --title` | Human-friendly title |
| `-s, --size` | `micro`, `small`, `medium`, or `large` |
| `-r, --risk` | `low`, `medium`, or `high` |

Task IDs must be kebab-case (alphanumeric + hyphens, 1-64 chars). Creates the spec in `.ai/specs/drafts/`.

JSON mode returns the created spec path, draft state, task metadata, and next recommended commands.

## scafld list

List specs, optionally filtered.

```bash
scafld list [FILTER]
```

Filter by status (`draft`, `approved`, `active`, `archive`) or search by task-id substring. Without a filter, lists all specs grouped by status.

## scafld status

Show detailed status of a spec.

```bash
scafld status <task-id> [--json]
```

Displays title, location, current status, phase progress, and last updated timestamp.

JSON mode also exposes the spec's stored `origin` block plus a live `sync`
payload showing current branch, head SHA, and any git drift reasons.

## scafld validate

Check a spec against the JSON schema.

```bash
scafld validate <task-id> [--json]
```

Checks required fields, valid status values, non-empty phases, kebab-case task-id, and flags TODO placeholders in actionable fields. Exit code 0 if valid, 1 if invalid.

## scafld harden

Interrogate a draft spec against grounded questions before approval.

```bash
scafld harden <task-id> [--json]
scafld harden <task-id> --mark-passed [--json]
```

Without flags, prints the `HARDEN MODE` prompt from `.ai/prompts/harden.md`, appends a new round to `harden_rounds`, and sets `harden_status: in_progress`. The agent then interviews you one question at a time, upstream decisions first. If the answer is already in the codebase, it should inspect the code instead of asking. Each recorded question carries a `grounded_in` value using one of `spec_gap:<field>`, `code:<file>:<line>`, or `archive:<task_id>`.

With `--mark-passed`, sets `harden_status: passed` and closes the latest round. Refuses if no round has been started. Before closing the round, scafld verifies that `code:` citations point at a real workspace file and in-range line, and that `archive:` citations point at a real archived spec. Unresolvable citations emit warnings but do not block the command.

Optional. `scafld approve` does not require harden to have run.

When `.ai/scafld/prompts/harden.md` exists, scafld prefers that managed prompt over the legacy `.ai/prompts/harden.md` path.

JSON mode returns `result.action` as `round_opened` or `round_passed`, the current round number, and any citation warnings.

## scafld approve

Move a spec from drafts to approved.

```bash
scafld approve <task-id> [--json]
```

Validates the spec first. Updates the status field, timestamp, and planning_log.

JSON mode returns the lifecycle transition from drafts to approved.

## scafld start

Move a spec from approved to active.

```bash
scafld start <task-id> [--json]
```

Sets status to `in_progress`.

JSON mode returns the lifecycle transition from approved to active.

## scafld branch

Create or bind a working branch for a spec and record the binding in the spec's
`origin` metadata.

```bash
scafld branch <task-id> [--name BRANCH] [--base REF] [--bind-current] [--json]
```

| Flag | Description |
|------|-------------|
| `--name` | Branch name to create or bind. Defaults to the task-id. |
| `--base` | Base ref for branch creation. Defaults to the repo's default branch when scafld can resolve one. |
| `--bind-current` | Record the already-checked-out branch without creating or switching. |

Default behavior is explicit and safe: if the target branch already exists,
scafld checks it out; otherwise it creates it from the resolved base ref.
Switching with engineering changes is refused. Detached HEAD mutation is also
refused unless you bind the current branch explicitly after checking out a real
branch yourself.

JSON mode returns the binding action (`created_branch`, `checked_out_existing`,
or `bound_current`), the stored `origin` metadata, and the immediate post-bind
`sync` state.

## scafld sync

Compare a spec's recorded `origin` binding against the live git workspace.

```bash
scafld sync <task-id> [--json]
```

`sync` is diagnostic. It does not try to repair drift. It reports:

- branch mismatch
- detached HEAD
- engineering changes outside scafld control-plane files
- remote or upstream mismatch when the spec recorded those facts

When computing drift, scafld ignores its own control-plane artifacts under
`.ai/specs/`, `.ai/reviews/`, `.ai/logs/`, plus `.ai/config.local.yaml`.

Exit code 0 means `in_sync`. Exit code 1 means drift or unavailable git state.

JSON mode returns the stored `origin` block, the live `sync` payload with
expected-vs-actual git facts, and a structured `git_drift` or
`git_state_unavailable` error when the workspace is out of sync.

## scafld summary

Render one concise deterministic summary from spec, review, and origin state.

```bash
scafld summary <task-id> [--json]
```

Default output is markdown suitable for issue comments, chat updates, or CI job
summaries. JSON mode returns the same projection model plus the rendered
markdown block so wrappers do not have to reformat task state themselves.

## scafld checks

Render a CI-friendly check payload from the same projection model.

```bash
scafld checks <task-id> [--json]
```

The check status is:

- `failure` when review is failing, recorded acceptance criteria failed, or git
  sync is drifted
- `success` when review passed and the task is otherwise green
- `pending` while work is still in flight without a passing review yet

JSON mode returns:

- the full projection model
- `check.status`, `check.summary`, and `check.details`
- a markdown summary derived from the same model

Exit code 1 means the projected check state is `failure`.

## scafld pr-body

Render a deterministic markdown PR body from spec state.

```bash
scafld pr-body <task-id> [--json]
```

The body includes workflow state, current branch/sync details, review state,
acceptance progress, objectives, and risk notes. JSON mode returns both the
underlying model and the rendered markdown.

## scafld exec

Run acceptance criteria and record results.

```bash
scafld exec <task-id> [-p PHASE] [-r/--resume] [--json]
```

| Flag | Description |
|------|-------------|
| `-p, --phase` | Run criteria for a specific phase only (e.g. `phase1`) |
| `-r, --resume` | Skip criteria that already passed |

Runs each criterion's command, compares against expected, records pass/fail with timestamp and output in the spec file. Default timeout is 600 seconds per criterion, overridable with `timeout_seconds`.

JSON mode returns one result per acceptance criterion plus a summary of passed, failed, manual, and resume-skipped criteria.

## scafld audit

Check for scope drift by comparing declared spec changes against the current git working tree or an explicit base ref.

```bash
scafld audit <task-id> [-b BASE] [--json]
```

| Flag | Description |
|------|-------------|
| `-b, --base` | Git reference for historical comparison. Without it, `audit` inspects the current staged, unstaged, and untracked working tree files. |

Reports three categories: declared and changed (green), changed but not in spec (scope creep, red), in spec but not changed (yellow). scafld-managed execution artifacts under `.ai/specs/`, `.ai/reviews/`, `.ai/logs/`, plus the local override file `.ai/config.local.yaml`, are ignored. Exit code 1 if scope creep detected.

When active specs intentionally share a coordination file, mark that change with
`ownership: shared` in every overlapping spec. Audit then reports the file as
shared instead of conflicting. Mixed or exclusive overlap still fails.

JSON mode returns explicit `declared`, `matched`, `undeclared`, `missing`,
`shared_with_other_active`, `active_overlap`, and `files` fields so callers do
not have to reconstruct audit state. Each `files[]` entry includes the file
path, primary audit status, whether it was declared or changed, overlap state,
and any overlapping active specs.

## scafld diff

Show git history and uncommitted changes for a spec file.

```bash
scafld diff <task-id>
```

## scafld review

Run automated passes and scaffold adversarial review.

```bash
scafld review <task-id> [--json]
```

Runs spec_compliance and scope_drift checks, then creates/appends a Review Artifact v3 in `.ai/reviews/`. The scaffolded metadata records the reviewed `HEAD`, whether the workspace was dirty, and a fingerprint of the reviewed staged, unstaged, and untracked changes.

JSON mode returns the opened review round, review file path, automated pass results, required adversarial sections, and the reviewer handoff prompt.

## scafld complete

Archive a spec as completed, gated on review.

```bash
scafld complete <task-id> [--json] [--human-reviewed --reason TEXT]
```

Requires a passing review whose recorded git state still matches the current repo. With `--human-reviewed --reason`, bypasses the review gate with an audited override (requires interactive terminal and explicit confirmation).

JSON mode returns either a structured `review_gate_blocked` error envelope or a completion payload with archive path, review verdict, pass results, and override metadata.

## scafld fail

Archive a spec as failed.

```bash
scafld fail <task-id> [--json]
```

JSON mode returns the lifecycle transition into the archived failed state.

## scafld cancel

Archive a spec as cancelled.

```bash
scafld cancel <task-id> [--json]
```

JSON mode returns the lifecycle transition into the archived cancelled state.

## scafld report

Aggregate statistics across all specs.

```bash
scafld report [--json]
```

Reports total specs, breakdown by status/size/risk/month, self-eval averages, exec pass rates, and phase statistics. Also prints actionable triage for stale drafts, approved specs waiting to start, active specs with no exec evidence, and active specs whose latest review no longer matches the current git state.

JSON mode returns the same aggregate totals plus machine-readable triage collections.

## scafld update

Refresh the managed scafld bundle in the current workspace, or scan and refresh many workspaces at once.

```bash
scafld update
scafld update --scan-root ~/dev
scafld update --self --scan-root ~/dev
```

| Flag | Description |
|------|-------------|
| `--scan-root PATH` | Recursively find scafld workspaces under `PATH` and refresh each one |
| `--self` | Run `git pull --ff-only` in the current scafld checkout before syncing workspaces |
| `--verbose` | Print each created/updated managed file |

`scafld update` only refreshes `.ai/scafld/` and its manifest. It does not overwrite repo-owned `AGENTS.md`, `CLAUDE.md`, `CONVENTIONS.md`, or project-specific `.ai/config.yaml` edits.

## scafld --version

Print the version number.
