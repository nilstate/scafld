---
spec_version: '2.0'
task_id: release-trusted-publishing
created: '2026-04-27T09:03:46Z'
updated: '2026-04-27T09:43:01Z'
status: completed
harden_status: in_progress
size: micro
risk_level: medium
---

# Migrate PyPI release to trusted publishing

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

The release workflow authenticates to PyPI with a long-lived `PYPI_API_TOKEN` repository secret. Trusted publishing on PyPI removes the shared secret entirely: the publish job exchanges its short-lived GitHub OIDC token for a per-release upload credential, scoped to the `nilstate/scafld` repository, the `Release` workflow, and the `publish-pypi` job. Migrating shrinks the blast radius of a leaked token and removes a yearly rotation chore.
Switch the PyPI publish job to OIDC trusted publishing. Keep the npm publish path on its current `NPM_TOKEN` flow until npm's OIDC story settles; npm tokens are explicitly out of scope here.

## Context

CWD: `.`

Packages:
- `.github`
- `docs`

Files impacted:
- `.github/workflows/release.yml` (task-selected) - Owns the publish-pypi job; needs `id-token: write` and the password removed.
- `docs/release.md` (task-selected) - New maintainer-facing release doc that records the trusted publisher binding and rollback path.

Invariants:
- `release_workflow_must_remain_idempotent_per_tag`
- `no_long_lived_publish_credentials_in_workflow`
- `tag_to_pypi_release_path_unchanged_for_operators`

Related docs:
- `docs/release.md`
- `.github/workflows/release.yml`

## Objectives

- PyPI publish authenticates via GitHub OIDC trusted publishing instead of a shared API token.
- Document the PyPI trusted publisher registration so a new maintainer can re-bind the workflow if needed.
- Remove the `PYPI_API_TOKEN` reference from the workflow once trusted publishing is verified.

## Scope



## Dependencies

- None.

## Assumptions

- The PyPI project `scafld` already exists; trusted publisher binding can be added via the PyPI project settings.
- `pypa/gh-action-pypi-publish@release/v1` accepts trusted publishing with no `password` input when the job has `id-token: write`.
- The workflow filename, job id, and environment name are stable enough to use in the trusted publisher binding.

## Touchpoints

- release workflow: publish-pypi job permissions, inputs, and dependencies.
- operator docs: How to register or re-register the PyPI trusted publisher and what to do if the OIDC handshake fails.

## Risks

- First release after the cutover fails because the trusted publisher binding on PyPI is missing or mistyped.
- GitHub job permissions are tightened more than necessary and break artifact download.

## Acceptance

Profile: light

Definition of done:
- [x] `dod1` publish-pypi job has `permissions.id-token: write` and no `password` input on `pypa/gh-action-pypi-publish`.
- [x] `dod2` release.yml no longer references `secrets.PYPI_API_TOKEN`.
- [x] `dod3` docs/release.md (or a release runbook) describes the PyPI trusted publisher binding fields and the rollback plan.

Validation:
- [ ] `v1` boundary - publish-pypi step still references the canonical pypi-publish action.
  - Command: `grep -q 'pypa/gh-action-pypi-publish' .github/workflows/release.yml`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` boundary - release.yml no longer references the PyPI token secret.
  - Command: `sh -c '! grep -q PYPI_API_TOKEN .github/workflows/release.yml'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` boundary - publish-pypi job declares OIDC permission.
  - Command: `grep -qE 'id-token:[[:space:]]+write' .github/workflows/release.yml`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Cut publish-pypi over to trusted publishing

Goal: Remove the PyPI token from the workflow, add OIDC permission to the publish job, and document the trusted publisher binding.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` boundary - release.yml has no PyPI token reference.
  - Command: `sh -c '! grep -q PYPI_API_TOKEN .github/workflows/release.yml'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` boundary - publish-pypi job declares id-token write.
  - Command: `grep -qE 'id-token:[[:space:]]+write' .github/workflows/release.yml`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_3` boundary - publish-pypi step still references the canonical pypi-publish action.
  - Command: `grep -q 'pypa/gh-action-pypi-publish' .github/workflows/release.yml`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_4` boundary - Maintainer doc describes the trusted publisher binding.
  - Command: `grep -qiE 'trusted publisher|trusted publishing' docs/release.md`
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
Verdict: pass_with_issues
Timestamp: '2026-04-27T09:43:01Z'
Review rounds: 3
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

- 2026-04-27T09:03:46Z - user - Spec created via scafld plan.
- 2026-04-27T09:10:00Z - agent - Replaced placeholder with a trusted-publishing migration plan scoped to PyPI only.
- 2026-04-27T09:09:22Z - cli - Spec approved
- 2026-04-27T09:11:15Z - cli - Execution started
- 2026-04-27T09:43:01Z - cli - Spec completed
