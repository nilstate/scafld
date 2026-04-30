---
spec_version: '2.0'
task_id: build-to-block-dogfood
created: '2026-04-23T14:52:22Z'
updated: '2026-04-23T15:15:23Z'
status: completed
harden_status: passed
size: medium
risk_level: medium
---

# Dogfood mixed repo detection through the new surface

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

Dogfood scafld on scafld itself by fixing the repo-context detector used by `plan` and `new`. In the current repo, the detector sees `package.json` first, classifies the workspace as Node-only, and emits useless placeholder commands in the draft even though the Python toolchain has real compile and test commands. The cut should make mixed Python+Node repos first-class, prefer concrete commands over placeholders, and prove the new behavior through the agent-facing flow and smoke coverage.

## Context

CWD: `.`

Packages:
- `scafld/config.py`
- `scafld/spec_templates.py`
- `tests/update_smoke.sh`
- `tests/agent_surface_smoke.sh`
- `README.md`
- `docs/configuration.md`

Files impacted:
- none

Invariants:
- `domain_boundaries`
- `config_from_env`

Related docs:
- none

## Objectives

- Detect mixed Python+Node repos instead of short-circuiting at the first stack marker.
- Prefer concrete compile/test/lint/typecheck commands from the richer stack when the other side only has placeholders.
- Keep node-only, python-only, and fallback detection behavior stable.
- Prove the fix by dogfooding `plan` on a mixed fixture and by keeping the compatibility/update smokes green.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- repo detection: Stack detection and command selection in scafld/config.py.
- spec scaffolding: Draft generation and surfaced repo context in scafld/spec_templates.py.
- dogfood tests: Smoke coverage for mixed repo detection and agent-surface planning output.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` Mixed repos surface a mixed summary and concrete commands instead of fallback placeholders when one stack provides them.
- [ ] `dod2` Node-only, Python-only, and fallback fixtures still produce stable suggested commands.
- [ ] `dod3` The documented operator surface matches the new mixed-detection behavior.

Validation:
- [ ] `v1` compile - Compile the scafld package after the detection changes.
  - Command: `python3 -m compileall scafld`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test - Run the mixed repo detection smoke.
  - Command: `bash tests/mixed_repo_detection_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test - Run the broader update smoke to protect existing detection behavior.
  - Command: `bash tests/update_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Make mixed repo detection first-class

Goal: Update repo detection so a workspace can be recognized as mixed Python+Node and command selection prefers concrete values over fallback placeholders.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Mixed repo fixtures resolve to concrete commands and a mixed summary.
  - Command: `bash tests/mixed_repo_detection_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Protect and document the dogfood path

Goal: Extend the existing smoke and operator documentation so the new detection model is exercised through the taught surface and stays understandable.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac2_1` compile - scafld still compiles after the documentation and smoke updates.
  - Command: `python3 -m compileall scafld`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` test - The broader detection/update regressions stay green.
  - Command: `bash tests/update_smoke.sh && bash tests/agent_surface_smoke.sh`
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
Timestamp: '2026-04-23T15:14:05Z'
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

- 2026-04-23T14:52:22Z - user - Spec created via scafld plan.
- 2026-04-24T05:10:00Z - assistant - Re-scoped the dogfood task around the real mixed Python+Node detection failure observed in scafld plan.
- 2026-04-23T14:55:13Z - cli - Spec approved
- 2026-04-23T14:55:19Z - cli - Execution started
- 2026-04-23T15:15:23Z - cli - Spec completed
