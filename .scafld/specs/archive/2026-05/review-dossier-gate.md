---
spec_version: '2.0'
task_id: review-dossier-gate
created: '2026-05-08T16:13:23Z'
updated: '2026-05-09T12:45:06Z'
status: completed
harden_status: passed
size: large
risk_level: high
---

# Review Dossier Gate

## Current State

Status: completed
Current phase: none
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-05-09T12:45:06Z
Review gate: pass

## Summary

Make `scafld review` produce a rich adversarial defect dossier instead of a
thin pass/fail packet.

The current review gate is structurally too narrow. Providers return
`verdict + findings[]`, and each finding is only `id`, `severity`, and
`summary`. That shape is enough to block completion, but it is not enough to
drive high-quality repair. It also encourages reviewers to stop after one or
two high-confidence blockers because a single blocker fails the gate.

The new shape separates review into two responsibilities:

- **Discovery** - inspect the task from multiple attack angles and record the
  concrete work performed by the reviewer.
- **Gate** - decide whether any discovered issue blocks completion.

The runtime should store and project a durable Review Dossier containing:

- the verdict
- structured findings with evidence, impact, reproduction, suggested repair,
  validation advice, source location, confidence, category, and related spec
  context
- an attack log that records what was inspected and what was deliberately not
  inspected
- reviewer/provider provenance
- mode information: `discover` or `verify`
- review budget information so operators can see whether the review was broad
  enough to trust

After a failed review, the default next review should not blindly rerun the full
provider challenge. It should verify open blockers and inspect the delta since
the previous review. A full rediscovery pass remains available when explicitly
requested or when the task changes enough to invalidate prior review evidence.

This is a hard contract update. Do not keep a hidden legacy ReviewPacket parse
path. If a provider emits the old packet shape, scafld should fail closed with a
clear error that names the required dossier fields.

The clean model has three nouns:

- **Review Context** - what the reviewer receives. Deterministic, bounded, and
  source-provenanced.
- **Review Dossier** - what the reviewer returns. Rich findings, attack log,
  verdict, budget, and provider provenance.
- **Review State** - what scafld stores and projects. Open blockers, fixed
  findings, accepted risks, and exact next command.

## Context

CWD: `.`

Packages:
- `internal/core/review`
- `internal/core/reviewcontext`
- `internal/core/session`
- `internal/core/reconcile`
- `internal/core/spec`
- `internal/app/review`
- `internal/app/status`
- `internal/app/handoff`
- `internal/app/complete`
- `internal/app/report`
- `internal/adapters/cli`
- `internal/adapters/cli/review`
- `internal/adapters/config`
- `internal/adapters/corebundle`
- `internal/adapters/providers`
- `test/e2e`
- `docs`

Files impacted:
- `.scafld/specs/drafts/review-dossier-gate.md`
- `.scafld/core/prompts/review.md`
- `.scafld/core/schemas/review_dossier.json`
- `.scafld/core/schemas/review_packet.json`
- `.scafld/core/config.yaml`
- `internal/adapters/corebundle/assets/core/prompts/review.md`
- `internal/adapters/corebundle/assets/core/schemas/review_dossier.json`
- `internal/adapters/corebundle/assets/core/schemas/review_packet.json`
- `internal/adapters/corebundle/assets/core/config.yaml`
- `internal/core/review/model.go`
- `internal/core/review/model_test.go`
- `internal/core/reviewcontext/model.go`
- `internal/core/reviewcontext/model_test.go`
- `internal/core/reconcile/model.go`
- `internal/core/reconcile/model_test.go`
- `internal/core/spec/model.go`
- `internal/app/review/review.go`
- `internal/app/review/review_test.go`
- `internal/app/status/status.go`
- `internal/app/status/status_test.go`
- `internal/app/handoff/handoff.go`
- `internal/app/handoff/handoff_test.go`
- `internal/app/complete/complete.go`
- `internal/app/complete/complete_test.go`
- `internal/app/report/report.go`
- `internal/app/report/report_test.go`
- `internal/adapters/cli/cli.go`
- `internal/adapters/cli/cli_test.go`
- `internal/adapters/cli/review/help.go`
- `internal/adapters/cli/review/selection.go`
- `internal/adapters/cli/review/selection_test.go`
- `internal/adapters/config/config.go`
- `internal/adapters/config/config_test.go`
- `internal/adapters/providers/provider.go`
- `internal/adapters/providers/provider_test.go`
- `test/e2e/lifecycle_test.go`
- `docs/review.md`
- `docs/run-artifacts.md`
- `docs/configuration.md`
- `docs/cli-reference.md`
- `README.md`

