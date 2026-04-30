---
spec_version: '2.0'
task_id: external-review-observability-truth
created: '2026-04-26T11:15:19Z'
updated: '2026-04-26T12:03:37Z'
status: completed
harden_status: passed
size: small
risk_level: medium
---

# External review observability truth

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

The external review runner now has a stricter prose contract, but its observability still overstates what scafld actually knows: failed provider calls can vanish without raw artifacts, challenger invocations are recorded only after successful validation, `fallback_policy: warn` does not warn at provider resolution time, human reports omit useful denominators, and review-signal labels call parser-compliant clean notes "evidence" even though the parser can only verify format. Tighten those truth surfaces without changing review verdict semantics.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`
- `docs`

Files impacted:
- `scafld/review_runner.py` (task-selected) - External runner invocation, diagnostics, warning, and provenance behavior live here.
- `scafld/commands/review.py` (task-selected) - The review command should record successful provider telemetry only after the completed review artifact is accepted.
- `scafld/review_workflow.py` (task-selected) - Candidate review artifact validation failures need diagnostics before the invalid artifact is discarded.
- `scafld/session_store.py` (task-selected) - Provider invocation entries need status/timing fields and report summaries need to count them.
- `scafld/adapter_runtime.py` (task-selected) - Optional provider adapters should record provider invocation status consistently with external review runs.
- `scafld/commands/reporting.py` (task-selected) - Human and JSON reports expose isolation downgrade and clean-review signal wording.
- `scafld/review_signal.py` (task-selected) - The "clean review" metric should describe format compliance, not proven evidence.
- `scripts/real_standard.py` (task-selected) - Cohort markdown is another human-facing review-signal report and must use the same honest clean-review wording.
- `tests/review_gate_smoke.sh` (task-selected) - External runner failure and fallback behavior should be covered by smoke cases.
- `tests/report_metrics_smoke.sh` (task-selected) - Provider telemetry report output should prove denominator and renamed clean-review fields.
- `tests/real_standard_cohort_smoke.sh` (task-selected) - Cohort smoke should protect the renamed human-facing clean-review label.
- `docs/review.md` (task-selected) - Operator-facing docs should state what diagnostics and review-signal metrics mean.

Invariants:
- `review_artifact_stays_scafld_owned`
- `provider_telemetry_must_not_overclaim`
- `failed_external_runs_leave_debuggable_artifacts`
- `packaged_cli_is_the_truth_surface`

Related docs:
- none

## Objectives

- Persist raw external-runner output and error context under run diagnostics before validating or discarding the response.
- Record challenger provider invocations for success, nonzero exit, timeout, and validation failure with status/timing fields.
- `fallback_policy: warn` is visible when provider auto-resolution falls back to Claude.
- Make report wording and denominators accurately reflect what scafld can verify.
- Keep review verdict parsing and completion gates unchanged.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- external review runner: Failed subprocesses and malformed model responses should be observable after the command exits.
- provider telemetry: Session entries should report attempted invocations, not only successful completed review rounds.
- reporting language: Human and JSON report labels should not imply stronger evidence than parser checks can prove.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` Failed, timed-out, and malformed external review attempts leave a diagnostics file path in the command error details and session provider telemetry.
- [ ] `dod2` Provider invocation session entries include status, started_at, completed_at, exit_code, timed_out, timeout_seconds, and diagnostic_path.
- [ ] `dod3` Auto fallback to Claude with `fallback_policy: warn` prints a visible warning and report output includes downgrade denominators.
- [ ] `dod4` Review-signal output uses "format-compliant clean reviews" wording while preserving backward-compatible JSON fields for existing consumers.

Validation:
- [ ] `v1` compile - Compile the Python sources after telemetry changes.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Review gate smoke proves external-runner diagnostics and fallback warning behavior.
  - Command: `bash tests/review_gate_smoke.sh external-runner-observability`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - Report metrics smoke proves provider telemetry denominators and clean-review label compatibility.
  - Command: `bash tests/report_metrics_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: External runner diagnostics and invocation status

Goal: Persist raw failed/malformed external outputs and record provider attempts consistently regardless of outcome.

Status: completed
Dependencies: none

Changes:
- `scafld/review_runner.py` (all) - Add diagnostics persistence, spawn-time/final status telemetry, warning propagation for auto Claude fallback, long Codex output flag, and clearer provenance hashes.
- `scafld/commands/review.py` (all) - Record successful external provider invocations after the candidate review artifact is accepted, and record invalid artifact failures with diagnostics.
- `scafld/review_workflow.py` (all) - Persist the raw external output plus candidate review artifact when downstream artifact parsing rejects a normalized external response.
- `scafld/session_store.py` (all) - Allow provider invocation entries to carry status, timing, exit code, timeout, warning, and diagnostic path fields without breaking existing report summaries.
- `scafld/adapter_runtime.py` (all) - Record adapter provider invocation status and exit code after subprocess completion so adapter telemetry uses the same status vocabulary.
- `tests/review_gate_smoke.sh` (all) - Add focused smoke coverage for nonzero/malformed external output diagnostics, session telemetry, and fallback warning visibility.

Acceptance:
- [x] `ac1_1` integration - External-runner observability smoke passes.
  - Command: `bash tests/review_gate_smoke.sh external-runner-observability`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Honest report language and denominators

Goal: Make human and JSON reporting labels match what parser and telemetry data can actually prove.

Status: completed
Dependencies: none

Changes:
- `scafld/review_signal.py` (all) - Add format-compliant clean-review fields while preserving legacy clean_review_with_evidence keys as aliases.
- `scafld/commands/reporting.py` (all) - Print isolation downgrade denominators and use format-compliant clean-review wording.
- `tests/report_metrics_smoke.sh` (all) - Prove report denominator output and JSON compatibility for the renamed clean-review metric.
- `docs/review.md` (all) - Document failed external diagnostics and clarify that clean-review metrics are parser-format compliance signals.
- `scripts/real_standard.py` (all) - Use format-compliant clean-review wording in cohort markdown while reading the legacy JSON key as a fallback.
- `tests/real_standard_cohort_smoke.sh` (all) - Assert the cohort markdown label no longer overclaims evidence.

Acceptance:
- [x] `ac2_1` integration - Report metrics smoke passes with denominator and metric wording.
  - Command: `bash tests/report_metrics_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` compile - Python sources compile after report updates.
  - Command: `python3 -m compileall scafld cli`
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
Timestamp: '2026-04-26T12:03:30Z'
Review rounds: 8
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

- 2026-04-26T11:15:19Z - user - Spec created via scafld plan
- 2026-04-26T11:18:00Z - assistant - Replaced the placeholder with the subset of review feedback that affects release observability and truthfulness.
- 2026-04-26T11:17:02Z - cli - Spec approved
- 2026-04-26T11:17:02Z - cli - Execution started
- 2026-04-26T12:03:37Z - cli - Spec completed
