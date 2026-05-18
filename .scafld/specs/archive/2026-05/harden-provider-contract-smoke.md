---
spec_version: '2.0'
task_id: harden-provider-contract-smoke
created: '2026-05-18T04:12:19Z'
updated: '2026-05-18T04:16:36Z'
status: cancelled
harden_status: needs_revision
size: small
risk_level: low
---

# Harden provider issue contract smoke

## Current State

Status: cancelled
Current phase: none
Next: done
Reason: cancel
Blockers: none
Allowed follow-up command: `none`
Latest runner update: none
Review gate: not_started

## Summary

Implement harden-provider-contract-smoke.

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

Status: pending
Dependencies: none

Objective: Complete the requested change.

Changes:
- Implement the requested behavior.

Acceptance:
- [ ] `ac1` command - Primary validation command
  - Command: `go version`
  - Expected kind: `exit_code_zero`
  - Status: pending

## Rollback

- none

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
Started: 2026-05-18T04:12:24Z
Ended: 2026-05-18T04:12:24Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: The draft is not ready for approval. It is a placeholder spec with empty scope, unrelated acceptance, no rollback, and no authoritative contract definition for the provider issue smoke.

Checks:
- path audit
  - Grounded in: spec_gap:Scope And Touchpoints
  - Result: failed
  - Evidence: The draft declares no implementation paths or new artifacts; scope is empty, so there is nothing concrete to verify or bound.
- command audit
  - Grounded in: spec_gap:Planned Phases
  - Result: failed
  - Evidence: `go version` only verifies toolchain presence, not the requested provider issue contract smoke behavior.
- scope/migration audit
  - Grounded in: spec_gap:Scope And Touchpoints
  - Result: failed
  - Evidence: The draft makes no migration or compatibility claims, but also does not define whether provider output contract changes affect public schemas or review/harden interfaces.
- acceptance timing audit
  - Grounded in: spec_gap:Planned Phases
  - Result: failed
  - Evidence: The only acceptance criterion, `go version`, can be evaluated before any implementation and therefore cannot prove the phase's requested behavior after implementation.
- rollback/repair audit
  - Grounded in: spec_gap:Acceptance And Rollback
  - Result: failed
  - Evidence: Rollback/repair is omitted. Even for a low-risk smoke test, the draft should state whether repair is reverting the test-only/spec change or rerunning a specific scafld command.
- design challenge
  - Grounded in: spec_gap:Summary
  - Result: failed
  - Evidence: The plan does not name the underlying workflow problem, the authoritative provider contract shape, ownership boundaries, golden examples, or dogfood path. Approval would require invention during build.

Issues:
- [high/blocks approval] `harden-1` design_challenge - The draft has no executable product or system objective.
  - Status: open
  - Grounded in: spec_gap:Summary
  - Evidence: Summary says only "Implement harden-provider-contract-smoke" and the phase says "Complete the requested change" / "Implement the requested behavior" without defining provider issue contract smoke behavior.
  - Recommendation: Rewrite the summary and objective to state the actual workflow problem and expected behavior, for example that provider harden output must preserve issue contract fields (`severity`, `status`, `blocks_approval`, evidence, recommendation, question metadata) in a smoke fixture or test.
- [high/blocks approval] `harden-2` scope - Scope is empty and cannot pass path or ownership audit.
  - Status: open
  - Grounded in: spec_gap:Scope And Touchpoints
  - Evidence: Scope And Touchpoints is empty, so implementation would require guessing which provider, prompt, fixture, parser, test, or command path owns the contract smoke.
  - Recommendation: Declare the exact files/packages to inspect or change, and mark any new fixtures/tests as new. Include ownership boundaries so build does not edit unrelated provider or lifecycle code.
- [high/blocks approval] `harden-3` acceptance - Acceptance criterion is unrelated to the requested behavior.
  - Status: open
  - Grounded in: spec_gap:Planned Phases
  - Evidence: Acceptance `ac1` is only `go version`, which validates that Go is installed but not that the provider issue contract smoke exists or passes.
  - Recommendation: Replace or supplement `go version` with a runnable repo command that exercises the smoke, such as a focused Go test or scafld command, and state the working directory.
