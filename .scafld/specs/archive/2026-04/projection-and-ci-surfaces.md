---
spec_version: '2.0'
task_id: projection-and-ci-surfaces
created: '2026-04-21T06:43:00Z'
updated: '2026-04-21T09:22:23Z'
status: completed
harden_status: not_run
size: large
risk_level: medium
---

# Project scafld state onto PR, issue, and CI surfaces

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

Make scafld project its task state naturally onto the engineering surfaces people already live in: branches, PR bodies, issue comments, and CI checks. The goal is not to make scafld a hosted forge client. The goal is to make scafld the authoritative workflow object and let GitHub or runx consume projections without inventing a second source of truth. This work adds projection commands and CI-friendly outputs so wrappers can render the same state everywhere.

## Context

CWD: `/home/kam/dev/scafld`

Packages:
- `cli`
- `scafld`
- `tests`
- `docs`
- `.github`

Files impacted:
- `cli/scafld` (all) - New projection commands and CI-friendly summary/check outputs belong in the CLI.
- `scafld/output.py` (all) - Projection rendering should reuse shared structured output and markdown rendering helpers.
- `docs/cli-reference.md` (all) - The CLI reference must describe summary/projection commands and their output contracts.
- `docs/review.md` (all) - Review docs should explain how review state projects into PR/check surfaces.
- `docs/quickstart.md` (all) - Quickstart should show a natural issue-to-branch-to-PR flow.
- `docs/github-flow.md` (all) - A new doc should explain how to use scafld projections from GitHub Actions or wrappers.
- `.github/workflows/review-gate-smoke.yml` (all) - CI should prove that scafld can emit check-friendly structured summaries.
- `tests/projection_surface_smoke.sh` (all) - A new smoke test should exercise summary, checks, and PR body rendering.

Invariants:
- `spec_remains_the_single_source_of_truth`
- `projection_commands_do_not_require_network_access`
- `markdown_and_json_views_of_the_same_state_stay_aligned`
- `ci_outputs_are_machine_consumable`
- `projection_surfaces_do_not_reinvent_workflow_state`

Related docs:
- `docs/quickstart.md`
- `docs/review.md`
- `docs/cli-reference.md`
- `docs/lifecycle.md`

## Objectives

- Add projection commands that render PR bodies, issue/PR comments, check summaries, and concise task summaries from the spec.
- Expose CI-friendly structured status and check payloads so GitHub Actions or runx can publish the same truth.
- Keep projections deterministic and derived from spec/origin/review state rather than ad hoc prompt output.
- Document a natural engineering flow that uses projections without turning scafld into a forge SDK.
- Prove the surfaces with smoke coverage and one repository-local CI example.

## Scope



## Dependencies

- command-core-integrity-hardening
- native-lifecycle-json-contracts
- git-bound-task-origins

## Assumptions

- Projection commands should be deterministic views over task state, not another workflow authoring surface.
- A GitHub-friendly flow can be documented and proven locally without hardcoding GitHub API behavior into scafld.
- CI surfaces should prefer JSON payloads and concise markdown blocks over long prose.

## Touchpoints

- projection commands: Render task summaries, PR bodies, comment blocks, and checks from scafld state.
- CI integration: Provide structured check payloads and a repo-local proof path in CI.
- operator docs: Teach a natural engineering-system flow using projections.

## Risks

- Projection commands could fork the truth if they accept too much manual customization.
- Markdown and JSON projections could drift from each other.
- CI docs could over-promise direct GitHub integration that scafld itself does not own.

## Acceptance

Profile: strict

Definition of done:
- [x] `dod1` Projection commands exist for at least summary, checks, and PR body/comment output.
- [x] `dod2` Projection output can be rendered as both concise markdown and structured JSON.
- [x] `dod3` A repo-local CI proof consumes the structured outputs without scraping terminal text.
- [x] `dod4` Docs explain how projections make scafld feel native to Git/PR/issue flow.

Validation:
- [ ] `v1` integration - Projection surface smoke passes.
  - Command: `./tests/projection_surface_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` documentation - Docs include the GitHub/CI projection flow.
  - Command: `rg -n "pr-body|checks|summary|GitHub" docs/cli-reference.md docs/quickstart.md docs/github-flow.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Build projection models

Goal: Create a single internal projection model that can render markdown and JSON views over task state.

Status: completed
Dependencies: none

Changes:
- `scafld/output.py` (all) - Add projection-model helpers that render concise markdown and JSON from task, review, and origin state.
- `cli/scafld` (all) - Add projection-oriented commands such as `summary`, `checks`, and `pr-body` with shared structured output.

Acceptance:
- [x] `ac1_1` integration - Projection smoke proves markdown and JSON rendering from the same task state.
  - Command: `./tests/projection_surface_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Make CI consume projections

Goal: Prove that repo-local CI can consume scafld projections without terminal scraping.

Status: completed
Dependencies: phase1

Changes:
- `.github/workflows/review-gate-smoke.yml` (all) - Add a step or job that consumes scafld structured check/summary output as part of repo-local proof.
- `tests/projection_surface_smoke.sh` (all) - Exercise summary, checks, and PR body projections from fixture specs and review state.

Acceptance:
- [x] `ac2_1` integration - Projection smoke passes with CI-oriented payload assertions.
  - Command: `./tests/projection_surface_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Document the engineering flow

Goal: Explain how projection surfaces map scafld naturally into issue, PR, and CI workflows.

Status: completed
Dependencies: phase2

Changes:
- `docs/github-flow.md` (all) - Document the natural Git/issue/PR/CI flow that consumes scafld projections.
- `docs/review.md` (all) - Explain how review state projects into summary, checks, and PR-body surfaces.
- `docs/quickstart.md` (all) - Show an issue-to-branch-to-PR flow that uses summary/check/pr-body projections.
- `docs/cli-reference.md` (all) - Document the new projection commands and their markdown/JSON outputs.
- `README.md` (all) - Describe scafld as the workflow kernel behind normal engineering surfaces.

Acceptance:
- [x] `ac3_1` documentation - Projection docs describe the natural engineering-system flow.
  - Command: `rg -n "engineering system|pr-body|checks|summary|issue" README.md docs/quickstart.md docs/github-flow.md docs/cli-reference.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Rollback

Strategy: manual

Commands:
- none

## Review

Status: not_started
Verdict: incomplete
Timestamp: '2026-04-21T09:22:23Z'
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

- 2026-04-21T06:43:00Z - codex - Mapped the projection surfaces needed for scafld to sit naturally underneath PR, issue, and CI workflows.
- 2026-04-21T09:13:07Z - cli - Spec approved
- 2026-04-21T09:13:15Z - cli - Execution started
- 2026-04-21T09:21:52Z - codex - Built one deterministic projection model over spec, review, origin, and sync state; added summary/checks/pr-body commands, proved the markdown and JSON views with a dedicated projection smoke, wired that smoke into CI, and documented the issue/PR/check flow.
- 2026-04-21T09:22:23Z - cli - Spec completed
