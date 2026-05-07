---
spec_version: '2.0'
task_id: dogfood-lifecycle-gates
created: '2026-05-07T14:22:15Z'
updated: '2026-05-07T15:06:44Z'
status: completed
harden_status: passed
size: small
risk_level: medium
---

# Dogfood lifecycle gates

## Current State

Status: completed
Current phase: none
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-05-07T15:06:44Z
Review gate: pass

## Summary

Verify the current scafld changes through the real plan, harden, approve, build, review, and complete lifecycle.

## Objectives

- Verify the current scafld release-candidate changes with scafld's own lifecycle gates.
- Prove the build gate still runs the full repository validation command.
- Prove the review and completion gates accept only structured `codex`, `claude`, or `command` review evidence.

## Scope

- Report metrics and archive-aware reporting.
- Configure detectors and sparse config shape.
- Review baseline/scope behavior and blocked handoff surfaces.
- Documentation examples for spec, status, ReviewPacket, session, and report outputs.
- Dogfood-discovered lifecycle gate fixes in `internal/app/build`, `internal/app/complete`, `internal/app/handoff`, `internal/app/review`, `internal/core/review`, and `internal/core/workspace`.
- Runtime support for session evidence, git workspace snapshots, markdown projection, config loading, managed core updates, and CLI review/configure/help surfaces.

## Dependencies

- Local Go toolchain.
- Repository `make check` target.
- External review provider for deterministic gate exercise.

## Assumptions

- This dogfood run is a gate exercise for the current patch, not an additional production code feature.
- Codex review is the adversarial evidence for this dogfood run; command and Claude remain supported completion providers.
- `make check` remains the authoritative repository validation command.

## Touchpoints

- `internal/adapters/cli`
- `internal/adapters/cli/review`
- `internal/adapters/config`
- `internal/app/report`
- `internal/app/build`
- `internal/app/complete`
- `internal/app/handoff`
- `internal/app/harden`
- `internal/app/configure`
- `internal/adapters/cli/configure`
- `internal/adapters/corebundle`
- `internal/adapters/git`
- `internal/adapters/jsonstore`
- `internal/adapters/markdown`
- `internal/app/approve`
- `internal/app/review`
- `internal/core/review`
- `internal/core/session`
- `internal/core/workspace`
- `docs/**`
- `.scafld/config.yaml`
- `.scafld/core/config.yaml`
- `.scafld/core/prompts/review.md`
- `.scafld/prompts/review.md`

## Risks

- Risk: command-provider review can rubber-stamp quality. Mitigation: treat it only as gate-mechanics dogfood and keep `make check` as the build evidence.
- Risk: dogfood artifacts add repository noise. Mitigation: archive the spec through the lifecycle so the session/spec record is complete.
- Risk: sparse config removes visible defaults from project config. Mitigation: keep the full example shape in `.scafld/core/config.yaml` and document the split.

## Acceptance

Profile: standard

Validation:
- none

## Phase 1: Implementation

Status: completed
Dependencies: none

Objective: Complete the requested change.

Changes:
- Exercise plan, harden, approval, build, review, and complete with the current scafld binary.
- Record acceptance evidence through the normal session-first build path.
- Record a structured external ReviewPacket and archive through `complete`.

Acceptance:
- [x] `ac1` command - Primary validation command
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-46

## Rollback

- Remove `.scafld/specs/**/dogfood-lifecycle-gates.md` and `.scafld/runs/dogfood-lifecycle-gates/` if the dogfood artifact itself is not wanted in the release commit.

## Review

Status: completed
Verdict: pass

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
Started: 2026-05-07T14:22:19Z
Ended: 2026-05-07T14:22:55Z

Questions:
- What proves report metrics are derived from session evidence rather than static spec prose?
  - Grounded in: code:internal/app/report/report.go:51
  - Recommended answer: `report.Run` accepts a session store and summarizes session ledgers; keep `make check` as acceptance so report tests prove the metric path.
  - If unanswered: Do not claim session-derived metrics in docs.
  - Answered with: Accepted; report now reads session ledgers and the dogfood acceptance is `make check`.
- Where should the full config shape live if project config is sparse?
  - Grounded in: code:internal/adapters/corebundle/bundle.go:342
  - Recommended answer: Keep the full shape in `.scafld/core/config.yaml`; generate sparse committed config and local override stubs only.
  - If unanswered: Keep the old full project config to avoid hiding options.
  - Answered with: Accepted; sparse project/local config plus full managed core example.
- How does this patch avoid dirty-workspace review friction while still blocking real scope drift?
  - Grounded in: code:internal/app/review/review.go:81
  - Recommended answer: Approval/build baseline plus derived task scope should separate unchanged pre-existing dirt from new task changes and new out-of-scope drift.
  - If unanswered: Do not release the review changes.
  - Answered with: Accepted; review now derives scope from spec and compares against the session baseline.


## Planning Log

- none
