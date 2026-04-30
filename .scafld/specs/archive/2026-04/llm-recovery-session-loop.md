---
spec_version: '2.0'
task_id: llm-recovery-session-loop
created: '2026-04-23T08:49:58Z'
updated: '2026-04-23T12:56:07Z'
status: completed
harden_status: not_run
size: large
risk_level: high
---

# Session Recovery And Phase Compaction

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

Add the run-time ledger beneath the existing execution flow. scafld should record execution attempts in session.json, write compact phase summaries at phase boundaries, preserve full diagnostics in .ai/runs, and surface recovery as a handoff kind rather than a separate subsystem. When recovery reaches the configured cap, exec should stop emitting new recovery handoffs, mark the criterion `failed_exhausted`, and require a human.

## Context

CWD: `.`

Packages:
- `scafld/acceptance.py`
- `scafld/commands/execution.py`
- `scafld/runtime_contracts.py`
- `scafld/handoff_renderer.py`
- `docs/`
- `tests/`

Files impacted:
- `scafld/session_store.py` (all) - New module for session.json creation, append-mostly updates, and phase summaries.
- `scafld/acceptance.py` (1-260) - Acceptance outcomes must preserve full diagnostics for session-linked artifacts while keeping spec snippets short.
- `scafld/commands/execution.py` (1-420) - exec should initialize session state, append attempts, write phase summaries, and emit recovery handoffs.
- `scafld/error_codes.py` (1-120) - Execution needs a distinct recovery_exhausted error code.
- `scafld/handoff_renderer.py` (1-260) - Recovery handoffs should render from session state, failed criteria, and diagnostic references.
- `scafld/commands/handoff.py` (1-220) - handoff should be able to regenerate the current recovery handoff from session state.
- `docs/execution.md` (all) - Document session ledger rules, phase compaction, and diagnostics.
- `docs/cli-reference.md` (all) - Document recovery selection through scafld handoff.
- `docs/run-artifacts.md` (all) - Document failed_exhausted and blocked-phase session state.
- `tests/session_recovery_smoke.sh` (all) - New smoke test for exec/session/recovery handoff flow.
- `tests/recovery_cap_smoke.sh` (all) - Smoke test for recovery exhaustion and human-required output.

Invariants:
- `domain_boundaries`
- `public_api_stable`
- `no_legacy_code`
- `config_from_env`

Related docs:
- `plans/llm-performance-cutover.md`
- `docs/execution.md`
- `docs/run-artifacts.md`

## Objectives

- Create session.json as the only durable execution-state source.
- Write phase_summaries[] at phase boundaries so later handoffs read compact prior state.
- Persist full diagnostics under .ai/runs/{task-id}/diagnostics/ while keeping short spec snippets.
- Represent recovery purely as handoff.kind = recovery plus counters and limits recorded in session.
- At the recovery cap, record failed_exhausted, block the phase, and surface human_required instead of another handoff.

## Scope



## Dependencies

- llm-run-schema-and-config should land first so session contracts and .ai/runs ownership are defined.
- llm-context-pack-prompt-compiler should land first so recovery reuses the handoff renderer.

## Assumptions

- External agents still apply repairs; scafld only compiles the recovery handoff.
- Spec result_output remains intentionally short for audit readability.

## Touchpoints

- Execution ledger: session.json records attempts, recovery counts, selectors, and phase summaries.
- Phase-boundary compaction: exec writes compact summaries when a phase completes and later handoffs read those summaries instead of raw prior output.
- Diagnostics: Full stdout/stderr is written to run artifacts referenced from session attempts.
- Recovery: Failures generate recovery handoffs through the same renderer used for phase and review.
- Recovery cap: Exhausted criteria are marked in session, block the phase, and stop generating new recovery handoffs.

## Risks

- Command diagnostics can contain secrets.
- Session updates can drift if handoffs are treated as state.
- Unbounded retries can hide broken criteria.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` scafld exec creates or updates session.json as the durable run ledger.
- [ ] `dod2` Successful phase advancement writes a phase summary and later handoffs consume it.
- [ ] `dod3` Failed criteria persist full diagnostics in .ai/runs and emit a recovery handoff until the configured cap.
- [ ] `dod4` Exhausted criteria become failed_exhausted, block the phase, and return human_required without adding new public commands.

Validation:
- [ ] `v1` custom - Validate this draft spec.
  - Command: `python3 cli/scafld validate llm-recovery-session-loop`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test - Run session recovery smoke test.
  - Command: `bash tests/session_recovery_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test - Run recovery cap smoke test.
  - Command: `bash tests/recovery_cap_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` test - Run lifecycle smoke to prove existing exec behavior still works.
  - Command: `bash tests/lifecycle_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Session ledger

Goal: Create the append-mostly session store under .ai/runs/{task-id}/session.json.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Runtime contract smoke tests cover session creation and path safety.
  - Command: `bash tests/run_contracts_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Execution attempts and diagnostics

Goal: Record execution attempts and preserve full diagnostics in run artifacts.

Status: pending
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` test - Session recovery smoke test captures attempts and diagnostics.
  - Command: `bash tests/session_recovery_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Phase compaction and bounded recovery

Goal: Write phase summaries, render recovery handoffs from session state, and stop cleanly at the recovery cap.

Status: pending
Dependencies: phase2

Changes:
- none

Acceptance:
- [x] `ac3_1` test - Recovery handoff, recovery cap, and lifecycle smoke tests pass.
  - Command: `bash tests/session_recovery_smoke.sh && bash tests/recovery_cap_smoke.sh && bash tests/lifecycle_smoke.sh`
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

Estimated effort hours: 14.0
Actual effort hours: none
AI model: unspecified
React cycles: none

Tags:
- llm-performance
- session
- recovery
- execution

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

- 2026-04-23T13:32:00Z - agent - Rewrote recovery/session cutover around session as ledger and recovery as a handoff kind.
- 2026-04-23T12:53:05Z - cli - Spec approved
- 2026-04-23T12:53:06Z - cli - Execution started
- 2026-04-23T12:56:07Z - cli - Spec completed
