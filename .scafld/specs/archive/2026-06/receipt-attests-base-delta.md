---
spec_version: '2.0'
task_id: receipt-attests-base-delta
created: '2026-06-03T13:58:14Z'
updated: '2026-06-04T08:58:53Z'
status: completed
harden_status: not_run
size: medium
risk_level: medium
---

# Attest a base-to-head delta, not only working-tree-vs-HEAD

## Current State

Status: completed
Current phase: final
Next: done
Reason: finalization receipt passed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-06-04T08:56:51Z
Review gate: not_started

## Summary

Plumb `base_ref` through the finalize request and receipt, then sign a single snapshot mode enum so verify can recompute the same fingerprint mode. Receipts must distinguish default working-tree snapshots from base-delta snapshots.

## Objectives

- Add a signed `snapshot_mode` enum to receipts.
- Carry `base_ref` from finalize input to git snapshotting and receipt body.
- Expose `base_ref` in the host-facing MCP schema and installed finalize skill/command so production agent calls can request base-delta receipts.
- Make verify recompute using the signed snapshot mode, compare the recomputed `base_commit`, and fail unknown modes.
- Keep existing working-tree mode behavior unchanged when no base ref is supplied.

## Scope

- In scope: finalize request parsing, app finalize input, receipt body, git snapshot input, verify snapshot input, and tests.
- Out of scope: changing the tree writer or diff algorithms beyond passing the existing `BaseRef` consistently.

## Dependencies

- none

## Assumptions

- `BaseCommit` is the merge-base of `base_ref` and `HEAD` in base-delta mode.
- Working-tree mode remains the default for existing requests.

## Touchpoints

- internal/core/receipt/receipt.go
- internal/app/finalize/finalize.go
- internal/adapters/cli/finalize/run.go
- internal/app/verify/verify.go
- internal/adapters/cli/verify/verify.go
- internal/adapters/git/git.go
- internal/app/finalize/finalize_test.go
- internal/app/verify/verify_test.go
- internal/adapters/cli/finalize/run_test.go
- internal/adapters/mcp/finalize/server.go
- internal/adapters/corebundle/assets/initwire/claude/skills/finalize/SKILL.md
- internal/adapters/corebundle/assets/initwire/claude/commands/finalize.md

## Risks

- Adding a receipt field changes canonical bytes.
  - Mitigation: update the conformance corpus in the same release sequence.

## Acceptance

Profile: strict

Validation:
- [x] `v1` validate spec - This spec validates clean.
  - Command: `go run ./cmd/scafld validate receipt-attests-base-delta`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-23

## Phase 1: signed snapshot mode

Status: pass
Dependencies: canonical-receipt-conformance-corpus

Objective: Add the signed snapshot mode and base-ref plumbing through finalize and verify.

Changes:
- internal/core/receipt/receipt.go - add `snapshot_mode` validation and canonical coverage.
- internal/app/finalize/finalize.go - set snapshot mode from whether `BaseRef` is present.
- internal/adapters/cli/finalize/run.go - parse `base_ref` from finalize JSON and pass it to the app.
- internal/adapters/mcp/hostgate/server.go - advertise `base_ref` and `scope_hint` in the `finalize` schema.
- internal/adapters/corebundle/assets/initwire/claude/skills/finalize/SKILL.md - instruct agents to pass `base_ref` for branch or PR work.
- internal/app/verify/verify.go - reject unknown snapshot modes, pass signed mode plus CI target/base data to snapshotter, and require recomputed `base_commit` to match the receipt.
- tests - cover default working-tree mode, base-delta mode, and unknown mode rejection.

Acceptance:
- [x] `ac1_1` mode tests pass - finalize and verify cover both snapshot modes.
  - Command: `go test ./internal/app/finalize ./internal/app/verify ./internal/adapters/cli/finalize -run 'SnapshotMode|BaseRef|BaseDelta|UnknownMode'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-24
- [x] `ac1_2` receipt schema names snapshot_mode - the signed receipt body contains the mode.
  - Command: `rg -n 'SnapshotMode|snapshot_mode' internal/core/receipt internal/app/finalize internal/app/verify`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-25
- [x] `ac1_3` production caller exposes base_ref - MCP schema and installed skill mention the base-ref request field.
  - Command: `rg -n 'base_ref|BaseRef' internal/adapters/mcp/finalize internal/adapters/corebundle/assets/initwire/claude`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-26

## Rollback

- Remove `snapshot_mode` and `base_ref` request plumbing; verify returns to working-tree-only recomputation.

## Review

Status: not_started
Verdict: none

Findings:
- none

## Self Eval

- none

## Deviations

- none

## Metadata

- created_by: scafld

## Origin

Created by: scafld
Source: plan

## Harden Rounds

- none

## Planning Log

- Repaired placeholder spec against existing `git.SnapshotInput.BaseRef`.
