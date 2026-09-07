---
spec_version: '2.0'
task_id: harden-observation-ledger
created: '2026-06-07T06:48:43Z'
updated: '2026-06-07T08:07:49Z'
status: completed
harden_status: passed
size: large
risk_level: medium
---

# harden-observation-ledger

## Current State

Status: completed
Current phase: final
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-06-07T08:07:49Z
Review gate: pass

## Summary

Harden's data model encodes one idea, a grounded observation about the draft, three separate times: a `checks` ledger, an `issues` list, and a provider-only `attack_log`, plus a self-reported `verdict` that then has to be consistency-checked against its own derivation. Collapse all of it to a single grounded-observation ledger over the six failure-mode dimensions (path, command, scope, timing, rollback, design). Each observation is a dimension, a result (clean, advisory, blocks, or n/a), and a filesystem-verifiable anchor, with an optional note and unanswered-default. The verdict is always derived (not_ready iff a dimension is uncovered or a blocking observation is unresolved) and never written by the reviewer, so the rubber-stamp guard, the severity field, and the attack_log all stop being necessary. Provider emits the ledger as JSON, manual writes it as markdown rows, and one anchor check verifies both, which dissolves the manual/provider asymmetry instead of reconciling it.

## Objectives

- Replace `Check`, `Issue`, and `AttackLogEntry` with one `Observation{dimension, result, anchor, note?, default?, status?}` across the core model, the persisted round, and the provider schema.
- Derive the verdict from the ledger in one place; stop letting the reviewer assert it, and delete the verdict-consistency guard it required.
- Run one shared anchor-verifying gate over the persisted ledger from both the manual and provider paths, so provider citations are filesystem-verified exactly like manual ones.
- Drop `attack_log` and `severity` entirely; a derived verdict over verified anchors makes proof-of-work structural, and severity is read by nothing.
- Keep the six-dimension coverage requirement: a passed round still proves every dimension was examined.

## Scope

- In scope: `internal/core/harden` (model and schema), the `internal/core/spec` harden round types, `internal/app/harden`, the harden markdown parser and renderer, the harden prompt, and the harden submit adapters.
- Out of scope: making harden gate `approve` or `finalize`, changing the review gate's dossier, and migrating historical harden rounds (the format is ephemeral and pre-approval).

## Dependencies

- none

## Assumptions

- Harden gates nothing downstream (verified: `approve`, `finalize`, and the receipt do not read `harden_status`), so breaking the round format is safe and needs no migration.
- The six dimensions are the high-frequency build-time killers; forced coverage is the value of harden and is kept.
- Filesystem-anchor verification is the one mechanical defense against a fabricating reviewer and is applied to every observation regardless of mode.
- The reviewer never writes the verdict; scafld derives it, so a bare "ready" with no verified observations fails by construction.

## Touchpoints

- internal/core/harden/model.go
- internal/core/harden/schema.go
- internal/core/spec/model.go
- internal/app/harden/harden.go
- internal/adapters/markdown/parser.go
- internal/adapters/markdown/renderer.go
- internal/adapters/mcp/hardensubmit/server.go
- internal/adapters/cli/hardensubmit/run.go
- .scafld/core/prompts/harden.md
- internal/adapters/corebundle/assets/core/prompts/harden.md
- internal/core/harden/model_test.go
- internal/app/harden/harden_test.go

## Risks

- The change breaks any in-flight harden round still using the checks/issues format.
  - Mitigation: harden rounds are ephemeral pre-approval state; reopen the round under the new shape. No archived or approved spec depends on the old format.
- A cross-cutting type rename leaves the tree non-compiling between phases.
  - Mitigation: phases 1 and 2 are verified structurally; a green `go build ./...` and `go test` is the phase 3 gate, by design.
- The provider schema change requires structured-output providers to emit the new shape.
  - Mitigation: the strict schema is regenerated from the same `RequiredDimensions`, so provider output and the validator stay single-sourced.

## Acceptance

Profile: standard

Validation:
- [x] `v1` validate spec - This spec validates clean.
  - Command: `go run ./cmd/scafld validate harden-observation-ledger`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-30

## Phase 1: One observation model, derived verdict

Status: completed
Dependencies: none

Objective: Collapse the core harden types to a single observation and derive the verdict from the ledger.

Changes:
- internal/core/harden/model.go - replace `Check`, `Issue`, and `AttackLogEntry` with `Observation{Dimension, Result, Anchor, Note, Default, Status}`; add `RequiredDimensions`; derive verdict in `VerdictFromDossier` from uncovered dimensions and unresolved `blocks`; reduce `ValidateDossier` to ledger shape with no attack_log and no self-reported verdict.
- internal/core/harden/schema.go - provider output schema becomes `{summary, observations[]}` with dimension and result enums; verdict is no longer a provider field.

Acceptance:
- [x] `ac1_1` observation model present, old types gone - the single record replaces checks/issues/attack_log.
  - Command: `rg -q 'observations' internal/core/harden/schema.go && ! rg -q '"verdict"' internal/core/harden/schema.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6

## Phase 2: One persisted ledger, one shared gate

Status: completed
Dependencies: none

Objective: Persist the ledger on the round and verify it through a single anchor-checking gate shared by both modes.

Changes:
- internal/core/spec/model.go - replace `HardenRound.Checks` and `Issues` (and `HardenCheck`, `HardenIssue`) with `Observations []HardenObservation`.
- internal/app/harden/harden.go - `hardenObservationSkeleton` seeds the six dimension rows; `roundFromDossier` maps observations; one `verifyHardenObservations(root, observations, allowOpenBlocks)` gate (all six dimensions present, each anchor filesystem-verified, `blocks` resolved for manual pass) is called by both `markPassed` and `runProviderHarden`; thread `input.Root` into the provider path; derive verdict and blockers from the ledger.
- internal/adapters/markdown/parser.go and renderer.go - parse and render the `Observations:` block in place of `Checks:` and `Issues:`.

Acceptance:
- [x] `ac2_1` persisted round carries observations - checks/issues are gone from the spec model.
  - Command: `rg -q 'Observations \[\]HardenObservation' internal/core/spec/model.go && ! rg -q 'HardenCheck|HardenIssue' internal/core/spec/model.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-11
