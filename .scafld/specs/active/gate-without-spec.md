---
spec_version: '2.0'
task_id: gate-without-spec
created: '2026-06-03T13:58:14Z'
updated: '2026-06-04T00:00:00Z'
status: review
harden_status: not_run
size: medium
risk_level: high
---

# Run the accountability finalize without a hand-authored spec

## Current State

Status: review
Current phase: final
Next: review
Reason: exit code was 0
Blockers: none
Allowed follow-up command: `scafld review gate-without-spec`
Latest runner update: 2026-06-04T09:00:44Z
Review gate: not_started

## Summary

Make the finalize compose path work when no Markdown spec exists by synthesizing scope and acceptance from explicit finalize input and the base diff. Empty scope remains a hard refusal so the finalize never signs an unbounded or meaningless receipt.

## Objectives

- Make `specStore.Load` optional in `scafld finalize`.
- Derive task metadata from the request when a spec is absent.
- Derive scope from `scope_hint` or base-diff paths, and refuse empty scope.
- Remove the git adapter whole-repo fallback for empty scope; callers must pass `.` when whole-repo scope is intentional.
- Add `acceptance_declared` to the signed receipt body so no-spec/empty-acceptance receipts state whether criteria existed rather than leaving that implicit.
- Persist the receipt/session under the requested task id even without a spec file.

## Scope

- In scope: CLI finalize request model, scope derivation, fallback task model, session/receipt path behavior, and tests.
- Out of scope: `plan`, `approve`, `build`, and lifecycle completion without specs.

## Dependencies

- `headline-path-executes`: task receipt selection and default gate/verify path must exist before no-spec finalize is useful as an agent headline flow.

## Assumptions

- A no-spec finalize request still includes a stable `task_id`.
- A no-spec request may include explicit acceptance criteria later, but this task can start with empty acceptance only if scope is non-empty.

## Touchpoints

- internal/adapters/cli/finalize/run.go
- internal/adapters/git/git.go
- internal/adapters/jsonstore/session_store.go
- internal/app/finalize/finalize.go
- internal/adapters/cli/finalize/run_test.go

## Risks

- A no-spec path could silently review too much or too little.
  - Mitigation: empty scope refuses, and response names the derived scope.

## Acceptance

Profile: strict

Validation:
- [x] `v1` validate spec - This spec validates clean.
  - Command: `go run ./cmd/scafld validate gate-without-spec`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-19

## Phase 1: no-spec finalize compose path

Status: pass
Dependencies: headline-path-executes

Objective: Let finalize requests without a matching Markdown spec synthesize the minimal app input from request and git state.

Changes:
- internal/adapters/cli/finalize/run.go - tolerate missing spec, synthesize title/task metadata, derive scope, and reject empty scope with a structured error.
- internal/adapters/git/git.go - expose base-diff changed paths if needed by finalize scope derivation and reject empty `SnapshotInput.Scope` instead of falling back to whole-repo scope.
- internal/core/receipt/receipt.go - add `acceptance_declared` to the signed body.
- internal/adapters/cli/finalize/run_test.go - cover explicit scope, base-diff scope, missing spec, and empty-scope refusal.

Acceptance:
- [x] `ac1_2` spec load is optional only in finalize - lifecycle code still uses specs normally.
  - Command: `rg -n 'missing spec|NoSpec|specStore.Load|Load\(' internal/adapters/cli/finalize internal/app/finalize`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-20
- [x] `ac1_3` empty scope refuses - git snapshot tests cover fail-closed empty scope and explicit `.` whole-repo scope.
  - Command: `go test ./internal/adapters/git -run 'SnapshotRejectsEmptyScope|SnapshotStableTreeSHA'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-21
- [x] `ac1_4` acceptance declaration is signed - receipt corpus includes `acceptance_declared`.
  - Command: `rg -n 'AcceptanceDeclared|acceptance_declared' internal/core/receipt internal/app/finalize`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-22

## Rollback

- Restore mandatory spec loading in `scafld finalize`; receipts remain only task-spec backed.

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

- Repaired placeholder spec against `internal/adapters/cli/finalize/run.go`.
