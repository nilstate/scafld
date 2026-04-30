---
spec_version: '2.0'
task_id: llm-run-schema-and-config
created: '2026-04-23T08:49:58Z'
updated: '2026-04-23T12:56:06Z'
status: completed
harden_status: not_run
size: large
risk_level: high
---

# Handoff Session Contracts And Config

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

Define the minimal configuration and generated-state contracts for the new value layer. This spec introduces versioned handoff and session contracts, a minimal `llm` config block, the sibling `*.md + *.json` handoff format, and the rule that `.ai/runs/` is generated state ignored by audit and sync while archived with the spec on lifecycle exit. It does not expand the spec schema unless a later implementation proves that necessary.

## Context

CWD: `.`

Packages:
- `.ai/config.yaml`
- `docs/`
- `scafld/`
- `tests/`

Files impacted:
- `scafld/runtime_contracts.py` (all) - Central constants and helpers for handoff/session schema versions and envelope keys.
- `.ai/config.yaml` (all) - Add the minimal llm config surface.
- `docs/configuration.md` (all) - Document only the v1 llm config block.
- `docs/run-artifacts.md` (all) - Document handoff/session contracts and .ai/runs layout.
- `scafld/commands/lifecycle.py` (1-260) - fail and cancel should archive run artifacts alongside the spec.
- `scafld/commands/review.py` (1-360) - complete should archive run artifacts alongside the completed spec.
- `docs/cli-reference.md` (all) - Document run archive paths and runtime artifact guarantees.
- `scafld/runtime_bundle.py` (1-140) - Managed bundle and workspace docs may need to know about runtime contracts and run state.
- `scafld/audit_scope.py` (1-120) - Ignore generated run state during audit and sync checks.
- `docs/installation.md` (all) - Mention .ai/runs as generated state.
- `tests/run_contracts_smoke.sh` (all) - New smoke test for handoff/session envelope expectations.
- `tests/package_smoke.sh` (all) - Ensure bundle assets and docs remain packaged correctly.

Invariants:
- `domain_boundaries`
- `public_api_stable`
- `config_from_env`

Related docs:
- `plans/llm-performance-cutover.md`
- `docs/configuration.md`
- `docs/run-artifacts.md`

## Objectives

- Define one shared handoff contract with schema_version, kind, and sibling `.md/.json` files.
- Define one shared session contract with schema_version, phase_summaries, and criterion state.
- Add the minimal v1 llm config surface: model_profile, context.budget_tokens, recovery.max_attempts.
- Treat .ai/runs as generated state and keep it out of audit/sync drift.
- Archive run artifacts with the spec on complete, fail, or cancel.
- Avoid spec schema growth unless implementation proves it is necessary.

## Scope



## Dependencies

- llm-performance-prose-cutover should land first so the mission and terms are stable.

## Assumptions

- Handoff and session live outside the spec.
- External agent usage and cost fields, if present later, can be optional fields on session rather than a new artifact.

## Touchpoints

- Runtime contracts: One place defines schema versions and stable field names.
- Config: One small llm block controls v1 runtime behavior.
- Generated state: .ai/runs is ignored by audit and sync drift.
- Retention: Run artifacts move to archive with the spec rather than accumulating indefinitely.
- Docs: run-artifacts and configuration docs teach the contract without growing extra concepts.

## Risks

- Config surface can bloat before the first measured win.
- Spec schema churn can invalidate existing specs.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` A single shared handoff contract and a single shared session contract are documented and versioned.
- [ ] `dod2` .ai/config.yaml documents only the minimal v1 llm block.
- [ ] `dod3` .ai/runs is documented as generated state, ignored by audit/sync, and archived with the spec.
- [ ] `dod4` Zero spec schema changes are stated explicitly and no unnecessary spec schema growth is introduced.

Validation:
- [ ] `v1` custom - Validate this draft spec.
  - Command: `python3 cli/scafld validate llm-run-schema-and-config`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test - Run runtime contract smoke coverage.
  - Command: `bash tests/run_contracts_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test - Run package smoke coverage.
  - Command: `bash tests/package_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` test - Run lifecycle smoke coverage for archived run retention.
  - Command: `bash tests/lifecycle_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Runtime contract constants

Goal: Define stable schema versions and contract fields for handoff and session.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Runtime contract smoke test passes.
  - Command: `bash tests/run_contracts_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Minimal llm config

Goal: Add the smallest config surface that supports v1 handoffs and recovery.

Status: pending
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` documentation - Config docs mention only the minimal v1 llm keys.
  - Command: `bash -lc 'for term in "model_profile" "budget_tokens" "max_attempts"; do rg -q "$term" .ai/config.yaml docs/configuration.md || exit 1; done'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Generated-state rules

Goal: Make `.ai/runs` an explicit generated-state area that does not pollute workflow governance.

Status: pending
Dependencies: phase2

Changes:
- none

Acceptance:
- [x] `ac3_1` test - Package and lifecycle smoke pass.
  - Command: `bash tests/package_smoke.sh && bash tests/lifecycle_smoke.sh`
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
Verdict: incomplete
Timestamp: '2026-04-23T12:56:06Z'
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

Estimated effort hours: 8.0
Actual effort hours: none
AI model: unspecified
React cycles: none

Tags:
- llm-performance
- contracts
- config

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

- 2026-04-23T08:49:58Z - agent - Rewrote the schema/config slice around minimal llm config and versioned handoff/session contracts.
- 2026-04-23T12:53:04Z - cli - Spec approved
- 2026-04-23T12:53:05Z - cli - Execution started
- 2026-04-23T12:56:06Z - cli - Spec completed
