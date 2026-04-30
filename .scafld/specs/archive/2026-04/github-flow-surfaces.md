---
spec_version: '2.0'
task_id: github-flow-surfaces
created: '2026-04-21T11:19:03Z'
updated: '2026-04-21T11:24:46Z'
status: completed
harden_status: not_run
size: small
risk_level: medium
---

# Polish GitHub flow surfaces

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

Make the git/PR/issue projection story legible to both humans and wrappers. Today `docs/github-flow.md` is still too thin to prove the intended engineering-system workflow, and human `scafld status` omits the origin source metadata that explains what branch binding is actually tied to. This round should expand the docs into a real issue-to-PR walkthrough, surface source/binding context in the terminal status view, and prove the new human output in the existing git-origin smoke.

## Context

CWD: `.`

Packages:
- `cli`
- `docs`
- `tests`

Files impacted:
- none

Invariants:
- `domain_boundaries`
- `human_output_remains_default`
- `spec_remains_the_single_source_of_truth`
- `wrappers_stay_thin`

Related docs:
- none

## Objectives

- Show source and binding facts in human `scafld status` when origin metadata exists.
- Expand the GitHub flow documentation into a concrete issue-to-branch-to-review-to-PR walkthrough.
- Prove the human status surface in smoke coverage so the docs match real operator output.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- cli/scafld: Render source, binding mode, and upstream context in the human status output.
- docs/github-flow.md: Document the complete GitHub-facing flow with end-to-end examples.
- docs/cli-reference.md: Document the richer human status surface.
- tests/git_origin_smoke.sh: Assert the new human status lines in a real git workspace.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [x] `dod1` Human `scafld status` shows origin source and binding context when present.
- [x] `dod2` `docs/github-flow.md` contains a concrete issue-to-PR walkthrough rather than a thin command list.
- [x] `dod3` Smoke coverage proves the human status surface in a git-bound task fixture.

Validation:
- [ ] `v1` integration - Git origin smoke passes with the richer human status surface.
  - Command: `./tests/git_origin_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` documentation - GitHub flow doc includes the end-to-end workflow sections.
  - Command: `grep -En -- "Issue -> Branch -> Review -> PR|scafld status|scafld checks|scafld pr-body" docs/github-flow.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Show GitHub Flow Clearly

Goal: Make the local status surface and GitHub flow docs reflect the real branch-bound workflow.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` integration - Git origin smoke passes with the richer human status surface.
  - Command: `./tests/git_origin_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` documentation - GitHub flow doc includes the end-to-end workflow sections.
  - Command: `grep -En -- "Issue -> Branch -> Review -> PR|scafld status|scafld checks|scafld pr-body" docs/github-flow.md`
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
Timestamp: '2026-04-21T11:24:16Z'
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

- 2026-04-21T11:19:03Z - user - Spec created via scafld new
- 2026-04-21T11:20:00Z - assistant - Expanded the draft into a focused GitHub-flow ergonomics slice covering human status output, docs, and smoke proof.
- 2026-04-21T11:20:44Z - cli - Spec approved
- 2026-04-21T11:21:05Z - cli - Execution started
- 2026-04-21T11:24:46Z - cli - Spec completed
