---
spec_version: '2.0'
task_id: independence-honest-surfacing
created: '2026-06-03T13:58:14Z'
updated: '2026-06-04T00:00:00Z'
status: review
harden_status: not_run
size: small
risk_level: medium
---

# Surface review independence honestly as multi-model single-party

## Current State

Status: review
Current phase: final
Next: review
Reason: exit code was 0
Blockers: none
Allowed follow-up command: `scafld review independence-honest-surfacing`
Latest runner update: 2026-06-04T09:02:14Z
Review gate: not_started

## Summary

Make same-vendor or undetected-host review loud and machine-readable in finalize output and receipts. Keep `cross_vendor` as correlated-blind-spot resistance, not true party independence, and leave `verify.min_independence` as the policy knob that decides whether isolation-only receipts can merge.

## Objectives

- Add a signed independence reason/warning field so receipts explain why `isolation_only` or `cross_vendor` was assigned.
- Add a signed machine-readable downgrade enum for same-vendor, unknown-host, and unknown-reviewer cases.
- Include the same independence payload in both passing receipts and failing `finalize` JSON responses.
- Update operator-facing wording to avoid implying legal or organizational independence.
- Preserve verify's recomputation of independence from reviewer and host vendors, including the downgrade enum.

## Scope

- In scope: `receipt.Independence` fields, finalize response serialization, verify tests, and docs/assets wording.
- Out of scope: hosted review, third-party signing, or changing provider selection order.

## Dependencies

- `host-gate-and-receipt`: defines signed receipts and finalize output.
- `ci-verify-merge-gate`: defines `verify.min_independence` behavior.

## Assumptions

- Unknown host detection must not be treated as distinct from any reviewer.
- `cross_vendor` means different model/vendor family, not independent legal party.

## Touchpoints

- internal/core/receipt/receipt.go
- internal/app/finalize/finalize.go
- internal/adapters/cli/finalize/run.go
- internal/app/verify/verify.go
- internal/adapters/providers/provider.go
- internal/adapters/corebundle/assets/agentdocs/AGENTS.md
- internal/adapters/corebundle/assets/agentdocs/CLAUDE.md

## Risks

- Adding signed fields can break old receipts.
  - Mitigation: keep the change in schema version 1 only if the field is optional in decoding but always populated by new gates.

## Acceptance

Profile: strict

Validation:
- [x] `v1` validate spec - This spec validates clean.
  - Command: `go run ./cmd/scafld validate independence-honest-surfacing`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-17

## Phase 1: signed and returned independence reason

Status: pass
Dependencies: none

Objective: Stamp an explicit reason/warning for the computed independence level into the receipt and finalize response without trusting host self-reporting.

Changes:
- internal/core/receipt/receipt.go - add `reason` plus `downgraded` constants to `Independence`.
- internal/app/finalize/finalize.go - complete the independence payload from the selected/reviewed provider and host relationship, including pre-review failure branches.
- internal/adapters/cli/finalize/run.go - include the independence payload in both receipt and no-receipt JSON responses.
- internal/app/verify/verify.go - keep recomputation authoritative and reject contradictory levels, distinct flags, or downgrade enums.
- internal/adapters/corebundle/agent_docs_test.go - guard against wording that overclaims legal or organizational independence.
- tests - cover same-vendor, unknown-host, unknown-reviewer, forged downgrade, and cross-vendor cases.

Acceptance:
- [x] `ac1_1` independence tests pass - same-vendor, unknown-host, and cross-vendor behavior is covered.
  - Command: `go test ./internal/app/finalize ./internal/app/verify ./internal/adapters/cli/finalize ./internal/adapters/providers -run 'Independence|SameVendor|UnknownHost|CrossVendor|GateResponse'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-18
- [x] `ac1_2` wording is honest - installed docs avoid overclaiming party independence.
  - Command: `rg -n 'multi-model|correlated|single-party|cross_vendor' internal/adapters/corebundle/assets/agentdocs docs README.md`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-19
- [x] `ac1_3` failed finalize responses surface independence - acceptance-failure responses carry the same independence payload.
  - Command: `go test ./internal/app/finalize ./internal/adapters/cli/finalize -run 'FailsFastOnAcceptance|FailedAcceptanceCarriesCriterionDetails'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-20
- [x] `ac1_4` overclaim guard is executable - docs fail tests if banned independence wording appears.
  - Command: `go test ./internal/adapters/corebundle -run 'OverclaimIndependence'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-21

## Rollback

- Remove the optional reason/warning field and revert docs. Existing receipts remain decodable if the field is optional.

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

- Repaired placeholder spec against verify's existing independence recomputation.
