---
spec_version: '2.0'
task_id: markdown-first-harden-shape-gate
created: '2026-07-19T21:01:56Z'
updated: '2026-07-19T21:35:11Z'
status: completed
harden_status: passed
size: large
risk_level: high
---

# Markdown-first harden shape gate

## Current State

Status: completed
Current phase: final
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-07-19T21:35:11Z
Review gate: pass

## Summary

Make scafld hardening operate on the raw Markdown spec as the source of truth, require an explicit shape decision before approval, and share that source-backed agent context across harden/review/handoff surfaces.

## Objectives

- Keep the Markdown task spec as the only durable task contract visible to agents and providers.
- Replace lossy harden/review context projections with a shared source-backed agent context renderer.
- Make harden answer the design-shape question before approval: keep, shrink, reframe, or reject.
- Preserve harden evidence in the Markdown spec without creating a second internal planning source of truth.
- Make context mandatory in normal agent-facing command output: context prints by default, with an explicit suppress flag only where needed for script consumers.
- Preserve review as a mandatory completion gate.
- Dogfood the new context through scafld's own harden, review, build, and completion surfaces.

## Scope

- Extend the shared context renderer and add a narrow source loader so raw spec Markdown is available as a non-omittable source section.
- Teach Markdown spec loading to expose raw source bytes, path, and digest where agent-facing commands need them.
- Update harden provider/manual context to include raw Markdown first and derived indexes second.
- Extend harden dossier/spec storage only enough to persist the shape decision and required spec edits under `## Harden Rounds`.
- Update review, handoff, status, and adapter context to consume the same source-backed context path where they present a task contract to an agent.
- Keep review required for completion; debug/suppress helpers may exist, but the mandatory lifecycle must not depend on agents remembering to ask for context or review.
- Update harden prompts, schema, docs, and tests to make the existence/minimal-shape check mandatory.

## Dependencies

- Existing Markdown spec parser and targeted updater must continue preserving unknown human-owned sections.
- Existing review context budget behavior must remain deterministic for provider transports.

## Assumptions

- Raw Markdown is small enough for normal specs; if it exceeds the configured context budget, scafld should fail closed for harden/review provider packets instead of silently omitting the source contract.
- Derived sections are still useful as indexes, but they must be labeled as derived and cannot override the raw spec.
- Existing harden ledgers without a shape decision should remain parseable and validate as historical evidence.
- Review is already a lifecycle gate; this change should harden that invariant rather than create a bypass.

## Touchpoints

- `internal/core/reviewcontext`
- `internal/app/specsource`
- `internal/core/spec/model.go`
- `internal/core/harden/model.go`
- `internal/core/harden/schema.go`
- `internal/core/prompts/prompts.go`
- `internal/adapters/cli/cli.go`
- `internal/adapters/cli/helpers.go`
- `internal/adapters/cli/harden/selection.go`
- `internal/adapters/cli/help/help.go`
- `internal/adapters/cli/output/output.go`
- `internal/adapters/cli/review/help.go`
- `internal/adapters/cli/adapter/run.go`
- `internal/adapters/markdown/spec_store.go`
- `internal/adapters/markdown/parser.go`
- `internal/adapters/markdown/renderer.go`
- `internal/adapters/providers/invoke.go`
- `internal/adapters/mcp/hardensubmit/server.go`
- `internal/app/harden/context.go`
- `internal/app/harden/harden.go`
- `internal/app/review/context.go`
- `internal/app/review/review.go`
- `internal/app/status/status.go`
- `internal/app/handoff/handoff.go`
- `internal/adapters/corebundle/assets/core/schemas/harden_dossier.json`
- `.scafld/core/prompts/harden.md`
- `.scafld/core/schemas/harden_dossier.json`
- `internal/adapters/corebundle/assets/core/prompts/harden.md`
- `internal/adapters/cli/cli_test.go`
- `internal/adapters/cli/output/output_test.go`
- `internal/adapters/markdown/spec_store_test.go`
- `internal/adapters/mcp/hardensubmit/server_test.go`
- `internal/adapters/providers/provider_test.go`
- `internal/app/handoff/handoff_test.go`
- `internal/app/harden/harden_test.go`
- `internal/app/review/review_test.go`
- `internal/app/status/status_test.go`
- `internal/core/harden/model_test.go`
- `internal/core/reviewcontext/model_test.go`

## Risks

- Large specs could exceed provider context budgets.
  - Mitigation: raw spec is non-omittable; provider commands fail with a clear budget error instead of receiving incomplete contract context.
- Expanding harden output could become another overbuilt CLI surface.
  - Mitigation: no new lifecycle command; keep the change inside the existing `harden` gate and Markdown ledger.
- Backward compatibility with older harden rounds could break existing projects.
  - Mitigation: parse missing shape decisions as historical/unknown and require the new decision only for new provider/manual pass validation.
- Review and harden could diverge again if implemented separately.
  - Mitigation: shared context package and shared tests over harden/review/handoff/adapter rendering.

## Acceptance

Profile: standard

Validation:
- none

## Phase 1: Shared Markdown Source Context

Status: completed
Dependencies: none

Objective: Build one reusable agent-context path that treats raw spec Markdown as the canonical task contract.

Changes:
- Add or refactor a shared context renderer so harden/review/handoff/adapter do not each invent a task-contract projection.
- Expose raw spec Markdown path, bytes, and sha256 from the Markdown adapter or a narrow source loader.
- Make the raw Markdown section non-omittable for provider contexts.
- Keep derived sections as budgeted indexes with explicit source/digest labeling.

Acceptance:
- [x] `ac-shared-context` command - Shared context includes raw spec Markdown before derived indexes
  - Command: `go test ./internal/core/reviewcontext ./internal/app/harden ./internal/app/review`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6

