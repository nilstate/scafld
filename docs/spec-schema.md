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
- `browser_evidence`

The current executable fields are `Command` and `Expected kind`. Criteria with
type `browser` use `browser_evidence`; the command owns the browser runner and
auth flow, then writes structured browser evidence to stdout. Additional testing
detail belongs in the criterion title or surrounding prose until it earns a
runtime field.

## Hardening

`harden_status` is separate from lifecycle `status`. Values are `not_run`,
`in_progress`, `passed`, and `failed`.

`scafld harden <task-id>` appends a round under `## Harden Rounds`:

````markdown
## Harden Rounds

### round-1

Status: in_progress
Started: 2026-05-04T00:00:00Z
Ended: none
Verdict: needs_revision
Provider: codex
Model: gpt-5.5
Output format: codex.output_file
Summary: The draft needs one ownership decision before approval.

Checks:
- Path audit
  - Grounded in: code:src/auth/session.ts:84
  - Result: passed
  - Evidence: Existing session owner and target path verified.
- Command audit
  - Grounded in: spec_gap:Acceptance
  - Result: not_applicable
  - Evidence: Docs-only change has no runnable command beyond final validation.
- Scope/migration audit
  - Grounded in: spec_gap:Risks
  - Result: passed
  - Evidence: No migration or compatibility fallback is introduced.
- Acceptance timing audit
  - Grounded in: spec_gap:Phases
  - Result: passed
  - Evidence: Criteria run after the phase creates the target files.
- Rollback/repair audit
  - Grounded in: spec_gap:Rollback
  - Result: not_applicable
  - Evidence: No runtime rollback is required for the documented change.
- Design challenge
  - Grounded in: spec_gap:Summary
  - Result: passed
  - Evidence: The plan fixes the root cause without adding aliases or fallback behavior.

Questions:
- Which module owns session cleanup?
  - Grounded in: code:src/auth/session.ts:84
  - Recommended answer: Use the existing cleanupSession owner.
  - If unanswered: Default to the existing cleanup path.
  - Answered with: Use cleanupSession.

Design objections:
- `objection-1` high - The draft may add a second session cleanup path.
  - Grounded in: code:src/auth/session.ts:84
  - Evidence: cleanupSession already owns this lifecycle.
  - Recommendation: Reuse the existing owner or explicitly justify the split.

Recommended edits:
- Scope
  - Grounded in: spec_gap:Scope
  - Recommendation: Declare cleanupSession as the owner before approval.
````

Each check and question should carry one `Grounded in` value matching
`spec_gap:<field>`, `code:<file>:<line>`, or `archive:<task_id>`. Checks use
`Result: passed`, `Result: failed`, or `Result: not_applicable`; only `passed`
and `not_applicable` can close a round. Questions are optional, but any
recorded question must have a recommended answer and final answer before
`scafld harden <task-id> --mark-passed` closes the round.

Provider-backed hardening fills `Verdict`, `Provider`, `Model`,
`Output format`, `Summary`, `Design objections`, and `Recommended edits`
from a strict `HardenDossier`. Manual hardening may omit those provenance
fields, but should still record the required checks.

## Reconcile Contract

The session ledger records raw execution events. The spec is the living,
human-readable projection plus task prose. Runtime writes append session first,
then update the relevant spec sections. If a spec write fails after a session
write, the reconcile package can rebuild runner-derived sections from session
data while preserving task prose.

Session phase state is stored as `phase_blocks[phase_id]` with `status`,
optional `reason`, and `updated_at`. Reconciliation uses that map, not Markdown
checkmarks, to project phase status into the spec.