Invariants:
- `no_legacy_code` - no hidden parser fallback for old review packet shape.
- `domain_boundaries` - core owns dossier types; app orchestrates; adapters
  only provide config, CLI, transport, and provider IO.
- `session_as_truth` - accepted review state projects from session entries, not
  diagnostics or Markdown scraping.
- `deterministic_state` - same spec, session, config, and workspace baseline
  produce the same review mode, context packet, and follow-up command.
- `review_gate_is_adversarial` - local smoke providers cannot satisfy
  completion.
- `no_secret_context` - review context cannot include local/private files,
  `.scafld/config.local.yaml`, `.priv/**`, `.git/**`, or `.env*`.

Related docs:
- `docs/review.md`
- `docs/run-artifacts.md`
- `docs/configuration.md`
- `docs/cli-reference.md`
- `docs/introduction.md`

## Objectives

- Replace thin review findings with structured Review Dossier findings.
- Require attack-log coverage so a reviewer cannot pass after checking only a
  vague diff summary.
- Make repair context rich enough that the next agent can fix an issue without
  opening raw provider diagnostics.
- Avoid expensive blind review cycles by adding verification mode for open
  findings and delta-only changes.
- Keep `complete` strict: completion requires a passing external, command, or
  audited human review dossier.
- Preserve determinism and session-first projection.
- Update docs and examples so operators understand review as a defect dossier,
  not just an AI opinion.

## Scope

- Introduce a new `ReviewDossier` core model and replace the current packet
  model at the provider boundary.
- Update review schema, prompts, providers, parser, validation, session
  encoding, status JSON, handoff, spec projection, and docs.
- Add review modes:
  - `discover` - first broad adversarial review.
  - `verify` - verify previous open blockers and inspect only relevant deltas.
- Make `scafld review <task-id>` choose the correct mode automatically from
  session state, with explicit flags for operators.
- Add CLI flags for `--mode`, `--full`, `--verify`, `--max-findings`,
  `--min-attack-angles`, and `--review-depth`.
- Add config defaults for review depth, finding limit, attack-angle minimum,
  and rerun policy.
- Add structured attack logs to review output, status JSON, handoff, and spec
  projection.
- Add tests that prove old packet shape is rejected with a teaching error.

Out of scope:
- Provider-specific multi-agent orchestration.
- Long-term issue tracker integration.
- Semantic repository indexing.
- Auto-fixing review findings.
- Reintroducing `.scafld/reviews/` as a source of truth.
- Compatibility aliases or hidden old-packet fallback behavior.

## Dependencies

- Existing review context packet from `internal/core/reviewcontext`.
- Existing session replay and review projection.
- Existing provider integrations for Claude, Codex, command, and local test
  provider.
- Existing workspace baseline and scope-drift logic.

## Assumptions

- Review providers can be prompted to emit richer JSON as long as the schema is
  concrete and examples are included.
- The command provider is a public-ish integration surface, but this contract
  can still hard-cut because the old packet is not rich enough for the product
  requirement.
- Expensive external provider calls should be minimized after the first failed
  review.
- The repair agent should consume `scafld status --json` and `scafld handoff`,
  not raw diagnostics, for accepted review findings.
- Human-reviewed overrides remain exceptional and audited.

## Touchpoints

- Provider prompt wording and schema.
- Review packet parsing and validation.
- Session entries and reconcile projection.
- CLI review mode selection and help.
- Status, handoff, complete, and report surfaces.
- Docs and examples.
- Dogfood release flow.

## Risks

- **Schema bloat** - mitigate with required fields for blockers and optional
  fields for non-blockers, but keep the packet readable.
- **Provider noncompliance** - fail closed with actionable errors and tests for
  malformed/old packets.
- **Longer first reviews** - offset with verification mode and explicit budgets.
- **False precision** - require confidence and evidence, but do not pretend a
  finding is proven if it is hypothesis-only.
- **Over-blocking on non-critical issues** - clarify severity rules and require
  impact for every completion-blocking issue.
- **Context truncation** - protect structured sections before free-form docs so
  acceptance evidence and open findings are never crowded out by README text.

## Acceptance

Profile: strict

Validation:
- [x] `v1` test - Full Go test suite passes.
  - Command: `go test ./...`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-176
