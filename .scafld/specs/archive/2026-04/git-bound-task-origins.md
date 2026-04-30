---
spec_version: '2.0'
task_id: git-bound-task-origins
created: '2026-04-21T06:42:00Z'
updated: '2026-04-21T09:12:29Z'
status: completed
harden_status: passed
size: large
risk_level: high
---

# Bind scafld specs to Git branches and work origins

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

Make Git state and engineering origins first-class in scafld. A task should be able to say where it came from, which branch it owns, which base ref it targets, what repo/remote it belongs to, and whether the live workspace has drifted away from the spec. This is the load-bearing bridge between scafld as an internal workflow file and scafld as the kernel underneath everyday Git, issue, and PR flow.

## Context

CWD: `/home/kam/dev/scafld`

Packages:
- `cli`
- `scafld`
- `tests`
- `docs`
- `.ai`

Files impacted:
- `.ai/schemas/spec.json` (all) - The spec schema needs a first-class origin and git-binding model.
- `cli/scafld` (all) - New branch and sync commands plus origin-aware status/start flows belong in the CLI.
- `scafld/git_state.py` (all) - Git helpers need to capture branch, base-ref, remote, and worktree drift details.
- `scafld/spec_store.py` (all) - Origin metadata should live beside spec state and move with the spec.
- `docs/lifecycle.md` (all) - Lifecycle docs should explain how branch/origin binding relates to task state.
- `docs/cli-reference.md` (all) - The CLI reference should describe the new branch and sync commands and their structured outputs.
- `docs/workspaces.md` (all) - Workspace docs should explain branch creation, repo assumptions, and sync semantics.
- `docs/spec-schema.md` (all) - The new origin model should be documented explicitly.
- `tests/git_origin_smoke.sh` (all) - A new smoke test should prove branch creation and sync drift detection.
- `tests/test_git_state.py` (all) - Unit coverage should lock down the live Git-state helpers that power branch binding and sync drift detection.

Invariants:
- `origin_metadata_is_provider_neutral`
- `git_mutation_stays_explicit_and_safe`
- `spec_and_branch_relationship_is_first_class`
- `sync_reports_drift_instead_of_hiding_it`
- `status_can_surface_origin_without_network_access`

Related docs:
- `docs/lifecycle.md`
- `docs/workspaces.md`
- `docs/spec-schema.md`
- `README.md`

## Objectives

- Add a first-class spec metadata block for origin, repo, branch, base-ref, and sync state.
- Introduce `scafld branch <task-id>` to create or bind a working branch safely.
- Introduce `scafld sync <task-id>` to compare spec metadata with live Git state and report drift.
- Make `status --json` surface origin and sync details without requiring wrapper inference.
- Keep the model provider-neutral so GitHub, GitLab, and repo-local callers can all project onto it.

## Scope



## Dependencies

- command-core-integrity-hardening
- native-lifecycle-json-contracts

## Assumptions

- The origin model should store remote-facing identifiers and URLs, but remote mutation remains outside scafld.
- The branch command should prefer safety over convenience and refuse surprising mutations.
- Sync is diagnostic first; it should report drift before any tool tries to repair it.

## Touchpoints

- spec schema: Add first-class origin and Git binding fields to the task spec.
- git runtime: Capture branch, remote, base-ref, head SHA, dirty state, and sync drift.
- CLI workflow: Add branch and sync commands and expose origin-aware status.
- docs and smoke: Explain and verify the new Git-bound workflow.

## Risks

- Branch creation could mutate the wrong branch or dirty workspace if safety checks are too loose.
- Origin metadata could become GitHub-shaped and leak provider assumptions into the kernel.
- Sync output could become prose-only and repeat today's brittleness.

## Acceptance

Profile: strict

Definition of done:
- [x] `dod1` Specs can store origin, repo, branch, base-ref, and sync metadata natively.
- [x] `dod2` `scafld branch` safely creates or binds a task branch and records it in the spec.
- [x] `dod3` `scafld sync` reports current-vs-expected drift in structured form.
- [x] `dod4` `status --json` exposes origin and sync state directly.
- [x] `dod5` Docs and smoke coverage explain and prove the Git-bound task model.

Validation:
- [ ] `v1` integration - Git origin smoke passes.
  - Command: `./tests/git_origin_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` documentation - Spec schema docs include the origin model.
  - Command: `rg -n "origin|branch|base_ref|sync" docs/spec-schema.md docs/workspaces.md docs/lifecycle.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Define origin and git schema

Goal: Extend the spec schema so branch and origin metadata are first-class and portable.

Status: completed
Dependencies: none

Changes:
- `.ai/schemas/spec.json` (all) - Add a top-level origin/git binding block that captures source system, source id/url, repo, remote, branch, base_ref, head_sha, and sync facts.
- `docs/spec-schema.md` (all) - Document the origin/git binding model with one concise example spec.

Acceptance:
- [x] `ac1_1` documentation - Schema docs publish the new origin and binding fields.
  - Command: `rg -n "origin|branch|base_ref|head_sha" docs/spec-schema.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Add branch and sync commands

Goal: Create explicit commands to bind a task to a branch and report live Git drift.

Status: completed
Dependencies: phase1

Changes:
- `cli/scafld` (all) - Add `branch` and `sync` commands plus origin-aware status reporting.
- `scafld/git_state.py` (all) - Capture branch, upstream/base refs, head SHA, dirty state, and diff fingerprints for sync.
- `scafld/spec_store.py` (all) - Persist origin and branch binding changes safely inside the spec.
- `tests/test_git_state.py` (all) - Add focused unit coverage for branch/base detection and sync facts.

Acceptance:
- [x] `ac2_1` integration - Git origin smoke proves branch creation and drift reporting.
  - Command: `./tests/git_origin_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Document the git-bound flow

Goal: Explain how origin, branch, sync, and status fit into the scafld lifecycle.

Status: completed
Dependencies: phase2

Changes:
- `docs/lifecycle.md` (all) - Describe how lifecycle state and Git binding relate, including when sync warnings appear.
- `docs/cli-reference.md` (all) - Document `scafld branch`, `scafld sync`, and the origin-aware status JSON contract.
- `docs/workspaces.md` (all) - Document branch creation, repo assumptions, detached-head/dirty-worktree behavior, and sync usage.
- `tests/git_origin_smoke.sh` (all) - Create fixture repos, bind branches, induce drift, and assert structured sync/status output.

Acceptance:
- [x] `ac3_1` documentation - Workspace and lifecycle docs explain the Git-bound workflow.
  - Command: `rg -n "scafld branch|scafld sync|origin" docs/lifecycle.md docs/workspaces.md`
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
Timestamp: '2026-04-21T09:12:29Z'
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

- 2026-04-21T06:42:00Z - codex - Mapped the Git/origin model needed for scafld to feel like a natural part of branch and issue flow instead of a parallel system.
- 2026-04-21T08:53:55Z - cli - Spec approved
- 2026-04-21T08:54:02Z - cli - Execution started
- 2026-04-21T09:11:50Z - codex - Added provider-neutral origin metadata and git sync snapshots to the spec schema, implemented explicit branch/sync commands with safe branch mutation rules, surfaced origin and sync state in status JSON, and proved the flow with dedicated git-origin smoke plus focused git_state unit coverage.
- 2026-04-21T09:12:29Z - cli - Spec completed
