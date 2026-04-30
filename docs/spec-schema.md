---
title: Spec Schema
description: Living spec reference
---

# Spec Schema

Schema file: `.scafld/core/schemas/spec.json`.

Task specs live only at `.scafld/specs/**/*.md`. A spec is Markdown with YAML
front matter, prose sections, labeled runner state, phase headings, and
checklists:

```markdown
---
spec_version: "2.0"
task_id: fix-typo
created: "2026-04-30T00:00:00Z"
updated: "2026-04-30T00:00:00Z"
status: draft
harden_status: not_run
size: small
risk_level: low
---

# Fix typo

## Summary

Human-owned prose.

## Phase 1: Fix typo

Goal:

Human-owned phase objective.

Acceptance:
- [ ] `ac1_1` test
  - Command: `grep -q 'the' README.md`
  - Expected kind: `exit_code_zero`
```

## Shape

Front matter owns document identity and lifecycle status. The H1 owns the task
title. `## Phase N: Name` owns both the stable phase id (`phaseN`) and the
human-visible phase name.

The grammar is heading-based. scafld locates exact top-level `##` sections with
a fence-aware scanner, so heading-like text inside fenced code examples is prose
and cannot become a spec section. Runner-authored sections are replaced as whole
section bodies under their `##` heading. Human-authored sections are preserved
byte-for-byte unless their normalized model value changes. If the phase headings
on disk do not match the phase ids in the model being written, the writer fails
instead of rebuilding the whole document.

Each phase uses fixed labels:

- `Goal`
- `Status`
- `Dependencies`
- `Changes`
- `Acceptance`

Top-level runner state lives in readable sections such as `## Current State`,
`## Acceptance`, `## Rollback`, `## Review`, `## Self Eval`, `## Metadata`,
`## Origin`, `## Harden Rounds`, and `## Planning Log`.

Fenced code blocks are only prose examples. They are not a task-spec data
language.

## Acceptance

Executable command criteria require `expected_kind`.

Supported values:

- `exit_code_zero`
- `exit_code_nonzero`
- `no_matches`

Optional criterion fields include `expected_exit_code`, `cwd`,
`timeout_seconds`, and `evidence_required`.

## Reconcile Contract

The session ledger records raw execution events. The spec is the living,
human-readable projection plus task prose. Runtime writes append session first,
then update the relevant spec sections. If a spec write fails after a session
write, `scafld reconcile` can rebuild runner-derived sections from session data
while preserving task prose.

Session phase state is stored as `phase_blocks[phase_id]` with `status`,
optional `reason`, and `updated_at`. Reconciliation uses that map, not Markdown
checkmarks, to project phase status into the spec.
