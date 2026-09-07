---
spec_version: '2.0'
task_id: release-readiness-no-drift-252
created: '2026-07-21T04:32:38Z'
updated: '2026-07-21T08:30:40Z'
status: cancelled
harden_status: passed
size: medium
risk_level: medium
---

# Release Readiness No Drift 2.5.2

## Current State

Status: cancelled
Current phase: final
Next: done
Reason: superseded by release-readiness-no-drift-252-final dogfood release-readiness lane
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-07-21T04:33:33Z
Review gate: not_started

## Summary

Prepare scafld 2.5.2 patch release readiness with harden/review anti-bloat contracts, reasoned harden overrides, and synchronized release metadata.

## Objectives

- Harden and review prompts tell agents to reject overengineering and marginal-surface bookkeeping unless a real defect or architectural boundary break is proven.
- Harden `needs_revision` findings can be accepted only through an explicit `approve --reason` path that records a `harden_override` receipt and marks the harden state `overridden`.
- Package, installer, verifier, GitHub action, workflow, and managed initwire defaults are synchronized on unreleased patch version `2.5.2`.
- Release readiness is proven by full checks, local release artifacts, installer smoke, package-manager rendering, and whitespace diff validation.

## Scope

- In scope: `internal/core/hardengate`, harden/approve/status/review app surfaces, CLI output/help, provider and MCP submit descriptions, core prompt assets, spec schema, markdown parse/render support, release metadata, verifier defaults, and release parity tests.
- In scope: local dev binary rebuild so the machine default dogfoods current code.
- Out of scope: publishing GitHub, npm, PyPI, Homebrew, Scoop, WinGet, or OCI releases.
- Out of scope: rewriting historical `.scafld/receipts` that record old model names or old release evidence.

## Dependencies

- Existing scafld release scripts and package-manager templates.
- Installed Go, Node, npm, and Python toolchains used by `make check` and release smoke scripts.
- Local Git workspace state for `git diff --check` and scafld review material projection.

## Assumptions

- `2.5.1` is already tagged and published, so this patch readiness lane must prepare `2.5.2`.
- Historical receipt contents are immutable audit evidence, not active configuration drift.
- Provider model legacy strings in config tests are compatibility inputs that should continue upgrading to rolling/latest defaults.

## Touchpoints

- `internal/core/hardengate/**`
- `internal/app/{approve,harden,review,status}/**`
- `internal/adapters/{cli,providers,mcp,markdown,corebundle}/**`
- `internal/adapters/corebundle/assets/core/prompts/{harden,review}.md`
- `internal/adapters/corebundle/assets/core/schemas/spec.json`
- `docs/**`, `README.md`
- `package/npm/package.json`, `package/pypi/**`
- `scripts/{set-release-version,scafld-verify,build-release-artifacts,smoke-release-installers,check-release-size}.sh`
- `.github/actions/scafld-verify/action.yml`, `.github/workflows/scafld-verify.yml`
- `test/release/packaging_test.go`

## Risks

- Over-tightening prompts could make agents ignore legitimate surface defects; review/harden wording must allow blockers when a real defect, violated invariant, or broken adapter boundary is verified.
- Release metadata can drift when bump scripts do not own every versioned surface; the parity test must fail closed on all active release defaults.
- Approving over harden findings without a receipt would recreate harden churn; the approval gate must require `--reason` whenever harden evidence is incomplete, stale, failed, or `needs_revision`.

## Acceptance

Profile: standard

## Phase 1: Release Readiness No Drift 2.5.2

Status: completed
Dependencies: none

Objective: Prepare scafld 2.5.2 patch release readiness with harden/review anti-bloat contracts, reasoned harden overrides, and synchronized release metadata.

Changes:
- Add a shared harden gate projection consumed by harden/status/approve.
- Add reasoned harden override approval receipts and human status `next action` output.
- Strengthen harden and review prompt/tool contracts against overengineering and marginal-surface bookkeeping.
- Extend release parity tests and `scripts/set-release-version.sh` so all active `2.5.2` release surfaces move together.
- Build and smoke local `2.5.2` release artifacts without publishing.

Acceptance:
- [x] `ac1` command - Configured validation command
  - Command: `make check && scripts/smoke-release-installers.sh 2.5.2 && scripts/check-release-size.sh 2.5.2 && git diff --check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6

## Rollback

- Revert this patch before tagging if release checks or dogfood review find a real blocker.
- If package metadata was bumped but release is abandoned, run `scripts/set-release-version.sh 2.5.1` only as part of an explicit rollback commit before publishing anything.

## Review

Status: not_started
Verdict: none

Findings:
- none

## Self Eval

- Full `make check` must pass after the final version bump.
- Local release artifact build for `2.5.2` must pass size checks.
- npm and PyPI launchers must install and report `2.5.2` against local release assets.
- Active version/reference scans must show no `2.5.1` or `2.4.8` release defaults outside immutable historical receipts.

## Deviations

- none

## Metadata

- created_by: scafld

## Origin

Created by: scafld
Source: plan

## Harden Rounds

### round-1

Status: passed
Started: 2026-07-21T04:33:19Z
Ended: 2026-07-21T04:33:19Z
Spec digest: e03a2613b6ce805ba5ed54fa716b7f796a8a82234b7df789b57276d405a9aeac
Verdict: pass
Provider: local
Output format: local.fixture
Summary: Local provider smoke hardening passed.
Shape decision: keep
True shape: Local smoke hardening keeps the draft shape for transport validation.
Minimal plan: Submit a schema-valid harden dossier without changing files.
Shared owner: internal/core/harden
Adapter boundaries: local provider emits the dossier; harden app derives the verdict
Required spec edits:

Observations:
- design
  - Result: clean
  - Anchor: spec_gap:Summary
  - Note: Local smoke provider records required harden observations.
- scope
  - Result: clean
  - Anchor: spec_gap:Scope
  - Note: Local smoke provider records required harden observations.
- path
  - Result: clean
  - Anchor: spec_gap:Context
  - Note: Local smoke provider records required harden observations.
- command
  - Result: clean
  - Anchor: spec_gap:Acceptance
  - Note: Local smoke provider records required harden observations.
- timing
  - Result: clean
  - Anchor: spec_gap:Acceptance
  - Note: Local smoke provider records required harden observations.
- rollback
  - Result: clean
  - Anchor: spec_gap:Rollback
  - Note: Local smoke provider records required harden observations.


## Planning Log

- none
