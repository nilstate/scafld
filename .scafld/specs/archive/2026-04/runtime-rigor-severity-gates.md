---
spec_version: '2.0'
task_id: runtime-rigor-severity-gates
created: '2026-04-28T12:00:00Z'
updated: '2026-04-28T03:23:32Z'
status: completed
harden_status: in_progress
size: small
risk_level: low
---

# Severity-gated complete + single-round-by-default review semantics

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

Today `scafld review` returns verdict + findings. `evaluate_review_gate` (review_workflow.py:696) blocks complete when verdict is `fail`. `pass_with_issues` (non-blocking findings only) ships through. That part already works. What 1.7.0 adds is explicit operator control over what "blocking" means and the documented single-round-by-default contract.
Two changes:

  1. `review.gate_severity` config setting (default `blocking`).
     Reads via a small loader in review_workflow.py — config has
     no allowlist mechanism, just deep-merged YAML. When
     `medium`, non-blocking findings tagged `medium` or higher
     become gate-blocking. When `low`, every finding blocks
     (strict iteration). The default keeps current behavior.

  2. `scafld complete` surfaces an `advisory: N finding(s)` line
     when there are non-blocking findings under the gate
     threshold, so the operator/agent knows what was deferred
     without re-reading the review file.

Plus: doc + smoke promote the stopping rule from CLAUDE.md discipline → CLI behavior. A `pass` or `pass_with_issues` verdict means one round is enough. Iteration is bounded by blocking severity, not finding count.
Hard cutover: existing review files keep gating as today under the default. Operators who set `gate_severity: medium` need findings with severity declarations (already required by `FINDING_LINE_RE` in reviewing.py:14).

## Context

CWD: `.`

Packages:
- none

Files impacted:
- `scafld/reviewing.py` (all) - Extract severity from FINDING_LINE_RE group 1; expose structured findings on parsed payload.
- `scafld/review_workflow.py` (all) - Add gate_severity reader; evaluate_review_gate threshold check on non-blocking findings.
- `scafld/runtime_guidance.py` (all) - review_gate_snapshot surfaces threshold + advisory count + severity breakdown to status/complete.
- `scafld/commands/review.py` (all) - cmd_complete prints the `advisory: N finding(s)` line on the success path.
- `tests/test_review_gate_severity.py` (all) - Cover default + medium + low gate thresholds + advisory output.
- `tests/review_gate_smoke.sh` (all, shared) - Smoke for severity-gated complete + advisory path.
- `docs/configuration.md` (all) - Document review.gate_severity and the single-round-by-default contract.
- `CLAUDE.md` (all, shared) - Trim the discipline section; point at the CLI behavior.

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
- [x] `dod1` Parsed findings carry structured severity (critical|high|medium|low) on the review_data payload.
- [x] `dod2` `review.gate_severity` defaults to `blocking`; only Blocking-section findings gate complete.
- [x] `dod3` Setting `review.gate_severity: medium` makes non-blocking medium-or-higher findings block complete with a named gate_reason.
- [x] `dod4` `scafld complete` surfaces `advisory: N finding(s)` when non-blocking findings exist under the threshold.
- [x] `dod5` Operator docs + smoke cover the threshold + advisory paths; CLAUDE.md discipline section points at CLI.

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
  - Command: `python3 -m unittest tests.test_review_gate_severity`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` boundary - Docs cover gate_severity.
  - Command: `grep -q 'gate_severity' docs/configuration.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Structured severity on findings + threshold-aware gate

Goal: Parse severity into the review_data payload. evaluate_review_gate respects review.gate_severity. Default keeps current behavior (only Blocking-section findings gate).

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
  - Command: `python3 -m unittest tests.test_review_gate_severity`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Advisory line on complete output

Goal: cmd_complete prints `advisory: N finding(s)` when non-blocking findings exist under the threshold.

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
  - Command: `python3 -m unittest tests.test_review_gate_severity`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Smoke + docs (single-round-by-default)

Goal: Smoke-prove threshold + advisory paths. Document single-round-by-default semantics. Trim CLAUDE.md discipline section.

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
- [x] `ac3_2` boundary - Severity smokes registered.
  - Command: `sh -c 'grep -q case_complete_advisory_findings_pass tests/review_gate_smoke.sh && grep -q case_complete_blocks_on_medium_when_threshold_set tests/review_gate_smoke.sh'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_3` boundary - Docs cover gate_severity.
  - Command: `grep -qiE 'gate_severity|Review gate severity' docs/configuration.md`
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
Timestamp: '2026-04-28T03:23:00Z'
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

- 2026-04-28T12:00:00Z - user - 1.7.0 spec 2: severity-gated complete + single-round-by-default. Promotes stopping rule from doc to CLI.
- 2026-04-28T13:30:00Z - agent - Reshaped against verified code: dropped config.py (no allowlist) and commands/lifecycle.py (complete is in commands/review.py); added runtime_guidance.py for the snapshot surface.
- 2026-04-28T15:10:00Z - agent - Review 1 verdict=pass_with_issues. Two non-blocking mediums fixed
