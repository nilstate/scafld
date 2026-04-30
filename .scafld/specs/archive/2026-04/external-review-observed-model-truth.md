---
spec_version: '2.0'
task_id: external-review-observed-model-truth
created: '2026-04-26T12:22:27Z'
updated: '2026-04-26T13:21:31Z'
status: completed
harden_status: passed
size: small
risk_level: medium
---

# External review observed model truth

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

External review observability now records every success and failure, but its highest-value attribution fields remain mostly theoretical. `model_observed` is still empty, fallback warnings arrive after a successful downgraded run has already spent provider work, diagnostics preserve outputs without tying them to the exact prompt, provider invocation status is an unconstrained string, and provenance keeps a duplicate response hash alias.
This task closes those truth gaps without weakening the review gate: keep external prose validation strict, keep failed-run diagnostics, and make the telemetry say only what scafld can actually observe.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`
- `docs`

Files impacted:
- `scafld/review_runner.py` (task-selected) - External provider execution, observed model extraction, warnings, prompt diagnostics, isolation flags, and provenance hashes live here.
- `scafld/commands/review.py` (task-selected) - The review command currently owns successful provider telemetry and prints fallback warnings only after success.
- `scafld/review_workflow.py` (task-selected) - Downstream artifact diagnostics need prompt hash/context so discarded artifacts can be reproduced.
- `scafld/session_store.py` (task-selected) - Provider invocation status and confidence fields should be normalized before being aggregated by reports.
- `scafld/adapter_runtime.py` (task-selected) - Provider adapters should use the same validated status vocabulary and observed/requested model semantics where possible.
- `scafld/commands/reporting.py` (task-selected) - Report output should distinguish fallback downgrades from explicitly weaker provider isolation.
- `tests/review_gate_smoke.sh` (task-selected) - External runner stubs should prove model extraction, warning timing, prompt diagnostics, and provenance fields.
- `tests/report_metrics_smoke.sh` (task-selected) - Provider telemetry aggregation should prove validated statuses and the new isolation breakdown.
- `docs/review.md` (task-selected) - Operator docs should describe observed-model and diagnostic semantics.

Invariants:
- `provider_telemetry_must_not_overclaim`
- `failed_external_runs_leave_debuggable_artifacts`
- `review_gate_verdict_semantics_do_not_change`
- `packaged_cli_is_the_truth_surface`

Related docs:
- none

## Objectives

- Populate `model_observed` when provider output exposes an actual model id, while keeping `model_source` honest when it does not.
- Emit auto-to-Claude fallback warnings before the subprocess starts, not only after successful completion.
- Persist prompt hashes and useful prompt context in external-runner diagnostics and artifact diagnostics.
- Validate provider invocation status/confidence strings before they enter the session ledger.
- Clarify isolation telemetry so reports can distinguish auto fallback downgrades from explicitly configured weaker provider isolation.
- Remove the duplicate `response_sha256` provenance alias while preserving raw and canonical hashes.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- external review runner: Provider subprocess execution should surface pre-run warnings, observed model data, prompt fingerprints, and precise provenance without adding a looser review result contract.
- provider telemetry: Session provider invocation entries should use a small validated vocabulary and separate requested, observed, fallback, and isolation facts.
- report and docs: Human-facing reports and docs should explain what telemetry does and does not prove.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` External Claude and Codex stub outputs can populate `model_observed` and mark confidence/model_source as observed.
- [ ] `dod2` Auto fallback warnings are visible before provider subprocess execution, including on failed Claude fallback attempts.
- [ ] `dod3` External-runner diagnostics include prompt sha256 data and artifact diagnostics include the same prompt fingerprint.
- [ ] `dod4` Provider invocation status/confidence are validated and report output separates fallback downgrades from weaker isolation invocations.
- [ ] `dod5` External review provenance keeps raw and canonical response hashes and no longer emits a duplicate `response_sha256` alias.

Validation:
- [ ] `v1` compile - Compile Python sources after runner/session/report changes.
  - Command: `python3 -m compileall scafld cli scripts`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Review gate smoke proves observed model, pre-run warning, prompt diagnostics, and provenance fields.
  - Command: `bash tests/review_gate_smoke.sh external-runner-observed-model-truth`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - Report metrics smoke proves provider telemetry aggregation remains stable with validated statuses and isolation breakdowns.
  - Command: `bash tests/report_metrics_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Runner provenance and diagnostics

Goal: Make the external runner emit warnings before execution, extract observed model ids where provider output allows, attach prompt fingerprints to diagnostics, and prune duplicate response hash provenance.

Status: completed
Dependencies: none

Changes:
- `scafld/review_runner.py` (all) - Add provider-output model extraction for Claude JSON envelopes and Codex output hints/stderr where available, switch Claude to a parseable output mode without loosening the review prose contract, return pre-run warnings, include prompt sha256/context in diagnostics, and remove the duplicate `response_sha256` alias from provenance.
- `scafld/commands/review.py` (all) - Print external runner warnings before subprocess execution and record successful telemetry from runner-owned provenance without hiding artifact validation failures.
- `scafld/review_workflow.py` (all) - Include prompt sha256 data in downstream invalid-artifact diagnostics.
- `tests/review_gate_smoke.sh` (all) - Add focused smoke cases for Claude JSON model extraction, Codex model extraction hints, pre-run fallback warnings, prompt fingerprints in diagnostics, and absence of the duplicate response hash alias.

Acceptance:
- [x] `ac1_1` integration - External-runner observed-model smoke passes.
  - Command: `bash tests/review_gate_smoke.sh external-runner-observed-model-truth`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Session validation and reporting

Goal: Normalize provider telemetry values before aggregation and show weaker isolation separately from auto fallback downgrade count.

Status: completed
Dependencies: none

Changes:
- `scafld/session_store.py` (all) - Validate provider invocation status and confidence against explicit vocabularies while preserving existing default behavior.
- `scafld/adapter_runtime.py` (all) - Route adapter provider invocation statuses through the same validated vocabulary and keep requested-only model semantics honest.
- `scafld/commands/reporting.py` (all) - Aggregate weaker isolation counts separately from auto fallback downgrades and keep provider status reporting stable.
- `tests/report_metrics_smoke.sh` (all) - Assert validated status behavior and the new weaker-isolation denominator in human and JSON report output.
- `docs/review.md` (all) - Document observed-model provenance, prompt hashes in diagnostics, and isolation/downgrade report semantics.

Acceptance:
- [x] `ac2_1` integration - Report metrics smoke passes.
  - Command: `bash tests/report_metrics_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` compile - Python sources compile after session/report changes.
  - Command: `python3 -m compileall scafld cli scripts`
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
Timestamp: '2026-04-26T13:16:10Z'
Review rounds: 4
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

- 2026-04-26T12:22:27Z - user - Spec created via scafld plan
- 2026-04-26T12:24:00Z - assistant - Converted Claude's adversarial feedback into an implementation spec focused on observed model attribution, pre-run warnings, diagnostics, status validation, isolation reporting, and response hash cleanup.
- 2026-04-26T12:23:38Z - cli - Spec approved
- 2026-04-26T12:33:58Z - cli - Execution started
- 2026-04-26T13:21:31Z - cli - Spec completed
