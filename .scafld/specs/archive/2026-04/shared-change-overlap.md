---
spec_version: '2.0'
task_id: shared-change-overlap
created: '2026-04-21T10:39:01Z'
updated: '2026-04-21T10:54:57Z'
status: completed
harden_status: not_run
size: small
risk_level: medium
---

# Classify shared coordination changes

## Current State

Status: completed
Current phase: none
Next: none
Reason: none
Blockers: none
Allowed follow-up command: none
Latest runner update: none
Review gate: not_started

## Summary

Make active-spec overlap less brittle by letting specs declare intentionally shared coordination surfaces without suppressing real ownership conflicts. Today `scafld audit` fails any time two active specs declare the same file, even when the file is an expected shared plan or workflow surface. This round adds an explicit per-change ownership contract, teaches audit to distinguish shared coordination files from exclusive conflicts, and exposes that classification in the native JSON payload so wrappers do not need to infer it.

## Context

CWD: `.`

Packages:
- `cli`
- `tests`
- `docs`
- `schemas`

Files impacted:
- none

Invariants:
- `domain_boundaries`
- `machine_callers_never_need_terminal_scraping`
- `json_mode_uses_stable_envelopes`

Related docs:
- none

## Objectives

- Allow overlapping active specs only when every overlapping declaration is explicitly shared.
- Keep exclusive or mixed-ownership overlaps as hard audit failures.
- Return file-level structured audit data so callers can consume overlap state directly.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- cli/scafld: Classify declared changes by ownership and report shared versus conflicting overlaps.
- .ai/schemas/spec.json: Add the optional per-change ownership field to the schema.
- tests/json_contract_smoke.sh: Prove file-level audit payloads and shared overlap behavior in the packaged CLI.
- docs: Document ownership semantics and audit output categories.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [x] `dod1` Specs can mark a declared change as shared without weakening exclusive ownership by default.
- [x] `dod2` Audit distinguishes shared active overlap from conflicting active overlap.
- [x] `dod3` audit --json exposes file-level statuses and shared overlap details.

Validation:
- [ ] `v1` integration - JSON contract smoke covers shared and conflicting overlap states.
  - Command: `./tests/json_contract_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Teach Audit Shared Ownership

Goal: Add explicit shared ownership semantics for declared changes and surface them in audit output.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` integration - JSON contract smoke passes with shared overlap coverage.
  - Command: `./tests/json_contract_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Rollback

Strategy: per_phase

Commands:
- none

## Review

Status: not_started
Verdict: pass
Timestamp: '2026-04-21T10:54:26Z'
Review rounds: 1
Reviewer mode: none
Reviewer session: none
Round status: none
Override applied: none
Override reason: none
Override confirmed at: none
Reviewed head: none
Reviewed dirty: none
Reviewed diff: none
Blocking count: 0
Non-blocking count: none

Findings:
- none

Passes:
- `id`: spec_compliance
- `id`: scope_drift
- `id`: regression_hunt
- `id`: convention_check
- `id`: dark_patterns

## Self Eval

Status: not_started
Completeness: none
Architecture fidelity: none
Spec alignment: none
Validation depth: none
Total: none
Second pass performed: none

Notes:
none

Improvements:
- none

## Deviations

- none

## Metadata

Estimated effort hours: none
Actual effort hours: none
AI model: none
React cycles: none

Tags:
- none

## Origin

Source:
- none

Repo:
- none

Git:
- none

Sync:
- none

Supersession:
- none

## Harden Rounds

- none

## Planning Log

- 2026-04-21T10:39:01Z - user - Spec created via scafld new
- 2026-04-21T10:45:00Z - assistant - Expanded the draft into an audit ergonomics change with shared ownership semantics and JSON payload backfill.
- 2026-04-21T10:46:00Z - cli - Spec approved
- 2026-04-21T10:47:30Z - cli - Execution started
- 2026-04-21T10:54:57Z - cli - Spec completed
