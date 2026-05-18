---
spec_version: '2.0'
task_id: harden-advisory-gate
created: '2026-05-18T02:34:27Z'
updated: '2026-05-18T02:56:09Z'
status: completed
harden_status: passed
size: medium
risk_level: medium
---

# Separate harden blockers from advisories

## Current State

Status: completed
Current phase: final
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-05-18T02:56:09Z
Review gate: pass

## Summary

Harden currently treats every provider question, design objection, and recommended edit as approval-blocking. That makes a useful hardener expensive and cyclic: advisory detail forces another round even when the draft is executable. Rework harden around one typed issue model where the gate is derived only from failed required checks and open issues with `blocks_approval: true`. Preserve full harden detail in the spec and provider dossier; do not trim evidence, advisories, or attack logs.

## Objectives

- Preserve harden detail while preventing advisory-only findings from forcing another cycle.
- Give provider harden a bounded review-like contract: checks plus typed issues plus attack log, with verdict derived from approval blockers.
- Keep manual harden usable by documenting the same issue shape in the harden prompt.
- Keep existing spec parsing stable enough to read older harden rounds, while rendering new rounds in the unified issue shape.

## Scope

- Update harden core model, schema, validation, and verdict derivation.
- Update spec model, markdown parser, renderer, and app-layer harden recording/evidence checks.
- Update harden prompts, bundled core assets, docs, and focused tests.
- Do not change review semantics, approval semantics, provider binaries, or release packaging.

## Dependencies

- Go test suite.
- Existing strict provider submission paths for Codex and Claude.
- Existing markdown living-spec parser and renderer.

## Assumptions

- Required checks remain the proof that harden performed the required attacks.
- A failed required check always blocks approval.
- Advisories remain visible in Markdown and JSON even when they do not block approval.
- Provider harden may produce a structured dossier in one pass; manual harden may still be conversational, but both converge on the same issue contract.

## Touchpoints

- `internal/core/harden/model.go`
- `internal/core/harden/schema.go`
- `internal/core/spec/model.go`
- `internal/app/harden/harden.go`
- `internal/app/harden/context.go`
- `internal/adapters/markdown/parser.go`
- `internal/adapters/markdown/renderer.go`
- `internal/adapters/corebundle/assets/core/prompts/harden.md`
- `internal/adapters/corebundle/assets/prompts/harden.md`
- `.scafld/core/prompts/harden.md`
- `.scafld/prompts/harden.md`
- `internal/core/prompts/prompts.go`
- `docs/artifacts.md`
- `docs/spec-schema.md`
- `docs/cli-reference.md`

## Risks

- Archived harden rounds that predate the issue contract are not migrated by the Go runtime.
  - Mitigation: document the hard cutover and keep new harden rounds on `Issues` only.
- Provider schema changes can cause Claude/Codex harden output failures until the prompt and schema agree.
  - Mitigation: update schema, prompt, and tests together; validate strict dossier shape.
- Manual harden evidence validation could accidentally allow unresolved blockers.
  - Mitigation: add tests proving advisory issues pass, blocking issues fail, and invalid citations still fail.

## Acceptance

Profile: standard

Validation:
- [x] `ac1` command - Harden unit tests pass
  - Command: `go test ./internal/core/harden ./internal/app/harden ./internal/adapters/markdown`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-15
- [x] `ac2` command - Full Go suite passes
  - Command: `go test ./...`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-16
