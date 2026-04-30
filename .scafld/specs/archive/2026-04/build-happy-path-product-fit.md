---
spec_version: '2.0'
task_id: build-happy-path-product-fit
created: '2026-04-24T06:10:00Z'
updated: '2026-04-24T01:07:51Z'
status: completed
harden_status: passed
size: large
risk_level: medium
---

# Make Build Feel Easier Than A Raw Agent Loop

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

Tighten the happy path around `build` and `status` until scafld feels shorter and clearer than manually juggling agent turns, handoff paths, and lifecycle state. The work only counts if a medium brownfield task feels easier to drive through `build -> review -> complete` than through a raw agent loop. That means one obvious next action, crisp blocked-state guidance, and no operator archaeology to figure out what should happen next.

## Context

CWD: `.`

Packages:
- `scafld/commands/workflow.py`
- `scafld/commands/lifecycle.py`
- `scafld/commands/execution.py`
- `scafld/output.py`
- `README.md`
- `AGENTS.md`
- `docs/execution.md`
- `docs/cli-reference.md`
- `tests/`

Files impacted:
- `scafld/commands/workflow.py` (all) - build remains the public orchestration surface and should own the happy path.
- `scafld/commands/lifecycle.py` (1-420) - status should surface the same next-action guidance as build.
- `scafld/commands/execution.py` (220-620) - execution already knows recovery and block states; those signals need cleaner surfacing.
- `scafld/output.py` (all) - Human and JSON envelopes should present the next action consistently.
- `README.md` (1-220) - The default story should describe build as the shortest path through real work.
- `AGENTS.md` (1-220) - Agent guidance should teach one obvious command and one obvious next step.
- `docs/execution.md` (all) - Execution docs should describe the happy path and blocked states with zero operator guesswork.
- `docs/cli-reference.md` (all) - CLI docs should make build/status next-action semantics explicit.
- `tests/agent_surface_smoke.sh` (all) - Existing surface smoke should prove build remains the public path.
- `tests/lifecycle_smoke.sh` (all) - Lifecycle smoke should keep the end-to-end build path honest.
- `tests/build_happy_path_smoke.sh` (all) - New smoke should exercise approved, in_progress, blocked, and review-ready next-action states.
- `tests/status_next_action_smoke.sh` (all) - New smoke should prove status mirrors build guidance.

Invariants:
- `domain_boundaries`
- `public_api_stable`
- `no_test_logic_in_production`

Related docs:
- `README.md`
- `AGENTS.md`
- `docs/execution.md`
- `docs/cli-reference.md`

## Objectives

- Make build the obvious command for approved and in_progress work.
- Expose one canonical next action across build, status, review, and complete.
- Make blocked and recovery-required states obvious in both human and JSON output.
- Cut operator leakage from the default docs and prompts without removing advanced tools.
- Move scafld closer to the real standard that build feels easier than a raw agent loop.

## Scope



## Dependencies

- Assumes the slim public surface and role×gate handoff transport remain the base architecture.

## Assumptions

- The public command set stays `init, plan, approve, build, review, complete, status, list, report, handoff, update`.
- The next-action story should be derived from spec plus session, not from handoff contents.

## Touchpoints

- build orchestration: Public build flow should stay short, deterministic, and easy to resume.
- status control tower: status should tell a human or wrapper exactly what to do next.
- human output: Human-facing summaries should remove ambiguity without becoming verbose.
- json surface: Wrappers should get canonical next_action and block reason fields without guessing.

## Risks

- Next-action logic can drift if multiple commands compute it independently.
- Making output friendlier can bloat the command surfaces.
- A smoother build path could accidentally hide meaningful operator tools.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` build exposes one obvious next action for approved, active, blocked, and review-ready tasks.
- [ ] `dod2` status mirrors the same next action and block reason without requiring JSON interpretation.
- [ ] `dod3` Docs and agent guidance teach build as the shortest path through real work.
- [ ] `dod4` Smoke coverage proves the happy path feels like one coherent loop instead of a wrapper over old commands.

Validation:
- [ ] `v1` custom - Validate this draft spec.
  - Command: `python3 cli/scafld validate build-happy-path-product-fit`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` compile - Compile scafld after the happy-path refactor.
  - Command: `python3 -m compileall scafld`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test - Run the public-surface and lifecycle smokes.
  - Command: `bash tests/agent_surface_smoke.sh && bash tests/lifecycle_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` test - Run happy-path next-action coverage.
  - Command: `bash tests/build_happy_path_smoke.sh && bash tests/status_next_action_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Canonical next action

Goal: Derive and expose one shared next-action story across build and status.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Happy-path next-action smoke passes.
  - Command: `bash tests/build_happy_path_smoke.sh && bash tests/status_next_action_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Teach the short path

Goal: Update docs and agent guidance to present build as the easiest real path.

Status: completed
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` test - Existing public-surface smokes stay green.
  - Command: `bash tests/agent_surface_smoke.sh && bash tests/lifecycle_smoke.sh`
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
Timestamp: '2026-04-24T01:07:30Z'
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

- 2026-04-24T06:10:00Z - assistant - Drafted the build product-fit slice against the real-standard bar.
- 2026-04-24T01:03:21Z - cli - Spec approved
- 2026-04-24T01:03:37Z - cli - Execution started
- 2026-04-24T01:07:51Z - cli - Spec completed
