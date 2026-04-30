---
spec_version: '2.0'
task_id: runtime-rigor-schema-unification
created: '2026-04-27T15:43:46Z'
updated: '2026-04-27T15:43:46Z'
status: draft
harden_status: in_progress
size: small
risk_level: low
---

# Adopt jsonschema as soft dependency for spec and review packet

## Current State

Status: draft
Current phase: none
Next: none
Reason: none
Blockers: none
Allowed follow-up command: none
Latest runner update: none
Review gate: not_started

## Summary

`.scafld/core/schemas/spec.json` exists as Draft-07 JSON Schema and [lifecycle_runtime.py:69-134:validate_spec](../scafld/lifecycle_runtime.py) is a hand-written Python validator that re-implements a subset of the same rules. Drift is inevitable — confirmed by review findings during this cutover where the schema and runtime checker disagreed on the new acceptance fields.
Same pattern with the review packet: [.scafld/core/schemas/review_packet.json](../.scafld/core/schemas/review_packet.json) declares structure but [review_packet.py:normalize_review_packet](../scafld/review_packet.py) re-implements every rule in Python. The structured-output spec surfaced repeated drift across prompt + schema + normalizer.
This spec adopts `jsonschema` as a soft dependency. When importable (which it is in scafld's own venv), `validate_spec` and `normalize_review_packet` delegate structural checks to `jsonschema.validate(...)` against the canonical schema files. The hand-written Python checks stay as fallback for constrained installs that don't have `jsonschema`. Drift becomes one less degree of freedom: schema files become the single source of structural truth.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`

Files impacted:
- `scafld/lifecycle_runtime.py` (task-selected) - validate_spec delegates to jsonschema when importable; falls back to hand-written checks otherwise.
- `scafld/review_packet.py` (task-selected) - normalize_review_packet runs jsonschema.validate before its hand-written checks (which provide richer error messages).
- `scafld/runtime_bundle.py` (task-selected) - Single helper `try_import_jsonschema()` so callers don't replicate the soft-import dance.
- `setup.py` (task-selected) - Declare `jsonschema>=4` as an extras_require entry (`pip install scafld[strict]`); leave install_requires unchanged.
- `tests/test_lifecycle_runtime.py` (task-selected) - Coverage for delegation when jsonschema is importable AND when it's not (monkey-patched).
- `tests/test_review_packet.py` (task-selected) - Same coverage pattern for the review packet path.
- `docs/configuration.md` (task-selected) - Document the soft dependency and how to install it.

Invariants:
- `jsonschema_must_be_optional_at_install_time`
- `behavior_when_jsonschema_is_absent_must_match_1_6_x`
- `schema_validation_errors_must_name_the_offending_path`
- `hand_written_checks_must_remain_as_fallback_not_be_retired`
- `single_source_of_structural_truth_lives_in_schema_files`

Related docs:
- `plans/1.7.0-cutover.md`
- `.scafld/core/schemas/spec.json`
- `.scafld/core/schemas/review_packet.json`

## Objectives

- Eliminate spec/schema and packet/schema drift by making the schema files authoritative for structural rules.
- Keep scafld installable on constrained Python environments without `jsonschema`.
- Surface validation errors that name the offending JSON pointer / field path.

## Scope



## Dependencies

- None.

## Assumptions

- `jsonschema>=4` is what scafld's venv already pins; older versions support the same Draft-07 contract.
- Operators on constrained installs are a small minority; their UX stays exactly as 1.6.x.
- JSON Schema's error messages are good enough for the structural class of issues. Cross-field rules and citations stay in Python.

## Touchpoints

- spec validation: validate_spec runs schema check first (when available), then hand-written rules.
- review packet validation: normalize_review_packet runs schema check first (when available), then runtime contracts.
- soft dependency surface: Single import helper, single extras_require entry.

## Risks

- `jsonschema` raises differently than hand-written checks; operators see two different error styles.
- Soft import stays loaded across CLI invocations and inflates startup time.
- An older `jsonschema` (e.g., 3.x) is installed and Draft-07 support diverges.

## Acceptance

Profile: standard

Definition of done:
- [ ] `dod1` `try_import_jsonschema()` returns the module when available, None otherwise; cached.
- [ ] `dod2` `validate_spec` runs `Draft7Validator` checks against `.scafld/core/schemas/spec.json` when available; falls back to hand-written checks otherwise. Both modes accept a known-good spec; both reject a known-bad spec.
- [ ] `dod3` `normalize_review_packet` runs `Draft7Validator` checks against `.scafld/core/schemas/review_packet.json` when available; structural errors surface with the JSON pointer named.
- [ ] `dod4` Hand-written runtime contracts (cross-field invariants, citation rules) keep running on the post-schema-validated packet; they're not retired.
- [ ] `dod5` `setup.py` declares `jsonschema>=4` in `extras_require["strict"]`; `install_requires` is unchanged.
- [ ] `dod6` Operator docs link the optional dependency and what it buys.

Validation:
- [ ] `v1` compile - Python sources compile.
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test - Lifecycle runtime tests pass (delegation + fallback).
  - Command: `python3 -m unittest tests.test_lifecycle_runtime`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test - Review packet tests pass (delegation + fallback).
  - Command: `python3 -m unittest tests.test_review_packet`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` test - Existing acceptance tests still pass.
  - Command: `python3 -m unittest tests.test_acceptance`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v5` boundary - extras_require entry present in setup.py.
  - Command: `grep -qE 'jsonschema.*>=.*4' setup.py`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Soft import helper

Goal: Single helper that returns the jsonschema module when importable, None otherwise. Cached.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [ ] `ac1_1` compile - Sources compile.
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_2` test - Soft-import helper tests pass.
  - Command: `python3 -m unittest tests.test_lifecycle_runtime`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Spec validation delegation

Goal: validate_spec delegates structural checks to Draft7Validator when available; hand-written checks run after.

Status: pending
Dependencies: phase1

Changes:
- none

Acceptance:
- [ ] `ac2_1` test - Spec validation delegation tests pass.
  - Command: `python3 -m unittest tests.test_lifecycle_runtime`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Review packet validation delegation

Goal: normalize_review_packet runs Draft7Validator before hand-written checks; cross-field rules stay.

Status: pending
Dependencies: phase2

Changes:
- none

Acceptance:
- [ ] `ac3_1` compile - Sources compile after delegation added.
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac3_2` test - Review packet delegation tests pass.
  - Command: `python3 -m unittest tests.test_review_packet`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac3_3` boundary - extras_require entry declared.
  - Command: `grep -qE 'jsonschema.*>=.*4' setup.py`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
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
Verdict: none
Timestamp: none
Review rounds: none
Reviewer mode: none
Reviewer session: none
Round status: none
Override applied: none
Override reason: none
Override confirmed at: none
Reviewed head: none
Reviewed dirty: none
Reviewed diff: none
Blocking count: none
Non-blocking count: none

Findings:
- none

Passes:
- none

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

- 2026-04-27T15:43:46Z - user - Spec created via scafld plan as cutover #3 (per plans/1.7.0-cutover.md).
- 2026-04-27T15:43:46Z - agent - Replaced placeholder with the schema-unification plan: jsonschema as soft dep, schema-first validation, hand-written checks remain as the cross-field/citation layer.