- [x] `v2` test - Release check passes.
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-177
- [x] `v3` test - Review prompt and schema no longer describe the thin packet as
  - Command: `test -f .scafld/core/schemas/review_dossier.json && test ! -f .scafld/core/schemas/review_packet.json && rg -n 'review_dossier|attack_log|blocks_completion|severity' .scafld/core/schemas/review_dossier.json internal/adapters/corebundle/assets/core/schemas/review_dossier.json .scafld/core/prompts/review.md internal/adapters/corebundle/assets/core/prompts/review.md`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-178
- [x] `v4` test - Dogfood review context can be printed deterministically.
  - Command: `go test ./internal/adapters/cli -run TestRunReviewPrintContextDoesNotInvokeProvider`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-179

## Phase 1: Core Dossier Contract

Status: completed
Dependencies: none

Objective: Complete this phase.

Changes:
- `internal/core/review/model.go` (all, exclusive) - Replace or extend packet
- `.scafld/core/schemas/review_dossier.json` (all, exclusive) - Add the
- `.scafld/core/schemas/review_packet.json` (all, exclusive) - Remove the old
- `internal/adapters/corebundle/assets/core/schemas/review_dossier.json` (all,
- `internal/adapters/corebundle/assets/core/schemas/review_packet.json` (all,
- `internal/core/review/model_test.go` (all, exclusive) - Cover dossier
- `verdict`: `pass` or `fail`.
- `mode`: `discover` or `verify`.
- `provider`: provider name.
- `model`: provider model when known.
- `findings`: array of structured findings.
- `attack_log`: array of inspected targets and outcomes.
- `budget`: requested and actual review budget.
- `summary`: concise reviewer conclusion.
- `id` - stable snake/kebab identifier.
- `severity` - `critical`, `high`, `medium`, or `low`.
- `blocks_completion` - boolean gate decision independent of severity.
- `category` - correctness, boundary, error_path, state, contract_drift,
- `confidence` - `high`, `medium`, or `low`.
- `location` - file path plus optional line.
- `evidence` - what was read or observed.
- `impact` - why this matters.
- `reproducer` - command or scenario when available.
- `suggested_fix` - concrete repair direction.
- `validation` - how to prove the fix.
- `related_spec` - spec section, phase, or criterion when applicable.
- `review_pass` - configured pass that found the issue.
- `status` - `open`, `fixed`, `accepted_risk`, or `superseded`.
- `target` - file, caller set, invariant, acceptance criterion, or skipped
- `attack` - what the reviewer checked.
- `result` - `finding`, `clean`, or `skipped`.
- `notes` - bounded explanation.

Acceptance:
- [x] `ac1_1` test - Dossier validation accepts a complete blocking finding.
  - Command: `go test ./internal/core/review -run TestValidateDossierRequiresDossierShape`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-156
- [x] `ac1_2` test - Old thin packet shape is rejected.
  - Command: `go test ./internal/core/review -run TestParseTextAcceptsDossierAndRejectsLegacyPacket`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-157
- [x] `ac1_3` test - Findings separate severity from completion blocking.
  - Command: `go test ./internal/core/review -run TestSeverityAndCompletionBlockingAreSeparate`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-158
- [x] `ac1_4` test - Completion-blocking findings require evidence, impact,
  - Command: `go test ./internal/core/review -run TestBlockingFindingsRequireRepairContext`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-159

## Phase 2: Review Prompt And Context Budget

Status: completed
Dependencies: phase1

Objective: Complete this phase.

Changes:
- `.scafld/core/prompts/review.md` (all, exclusive) - Rewrite review
- `internal/adapters/corebundle/assets/core/prompts/review.md` (all,
- `internal/core/reviewcontext/model.go` (surgical) - Add reserved sections and
- `internal/core/reviewcontext/model_test.go` (surgical) - Cover reserved
- `internal/app/review/review.go` (surgical) - Include prior open findings,
- The reviewer must inspect every applicable configured attack angle.
- The reviewer must record clean checks for attack angles that did not produce
- The reviewer must not stop after the first blocker.
- The reviewer must cap output at `max_findings`, prioritizing highest impact.
- The reviewer must distinguish verified defects from hypotheses.
- A pass verdict requires attack-log evidence, not "nothing found".

Acceptance:
- [x] `ac2_1` test - Rendered review context includes required attack-log
  - Command: `go test ./internal/app/review ./internal/core/reviewcontext -run 'TestReviewContext|TestRenderMarkdown'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-160
- [x] `ac2_2` test - Reserved structured sections survive tight context
  - Command: `go test ./internal/core/reviewcontext -run TestRenderMarkdownAppliesBudgetAcrossSections`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-161
- [x] `ac2_3` test - Prompt no longer instructs the reviewer to stop after
  - Command: `! rg -n 'until you find a defect' .scafld/core/prompts/review.md internal/adapters/corebundle/assets/core/prompts/review.md`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-162

## Phase 3: Session Projection And Repair Surface

Status: completed
Dependencies: phase1

Objective: Complete this phase.

Changes:
- `internal/core/session/model.go` (surgical) - Ensure review dossier payloads
- `internal/core/reconcile/model.go` (surgical) - Replay latest dossier into
- `internal/core/spec/model.go` (surgical) - Add review dossier projection
- `internal/app/status/status.go` (surgical) - Include full structured review
- `internal/app/handoff/handoff.go` (surgical) - Render blocking findings with
- `internal/app/complete/complete.go` (surgical) - Require a passing dossier
- `docs/run-artifacts.md` (surgical) - Document dossier session entries and

Acceptance:
- [x] `ac3_1` test - Session replay preserves dossier findings without
  - Command: `go test ./internal/core/reconcile -run TestFromSessionProjectsLatestReviewFindings`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-163
- [x] `ac3_2` test - `status --json` includes repair-ready finding fields.
  - Command: `go test ./internal/app/status ./internal/adapters/cli -run 'TestStatusIncludesLatestReviewFindings|TestRunReviewSurfacesFindingsInReviewStatusAndHandoff'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-164
- [x] `ac3_3` test - Handoff includes evidence, impact, suggested fix, and
  - Command: `go test ./internal/app/handoff -run TestHandoffIncludesLatestReviewFindings`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-165

## Phase 4: Verification Review Mode

Status: completed
Dependencies: phase1, phase3

Objective: Complete this phase.

Changes:
- `internal/app/review/review.go` (surgical) - Add automatic mode selection:
- `internal/adapters/cli/review/help.go` (surgical) - Document `--mode`,
- `internal/adapters/cli/cli.go` (surgical) - Parse and pass review mode and
- `internal/adapters/config/config.go` (surgical) - Add config defaults for
- `internal/adapters/providers/provider.go` (surgical) - Pass review mode and
- `test/e2e/lifecycle_test.go` (surgical) - Cover discover -> fail ->
- `scafld review <task>` defaults to `discover` when no prior review exists.
- `scafld review <task>` defaults to `verify` when the latest accepted
- `scafld review <task> --full` forces `discover`.
- `scafld review <task> --verify` forces `verify`.
- `complete` accepts the latest `pass` dossier from `discover`, `verify`, or

Acceptance:
- [x] `ac4_1` test - Mode selection is deterministic from session state.
  - Command: `go test ./internal/app/review -run TestReviewModeSelection`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-166
- [x] `ac4_2` test - Failed review rerun defaults to `verify`, not full
  - Command: `go test ./internal/app/review -run TestReviewModeSelection`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-167
- [x] `ac4_3` test - `complete` accepts a passing verify dossier only when no
  - Command: `go test ./internal/app/complete -run TestCompleteAcceptsKnownExternalReviewProviders`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-168

## Phase 5: CLI, Config, And Docs

Status: completed
Dependencies: phase2, phase3, phase4

Objective: Complete this phase.

Changes:
- `.scafld/core/config.yaml` (surgical) - Add review dossier defaults.
- `internal/adapters/corebundle/assets/core/config.yaml` (surgical) - Keep
- `docs/review.md` (all, exclusive) - Reframe review as discovery + gate +
- `docs/configuration.md` (surgical) - Document review depth, budgets, mode
- `docs/cli-reference.md` (surgical) - Update review flags and examples.
- `README.md` (surgical) - Mention defect dossier as the repair surface.
- `internal/adapters/cli/cli_test.go` (surgical) - Cover help output.

Acceptance:
- [x] `ac5_1` test - Review help lists the new mode and budget flags.
  - Command: `go test ./internal/adapters/cli -run TestReviewHelpIncludesContextFlags`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-169
- [x] `ac5_2` test - Config defaults parse and expose review dossier settings.
  - Command: `go test ./internal/adapters/config -run TestConfigLoad`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-170
- [x] `ac5_3` test - Docs mention discovery, gate, verification, and dossier.
  - Command: `rg -n 'discovery|gate|verify|defect dossier|attack log' README.md docs/review.md docs/run-artifacts.md docs/configuration.md docs/cli-reference.md`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-171

## Phase 6: Dogfood And Release Readiness

Status: completed
Dependencies: phase1, phase2, phase3, phase4, phase5

Objective: Complete this phase.

Changes:
- `.scafld/specs/active/review-dossier-gate.md` (all, exclusive) - Dogfood
- `test/e2e/lifecycle_test.go` (surgical) - Add end-to-end dossier lifecycle
- `test/parity/parity_test.go` (surgical) - Ensure bundled and workspace

Acceptance:
- [x] `ac6_1` test - Full local checks pass.
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-172
- [x] `ac6_2` test - Dogfood status exposes build gate state without raw
  - Command: `go run ./cmd/scafld status review-dossier-gate --json`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-173
- [x] `ac6_3` test - Provider adapters parse valid dossier output.
  - Command: `go test ./internal/adapters/providers -run 'TestClaudeProviderBuildsRestrictedStreamJSONArgsAndExtractsStructuredOutput|TestCodexProviderBuildsReadOnlyEphemeralArgsAndReadsOutputFile|TestProviderContract'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-174
- [x] `ac6_4` test - The completed dogfood spec exposes the latest dossier in
  - Command: `go run ./cmd/scafld status review-dossier-gate --json`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-175

## Rollback

Strategy: per_phase

Commands:
- `git restore .scafld/core/prompts/review.md .scafld/core/schemas .scafld/core/config.yaml`
- `git restore internal/adapters/corebundle/assets/core/prompts/review.md internal/adapters/corebundle/assets/core/schemas internal/adapters/corebundle/assets/core/config.yaml`
- `git restore internal/core/review internal/core/reviewcontext internal/core/reconcile internal/core/spec`
- `git restore internal/app/review internal/app/status internal/app/handoff internal/app/complete internal/app/report`
- `git restore internal/adapters/cli internal/adapters/config internal/adapters/providers`
- `git restore docs README.md test/e2e test/parity`

## Review

Status: completed
Verdict: pass
Mode: verify
Summary: The Review Dossier replaces the legacy ReviewPacket with a verdict/mode/summary/findings/attack_log/budget shape that gates completion only when blocking findings carry location, evidence, impact, and validation. The dossier struct, JSON schema, validator, NDJSON parser (with legacy-frame teaching error), mode-selection rerun policy, scope-drift and workspace-mutation guards, completion-gate provider allowlist, and CLI/handoff/status/output projections all match the task contract. Unit and e2e tests cover the discover→repair→verify→pass flow, including invalid mode/verdict, missing required fields, blocker-without-repair-context, scope drift, workspace mutation, and the command-provider end-to-end happy path. No completion-blocking defects identified; no_legacy_code invariant is upheld (no shims, the legacy NDJSON frame types are explicitly rejected).

Attack log:
- `internal/core/review/model.go::Dossier and .scafld/core/schemas/review_dossier.json`: spec_compliance: walk every required dossier field (verdict, mode, summary, findings with location/evidence/impact/validation/reproducer/suggested_fix, attack_log, budget, provider/model/session_id, event_summary) and confirm struct, JSON schema, and ValidateDossier all enforce the same shape -> clean (Dossier struct (model.go:128-141), JSON schema (review_dossier.json) with additionalProperties:false, ValidateDossier (model.go:314-345) and ValidateFinding (model.go:347-376) consistently require location/evidence/impact/validation on blocking findings; verdict must equal VerdictFromFindings; attack_log min length 1; severity/confidence/status/result values pinned to enums.)
- `internal/app/review/review.go::reviewMode and .scafld/core/config.yaml review.dossier.rerun_policy`: spec_compliance: confirm verify_open_blockers auto-flip works (latestReviewHasOpenBlockers walks ledger, decodes dossier, returns OpenBlockerCount>0) and that --verify/--full/--mode flags can force discover via ForceMode -> clean (reviewMode (review.go:328-349) honors ForceMode, then rerun_policy (verify_open_blockers default), then explicit Mode; latestReviewHasOpenBlockers (review.go:351-368) reads from session ledger and uses DecodeDossier so legacy-empty entries safely report no blockers.)
- `internal/core/review/model.go::ParseNDJSON`: regression_hunt + no_legacy_code: probe whether legacy NDJSON frames (verdict, finding, workspace_mutation, done) still parse silently anywhere -> clean (ParseNDJSON (model.go:243-289) explicitly switch-cases legacy frame types and returns ErrInvalidDossier with a teaching error 'legacy review frame %q is no longer supported; emit one review dossier'. No fallback decoder remains; ParseText delegates to ParseNDJSON when frames carry a type field.)
- `internal/app/review/review.go::reviewBlockingMutations + deriveReviewScope/reviewScopePathAllowed`: scope_drift + dark_patterns: confirm the mutation guard fails closed when the provider mutates within scope, that scope drift since the approval baseline blocks before provider spend, and that derived scope filtering excludes .git/.priv/.env*/.scafld/config.local.yaml/.scafld/reviews -> clean (Scope drift path builds a synthetic fail dossier and records a review_attempt 'blocked' entry before invoking the provider (review.go:129-148). reviewBlockingMutations (review.go:478-496) compares pre/post snapshots within normalized scope and always treats the current spec file as in-scope. reviewScopePathAllowed (review.go:553-574) per-segment denies any .env* path plus the four denylisted roots; workspaceMutationFinding/scopeDriftFinding emit the required location/evidence/impact/validation.)
- `internal/app/complete/complete.go and internal/core/review/model.go::ValidCompletionProvider`: spec_compliance: verify the completion gate requires (a) latest review entry status == pass, (b) provider in allowlist, (c) decodable dossier, (d) zero open blockers, (e) audited human override when provider==human -> clean (ValidCompletionProvider (model.go:206-213) enumerates {codex, claude, command, human}; complete.go enforces the gate using OpenBlockerCount and DecodeDossier. The 'local' provider remains excluded, matching the contract that local is smoke-test only.)
- `internal/adapters/markdown/spec_store.go and internal/core/reconcile/model.go`: convention_check + dark_patterns: confirm spec markdown round-trips Findings (Location/Evidence/Impact/Validation) and that AttackLog/Budget recover from the session ledger replay so the on-disk markdown is not the authoritative source for those projections -> clean (Markdown render emits Findings with full repair context. Reconcile.FromSession decodes the latest review entry's dossier JSON and projects Mode/Summary/Findings/AttackLog/Budget into spec.Review, so a fresh load reconstitutes the full dossier even when markdown alone would lose Attack log / Budget detail. The spec.json review_finding sub-schema is narrower than the runtime model but markdown is the authoritative on-disk format, so this is not a contract violation.)
- `internal/app/handoff/handoff.go::writeLatestReviewFindings + internal/app/status/status.go::ReviewInfo + internal/adapters/cli/output/output.go::Review`: spec_compliance + convention_check: confirm Verdict/Mode/Summary/Findings/AttackLog/Budget surface consistently across handoff markdown, status JSON, and CLI rendering for both pass and fail outcomes -> clean (All three call sites read from spec.Review (or recompute from the dossier) and render the same fields. ReviewInfo exposes OpenBlockers separately from Findings so automation can gate on it; the CLI output also surfaces provider:model and the next command, matching the dossier provenance contract.)
- `test/e2e/lifecycle_test.go + internal/app/review/review_test.go + internal/core/review/model_test.go`: error_path + boundary: confirm test coverage rejects invalid verdict/mode/missing summary/missing attack_log/blocker-without-location-evidence-impact-validation/pass+blocker contradiction/mode mismatch/scope drift/workspace mutation/derived-scope filtering and that an end-to-end command-provider flow exercises the dossier path -> clean (Validation table tests cover the rejection cases enumerated in ValidateDossier/ValidateFinding; review_test.go exercises mode auto-flip, mutation-guard appending, scope-drift fail-closed before provider; lifecycle e2e drives discover→repair→verify→pass with NDJSON {type:'dossier'} frames through the command provider, matching the runtime parser contract.)

Findings:
- none

## Self Eval

Status: not_started
Completeness: none
Architecture fidelity: none
Spec alignment: none
Validation depth: none
Total: none
Second pass performed: none

Notes:
none

Improvements:
- none

## Deviations

- none

## Metadata

Estimated effort hours: 20
Actual effort hours: none
AI model: none
React cycles: none

Tags:
- review
- dossier
- adversarial-review
- repair-loop
- provider-contract

## Origin

Source:
- User request: review is not effective enough, finds too few issues, locks
  into costly cycles, and should surface more context to the host.

Repo:
- `github.com/nilstate/scafld`

Git:
- base: `7632879`

Sync:
- none

Supersession:
- none

## Harden Rounds

### round-1

Status: passed
Started: 2026-05-09T00:55:54Z
Ended: 2026-05-09T00:55:59Z

Questions:
- none


## Planning Log

- 2026-05-08T16:13:23Z - codex - Created full draft spec for converting
  review from a thin verdict packet into a durable defect dossier with attack
  log, verification mode, structured findings, docs, config, and dogfood gates.