- [medium/blocks approval] `harden-4` question - Authority for duplicated contract facts is unspecified.
  - Status: open
  - Grounded in: spec_gap:Scope And Touchpoints
  - Evidence: The draft does not identify what artifact is authoritative when provider output, prompts, schema structs, fixtures, and spec text encode the same issue fields.
  - Recommendation: Name one authoritative source and require all duplicated examples/fixtures to derive from or match it.
  - Question: Which artifact is authoritative for the provider issue contract smoke: the Go schema/parser, managed prompts, golden fixtures, or the task spec?
  - Recommended answer: Use the Go schema/parser as the authoritative contract, with prompts and fixtures serving as tests/examples that must match it.
  - If unanswered: Treat the existing code-level schema/parser as authoritative and update tests/fixtures to match it; do not create a parallel contract source.
- [medium/blocks approval] `harden-5` rollback - Rollback and repair path is missing.
  - Status: open
  - Grounded in: spec_gap:Acceptance And Rollback
  - Evidence: Acceptance And Rollback only says `Validation profile: standard`; no rollback, repair, or human recovery command is named.
  - Recommendation: Add a realistic repair path, such as reverting the focused test/fixture change or rerunning the exact validation command after fixing provider contract output; include any scafld dogfood command if applicable.
- [low/advisory] `harden-6` test_design - Examples or golden fixtures would make the intended contract easier to preserve.
  - Status: open
  - Grounded in: spec_gap:Planned Phases
  - Evidence: The task name implies a smoke test for harden provider issue contracts, but no example/golden shape is specified.
  - Recommendation: Add a minimal golden fixture or expected output example proving open blocking issue, open advisory issue, fixed issue, and question metadata handling if those are in scope.

### round-2

Status: needs_revision
Started: 2026-05-18T04:13:36Z
Ended: 2026-05-18T04:13:36Z
Verdict: needs_revision
Provider: claude
Model: claude-opus-4-7
Output format: claude.mcp_submit_harden
Summary: Round 2: draft is unchanged from round 1. Scope, objectives, risks, and rollback are still empty; the only acceptance is `go version`, which does not exercise the provider issue contract. All six audit checks fail. Approval would require invention during build, so the round must remain open with blocking issues.

Checks:
- path audit
  - Grounded in: spec_gap:Scope And Touchpoints
  - Result: failed
  - Evidence: Scope, Touchpoints, and Dependencies are all `- none` (.scafld/specs/drafts/harden-provider-contract-smoke.md:33-47); no files, packages, fixtures, or new artifacts are declared, so there is nothing to verify or bound.
- command audit
  - Grounded in: spec_gap:Phase 1: Implementation
  - Result: failed
  - Evidence: The only declared command is `go version` (.scafld/specs/drafts/harden-provider-contract-smoke.md:73). It runs from any directory and exercises no provider-contract behavior, so it does not validate the phase.
- scope/migration audit
  - Grounded in: spec_gap:Scope
  - Result: failed
  - Evidence: The draft makes no claim about provider output schema, prompts, or fixtures (`Scope: - none`, `Risks: - none`). The harden_dossier schema is a public provider contract (.scafld/core/schemas/harden_dossier.json:1), so silence on compatibility cannot be treated as `no migration needed`.
- acceptance timing audit
  - Grounded in: spec_gap:Acceptance
  - Result: failed
  - Evidence: `go version` can be evaluated before any code change (.scafld/specs/drafts/harden-provider-contract-smoke.md:71-74); it cannot prove that the phase produced the requested behavior, so phase-after timing is meaningless.
- rollback/repair audit
  - Grounded in: spec_gap:Rollback
  - Result: failed
  - Evidence: Rollback section is `- none` (.scafld/specs/drafts/harden-provider-contract-smoke.md:76-78). Even for a low-risk smoke, the draft must state whether repair is reverting the focused test/fixture or rerunning a named scafld command.
- design challenge
  - Grounded in: spec_gap:Summary
  - Result: failed
  - Evidence: Summary is `Implement harden-provider-contract-smoke` (line 27) and the phase says `Implement the requested behavior` (line 68). The plan does not name the workflow problem, the authoritative artifact, the golden shape, or ownership boundaries; approval would require invention.

