---
spec_version: '2.0'
task_id: reviewer-read-jail-tests
created: '2026-06-03T13:58:14Z'
updated: '2026-06-04T09:04:43Z'
status: completed
harden_status: not_run
size: medium
risk_level: high
---

# Behaviorally test the reviewer read-jail

## Current State

Status: completed
Current phase: final
Next: done
Reason: finalization receipt passed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-06-04T09:03:09Z
Review gate: not_started

## Summary

Split reviewer read-jail proof into a hermetic test of the sandbox CWD, HOME, exact env, and evidence materialization scafld owns, plus an explicitly skipped credentialed smoke path for real provider CLIs.

## Objectives

- Prove the receipt-grade reviewer runs from a scafld-controlled evidence sandbox, not the live repo root.
- Prove `HOME` and `XDG_CONFIG_HOME` are sandbox-owned and host memory is not autoloaded.
- Prove only evidence files are materialized into the read root and outside paths are not present.
- Keep real provider CLI behavior as an opt-in smoke test, not a CI dependency.

## Scope

- In scope: tests under `internal/adapters/providers` and test helpers/fakes.
- Out of scope: OS-level sandboxing claims scafld cannot enforce portably.

## Dependencies

- `evidence-control-sandbox`: provides env scrub and evidence sandboxing.
- `context-isolated-reviewer`: defines receipt-grade provider selection.

## Assumptions

- The hermetic fake reviewer can prove scafld-owned process inputs, but it cannot prove a closed-source provider never opens arbitrary files after launch.

## Touchpoints

- internal/adapters/providers/evidence_sandbox.go
- internal/adapters/providers/env_scrub.go
- internal/adapters/providers/provider.go
- internal/adapters/providers/evidence_sandbox_test.go
- internal/adapters/providers/provider_test.go

## Risks

- Tests could overclaim true OS confinement.
  - Mitigation: name the test read-jail as scafld-controlled process inputs and keep real provider smoke optional.

## Acceptance

Profile: strict

Validation:
- [x] `v1` validate spec - This spec validates clean.
  - Command: `go run ./cmd/scafld validate reviewer-read-jail-tests`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-12

## Phase 1: hermetic read-jail tests

Status: pass
Dependencies: none

Objective: Add CI-safe tests proving sandbox CWD, HOME, env mode, evidence paths, and cleanup behavior for receipt-grade review.

Changes:
- internal/adapters/providers/evidence_sandbox_test.go - assert sandbox CWD/read roots/provenance, blocklisted instruction files, path traversal rejection, and cleanup.
- internal/adapters/providers/provider_test.go - ensure selected receipt-grade providers carry read roots, exact env, sandbox HOME/XDG overrides, and memory-autoload-disabled facts.

Acceptance:
- [x] `ac1_2` no OS confinement overclaim - spec and tests frame the guarantee as scafld-controlled process inputs, not provider-proof OS isolation.
  - Command: `rg -n 'OS-level sandboxing claims|scafld-controlled process inputs|MemoryAutoloadDisabled' .scafld/specs/active/reviewer-read-jail-tests.md internal/adapters/providers`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-13

## Rollback

- Remove the added tests. No runtime code migration is required unless the tests expose a real gap.

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

- Repaired placeholder spec against existing provider sandbox code.
