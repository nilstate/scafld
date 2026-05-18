---
spec_version: '2.0'
task_id: provider-backed-harden-real-provider
created: '2026-05-17T13:22:56Z'
updated: '2026-05-17T13:30:27Z'
status: cancelled
harden_status: in_progress
size: small
risk_level: low
---

# Provider-backed harden real provider smoke

## Current State

Status: cancelled
Current phase: none
Next: done
Reason: dogfood exposed provider failure round closure bug; replaced by follow-up smoke after fix
Blockers: provider hardening not yet recorded
Allowed follow-up command: `scafld harden provider-backed-harden-real-provider --provider <provider>`
Latest runner update: none
Review gate: not_started

## Summary

Verify scafld harden can delegate to a real external provider and record a strict HardenDossier.

## Objectives

- Verify that provider-backed harden can invoke a real external provider, receive
  one strict `HardenDossier`, and project that result into the draft spec.
- Confirm the provider path uses the same required harden checks as manual
  hardening: path audit, command audit, scope/migration audit, acceptance timing
  audit, rollback/repair audit, and design challenge.
- Keep the dogfood task non-mutating so it validates the already implemented
  feature without introducing a second feature change.

## Scope

- In: run real external-provider harden against this draft.
- In: inspect the resulting harden round for provider provenance, verdict,
  required checks, attack log, and any objections/questions.
- In: run targeted tests for provider transport, harden app behavior, MCP submit
  wrappers, and the generic MCP submit server.
- Out: change provider-backed harden implementation unless the provider exposes a
  real defect.
- Out: release, package manager updates, or unrelated documentation changes.

## Dependencies

- Provider CLI availability on the local machine.
- Existing scafld provider configuration and environment credentials.
- Go test toolchain for focused package validation.

## Assumptions

- This dogfood task should not edit production source files unless the harden
  provider finds a real issue in the implementation.
- A `needs_revision` harden verdict is useful evidence, not a failure of the
  dogfood itself, if the objections are grounded and actionable.
- Local/command provider dogfood already passed; this task exists specifically
  to prove the real provider channel.

## Touchpoints

- `.scafld/specs/drafts/provider-backed-harden-real-provider.md`
- `internal/adapters/providers/provider.go`
- `internal/app/harden/harden.go`
- `internal/app/harden/context.go`
- `internal/adapters/mcp/hardensubmit/server.go`
- `internal/platform/mcpsubmit/server.go`
- `internal/core/harden/`

## Risks

- Provider produces a valid critique with `needs_revision`; mitigate by treating
  the critique as design feedback and only changing implementation if the
  finding is real.
- Provider invocation fails because credentials or CLI state are unavailable;
  mitigate by recording the failure and leaving local/command dogfood evidence
  intact.
- Dogfood becomes self-referential ceremony; mitigate by keeping this task
  non-mutating unless the provider identifies a concrete defect.
- Provider cost or latency is non-zero; mitigate with one narrow draft and one
  focused validation command.

## Acceptance

Profile: standard

Validation:
- [ ] `provider-harden-tests` command - Provider-backed harden packages pass focused tests
  - Command: `go test ./internal/adapters/providers ./internal/app/harden ./internal/adapters/mcp/... ./internal/platform/mcpsubmit`
  - Expected kind: `exit_code_zero`
  - Status: pending

## Phase 1: Provider Dogfood Validation

Status: pending
Dependencies: none

Objective: Validate the real external-provider harden path without broadening the implementation.

Changes:
- Run provider-backed harden against this draft with a real external provider.
- Inspect the recorded harden round for provider provenance, verdict, required
  checks, questions, design objections, recommended edits, and attack log.
- Run the focused Go test command for provider transport, harden app behavior,
  MCP submit wrappers, and the generic MCP submit server.
- Do not edit production source unless the provider identifies a grounded defect
  in the implementation.

Acceptance:
- [ ] `ac1` command - Primary validation command
  - Command: `go test ./internal/adapters/providers ./internal/app/harden ./internal/adapters/mcp/... ./internal/platform/mcpsubmit`
  - Expected kind: `exit_code_zero`
  - Status: pending

## Rollback

- Archive or cancel this dogfood spec if external provider credentials are not
  available.
- If provider invocation fails after scafld has opened an in-progress harden
  round, preserve that round as evidence, fix the CLI/credential/output issue,
  and rerun `scafld harden provider-backed-harden-real-provider --provider
  <provider>` to create a new round.
- Revert only changes made for this dogfood task; do not revert the
  provider-backed harden implementation that has already passed local/command
  validation.

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

### round-1

Status: needs_revision
Started: 2026-05-17T13:23:24Z
Ended: 2026-05-17T13:23:24Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: The provider-backed harden implementation surfaces needed by this dogfood are present, including required check names, strict dossier validation, submit_harden wrapper, and round projection. The draft should not pass hardening yet because the phase wording contradicts the non-mutating scope, rollback omits partial provider-round repair, top-level validation says none despite a phase test command, and the acceptance command could not be executed in this read-only environment.

Checks:
- path audit
  - Grounded in: code:.scafld/specs/drafts/provider-backed-harden-real-provider.md:65
  - Result: passed
  - Evidence: The draft spec exists and declares touchpoints at lines 65-73. Each named path was found by `rg --files`: `internal/adapters/providers/provider.go`, `internal/app/harden/harden.go`, `internal/app/harden/context.go`, `internal/adapters/mcp/hardensubmit/server.go`, `internal/platform/mcpsubmit/server.go`, and `internal/core/harden/`.
