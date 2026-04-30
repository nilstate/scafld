---
spec_version: '2.0'
task_id: llm-context-pack-prompt-compiler
created: '2026-04-23T08:49:58Z'
updated: '2026-04-23T12:56:07Z'
status: completed
harden_status: not_run
size: large
risk_level: high
---

# Handoff Renderer And Templates

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

Build one handoff renderer with a kind enum and make the existing prompt files renderer templates instead of standalone artifacts. Add the single new read-only command, `scafld handoff`, so external harnesses can ask for the current phase, recovery, or review handoff without driving lifecycle moves. Every handoff should render as a sibling `*.md + *.json` pair from one renderer.

## Context

CWD: `.`

Packages:
- `scafld/`
- `scafld/commands/`
- `.ai/prompts/`
- `docs/`
- `tests/`

Files impacted:
- `scafld/handoff_renderer.py` (all) - New module for rendering all handoff kinds from one code path.
- `scafld/commands/handoff.py` (all) - New read-only command for emitting handoffs.
- `scafld/commands/surface.py` (1-260) - Register the new handoff command.
- `.ai/prompts/plan.md` (all) - Planning template source of truth.
- `.ai/prompts/exec.md` (all) - Phase template source of truth.
- `.ai/prompts/review.md` (all) - Review template source of truth.
- `.ai/prompts/recovery.md` (all) - Recovery template source of truth.
- `docs/cli-reference.md` (all) - Document `scafld handoff`.
- `docs/execution.md` (all) - Document phase and recovery handoffs.
- `docs/review.md` (all) - Document review handoffs.
- `tests/run_contracts_smoke.sh` (all) - Smoke test for runtime contracts and phase handoff generation.
- `tests/review_handoff_smoke.sh` (all) - Smoke test for review handoff generation.
- `tests/lifecycle_smoke.sh` (all) - Smoke test for default review-handoff selection after completion.

Invariants:
- `domain_boundaries`
- `public_api_stable`
- `config_from_env`

Related docs:
- `plans/llm-performance-cutover.md`
- `docs/execution.md`
- `docs/review.md`
- `docs/run-artifacts.md`

## Objectives

- Use one renderer for phase, recovery, and review handoffs.
- Make `.ai/prompts/*.md` the source templates for handoff rendering.
- Render every handoff as a sibling `.md/.json` pair with stable metadata.
- Expose a single read-only `scafld handoff` command.
- Define default selector behavior for active phase, fallback phase1, and review-after-complete.
- Keep the handoff immutable and never use it as run state.

## Scope



## Dependencies

- llm-run-schema-and-config should land first so handoff/session contracts are stable.

## Assumptions

- Session-dependent recovery details can be partially scaffolded here and completed in the session/recovery spec.
- Planning templates can be subordinated now even if planning handoffs are not publicly surfaced in v1.

## Touchpoints

- Renderer: One renderer code path accepts kind and selector and emits a handoff.
- Templates: Prompt files become the one source of truth for rendered handoff prose.
- CLI: External harnesses can retrieve a handoff without lifecycle mutation.
- Docs: Execution and review docs explain how handoffs are obtained.

## Risks

- Templates and rendered handoffs can drift.
- The command surface can regrow.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` One renderer can emit sibling `.md/.json` handoffs of kind phase, review, and recovery.
- [ ] `dod2` Prompt files are consumed as templates, not parallel artifacts.
- [ ] `dod3` scafld handoff returns phase and review handoffs in human and JSON forms with the correct defaults.
- [ ] `dod4` Recovery template exists and is wired into the renderer contract.

Validation:
- [ ] `v1` custom - Validate this draft spec.
  - Command: `python3 cli/scafld validate llm-context-pack-prompt-compiler`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test - Run runtime contract smoke test.
  - Command: `bash tests/run_contracts_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test - Run review handoff and lifecycle smoke tests.
  - Command: `bash tests/review_handoff_smoke.sh && bash tests/lifecycle_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Template hierarchy

Goal: Make prompt files the single prose source for handoff rendering.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` documentation - Template docs mention handoff rendering.
  - Command: `bash -lc 'for f in .ai/prompts/plan.md .ai/prompts/exec.md .ai/prompts/review.md .ai/prompts/recovery.md; do rg -qi "template|handoff" "$f" || exit 1; done'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Unified handoff renderer

Goal: Render all handoff kinds from one code path.

Status: pending
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` test - Handoff smoke test covers phase and review handoffs.
  - Command: `bash tests/run_contracts_smoke.sh && bash tests/review_handoff_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Read-only handoff command

Goal: Expose handoff retrieval without lifecycle mutation.

Status: pending
Dependencies: phase2

Changes:
- none

Acceptance:
- [x] `ac3_1` test - Handoff smoke passes.
  - Command: `bash tests/run_contracts_smoke.sh && bash tests/review_handoff_smoke.sh && bash tests/lifecycle_smoke.sh`
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

Estimated effort hours: 10.0
Actual effort hours: none
AI model: unspecified
React cycles: none

Tags:
- llm-performance
- handoff
- templates

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

- 2026-04-23T08:49:58Z - agent - Rewrote the handoff compiler slice around one renderer, template subordination, and one new command.
- 2026-04-23T12:53:04Z - cli - Spec approved
- 2026-04-23T12:53:05Z - cli - Execution started
- 2026-04-23T12:56:07Z - cli - Spec completed
