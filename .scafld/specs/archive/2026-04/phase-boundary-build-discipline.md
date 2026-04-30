---
spec_version: '2.0'
task_id: phase-boundary-build-discipline
created: '2026-04-25T08:42:10Z'
updated: '2026-04-25T08:51:03Z'
status: completed
harden_status: in_progress
size: medium
risk_level: high
---

# Make build advance one phase at a time

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

Fix the execution model so scafld respects phase boundaries instead of silently validating future phases when their checks also happen to pass. Today `build` calls `exec_snapshot(..., phase=None)` and `exec_snapshot()` runs every runnable acceptance criterion in the spec, then marks later phases complete if their criteria pass. That breaks the handoff model: an operator can consume the phase1 handoff, rerun `build`, and discover phase2 and phase3 already completed without ever seeing their handoffs. The clean model is one phase per default `build` or `exec`: run the current open phase, record its results, and if it passes emit the next phase handoff instead of executing the next phase automatically.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`
- `docs`

Files impacted:
- `scafld/execution_runtime.py` (task-selected) - Default execution scope should resolve to the current open phase rather than the whole spec.
- `scafld/workflow_runtime.py` (task-selected) - Build should preserve the phase boundary and return the next handoff instead of over-executing.
- `scafld/commands/execution.py` (task-selected) - Human output should say which phase actually ran when default exec scopes itself.
- `tests/phase_boundary_smoke.sh` (all) - A dedicated smoke should prove build and default exec stop at the next phase boundary.
- `docs/lifecycle.md` (task-selected) - The lifecycle docs should describe one-phase-at-a-time execution and handoff advancement.
- `docs/cli-reference.md` (task-selected) - CLI docs should explain default phase scoping for build and exec.

Invariants:
- `handoff_is_the_phase_boundary`
- `default_build_does_not_skip_unconsumed_phase_handoffs`
- `explicit_phase_execution_remains_available`
- `machine_output_names_the_actual_executed_phase`

Related docs:
- `AGENTS.md`
- `CONVENTIONS.md`
- `docs/lifecycle.md`
- `docs/review.md`
- `docs/cli-reference.md`

## Objectives

- Make default `build` execute only the current open phase.
- Make default `exec` execute only the current open phase unless `--phase` is explicitly supplied.
- After a phase passes, emit the next phase handoff instead of auto-running that next phase.
- Expose the executed phase explicitly in human and JSON output.
- Protect the behavior with a real CLI smoke that covers a multi-phase spec.

## Scope



## Dependencies

- None.

## Assumptions

- Phase boundaries are load-bearing and should not be crossed without another explicit `build`/`exec` invocation.
- If an operator wants to run a later phase intentionally, `--phase <id>` remains the escape hatch.
- A default exec/build call should be predictable enough that automation can treat the returned handoff as authoritative.

## Touchpoints

- execution semantics: Current-phase-only execution becomes the default contract.
- handoff progression: Passing one phase should surface the next handoff rather than consume it implicitly.
- automation output: JSON and terminal output should name the phase that actually ran.

## Risks

- Changing exec defaults could surprise advanced operators who were relying on whole-spec execution.
- Phase selection could regress recovery behavior if failed criteria no longer map cleanly to the active phase.
- Guidance/status could drift from actual execution semantics.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` Default build and exec run only the current open phase.
- [ ] `dod2` Passing one phase emits the next phase handoff without auto-completing the later phase.
- [ ] `dod3` Execution output states which phase actually ran.
- [ ] `dod4` A dedicated multi-phase smoke proves the new boundary behavior.
- [ ] `dod5` Lifecycle and CLI docs describe the one-phase-at-a-time model.

Validation:
- [ ] `v1` compile - Compile the Python sources after the execution model changes.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Dedicated phase-boundary smoke passes.
  - Command: `bash tests/phase_boundary_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - Lifecycle smoke still passes with the new build semantics.
  - Command: `bash tests/lifecycle_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Scope default execution to the current phase

Goal: Make default exec/build run the current open phase only.

Status: in_progress
Dependencies: none

Changes:
- `scafld/execution_runtime.py` (all) - Resolve the default execution scope to the current open phase and keep explicit `--phase` behavior intact.
- `scafld/workflow_runtime.py` (all) - Drive build through the same current-phase execution scope instead of whole-spec execution.
- `tests/phase_boundary_smoke.sh` (all) - Prove a multi-phase spec stops after phase1 and surfaces phase2 as the next handoff.

Acceptance:
- [ ] `ac1_1` integration - Phase-boundary smoke proves build stops after the current phase.
  - Command: `bash tests/phase_boundary_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Surface the executed phase honestly

Goal: Make human and JSON execution output name the phase that actually ran.

Status: in_progress
Dependencies: phase1

Changes:
- `scafld/execution_runtime.py` (all) - Include the executed phase in execution payloads and next-action output.
- `scafld/commands/execution.py` (all) - Print the resolved phase in default exec output so operators can see the actual execution boundary.
- `tests/phase_boundary_smoke.sh` (all) - Assert JSON and human output include the executed phase and next phase handoff.

Acceptance:
- [ ] `ac2_1` integration - Phase-boundary smoke proves execution output names the resolved phase.
  - Command: `bash tests/phase_boundary_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Document the build boundary

Goal: Make the one-phase-at-a-time model explicit in docs and keep the broader lifecycle smoke green.

Status: completed
Dependencies: phase2

Changes:
- `docs/lifecycle.md` (all) - Document that default build advances one phase boundary at a time and emits the next handoff after a pass.
- `docs/cli-reference.md` (all) - Document that default exec/build run the current phase unless `--phase` is supplied.

Acceptance:
- [x] `ac3_1` compile - Python sources still compile.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` integration - Lifecycle smoke stays green.
  - Command: `bash tests/lifecycle_smoke.sh`
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
Timestamp: '2026-04-25T08:50:30Z'
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

- 2026-04-25T08:42:10Z - user - Spec created via scafld plan
- 2026-04-25T08:48:00Z - assistant - Replaced the placeholder draft with a spec grounded in the live build auto-completion flaw discovered while executing the previous task.
- 2026-04-25T08:44:34Z - cli - Spec approved
- 2026-04-25T08:44:34Z - cli - Execution started
- 2026-04-25T08:51:03Z - cli - Spec completed
