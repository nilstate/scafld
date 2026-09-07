---
spec_version: '2.0'
task_id: headline-path-executes
created: '2026-06-03T13:58:14Z'
updated: '2026-06-04T07:32:51Z'
status: completed
harden_status: not_run
size: large
risk_level: high
---

# Make the finalize, receipt, and verify path execute by default

## Current State

Status: completed
Current phase: final
Next: done
Reason: finalization receipt passed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-06-04T07:30:35Z
Review gate: not_started

## Summary

Wire the installed headline path so `scafld init`, `finalize`, signed receipt persistence, and `scafld verify` execute end to end against the same authoritative task receipt. The finalize already writes both `.scafld/receipts/<task>.json` and `latest.json`; `latest.json` remains a local convenience copy, while CI and merge surfaces must resolve the task receipt or an explicitly supplied receipt path.

## Objectives

- Make the installed verification surfaces resolve the task receipt that the finalize writes, not treat `latest.json` as authoritative.
- Make default init produce the trust anchor, finalize MCP, Stop hook, and verify workflow without lifecycle-first confusion.
- Add an end-to-end test that initializes a workspace, runs a finalize with a hermetic fake reviewer, writes a receipt, and verifies that same receipt with an explicit trusted-key path.
- Make the installed workflow use a protected verify wrapper/trust anchor and a commit-visible task receipt, while keeping `latest.json` as local convenience.
- Keep the path executable without a hand-authored spec when the no-spec finalize mode is used.

## Scope

- In scope: end-to-end tests, initwire defaults, receipt selection scripts/actions, and verify workflow defaults.
- Out of scope: real hosted provider execution in CI. The e2e test may use a scafld-controlled fake reviewer binary.

## Dependencies

- none

## Assumptions

- The final integration test can use a fake receipt-grade reviewer binary because real provider credentials are operator-owned.
- The fake reviewer must still exercise the same CLI finalize path, signer, receipt, ledger, and verify command.

## Touchpoints

- internal/adapters/cli/finalize/run.go
- internal/adapters/cli/verify/verify.go
- internal/adapters/corebundle/initwire.go
- internal/adapters/corebundle/assets/initwire/ci/scafld-verify.yml
- test/e2e/

## Risks

- An overly fake e2e could pass without proving the real path.
  - Mitigation: fake only the external reviewer process; use real init, finalize compose, signing, receipt write, trust keys, and verify.

## Acceptance

Profile: strict

Validation:
- [x] `v1` validate spec - This spec validates clean.
  - Command: `go run ./cmd/scafld validate headline-path-executes`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-47

## Phase 1: end-to-end finalize receipt verify proof

Status: pass
Dependencies: none

Objective: Add and pass a real end-to-end test that runs the installed headline path through init, finalize, receipt persistence, and verify.

Changes:
- test/e2e/finalize_receipt_verify_test.go - new e2e proving default init plus fake receipt-grade reviewer can produce and verify the same receipt.
- internal/adapters/corebundle/assets/initwire/ci/scafld-verify.yml - ensure workflow verifies a committed receipt path with protected trust.
- internal/adapters/corebundle/assets/initwire/scripts/scafld-verify.sh - wrapper for protected trust, receipt selection, and pinned scafld install.
- scripts/scafld-verify.sh - choose a single changed task receipt in CI and keep `latest.json` only as non-CI fallback.
- .github/actions/scafld-verify/action.yml - verify the same explicit or task-resolved receipt path.

Acceptance:
- [x] `ac1_1` headline e2e passes - init, finalize, receipt, and verify execute against one artifact.
  - Command: `sh -c "rg -n 'func TestFinalizeMintsReceiptVerifyAcceptsAndRejectsTamper|func TestHeadlinePathExecutesFinalizeReceiptVerify' test/e2e/finalize_receipt_verify_test.go test/e2e/headline_path_test.go && go test ./test/e2e -run 'FinalizeMintsReceiptVerify|HeadlinePathExecutesFinalizeReceiptVerify'"`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-48
- [x] `ac1_2` full suite passes - final integration did not regress existing behavior.
  - Command: `go test ./...`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-49
- [x] `ac1_3` workflow points at finalize receipt path - installed CI verifies the path finalize writes by default.
  - Command: `sh -c "rg -n 'SCAFLD_RECEIPT_PATH' internal/adapters/cli/verify/verify.go internal/adapters/corebundle/assets/initwire/ci/scafld-verify.yml scripts .github/actions && rg -n 'scafld verify .*\\\\.scafld/receipts/|SCAFLD_RECEIPT_PATH' internal/adapters/corebundle/assets/initwire/ci/scafld-verify.yml scripts .github/actions"`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-50

## Rollback

- Remove the e2e and restore prior initwire verify defaults. Smaller mechanics can remain if their own specs pass.

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

- Repaired placeholder spec as the final umbrella over the six concrete mechanics.