- [x] `ac2_2` both paths share the anchor gate - the single verifier exists in the app layer.
  - Command: `rg -q 'verifyHardenObservations' internal/app/harden/harden.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-12

## Phase 3: Ledger prompt, surfaces, and green tree

Status: completed
Dependencies: none

Objective: Move the prompt and submit adapters to the ledger and make the whole tree build and pass.

Changes:
- .scafld/core/prompts/harden.md and the corebundle asset - rewrite around the observation ledger: six dimensions, one row each, result clean/advisory/blocks/n/a, grounded anchor, a note on non-clean rows, a default for questions; the verdict is computed, not written; no attack log.
- internal/adapters/mcp/hardensubmit/server.go and internal/adapters/cli/hardensubmit/run.go - accept and render the ledger shape.
- provider harden runtime - make provider harden bounded and diagnosable: a provider that loops without a terminal submit must fail as `harden_status: error` with a diagnostic artifact instead of leaving only a long stream of opaque event labels.
- tests - update `model_test.go`, `harden_test.go`, and the parser/renderer tests; cover a provider ledger with an unresolvable anchor failing, an all-clean ledger passing, and an unresolved `blocks` failing.

Acceptance:
- [x] `ac3_1` prompt describes the ledger, not the old shapes - observations in, attack log out.
  - Command: `rg -q 'observation' .scafld/core/prompts/harden.md && ! rg -qi 'attack log' .scafld/core/prompts/harden.md`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-17
- [x] `ac3_2` tree builds - the cross-cutting rename is consistent.
  - Command: `go build ./...`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-18
- [x] `ac3_3` harden and spec tests pass - manual and provider verification share behavior.
  - Command: `go test ./internal/core/harden/... ./internal/app/harden/... ./internal/core/spec/... ./internal/adapters/markdown/...`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-19
- [x] `ac3_4` provider harden is bounded and diagnosable - a non-terminating or invalid provider run records a terminal harden error with inspectable diagnostics.
  - Command: `go test ./internal/app/harden/... ./internal/adapters/providers/... -run 'Harden.*(Diagnostics|Timeout|Invalid|NonTerminating)'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-20

## Rollback

- Revert the observation model across core, spec, app, markdown, prompt, and adapters; harden returns to the checks/issues/attack_log shape.

## Review

Status: completed
Verdict: pass
Mode: verify
Provider: codex
Output: codex.output_file
Summary: No completion-blocking findings found. Review was read-only per instruction; recorded acceptance evidence was not rerun.

Attack log:
- `internal/app/harden/harden.go`: diff inspection -> clean (Inspected task-scoped harden app diff. Provider path now threads Root, maps observations, verifies persisted round evidence, and derives verdict after verification.)
- `internal/core/harden/model.go, internal/core/harden/schema.go`: core schema contract -> clean (Checked core harden model and schema. Provider payload is summary plus observations; old self-reported verdict/checks/issues/attack_log fields are rejected by strict decoding and not in the schema.)
- `internal/adapters/markdown/parser.go, internal/adapters/markdown/renderer.go, internal/core/spec/model.go`: persistence round-trip -> clean (Checked markdown parser/renderer and spec model for the persisted observation ledger replacing Checks/Issues.)
- `internal/adapters/providers/provider.go`: provider runtime diagnostics -> clean (Checked harden provider adapter path for use of HardenDossier schema, submit_harden tool, ParseText validation, and diagnostic wrapping on invalid provider output.)

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
Started: 2026-06-07T06:58:00Z
Ended: 2026-06-07T07:57:13Z
Verdict: needs_revision
Provider: claude
Model: opus
Output format: claude.mcp_submit_harden
Summary: Provider-backed hardening did not reach a terminal dossier in a usable time window, exposing a missing bounded-runtime and diagnostics requirement.

Observations:
- path
  - Result: clean
  - Anchor: spec_gap:Scope
  - Note: The implementation paths are named explicitly across core, app, markdown, MCP submit, prompt, schemas, and docs.
- command
  - Result: clean
  - Anchor: spec_gap:Acceptance
  - Note: Acceptance names focused Go tests and a green-tree run for the affected packages.
- scope
  - Result: clean
  - Anchor: spec_gap:Scope
  - Note: The spec deliberately changes harden only and excludes review attack logs.
- timing
  - Result: blocks
  - Anchor: spec_gap:Acceptance
  - Note: Provider harden can loop for minutes without a terminal dossier or inspectable diagnostics.
  - Default: Add an explicit phase-3 requirement and tests for bounded provider harden behavior: non-terminating or invalid provider output must close the round as harden_status error with a diagnostic artifact and precise recovery command.
  - Status: fixed
- rollback
  - Result: clean
  - Anchor: spec_gap:Rollback
  - Note: Rollback covers reverting the observation model across core, spec, app, markdown, prompt, and adapters.
- design
  - Result: clean
  - Anchor: spec_gap:Summary
  - Note: The single observation ledger directly resolves the manual/provider asymmetry and self-reported-verdict weakness.


## Planning Log

- none
