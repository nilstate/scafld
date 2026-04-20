---
title: CLI Reference
description: Complete command documentation
---

# CLI Reference

## scafld init

Bootstrap the `.ai/` workspace structure in the current directory.

```bash
scafld init
```

Creates directories, templates, config files, prompts, and the JSON schema. Safe to re-run -- updates templates without overwriting existing specs.

## scafld new

Create a new spec from the default template.

```bash
scafld new <task-id> [-t TITLE] [-s SIZE] [-r RISK]
```

| Flag | Description |
|------|-------------|
| `-t, --title` | Human-friendly title |
| `-s, --size` | `micro`, `small`, `medium`, or `large` |
| `-r, --risk` | `low`, `medium`, or `high` |

Task IDs must be kebab-case (alphanumeric + hyphens, 1-64 chars). Creates the spec in `.ai/specs/drafts/`.

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

## scafld validate

Check a spec against the JSON schema.

```bash
scafld validate <task-id> [--json]
```

Checks required fields, valid status values, non-empty phases, kebab-case task-id, and flags TODO placeholders in actionable fields. Exit code 0 if valid, 1 if invalid.

## scafld harden

Interrogate a draft spec against grounded questions before approval.

```bash
scafld harden <task-id>
scafld harden <task-id> --mark-passed
```

Without flags, prints the `HARDEN MODE` prompt from `.ai/prompts/harden.md`, appends a new round to `harden_rounds`, and sets `harden_status: in_progress`. The agent then interviews you, one grounded question at a time, until you stop the loop. Every question must cite its source using one of `spec_gap:<field>`, `code:<file>:<line>`, or `archive:<task_id>`.

With `--mark-passed`, sets `harden_status: passed` and closes the latest round. Refuses if no round has been started.

Optional. `scafld approve` does not require harden to have run.

## scafld approve

Move a spec from drafts to approved.

```bash
scafld approve <task-id>
```

Validates the spec first. Updates the status field, timestamp, and planning_log.

## scafld start

Move a spec from approved to active.

```bash
scafld start <task-id>
```

Sets status to `in_progress`.

## scafld exec

Run acceptance criteria and record results.

```bash
scafld exec <task-id> [-p PHASE] [-r/--resume]
```

| Flag | Description |
|------|-------------|
| `-p, --phase` | Run criteria for a specific phase only (e.g. `phase1`) |
| `-r, --resume` | Skip criteria that already passed |

Runs each criterion's command, compares against expected, records pass/fail with timestamp and output in the spec file. Default timeout is 600 seconds per criterion, overridable with `timeout_seconds`.

## scafld audit

Check for scope drift by comparing declared spec changes against actual git diff.

```bash
scafld audit <task-id> [-b BASE]
```

| Flag | Description |
|------|-------------|
| `-b, --base` | Git reference to diff against (default: `HEAD~1`, falls back to `main`/`master`) |

Reports three categories: declared and changed (green), changed but not in spec (scope creep, red), in spec but not changed (yellow). Exit code 1 if scope creep detected.

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

Runs spec_compliance and scope_drift checks, then creates/appends a Review Artifact v3 in `.ai/reviews/`.

## scafld complete

Archive a spec as completed, gated on review.

```bash
scafld complete <task-id> [--json] [--human-reviewed --reason TEXT]
```

Requires a passing review. With `--human-reviewed --reason`, bypasses the review gate with an audited override (requires interactive terminal and explicit confirmation).

## scafld fail

Archive a spec as failed.

```bash
scafld fail <task-id>
```

## scafld cancel

Archive a spec as cancelled.

```bash
scafld cancel <task-id>
```

## scafld report

Aggregate statistics across all specs.

```bash
scafld report
```

Reports total specs, breakdown by status/size/risk/month, self-eval averages, exec pass rates, and phase statistics.

## scafld --version

Print the version number.
