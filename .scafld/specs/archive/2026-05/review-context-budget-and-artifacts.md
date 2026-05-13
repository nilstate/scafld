---
spec_version: '2.0'
task_id: review-context-budget-and-artifacts
created: '2026-05-12T15:54:25Z'
updated: '2026-05-12T16:10:22Z'
status: completed
harden_status: not_run
size: medium
risk_level: medium
---

# Review context budget and artifact examples

## Current State

Status: completed
Current phase: final
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-05-12T16:10:22Z
Review gate: pass

## Summary

Make scafld's final 1% review ergonomics explicit and robust without weakening
the review gate. Review context should be cheaper because it is budgeted and
self-describing, not because the reviewer receives less truth. Multi-agent
workspace dirt should be classified clearly. `scafld config` should recognize
more real project surfaces. Docs should show concrete artifacts agents and
operators actually consume.

## Objectives

- Add an explicit review-context budget manifest that lists included,
  truncated, and omitted sections.
- Preserve strict ReviewDossier semantics and completion gates.
- Clarify review mode semantics and multi-agent drift classification in the
  provider context and docs.
- Extend config proposal detection for common project surfaces without applying
  config automatically.
- Add concrete artifact documentation for specs, status JSON, failed gates,
  review context, and review dossiers.

## Scope

- In: review context rendering and prompt wording.
- In: config scanner evidence and proposal suggestions.
- In: docs for review, configuration, and concrete artifacts.
- In: focused regression tests around context budgeting and detectors.
- Out: changing provider schemas, relaxing review validation, or adding
  compatibility aliases.
- Out: release publishing.

## Dependencies

- Existing ReviewDossier schema remains authoritative.
- Existing `review.context.max_bytes` remains the context body budget.
- Existing session baseline and ambient-drift classification remain the source
  of workspace truth.

## Assumptions

- Review cost should be controlled by deterministic packet budgeting and mode
  guidance, not by skipping the review gate.
- Omitted context is acceptable only when it is visible, sourced, and
  recoverable by the reviewer when needed.
- Config proposals should remain evidence-backed suggestions; the agent or
  operator applies only verified runtime config.

## Touchpoints

- `internal/core/reviewcontext`: deterministic Markdown rendering and budget
  manifest.
- `internal/app/review`: context packet sections and provider instructions.
- `internal/adapters/cli/config`: real-project detector coverage.
- `docs/review.md`, `docs/configuration.md`, `docs/artifacts.md`,
  `docs/sourcey.config.ts`: operator-facing documentation.

## Risks

- Over-budget context could accidentally hide important files if omissions are
  not explicit.
- Extra config detectors could overstate policy if they mutate config
  automatically.
- Docs examples could drift if they claim behavior not enforced by code.

## Acceptance

Profile: standard

Validation:
- [ ] `v1` focused-tests - Review context and config detector tests pass
  - Command: `go test ./internal/core/reviewcontext ./internal/app/review ./internal/adapters/cli/config ./internal/app/config`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` full-check - Full local gate passes
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` context-manifest - Context manifest and workspace classification are implemented and documented
  - Command: `rg 'Context Budget Manifest|Included sections|Truncated sections|Omitted sections|Workspace Classification' internal/core/reviewcontext/model.go internal/app/review/context.go docs/review.md docs/artifacts.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` artifact-docs - Artifact docs are linked from the docs nav
  - Command: `rg 'artifacts' docs/sourcey.config.ts && test -f docs/artifacts.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Implementation

Status: completed
Dependencies: none

Objective: Complete the requested change.

Changes:
- Add review context budget manifest rendering and tests.
- Add workspace classification and mode contract language to review context.
- Extend config scanner detectors for real-world package, API, and deployment
- Add concrete artifact docs and docs navigation.

Acceptance:
- [x] `ac1` command - Focused package tests pass
  - Command: `go test ./internal/core/reviewcontext ./internal/app/review ./internal/adapters/cli/config ./internal/app/config`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6
- [x] `ac2` command - Full local gate passes
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-7

## Rollback

- none

## Review

Status: completed
Verdict: pass
Mode: verify
Provider: codex
Output: codex.output_file
Summary: Read-only verification found no completion blockers in the review-context budget, workspace classification, config scanner, dossier gate, or documentation changes. Recorded acceptance evidence was treated as already executed.

Attack log:
- `workspace classification`: Reviewed scoped diff for task files and separated declared task scope from ambient drift. -> clean (Observed ambient drift in markdown adapter files only; not treated as a task finding.)
- `internal/core/reviewcontext`: Checked review context renderer budgeting behavior, manifest grouping, truncation markers, source listing, and omitted section handling. -> clean (Renderer accounts for section body bytes and makes truncated/omitted sections visible in the manifest.)
- `internal/app/review/context.go`: Checked provider context wording for review mode, read-only review, omitted context handling, and completion-blocking finding requirements. -> clean (Mode contract and provider instruction preserve the same completion gate for discover and verify.)
- `internal/core/review and review schema`: Checked review gate semantics against dossier validation and schema constraints. -> clean (No relaxation of verdict validation, blocking-finding requirements, or provider completion gate found.)
- `internal/adapters/cli/config`: Checked config scanner and suggestion additions for package scripts, Python/Ruby runners, lockfiles, API, container, and deployment surfaces. -> clean (New detectors are read-only and produce evidence-backed suggestions rather than applying config automatically.)
- `docs/review.md docs/configuration.md docs/artifacts.md`: Checked documentation updates for review context manifests, mode semantics, config detection surfaces, and concrete artifact examples. -> clean (Docs align with the implemented packet shape and operator workflow at review depth used here.)
- `acceptance evidence`: Reviewed recorded acceptance evidence without rerunning build/test commands, per read-only provider instruction. -> clean (Recorded evidence reports targeted Go tests and make check passed with exit code 0.)

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

- none