## Phase 2: Harden Shape Gate

Status: completed
Dependencies: phase1

Objective: Make harden fail or pass based on the spec's true shape, not only observation coverage.

Changes:
- Add a first-class harden shape decision with allowed values `keep`, `shrink`, `reframe`, and `reject`.
- Persist the decision and required spec edits under each harden round in Markdown.
- Require new harden provider dossiers to include the shape decision.
- Treat `shrink`, `reframe`, `reject`, or non-empty required spec edits as `needs_revision`.
- Preserve old harden rounds that do not contain a shape decision.

Acceptance:
- [x] `ac-harden-shape` command - Harden shape gate persists and validates keep/shrink/reframe/reject decisions
  - Command: `go test ./internal/core/harden ./internal/app/harden ./internal/adapters/markdown`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-11

## Phase 3: Agent Surfaces And Dogfood

Status: completed
Dependencies: phase1, phase2

Objective: Ensure agents receive the Markdown contract by default and cannot complete governed work without review.

Changes:
- Update harden prompt/schema/docs to make the right-to-exist/minimal-shape question mandatory.
- Update normal harden output so manual hardening prints the mandatory source-backed context by default, not only the prompt.
- Update review/handoff/status/adapter task-contract sections to include or reference the shared raw Markdown source context by default.
- Where terse output is necessary, use an explicit opt-out such as `--no-context`; do not make source context opt-in.
- Add tests proving completion still requires accepted review evidence and no context-only path bypasses the review gate.
- Run the full test suite and use scafld's own lifecycle to build, review, and complete this task.

Acceptance:
- [x] `ac-agent-surfaces` command - Agent-facing status, handoff, and adapter surfaces emit the Markdown source contract by default
  - Command: `go test ./internal/app/status ./internal/app/handoff ./internal/adapters/cli/adapter`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-16
- [x] `ac-review-required` command - Completion remains impossible without an accepted review gate
  - Command: `go test ./internal/app/complete ./internal/core/reviewgate`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-17
- [x] `ac-full-suite` command - Full scafld regression suite passes
  - Command: `go test ./...`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-18

## Rollback

- Revert harden schema/model additions and shared context calls.
- Restore provider prompts and docs to the previous observation-only contract.
- Existing Markdown specs remain readable because new harden fields are additive.

## Review

Status: completed
Verdict: pass
Mode: verify
Provider: codex:gpt-5.5
Output: codex.output_file
Summary: Known blocker F1 is fixed: required source Markdown is validated against the configured provider context budget before harden/review provider invocation, with tests covering no provider call and no review attempt write. I found no new completion blockers in the traced regression surface.

Attack log:
- `F1 required raw spec budget failure`: known-blocker trace -> clean (Verified `RenderMarkdownStrict` now validates required section body bytes before provider execution. Harden uses it at `internal/app/harden/harden.go:130`; review uses it at `internal/app/review/review.go:148`.)
- `internal/app/harden/harden_test.go:420 and internal/app/review/review_test.go:842`: provider invocation guard -> clean (Targeted tests assert oversized required context returns `reviewcontext.ErrRequiredContextTooLarge` and does not invoke harden/review providers.)
- `internal/app/review/review.go`: review attempt ordering -> clean (Review strict context validation happens before `startReviewAttempt`, and the test asserts no ledger entries are recorded on oversized required context.)
- `internal/app/harden/harden.go`: harden lifecycle failure recording -> clean (Harden opens the provider round, validates strict context, and closes the round as `HardenError` when required source context exceeds budget.)
- `RenderMarkdown callers`: non-provider context behavior -> clean (Manual/print-context paths still use non-strict rendering, so human-facing context remains visible by default while provider-bound packets fail closed.)
- `internal/app/harden/context.go and internal/app/review/context.go`: markdown source propagation -> clean (Harden/review context packets build the first required section via `SourceMarkdownSection`, preserving file provenance and keeping derived sections secondary.)
- `internal/app/complete/complete.go`: completion gate regression -> clean (Completion still projects reviewgate state and refuses completion unless the latest accepted review gate is passing; the context-only paths do not touch this flow.)
- `acceptance evidence`: acceptance evidence check -> clean (Recorded acceptance evidence reports `go test ./...` passed. Per provider instruction, I did not rerun build or test commands during this read-only review.)

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
Started: 2026-07-19T21:03:58Z
Ended: 2026-07-19T21:04:30Z

Observations:
- design
  - Result: clean
  - Anchor: spec_gap:Objectives
  - Note: The spec rejects the shortcut of adding an optional block and makes the true shape mandatory Markdown context plus mandatory review.
- scope
  - Result: clean
  - Anchor: spec_gap:Scope
  - Note: Scope covers the shared context owner, harden shape gate, prompt/schema/docs, and the review/handoff/status/adapter surfaces that can otherwise drift.
- path
  - Result: clean
  - Anchor: code:internal/app/harden/context.go:17
  - Note: Current harden context is a derived section list; the target path for replacing that behavior is identified.
- command
  - Result: clean
  - Anchor: spec_gap:Phases
  - Note: Each phase has a focused go test command, and the final phase includes the full `go test ./...` suite.
- timing
  - Result: clean
  - Anchor: spec_gap:Phases
  - Note: Shared context lands before the harden shape gate and agent-surface dogfood, so later checks can consume the common architecture.
- rollback
  - Result: clean
  - Anchor: spec_gap:Rollback
  - Note: Rollback is additive and keeps older Markdown specs parseable because new harden fields are optional for historical rounds.


## Planning Log

- 2026-07-19T21:01:56Z: Draft created from operator request to prevent Markdown/source-of-truth drift and make harden adversarial about feature existence and true shape.
