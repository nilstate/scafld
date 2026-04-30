---
spec_version: '2.0'
task_id: strict-yaml-spec-gates
created: '2026-04-25T09:06:27Z'
updated: '2026-04-25T09:14:35Z'
status: completed
harden_status: in_progress
size: medium
risk_level: high
---

# Make spec gates enforce real YAML integrity

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

Fix the integrity gap where scafld validates and approves specs using regex field reads while later runtime paths use `yaml.safe_load()`. Right now a spec with syntactically invalid YAML can pass `scafld validate`, pass `scafld approve`, and then blow up `scafld build` with a raw traceback. That is a broken gate and a bad operator experience. The clean model is: spec gates parse real YAML before approving anything, malformed specs fail with structured validation errors, and any remaining runtime path that loads a malformed spec returns a structured scafld error instead of a Python traceback.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`

Files impacted:
- `scafld/spec_store.py` (task-selected) - Central spec loading should convert YAML parser failures into structured ScafldError values.
- `scafld/lifecycle_runtime.py` (task-selected) - Validate and approve currently rely on regex checks instead of real YAML parsing.
- `scafld/execution_runtime.py` (task-selected) - Harden and execution paths still use direct yaml.safe_load calls that can traceback on malformed specs.
- `scafld/commands/execution.py` (task-selected) - Harden JSON error shaping should not crash while reporting malformed-spec failures.
- `tests/invalid_yaml_spec_smoke.sh` (all) - A disposable-repo smoke should prove validate/approve/build/harden all fail cleanly on malformed YAML.

Invariants:
- `validation_gates_must_be_stricter_than_runtime`
- `malformed_specs_must_fail_as_scafld_errors_not_python_tracebacks`
- `packaged_cli_is_the_truth_surface`

Related docs:
- `README.md`
- `docs/lifecycle.md`
- `docs/cli-reference.md`

## Objectives

- Make `scafld validate` reject malformed YAML before schema-style checks.
- Make `scafld approve` refuse malformed YAML because validation already rejects it.
- Convert malformed-spec runtime loads into structured scafld errors instead of raw tracebacks.
- Lock the behavior with a disposable-repo smoke that exercises the packaged CLI.

## Scope



## Dependencies

- None.

## Assumptions

- INVALID_SPEC_DOCUMENT is the right error family for malformed YAML parsing failures.
- Regex field readers are still fine as lightweight helpers once the document has already been parsed successfully.

## Touchpoints

- gate integrity: Validate and approve must stop bad specs before execution starts.
- runtime error shaping: Build and harden must surface malformed specs as structured command failures.
- operator trust: A malformed spec should never appear “valid” until a later traceback proves otherwise.

## Risks

- Tightening validation could reject malformed specs that old workflows accidentally depended on slipping through.
- Replacing direct yaml.safe_load calls could change non-error behavior in harden/execution flows.
- Runtime callers outside validate/approve could still traceback if they bypass the structured loader.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` Malformed YAML fails validate and approve with structured validation errors.
- [ ] `dod2` Malformed YAML in build or harden fails as a structured scafld error, not a traceback.
- [ ] `dod3` A disposable-repo smoke proves the packaged CLI behavior end to end.

Validation:
- [ ] `v1` compile - Compile the Python sources after the YAML integrity hardening.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Invalid-YAML smoke proves validate, approve, build, and harden all fail cleanly.
  - Command: `bash tests/invalid_yaml_spec_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Make validate and approve parse real YAML

Goal: Reject malformed specs at the gate instead of after approval.

Status: completed
Dependencies: none

Changes:
- `scafld/spec_store.py` (all) - Introduce structured malformed-YAML handling for spec document loads.
- `scafld/lifecycle_runtime.py` (all) - Make validate/approve parse real YAML and surface malformed-document errors as validation failures.
- `tests/invalid_yaml_spec_smoke.sh` (all) - Create a packaged-CLI smoke that proves validate and approve reject malformed YAML in a disposable repo.

Acceptance:
- [x] `ac1_1` integration - Invalid-YAML smoke proves validate and approve fail cleanly.
  - Command: `bash tests/invalid_yaml_spec_smoke.sh --phase gate`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Remove malformed-spec tracebacks from runtime surfaces

Goal: Make build and harden return structured scafld errors when a malformed spec still reaches runtime.

Status: completed
Dependencies: phase1

Changes:
- `scafld/execution_runtime.py` (all) - Route harden/execution spec loads through the structured loader instead of raw yaml.safe_load calls.
- `scafld/commands/execution.py` (all) - Keep harden JSON error shaping safe when the spec itself is malformed.
- `tests/invalid_yaml_spec_smoke.sh` (all) - Extend the smoke to prove build and harden return structured malformed-spec failures instead of tracebacks.

Acceptance:
- [x] `ac2_1` compile - Python sources compile after the structured YAML load changes.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` integration - Invalid-YAML smoke proves build and harden fail cleanly.
  - Command: `bash tests/invalid_yaml_spec_smoke.sh --phase runtime`
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
Timestamp: '2026-04-25T09:14:45Z'
Review rounds: 2
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

- 2026-04-25T09:06:27Z - user - Spec created via scafld plan
- 2026-04-25T09:08:40Z - assistant - Replaced the placeholder draft with a spec grounded in a live repro where validate and approve accepted malformed YAML that later crashed build.
- 2026-04-25T09:08:43Z - cli - Spec approved
- 2026-04-25T09:08:48Z - cli - Execution started
- 2026-04-25T09:14:35Z - cli - Spec completed