Issues:
- [high/blocks approval] `harden-1` design_challenge - Draft has no executable product or system objective; round 1 finding still open.
  - Status: open
  - Grounded in: spec_gap:Summary
  - Evidence: Summary (line 27) and Phase 1 changes (line 68) are placeholder sentences. Round-1 `harden-1` raised the same gap and the spec was not rewritten before round 2 started (round-2 begins at line 178 with no spec edits above it).
  - Recommendation: Rewrite Summary and the Phase 1 objective/changes to state the underlying workflow problem and the precise provider issue-contract behavior to smoke-test (e.g. severity/status/blocks_approval/evidence/recommendation/question fields round-tripping through provider output).
- [high/blocks approval] `harden-2` scope - Scope, Touchpoints, Dependencies are empty; build would have to guess which provider/parser/fixture is in scope.
  - Status: open
  - Grounded in: spec_gap:Scope
  - Evidence: Lines 33-47 list `- none` for Scope, Dependencies, Assumptions, and Touchpoints. Provider code lives across internal/adapters/providers/{provider.go,schema.go} and internal/app/harden/, but the spec does not bind the work to any of these.
  - Recommendation: Declare the specific files/packages to read or change (e.g. internal/adapters/providers/schema.go, internal/app/harden/harden.go, fixtures under internal/adapters/providers/), mark any new fixtures/tests as new, and state ownership boundaries so build does not edit unrelated provider or lifecycle code.
- [high/blocks approval] `harden-3` acceptance - Acceptance `ac1` is unrelated to the requested behavior.
  - Status: open
  - Grounded in: spec_gap:Acceptance
  - Evidence: `ac1` runs `go version` (line 73) with kind `exit_code_zero`. Round-1 `harden-3` raised the same gap; the criterion is unchanged in round 2.
  - Recommendation: Replace or supplement `ac1` with a runnable command that exercises the smoke from the repo root, e.g. `go test ./internal/adapters/providers/... -run HardenDossier` against a golden fixture, and declare the working directory.
- [medium/blocks approval] `harden-4` question - Authoritative source for the provider issue contract is unspecified.
  - Status: open
  - Grounded in: spec_gap:Scope
  - Evidence: Issue fields are duplicated across .scafld/core/schemas/harden_dossier.json, internal/adapters/providers/schema.go, the managed harden prompt in this packet, and any new spec fixture. Scope (line 33) does not name which is authoritative when they drift.
  - Recommendation: Pick one authoritative artifact and require all duplicates to derive from or match it; record the decision in Scope and call out the contract location.
  - Question: Which artifact is authoritative for the provider issue contract: the Go schema/parser (internal/adapters/providers/schema.go), the JSON schema (.scafld/core/schemas/harden_dossier.json), the managed prompt, or the new spec fixture?
  - Recommended answer: Treat the JSON schema (.scafld/core/schemas/harden_dossier.json) as authoritative for shape and the Go parser as the consumer that must accept it; prompts and fixtures are tests/examples that must match the schema.
  - If unanswered: Anchor on the JSON schema as authoritative and update prompts/fixtures/tests to match it; do not introduce a parallel contract source.
- [medium/blocks approval] `harden-5` rollback - No rollback or repair path is named, even for a low-risk smoke.
  - Status: open
  - Grounded in: spec_gap:Rollback
  - Evidence: Rollback section reads `- none` (lines 76-78); Acceptance section only names `Profile: standard` (line 55).
  - Recommendation: Add a realistic repair path: revert the focused fixture/test change or rerun the validation command after fixing provider output; if a scafld dogfood command applies (e.g. `scafld harden harden-provider-contract-smoke --provider claude`), name it explicitly.
- [low/advisory] `harden-6` test_design - No golden fixture or expected example proves the intended contract shape.
  - Status: open
  - Grounded in: spec_gap:Phase 1: Implementation
  - Evidence: Phase 1 (lines 60-74) names no fixture or expected JSON. The harden_dossier schema enumerates check names, severities, statuses, and the optional question/recommended_answer/if_unanswered fields (.scafld/core/schemas/harden_dossier.json:1), all of which the smoke should pin down.
  - Recommendation: Add a minimal golden HardenDossier covering: a blocking issue (blocks_approval=true), an advisory issue (blocks_approval=false), a fixed issue, and a `question` with `recommended_answer`/`if_unanswered`. Reference its path in Scope.


## Planning Log

- none