- command audit
  - Grounded in: code:.scafld/specs/drafts/provider-backed-harden-real-provider.md:105
  - Result: failed
  - Evidence: The acceptance command is present at lines 105-109, but invoking it from `/Users/kam/dev/0state/scafld` failed before tests ran: `go: creating work dir: mkdir /var/folders/.../T/go-build...: operation not permitted`. In this read-only session, that prevents verifying exit_code_zero evidence.
- scope/migration audit
  - Grounded in: code:internal/app/harden/harden.go:97
  - Result: passed
  - Evidence: The scope says this task is non-mutating except for a real defect, while the touched code path is already present: `harden.Run` delegates to `runProviderHarden` when `input.Provider != nil`, validates a `coreharden.Dossier`, and projects it into the latest round. No migration or public schema change is declared or needed for this dogfood task.
- acceptance timing audit
  - Grounded in: code:.scafld/specs/drafts/provider-backed-harden-real-provider.md:95
  - Result: failed
  - Evidence: The task is described as a non-mutating smoke of already implemented provider-backed harden at lines 36-37 and 41-45, but Phase 1 still says “Implementation” and “Implement the requested behavior” at lines 95-103. Acceptance can be evaluated after provider harden runs, but the phase wording is misleading and could authorize unnecessary source edits.
- rollback/repair audit
  - Grounded in: code:internal/app/harden/harden.go:103
  - Result: failed
  - Evidence: Rollback covers unavailable provider credentials and avoiding rollback of prior implementation at lines 111-117. It does not state how to repair a partially recorded provider harden round or a `needs_revision` dossier beyond generic archive/cancel, even though `runProviderHarden` saves an in-progress round before provider invocation at lines 103-121.
- design challenge
  - Grounded in: code:internal/core/harden/model.go:25
  - Result: failed
  - Evidence: The reason for the task is legitimate: lines 31-35 specifically test real-provider strict HardenDossier projection and required checks, while code requires required check names and derives `needs_revision` from failed checks/questions/objections/edits. The design risk is not the feature itself, but the draft’s generic implementation phase and weak repair instructions.

Questions:
- Should Phase 1 be rewritten as a validation-only dogfood phase rather than an implementation phase?
  - Grounded in: code:.scafld/specs/drafts/provider-backed-harden-real-provider.md:95
  - Recommended answer: Yes. Rename it to provider dogfood validation and list the concrete actions: run real-provider harden, inspect provider provenance/checks/attack log/verdict in the recorded round, and run the focused Go tests.
  - If unanswered: Treat Phase 1 as validation-only and do not edit production source unless a grounded provider finding identifies an actual implementation defect.
- What is the intended repair path if the provider fails after scafld has already opened and saved an in-progress harden round?
  - Grounded in: code:internal/app/harden/harden.go:116
  - Recommended answer: Document that the operator should preserve the failed round as evidence, fix CLI/credential/output issues, rerun provider-backed harden for a new round, or archive/cancel this dogfood spec if a real external provider is unavailable.
  - If unanswered: Assume failed provider invocation leaves evidence in the current dogfood task and the operator may rerun provider harden or archive/cancel the spec once provider availability is resolved.

Design objections:
- `objection-1` medium - The phase invites implementation work even though the task is supposed to be a non-mutating real-provider smoke.
  - Grounded in: code:.scafld/specs/drafts/provider-backed-harden-real-provider.md:95
  - Evidence: The draft says the dogfood is non-mutating and validates an already implemented feature, but the only phase is named `Implementation` and says `Implement the requested behavior`. That creates a contract conflict for the build step.
  - Recommendation: Rename Phase 1 to a dogfood/validation phase and replace `Implement the requested behavior` with concrete non-mutating actions: run provider-backed harden, inspect the recorded round, and run the focused test command.
- `objection-2` medium - Rollback does not cover the partial harden-round state created before provider invocation.
  - Grounded in: code:internal/app/harden/harden.go:116
  - Evidence: `runProviderHarden` saves an in-progress harden round before invoking the provider, then updates it only after provider return and dossier validation. If provider invocation fails or produces invalid output, the draft’s rollback only says archive/cancel for unavailable credentials, not how to leave or repair the ledger state.
  - Recommendation: Add a rollback/repair bullet saying failed provider invocation should be recorded as dogfood evidence, then rerun `scafld harden <task-id> --provider <provider>` after credentials/CLI are fixed or archive/cancel the dogfood spec if the external provider cannot be made available.

Recommended edits:
- Phase 1
  - Grounded in: code:.scafld/specs/drafts/provider-backed-harden-real-provider.md:95
  - Recommendation: Change `## Phase 1: Implementation`, `Objective: Complete the requested change`, and `Implement the requested behavior` to validation-specific wording aligned with the declared non-mutating scope.
- Acceptance
  - Grounded in: code:.scafld/specs/drafts/provider-backed-harden-real-provider.md:88
  - Recommendation: Move or mirror the focused Go test command into top-level Acceptance Validation instead of leaving `Validation: none`, so the task contract has a clear post-build validation surface.
- Rollback
  - Grounded in: code:.scafld/specs/drafts/provider-backed-harden-real-provider.md:111
  - Recommendation: Add a repair step for provider invocation failure after an in-progress round is saved: preserve evidence, rerun after fixing provider availability/output, or archive/cancel the dogfood task.

### round-2

Status: in_progress
Started: 2026-05-17T13:25:43Z
Ended: none

Checks:
- none

Questions:
- none


## Planning Log

- none