- [x] `ac3` command - Full project checks pass
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-17
- [x] `ac4` command - No whitespace damage
  - Command: `git diff --check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-18

## Phase 1: Harden Issue Contract

Status: completed
Dependencies: none

Objective: Add the unified harden issue model and make the harden verdict derive from failed checks plus open approval blockers only.

Changes:
- Add `HardenIssue` / `Issue` fields with `kind`, `severity`, `blocks_approval`, `status`, `grounded_in`, `summary`, `evidence`, `recommendation`, and optional question/default fields.
- Update `VerdictFromDossier`, `ValidateDossier`, provider schema, blocker text, and provider round recording.
- Preserve legacy question/design/edit data types for older parsed specs, but do not make advisory-only issues block.

Acceptance:
- [x] `phase1-ac1` command - Core/app harden tests pass
  - Command: `go test ./internal/core/harden ./internal/app/harden`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6

## Phase 2: Markdown, Prompt, And Docs

Status: completed
Dependencies: phase1

Objective: Render and parse harden issues in the living spec, teach manual/provider harden the blocker/advisory split, and update docs.

Changes:
- Render a new `Issues:` section with full evidence and advisory/blocker status.
- Parse `Issues:` while keeping older harden round sections readable.
- Update harden prompt copies and docs to say full detail is retained but only approval blockers gate.

Acceptance:
- [x] `phase2-ac1` command - Markdown tests pass
  - Command: `go test ./internal/adapters/markdown`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-11
- [x] `phase2-ac2` command - Docs mention `blocks_approval`
  - Command: `rg -n "blocks_approval|approval blocker|advisory" docs internal/adapters/corebundle/assets/core/prompts/harden.md internal/adapters/corebundle/assets/prompts/harden.md .scafld/core/prompts/harden.md .scafld/prompts/harden.md`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-12

## Rollback

- Revert harden model/schema/parser/renderer/app changes and prompt/doc updates in one commit.
- Re-run `go test ./internal/core/harden ./internal/app/harden ./internal/adapters/markdown` after rollback.

## Review

Status: completed
Verdict: pass
Mode: discover
Provider: codex
Output: codex.output_file
Summary: No completion-blocking findings found. The implementation separates advisory harden issues from approval blockers while preserving issue detail in rendering, parsing, provider schema, app gate logic, and docs. I did not rerun tests per the read-only review instruction and relied on the recorded acceptance evidence.

Attack log:
- `task-scoped changes and related harden paths`: Scoped diff review -> clean (Read the task diff for docs/artifacts.md, docs/spec-schema.md, and internal/adapters/markdown/renderer.go, then expanded to related harden schema, prompt, parser, app, and provider paths because the workspace had task-scope files already dirty from earlier phases.)
- `internal/adapters/markdown/parser.go internal/adapters/markdown/renderer.go internal/adapters/markdown/spec_store_test.go`: Parser/renderer round-trip -> clean (Checked that the new rendered `Issues:` format is parsed back into severity, gate, id, kind, summary, status, grounding, evidence, recommendation, question, recommended_answer, and if_unanswered fields.)
- `internal/core/harden/model.go internal/app/harden/harden.go internal/app/harden/harden_test.go`: Gate derivation regression hunt -> clean (Verified harden verdict derivation and mark-passed validation: failed required checks still block; open issues with blocks_approval=true still block; open advisory issues with blocks_approval=false are retained without blocking.)
- `internal/core/harden/schema.go .scafld/core/schemas/harden_dossier.json internal/adapters/mcp/hardensubmit/server.go internal/adapters/providers/provider.go`: Schema and provider contract consistency -> clean (Checked harden dossier schema, generated managed schema assets, MCP submit path, provider parsing, and local provider fixture. Provider submissions still parse through coreharden.ParseText and schema now uses the unified issues array.)
- `docs .scafld/core/prompts/harden.md .scafld/prompts/harden.md internal/core/prompts/prompts.go internal/adapters/corebundle/assets/prompts`: Documentation and prompt consistency -> clean (Searched docs and prompt assets for stale harden wording. The updated text consistently explains that issues preserve detail while only open approval blockers gate; remaining legacy question/object/design mentions are either parser compatibility or contextual harden guidance, not gate semantics.)

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
Started: 2026-05-18T02:49:25Z
Ended: 2026-05-18T02:49:25Z
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

Issues:
- none


## Planning Log

- 2026-05-18T02:45:00Z: Replaced generated skeleton with scoped harden blocker/advisory gate contract.
