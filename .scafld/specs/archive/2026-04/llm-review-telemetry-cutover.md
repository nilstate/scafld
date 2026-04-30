---
spec_version: '2.0'
task_id: llm-review-telemetry-cutover
created: '2026-04-23T08:49:58Z'
updated: '2026-04-23T12:56:07Z'
status: completed
harden_status: not_run
size: large
risk_level: high
---

# Review Handoffs And Report Metrics

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

Complete the cutover by giving review a generated handoff and making report read quality metrics from session rather than a separate telemetry subsystem. scafld should preserve the existing review artifact while generating a fresh review handoff and showing whether the value layer is moving execution quality.

## Context

CWD: `.`

Packages:
- `scafld/review_workflow.py`
- `scafld/reviewing.py`
- `scafld/commands/review.py`
- `scafld/commands/reporting.py`
- `scafld/projections.py`
- `docs/`
- `tests/`

Files impacted:
- `scafld/review_workflow.py` (1-460) - Review opening should generate or reference a review handoff derived from spec, diff, and session summary.
- `scafld/reviewing.py` (1-360) - Review metadata may need optional fields for handoff reference and reviewer isolation.
- `scafld/commands/review.py` (1-360) - review --json should expose review handoff paths without breaking current review flow.
- `scafld/commands/reporting.py` (1-320) - Report should expose session-derived LLM-performance metrics.
- `scafld/projections.py` (1-220) - Projection model can expose session summary fields.
- `docs/review.md` (all) - Document review as a fresh-context handoff stage.
- `docs/cli-reference.md` (all) - Document review JSON additions and report metrics.
- `docs/configuration.md` (all) - Document minimal llm config surface and optional usage fields inside session.
- `tests/review_handoff_smoke.sh` (all) - New smoke test for review handoff generation.
- `tests/report_metrics_smoke.sh` (all) - New smoke test for session-driven metrics.

Invariants:
- `domain_boundaries`
- `public_api_stable`
- `config_from_env`
- `no_test_logic_in_production`

Related docs:
- `plans/llm-performance-cutover.md`
- `docs/review.md`
- `docs/configuration.md`
- `docs/run-artifacts.md`

## Objectives

- Generate a review handoff from spec, diff, and compact session state.
- Keep Review Artifact v3 intact while adding optional handoff references.
- Make report derive first_attempt_pass_rate, recovery_convergence_rate, and attempts per phase from session.
- Surface the canonical metrics both per task and in aggregate.
- State the honest boundary that handoffs may be ignored by external agent harnesses.

## Scope



## Dependencies

- llm-recovery-session-loop should land first so report has session attempts and phase summaries to read.

## Assumptions

- Optional usage fields may be recorded inside session by external wrappers, but report must work without them.
- Review handoff generation can reuse the same renderer used for phase and recovery.

## Touchpoints

- Review handoff: Review gets a fresh-context handoff rather than inheriting executor chatter.
- Review artifact metadata: Metadata can reference the review handoff without changing required Review Artifact v3 fields.
- Reporting: report reads session and review outcomes instead of separate telemetry files, exposing canonical metrics per task and in aggregate.
- Agent-agnostic boundary: Docs and report describe that handoffs improve available input but do not force consumption.

## Risks

- Adding review metadata can break completion gate parsing.
- Metrics can overclaim causality.
- Session metrics may be incomplete when wrappers do not record usage.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` scafld review emits a review handoff and keeps existing review artifact behavior.
- [ ] `dod2` Review metadata remains backward-compatible and may reference the review handoff.
- [ ] `dod3` scafld report shows first_attempt_pass_rate, recovery_convergence_rate, and attempts per phase from session, both per task and in aggregate.
- [ ] `dod4` No separate telemetry artifact or command is introduced.

Validation:
- [ ] `v1` custom - Validate this draft spec.
  - Command: `python3 cli/scafld validate llm-review-telemetry-cutover`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test - Run review handoff smoke test.
  - Command: `bash tests/review_handoff_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test - Run report metrics smoke test.
  - Command: `bash tests/report_metrics_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` test - Run existing review gate smoke to prove compatibility.
  - Command: `bash tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Review handoff

Goal: Generate a fresh review handoff without changing the existing review lifecycle.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Review handoff smoke test passes.
  - Command: `bash tests/review_handoff_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Session-derived metrics

Goal: Teach report to measure the value layer from session outcomes.

Status: pending
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` test - Report metrics smoke test passes.
  - Command: `bash tests/report_metrics_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Docs and compatibility

Goal: Make the architecture legible without expanding the public surface.

Status: pending
Dependencies: phase2

Changes:
- none

Acceptance:
- [x] `ac3_1` test - Review, report, and review gate smoke tests pass.
  - Command: `bash tests/review_handoff_smoke.sh && bash tests/report_metrics_smoke.sh && bash tests/review_gate_smoke.sh`
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
Timestamp: '2026-04-23T12:56:07Z'
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

Estimated effort hours: 12.0
Actual effort hours: none
AI model: unspecified
React cycles: none

Tags:
- llm-performance
- review
- reporting
- session

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

- 2026-04-23T13:37:00Z - agent - Rewrote review/telemetry cutover around review handoffs and session-derived report metrics.
- 2026-04-23T12:53:05Z - cli - Spec approved
- 2026-04-23T12:53:06Z - cli - Execution started
- 2026-04-23T12:56:07Z - cli - Spec completed
