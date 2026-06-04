---
spec_version: '2.0'
task_id: canonical-receipt-conformance-corpus
created: '2026-06-03T13:58:14Z'
updated: '2026-06-04T09:03:09Z'
status: completed
harden_status: not_run
size: small
risk_level: medium
---

# Pin canonical receipt bytes with a golden conformance corpus

## Current State

Status: completed
Current phase: final
Next: done
Reason: finalization receipt passed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-06-04T09:02:15Z
Review gate: not_started

## Summary

Add a golden conformance corpus pinning `CanonicalJSON`, signed receipt body bytes, and `ReceiptDigest` behavior, including the intentional `ledger_head`-excluded digest asymmetry. Wire-format drift must become a mechanical unit-test failure.

## Objectives

- Pin the canonical byte string for a representative receipt body, including `acceptance_declared` and independence downgrade fields.
- Pin the digest for the same body with `ledger_head` cleared.
- Document in the fixture why signatures cover `ledger_head` but receipt digests exclude it.
- Preserve the receipt partition: `reviewed_context_provenance` and `ignored_unreviewed` are verified through review coverage; `spec_fingerprint` remains attested-only in schema v1.
- Keep the corpus local to `internal/core/receipt` so no adapter owns receipt canonicalization.

## Scope

- In scope: receipt fixture files under `internal/core/receipt/testdata/`.
- In scope: receipt package tests that load the fixture and compare exact canonical bytes and digest.
- Out of scope: removing attested-only fields or changing the signature algorithm or ledger-head formula.

## Dependencies

- `gate-without-spec`: adds `acceptance_declared`, so this corpus must be regenerated after that field lands.
- `independence-honest-surfacing`: adds `independence.downgraded`, so this corpus must pin the completed independence payload.

## Assumptions

- `ledger_head` remains part of the signed canonical body.
- `ReceiptDigest` keeps clearing `ledger_head` before hashing so the session hash chain is not recursively self-referential.

## Touchpoints

- internal/core/receipt/receipt.go
- internal/core/receipt/receipt_test.go
- internal/core/receipt/testdata/receipt_body.json (new)
- internal/core/receipt/testdata/receipt_body.canonical.json (new)
- internal/core/receipt/testdata/receipt_body.digest (new)

## Risks

- Fixture churn could hide intentional schema changes.
  - Mitigation: tests should fail with a small diff and fixture comments should explain when updating is valid.

## Acceptance

Profile: strict

Validation:
- [x] `v1` validate spec - This spec validates clean.
  - Command: `go run ./cmd/scafld validate canonical-receipt-conformance-corpus`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-13

## Phase 1: golden receipt corpus

Status: pass
Dependencies: none

Objective: Add fixture-backed tests that compare exact canonical receipt bytes and digest output.

Changes:
- internal/core/receipt/testdata/receipt_body.json - representative receipt body fixture with acceptance declaration and downgrade enum.
- internal/core/receipt/testdata/receipt_body.canonical.json - exact canonical JSON bytes expected from `CanonicalBody`.
- internal/core/receipt/testdata/receipt_body.digest - expected `ReceiptDigest` hex string.
- internal/core/receipt/receipt_test.go - conformance tests that load all three fixtures and explain the ledger-head asymmetry.

Acceptance:
- [x] `ac1_1` conformance tests pass - exact canonical bytes and digest are pinned.
  - Command: `rg -n 'ledger_head.*(exclude|cleared|digest)|digest.*ledger_head' internal/core/receipt/receipt_test.go internal/core/receipt/testdata`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-14

## Rollback

- Delete the conformance fixtures and tests. No runtime migration is required.

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

- Repaired placeholder spec against `internal/core/receipt/receipt.go`.
