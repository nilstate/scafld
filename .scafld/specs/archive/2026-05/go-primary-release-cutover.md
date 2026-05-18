---
spec_version: '2.0'
task_id: go-primary-release-cutover
created: '2026-05-04T03:18:52Z'
updated: '2026-05-04T03:18:59Z'
status: completed
harden_status: not_run
size: medium
risk_level: medium
---

# Cut scafld release packaging to Go

## Current State

Status: completed
Current phase: none
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-05-04T03:18:59Z
Review gate: pass

## Summary

Make the existing nilstate/scafld repo the primary Go v2 module and release npm/PyPI wrappers over verified GitHub binaries.

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
  - Command: `make check && make release-snapshot`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-2

## Rollback

- none

## Review

Status: completed
Verdict: pass

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

- none
