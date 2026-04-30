---
spec_version: '2.0'
task_id: runtime-rigor-acceptance
created: '2026-04-27T15:43:36Z'
updated: '2026-04-28T03:07:29Z'
status: completed
harden_status: in_progress
size: small
risk_level: medium
---

# Acceptance kinds and evidence requirement (strict-only, hard cutover)

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

`scafld build` advances phases when acceptance criteria pass via lenient substring matching in `acceptance.check_expected` — observed repeatedly during 1.7.0 cutover dogfooding where "compile + unittest both pass" marked phases complete despite missing implementation work. 1.7.0 replaces this with a structured contract.
Two changes ship together:

  1. `expected_kind` enum on each criterion: `exit_code_zero`,
     `exit_code_nonzero`, `no_matches`. Strict-only — undeclared
     `expected_kind` fails the criterion loudly without running
     the command. Legacy `expected:` strings (`"exit code 0"`,
     `"exit code N"`, `"no matches"`) auto-resolve to the new
     kinds at parse time so existing specs that use the documented
     strings continue to work; anything else fails with a
     one-line guidance error.

  2. `evidence_required` boolean per criterion. When true,
     criterion-pass requires non-empty stdout. Stops the "compile
     + unittest pass with no real work" pattern.

Hard cutover: no `lenient` mode opt-out. No `evidence_path` escape hatch (stdout-only). No `regex` / `json_path` matchers (deferred to a 1.7.x patch if pain shows up).

## Context

CWD: `.`

Packages:
- none

Files impacted:
- `scafld/acceptance.py` (all) - Core evaluator. Replaces check_expected with kind-routed matchers + strict gate + evidence-required check.
- `scafld/spec_parsing.py` (all) - parse_acceptance_criteria allowlist gains expected_kind, expected_exit_code, evidence_required.
- `.ai/schemas/spec.json` (all) - Adds expected_kind enum, expected_exit_code, evidence_required to criterion schema.
- `tests/test_acceptance.py` (all) - Unit coverage for each kind, the strict reject-before-run path, and the evidence_required gate.
- `tests/review_gate_smoke.sh` (all) - Strict-rejection smoke (legacy substring expected fails loudly).
- `docs/configuration.md` (all) - Document the new criterion fields and the hard cutover.
- `CLAUDE.md` (all) - Diminishing-returns stopping rule paired with the strict acceptance gate.

Invariants:
- none

Related docs:
- none

## Objectives

- None.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- None.

## Risks

- None.

## Acceptance

Profile: standard

Definition of done:
- [x] `dod1` Each `expected_kind` (`exit_code_zero`, `exit_code_nonzero`, `no_matches`) has a matcher in acceptance.py with pass + fail unit tests.
- [x] `dod2` Legacy `expected: "exit code 0"` / `"exit code N"` / `"no matches"` auto-resolve at parse time without spec edits; tests pin each form.
- [x] `dod3` Criteria with neither `expected_kind` nor a mappable legacy `expected:` string fail loudly without running the command (strict reject-before-run).
- [x] `dod4` `evidence_required: true` rejects criteria whose command exits 0 with empty stdout.
- [x] `dod5` Operator docs cover the new fields; the hard cutover (no lenient mode) is documented in plans/1.7.0-cutover.md.

Validation:
- [ ] `v1` compile
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test
  - Command: `python3 -m unittest tests.test_acceptance`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` boundary - Schema declares the three kinds (no regex/json_path).
  - Command: `sh -c 'grep -q exit_code_zero .ai/schemas/spec.json && ! grep -q expected_pattern .ai/schemas/spec.json && ! grep -q expected_json_path .ai/schemas/spec.json'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Acceptance kinds and strict-only evaluator

Goal: Add three structured kinds. Reject undeclared kinds before running the command. Preserve legacy `expected:` string mapping for the three documented forms only.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` compile
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` test
  - Command: `python3 -m unittest tests.test_acceptance`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_3` boundary - Schema declares three kinds only.
  - Command: `sh -c 'grep -q exit_code_zero .ai/schemas/spec.json'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Evidence required gate

Goal: Reject criteria whose command exits 0 with empty stdout when `evidence_required: true`. Stops the 'unittest passes with no real work' pattern.

Status: completed
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` compile
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` test
  - Command: `python3 -m unittest tests.test_acceptance`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Smoke and operator docs

Goal: Smoke-prove the strict-rejection path end-to-end and document the hard cutover.

Status: completed
Dependencies: phase2

Changes:
- none

Acceptance:
- [x] `ac3_1` boundary
  - Command: `bash -n tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` boundary - Strict-rejection smoke is registered.
  - Command: `grep -q 'case_acceptance_strict_rejects_undeclared_kind' tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_3` boundary - Docs cover the new kinds.
  - Command: `grep -qiE 'expected_kind|exit_code_zero' docs/configuration.md`
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
Verdict: pass_with_issues
Timestamp: '2026-04-28T03:05:07Z'
Review rounds: 7
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

- 2026-04-27T15:43:36Z - user - Spec created via scafld plan as the first 1.7.0 cutover spec.
- 2026-04-28T00:00:00Z - agent - Trimmed to hard-cutover shape: 3 kinds (no regex/json_path), evidence_required only (no evidence_path), strict-only (no lenient mode opt-out). Aligns with 1.7.0 'no backwards compat, hard cutovers always' direction.
- 2026-04-28T14:30:00Z - agent - Review 7 verdict=pass_with_issues. Shipping per stopping rule
- 2026-04-28T13:50:00Z - agent - Review 5 verdict=fail. Resolved all 3 blocking findings:
