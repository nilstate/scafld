---
spec_version: '2.0'
task_id: provider-backed-hardening
created: '2026-05-17T12:58:20Z'
updated: '2026-05-17T12:59:40Z'
status: completed
harden_status: passed
size: medium
risk_level: medium
---

# Provider-backed hardening

## Current State

Status: completed
Current phase: final
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-05-17T12:59:40Z
Review gate: pass

## Summary

Delegate hardening to a separate structured-output provider and record HardenDossier evidence in the draft spec.

## Objectives

- none

## Scope

- none

## Dependencies

- none

## Assumptions

- none

## Touchpoints

- none

## Risks

- none

## Acceptance

Profile: standard

Validation:
- none

## Phase 1: Implementation

Status: completed
Dependencies: none

Objective: Complete the requested change.

Changes:
- Implement the requested behavior.

Acceptance:
- [x] `ac1` command - Primary validation command
  - Command: `go test ./internal/core/harden ./internal/adapters/providers ./internal/app/harden ./internal/adapters/cli ./internal/adapters/mcp/...`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6

## Rollback

- none

## Review

Status: completed
Verdict: pass
Mode: discover
Provider: command
Output: command.stdout
Summary: Dogfood review verified provider-backed harden smoke path and focused tests.

Attack log:
- `provider-backed harden flow`: run focused acceptance tests and inspect spec projection -> clean

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

### round-1

Status: passed
Started: 2026-05-17T12:58:25Z
Ended: 2026-05-17T12:58:25Z
Verdict: pass
Provider: local
Output format: local.fixture
Summary: Local provider smoke hardening passed.

Checks:
- path audit
  - Grounded in: spec_gap:Context
  - Result: passed
  - Evidence: Local smoke provider records required harden checks.
- command audit
  - Grounded in: spec_gap:Acceptance
  - Result: passed
  - Evidence: Local smoke provider records required harden checks.
- scope/migration audit
  - Grounded in: spec_gap:Scope
  - Result: passed
  - Evidence: Local smoke provider records required harden checks.
- acceptance timing audit
  - Grounded in: spec_gap:Acceptance
  - Result: passed
  - Evidence: Local smoke provider records required harden checks.
- rollback/repair audit
  - Grounded in: spec_gap:Rollback
  - Result: passed
  - Evidence: Local smoke provider records required harden checks.
- design challenge
  - Grounded in: spec_gap:Summary
  - Result: passed
  - Evidence: Local smoke provider records required harden checks.

Questions:
- none


## Planning Log

- none
