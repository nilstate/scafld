---
spec_version: '2.0'
task_id: demote-lifecycle-install-footprint
created: '2026-06-03T13:58:14Z'
updated: '2026-06-04T08:22:31Z'
status: completed
harden_status: not_run
size: medium
risk_level: medium
---

# Demote the lifecycle install footprint to opt-in

## Current State

Status: completed
Current phase: final
Next: done
Reason: finalization receipt passed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-06-04T08:21:03Z
Review gate: not_started

## Summary

Stop teaching new workspaces that the plan-to-complete lifecycle is the default agent path. Install the finalize affordance by default, keep lifecycle docs/scripts available as operator material, and make any lifecycle helper install explicitly opt-in.

## Objectives

- Keep `finalize` as the only default MCP tool installed by `scafld init`.
- Stop default-installing lifecycle provider helper scripts when they are not needed for the finalize path.
- Keep the lifecycle helper-script list in one production source of truth so install tests cannot drift from filtering behavior.
- Add a default-footprint regression proving fresh init writes finalize-only MCP config without harden/review submit servers.
- Rewrite managed runtime docs so lifecycle is an operator path, not the agent's primary path.
- Preserve `scafld update` or an explicit option as the path for managed lifecycle helper assets if still needed.

## Scope

- In scope: corebundle install filtering, managed docs, init/update behavior, and tests.
- Out of scope: deleting lifecycle CLI commands or breaking existing workspaces that already have helper scripts.

## Dependencies

- `one-command-init-wiring`: installs the finalize affordance.
- `gate-without-spec`: makes the finalize path less dependent on lifecycle state.

## Assumptions

- Existing checked-in docs can still mention lifecycle for human operators.
- Default install should optimize for the host-facing finalize, not adapter scripts.

## Touchpoints

- internal/adapters/corebundle/bundle.go
- internal/adapters/corebundle/assets/core/README.md
- internal/adapters/corebundle/assets/core/OPERATORS.md
- internal/adapters/corebundle/assets/core/scripts/
- internal/adapters/corebundle/bundle_test.go
- internal/adapters/corebundle/agent_docs_test.go

## Risks

- Removing default scripts could surprise users relying on fresh `init` to create them.
  - Mitigation: keep update/opt-in path and document the lifecycle helper path as optional.

## Acceptance

Profile: strict

Validation:
- [x] `v1` validate spec - This spec validates clean.
  - Command: `go run ./cmd/scafld validate demote-lifecycle-install-footprint`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-13

## Phase 1: default install footprint

Status: pass
Dependencies: gate-without-spec

Objective: Make default `scafld init` install finalize affordances without lifecycle helper scripts, while preserving an explicit managed path for operator lifecycle material.

Changes:
- internal/adapters/corebundle/bundle.go - skip lifecycle helper scripts during default init and own the helper-script path list in one package-level source.
- internal/adapters/corebundle/bundle_test.go - derive lifecycle helper expectations from the production list and assert default init does not create them.
- internal/adapters/corebundle/initwire_test.go - assert fresh init creates finalize-only `.mcp.json` without harden/review submit servers.
- internal/adapters/corebundle/assets/core/README.md - frame lifecycle helpers as optional operator utilities.
- internal/adapters/corebundle/assets/core/OPERATORS.md - lead with gate/verify and demote lifecycle to human/CI.

Acceptance:
- [x] `ac1_1` corebundle install tests pass - default install footprint is covered.
  - Command: `go test ./internal/adapters/corebundle -run 'Init|Install|Lifecycle|AgentDocs|Gitignore'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-14
- [x] `ac1_2` initwire only installs finalize MCP - lifecycle submit servers are not default MCP tools.
  - Command: `rg -n 'review-submit-stdio|harden-submit-stdio|submit_review|submit_harden' internal/adapters/corebundle/assets/initwire`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-15
- [x] `ac1_3` docs lead with finalize and verify - managed docs explain lifecycle as optional/operator.
  - Command: `rg -n 'gate|scafld verify|optional.*lifecycle|operator' internal/adapters/corebundle/assets/core/README.md internal/adapters/corebundle/assets/core/OPERATORS.md`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-16

## Rollback

- Restore default installation of all core scripts and prior lifecycle-first docs.

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

- Repaired placeholder spec against corebundle assets and init wiring.
