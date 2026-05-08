---
spec_version: '2.0'
task_id: review-context-packet
created: '2026-05-08T02:28:10Z'
updated: '2026-05-08T05:15:43Z'
status: completed
harden_status: passed
size: large
risk_level: high
---

# Agent-Facing Review Context And Config Maturity

## Current State

Status: completed
Current phase: none
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-05-08T05:15:43Z
Review gate: pass

## Summary

Make adversarial review impossible to under-inform, make deterministic gates
obvious to agents, and make `scafld config` infer more real project shape.

`scafld review` already sends the task contract, scope, baseline, task changes,
acceptance evidence, and review agenda. That catches local code defects. It
does not yet guarantee the reviewer receives the broader product contract:
agent guidance, configured invariants, relevant docs, schemas, prior review
findings, and hard-won regression lessons.

Add a typed, deterministic review-context packet that is assembled before a
provider is invoked. Providers still receive Markdown, but that Markdown is
rendered from a structured packet with explicit sections, source provenance,
deduplication, ordering, and size limits. The same packet can be printed for
debugging so operators and agents can see exactly what the challenger saw.

At the same time, name the product concept in docs: scafld is strict about the
state machine but generous about repair context. Agents should know exactly
what state is trusted, why a gate blocked, and which command is allowed next.

Finally, extend `scafld config` so new workspaces get closer to a truthful
runtime config on first pass: more command surfaces, common monorepo package
layouts, version-manager environments, and review-context defaults.

## Context

CWD: `.`

Packages:
- `internal/app/review`
- `internal/app/config`
- `internal/core/review`
- `internal/core/reviewcontext`
- `internal/core/session`
- `internal/core/spec`
- `internal/adapters/cli`
- `internal/adapters/cli/help`
- `internal/adapters/cli/output`
- `internal/adapters/cli/review`
- `internal/adapters/config`
- `internal/adapters/corebundle`
- `internal/adapters/markdown`
- `internal/adapters/cli/config`
- `docs`

Files impacted:
- `.scafld/config.yaml`
- `.scafld/specs/active/review-context-packet.md`
- `internal/core/reviewcontext/*`
- `internal/app/review/review.go`
- `internal/app/review/review_test.go`
- `internal/app/config/config.go`
- `internal/app/config/config_test.go`
- `internal/app/config/doc.go`
- `internal/core/review/model.go`
- `internal/adapters/cli/cli.go`
- `internal/adapters/cli/cli_test.go`
- `internal/adapters/cli/helpers.go`
- `internal/adapters/cli/output/output.go`
- `internal/adapters/cli/review/help.go`
- `internal/adapters/cli/review/selection.go`
- `internal/adapters/cli/review/selection_test.go`
- `internal/adapters/config/config.go`
- `internal/adapters/config/config_test.go`
- `internal/adapters/corebundle/bundle.go`
- `internal/adapters/corebundle/bundle_test.go`
- `internal/adapters/cli/help/help.go`
- `internal/adapters/cli/config/doc.go`
- `internal/adapters/cli/config/render.go`
- `internal/adapters/cli/config/scanner.go`
- `internal/adapters/cli/config/scanner_test.go`
- `docs/installation.md`
- `docs/quickstart.md`
- `docs/introduction.md`
- `docs/execution.md`
- `README.md`
- `docs/cli-reference.md`
- `internal/adapters/corebundle/assets/core/config.yaml`
- `internal/adapters/corebundle/assets/core/README.md`
- `internal/adapters/corebundle/assets/core/OPERATORS.md`
- `internal/adapters/corebundle/assets/core/prompts/review.md`
- `internal/adapters/corebundle/assets/prompts/review.md`
- `.scafld/core/README.md`
- `.scafld/core/OPERATORS.md`
- `.scafld/core/config.yaml`
- `.scafld/core/prompts/review.md`
- `.scafld/prompts/review.md`
- `docs/review.md`
- `docs/run-artifacts.md`
- `docs/configuration.md`
- `docs/cli-reference.md`

Invariants:
- `no_legacy_code` - do not add compatibility fallbacks or hidden legacy path behavior.
- `domain_boundaries` - keep context assembly in core/app with adapters only supplying IO.
- `no_test_logic_in_production` - context fixtures stay in tests.
- `config_from_env` - never include local secrets or env-only values in reviewer context.

Related docs:
- `docs/review.md`
- `docs/run-artifacts.md`
- `docs/configuration.md`
- `docs/cli-reference.md`

## Objectives

- Define a typed review-context packet that captures the full reviewer brief as
  data before it is rendered into provider prompt text.
- Include product contract context, not only task-local context.
- Preserve determinism: same spec, session, config, and workspace baseline must
  produce the same packet bytes.
- Keep context bounded and inspectable so the reviewer gets enough signal
  without receiving an unbounded repo dump.
- Make context provenance explicit so review findings can distinguish trusted
  session state, untrusted spec prose, docs, config, and archived precedent.
- Add a CLI debug surface to print the exact context packet without invoking a
  provider.
- Document "deterministic but intuitive" gates as a core scafld concept.
- Improve config detection for common real-world project layouts and
  execution environments.

## Scope

- Add a new core review-context model and Markdown renderer.
- Refactor `internal/app/review` to build a review-context packet and render the
  provider prompt from it.
- Add deterministic collectors for session evidence, scope/baseline state,
  config invariants, review agenda, agent guidance, related docs, schemas, and
  bounded project memory.
- Add config for review-context inclusion and byte budgets.
- Add `scafld review <task-id> --print-context` to render the exact provider
  context and exit without invoking the provider.
- Update docs and bundled prompts to explain the review context contract.
- Update introduction/execution/review docs so strict gates always come with
  explicit next-command repair context.
- Add config detectors for workspace manifests, common package subdirectories,
  Procfile/docker-compose surfaces, and nested version-manager files.
- Keep runtime config parsing strict while making `scafld update` the explicit,
  deterministic repair path for generated project config shape drift.

Out of scope:
- Semantic search over the repository.
- Model-generated summaries before review.
- Persisting review context as source of truth.
- Sending local secrets, `.scafld/config.local.yaml`, `.priv/**`, `.git/**`, or
  environment values to providers.
- Reintroducing review files under `.scafld/reviews/`.
- Reintroducing broad global workspace freezes that make multi-agent repos
  unmanageable.

## Dependencies

- Existing review baseline support in `internal/app/review`.
- Existing session replay support in `internal/core/session`.
- Existing config loading and defaults in `internal/adapters/config`.
- Existing review provider prompt flow through `review.Request.Prompt`.

## Assumptions

- The provider still receives Markdown on stdin or equivalent prompt input.
- The packet renderer, not the provider, decides what context is included.
- Context assembly must be deterministic and local; it cannot depend on network
  calls or provider-specific introspection.
- Archived specs can be useful product memory only when selected by deterministic
  rules, such as exact changed-path overlap, task tags, or configured memory
  files.
- Root `AGENTS.md` and `CLAUDE.md` are the primary agent-facing project contract.
  Bundled copies are managed sources and drift-test anchors, not extra duplicate
  prompt payload.

## Touchpoints

- Review provider prompt construction.
- Review CLI help and options.
- Config schema/defaults for review context.
- Core bundle config and review prompt.
- Docs for review, artifacts, configuration, and CLI reference.
- Tests for review determinism, context inclusion, and exclusion of sensitive
  local files.

## Risks

- Token bloat: mitigate with per-section byte budgets, deterministic truncation,
  and source references for omitted material.
- Secret leakage: hard-block known local/private paths and never include
  `.scafld/config.local.yaml` or environment values.
- False confidence from stale docs: include source paths and hashes; stale docs
  are visible as inputs, not hidden truth.
- Context drift across providers: build one provider-neutral packet and one
  renderer before provider selection.
- Overfitting to scafld-only needs: expose generic config for memory files and
  related docs while keeping scafld's own defaults in its repo config.

## Acceptance

Profile: strict

Validation:
- [x] `v1` test - Full Go test suite passes.
  - Command: `go test ./...`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-270
- [x] `v2` test - Architecture boundaries still hold.
  - Command: `go test ./internal/arch -run 'ImportBoundaries|CoreIsPure|CoreTransitiveDepsAreStdlib|AppDoesNotImportAdapters|PortsAreUseCaseOwned|PortsAreNarrow|ProviderBoundary|CLIIsThin'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-271
- [x] `v3` test - Release check surface passes.
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-272

## Phase 1: Core packet model and renderer

Status: completed
Dependencies: none

Objective: Complete this phase.

Changes:
- `internal/core/reviewcontext/model.go` (all, exclusive) - Add packet, section,
- `internal/core/reviewcontext/render.go` (all, exclusive) - Render deterministic
- `internal/core/reviewcontext/model_test.go` (all, exclusive) - Cover stable
- `internal/core/review/model.go` (partial, shared) - Extend `Request` to carry

Acceptance:
- [x] `ac1_1` test - Rendering is byte-stable and the context budget is global.
  - Command: `go test ./internal/core/reviewcontext -run 'TestRenderMarkdownIsDeterministic|TestRenderMarkdownIncludesSourceProvenance|TestRenderMarkdownTruncatesWithOmissionCount|TestRenderMarkdownAppliesBudgetAcrossSections'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-254
- [x] `ac1_2` test - Core remains pure stdlib.
  - Command: `go test ./internal/arch -run CoreTransitiveDepsAreStdlib`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-255

## Phase 2: Context assembly in review app

Status: completed
Dependencies: phase1

Objective: Complete this phase.

Changes:
- `internal/app/review/review.go` (partial, shared) - Replace ad hoc prompt
- `internal/app/review/context.go` (all, exclusive) - Add collector logic for
- `internal/app/review/review_test.go` (partial, shared) - Cover context
- `internal/adapters/config/config.go` (partial, shared) - Add
- `internal/adapters/config/config_test.go` (partial, shared) - Cover default

Acceptance:
- [x] `ac2_1` test - Review prompt includes config invariants, review agenda,
  - Command: `go test ./internal/app/review -run TestReviewPromptCarriesTaskContractToProvider`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-256
- [x] `ac2_2` test - Review context excludes local/private files even when
  - Command: `go test ./internal/adapters/cli/review -run TestSelectPrintContextLoadsConfiguredFilesAndSkipsPrivateInputs`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-257
- [x] `ac2_3` test - Repeated context builds are byte-identical for unchanged
  - Command: `go test ./internal/app/review -run TestReviewContextBuildIsStable`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-258

## Phase 3: CLI debug surface

Status: completed
Dependencies: phase2

Objective: Complete this phase.

Changes:
- `internal/adapters/cli/cli.go` (partial, shared) - Add
- `internal/adapters/cli/review/help.go` (partial, shared) - Document
- `internal/adapters/cli/review/selection.go` (partial, shared) - Share review
- `internal/adapters/cli/review/selection_test.go` (partial, shared) - Cover
- `internal/adapters/cli/cli_test.go` (partial, shared) - Cover CLI behavior
- `internal/adapters/cli/helpers.go` (partial, shared) - Keep option parsing
- `internal/adapters/cli/output/output.go` (partial, shared) - Surface
- `internal/adapters/cli/help/help.go` (partial, shared) - Move help into its
- `docs/cli-reference.md` (partial, shared) - Add the new flag and expected

Acceptance:
- [x] `ac3_1` test - `--print-context` prints context and does not invoke the
  - Command: `go test ./internal/adapters/cli -run TestReviewPrintContextDoesNotInvokeProvider`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-259
- [x] `ac3_2` test - Review help lists provider/model/scope/context flags.
  - Command: `go test ./internal/adapters/cli -run TestReviewHelpIncludesContextFlags`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-260

## Phase 4: Docs, config, and bundled prompt alignment

Status: completed
Dependencies: phase3

Objective: Complete this phase.

Changes:
- `docs/review.md` (partial, shared) - Add "Review Context" explaining the
- `docs/run-artifacts.md` (partial, shared) - Clarify that review context is
- `docs/configuration.md` (partial, shared) - Document `review.context` budgets,
- `docs/installation.md` (partial, shared) - Keep install/setup flow aligned
- `docs/quickstart.md` (partial, shared) - Keep first-run workflow aligned with
- `README.md` (partial, shared) - Add concrete spec/status/review/report
- `.scafld/config.yaml` (partial, shared) - Keep this repo's runtime config in
- `.scafld/core/README.md` (partial, shared) - Keep managed operator docs
- `.scafld/core/OPERATORS.md` (partial, shared) - Keep managed operator docs
- `internal/adapters/corebundle/assets/core/config.yaml` (partial, shared) -
- `internal/adapters/corebundle/assets/core/README.md` (partial, shared) -
- `internal/adapters/corebundle/assets/core/OPERATORS.md` (partial, shared) -
- `.scafld/core/config.yaml` (partial, shared) - Keep the managed core example
- `internal/adapters/corebundle/assets/core/prompts/review.md` (partial,
- `internal/adapters/corebundle/assets/prompts/review.md` (partial, shared) -
- `.scafld/core/prompts/review.md` (partial, shared) - Keep managed core prompt
- `.scafld/prompts/review.md` (partial, shared) - Refresh project prompt only

Acceptance:
- [x] `ac4_1` test - Core bundle drift checks pass for review prompt and config.
  - Command: `go test ./internal/adapters/corebundle`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-261
- [x] `ac4_2` test - Docs mention `review.context` and `--print-context`.
  - Command: `rg -n "review\\.context|--print-context|Review Context" docs internal/adapters/corebundle/assets/core/config.yaml >/dev/null`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-262

## Phase 5: Dogfood and regression proof

Status: completed
Dependencies: phase4

Objective: Complete this phase.

Changes:
- `test/e2e/lifecycle_test.go` (partial, shared) - Add an e2e scenario that
- `.scafld/specs/drafts/review-context-packet.md` (partial, shared) - Update

Acceptance:
- [x] `ac5_1` test - Lifecycle e2e covers context preview before provider
  - Command: `go test ./test/e2e -run TestReviewContextPreview`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-263
- [x] `ac5_2` test - Full release check passes after dogfood.
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-264

## Phase 6: Agent-facing docs and config maturity

Status: completed
Dependencies: phase3

Objective: Complete this phase.

Changes:
- `docs/introduction.md` (partial, shared) - Name agent-facing deterministic
- `docs/execution.md` (partial, shared) - Clarify repair flow for acceptance
- `docs/configuration.md` (partial, shared) - Document config as an
- `internal/app/config/config.go` (all, exclusive) - Rename configure use case
- `internal/app/config/config_test.go` (all, exclusive) - Keep use-case tests
- `internal/app/config/doc.go` (all, exclusive) - Keep package docs aligned
- `internal/adapters/cli/config/doc.go` (all, exclusive) - Rename CLI package
- `internal/adapters/cli/config/render.go` (partial, shared) - Render config
- `internal/adapters/cli/config/scanner.go` (partial, shared) - Add richer
- `internal/adapters/cli/config/scanner_test.go` (partial, shared) - Cover

Acceptance:
- [x] `ac6_1` test - Config detects nested package surfaces and version
  - Command: `go test ./internal/adapters/cli/config -run 'TestScannerDetectsNestedProjectSurfaces|TestScannerSuggestsExecutionEnvironmentFromRubyVersionManagers'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-265
- [x] `ac6_2` test - Docs name agent-facing deterministic gates and review
  - Command: `rg -n "Agent-facing deterministic gates|Review Context|scafld config" docs README.md >/dev/null`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-266

## Phase 7: Strict config repair path

Status: completed
Dependencies: phase6

Objective: Complete this phase.

Changes:
- `internal/adapters/config/config.go` (partial, shared) - Reject sequence
- `internal/adapters/config/config_test.go` (partial, shared) - Cover the
- `internal/adapters/corebundle/bundle.go` (partial, shared) - Render generated
- `internal/adapters/corebundle/bundle_test.go` (partial, shared) - Cover
- `internal/adapters/cli/help/help.go` (partial, shared) - Explain the config
- `.scafld/specs/active/review-context-packet.md` (partial, shared) - Keep
- `README.md`, `docs/cli-reference.md`, `docs/configuration.md`,

Acceptance:
- [x] `ac7_1` test - Strict config loading rejects list-shaped invariant
  - Command: `go test ./internal/adapters/config -run TestConfigRejectsInvariantListWithUpdateHint`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-267
- [x] `ac7_2` test - `scafld update` normalizes generated invariant lists to
  - Command: `go test ./internal/adapters/corebundle -run 'TestUpdateNormalizesProjectConfigInvariantList|TestUpdateLeavesCurrentProjectConfigShapeAlone'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-268
- [x] `ac7_3` test - `scafld update --help` tells agents that update renders
  - Command: `go run ./cmd/scafld update --help | rg "strict runtime shape"`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-269

## Rollback

Strategy: per_phase

Commands:
- `git restore internal/core/reviewcontext internal/app/review internal/core/review internal/adapters/cli internal/adapters/config docs internal/adapters/corebundle .scafld/core .scafld/prompts`
- `git clean -fd internal/core/reviewcontext`

## Review

Status: completed
Verdict: pass

Findings:
- [non_blocking] `source_provenance_convention_undocumented` internal/app/review/review.go:559-574 hashes the rendered section body but labels Path as a file (e.g. .scafld/config.yaml#configured_invariants). The derived_ kind prefix and # fragment communicate this is a derived view, but neither reviewcontext.Source nor docs/review.md document that derived_* hashes are over the rendered slice rather than the file. Document the convention on Source or point Path at a stable record id.
- [non_blocking] `context_budget_default_is_tight` internal/adapters/config/config.go:189-200 ships a 16KiB MaxBytes default with seven default context files. The very packet rendered for this review reports 'context budget exhausted: omitted 39341 byte(s)', truncating later sections including acceptance_evidence. Consider raising the default or budgeting per-section so structured sections aren't crowded out by free-form docs.
- [non_blocking] `normalize_drops_invariant_descriptions_silently` internal/adapters/corebundle/bundle.go:122-131 converts list-shaped invariants.canonical to a mapping with empty description values. That is the right repair, but a strict reload then succeeds with invariants that have no human description. Surface this in scafld update output or update --help so operators know to fill descriptions after normalization.

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

Estimated effort hours: 12
Actual effort hours: none
AI model: none
React cycles: none

Tags:
- review
- context
- adversarial
- determinism

## Origin

Source:
- conversation: "review is only as good as the knowledge of the reviewing agent"

Repo:
- `github.com/nilstate/scafld`

Git:
- pending

Sync:
- none

Supersession:
- none

## Harden Rounds

### round-1

Status: passed
Started: 2026-05-08T04:21:35Z
Ended: 2026-05-08T04:22:11Z

Questions:
- Should `config` have any hidden command alias?
  - Grounded in: code:internal/adapters/cli/cli.go:48
  - Recommended answer: No. The command surface should expose one name only; hidden aliases create needless drift and undocumented behavior.
  - Answered with: Renamed command, import, package, docs, and bundled-core surfaces to `config`; no alias was added.
- How can an agent inspect reviewer context without spending another provider run?
  - Grounded in: code:internal/app/review/review.go:111
  - Recommended answer: Add `scafld review <task-id> --print-context` so the exact structured context packet is visible before provider invocation.
  - Answered with: Added typed context packet rendering and a print-only review path.
- How do we prevent review context from leaking local secrets?
  - Grounded in: code:internal/adapters/cli/review/selection.go:154
  - Recommended answer: Keep the configured file list deterministic but hard-skip private/local paths such as `.scafld/config.local.yaml`, `.priv/**`, `.git/**`, and `.env*`.
  - Answered with: Added safe context path filtering and tests for private path exclusion.
- How should config proposals handle nested Ruby apps in non-login shells?
  - Grounded in: code:internal/adapters/cli/config/scanner.go:349
  - Recommended answer: When a single nested `Gemfile` is detected and no root `Gemfile` exists, propose `BUNDLE_GEMFILE` plus version-manager shims as runtime execution config.
  - Answered with: Added nested package detection and `BUNDLE_GEMFILE` proposal support.


## Planning Log

- Observation: `internal/app/review.promptForModel` currently builds prompt text
  directly from the spec model, workspace baseline, task changes, scope drift,
  acceptance criteria, and configured review passes.
- Observation: `docs/review.md` already documents that reviewers receive task
  contract, declared scope, baseline, changes, evidence, read-only instruction,
  and configured agenda.
- Observation: `scafld handoff` exposes blocked acceptance and latest accepted
  review findings, but it is repair transport and not the same as a full
  provider review brief.
- Decision: implement a typed context packet rather than adding more ad hoc
  sections to the prompt string.
- Decision: deterministic local collectors only; no semantic search or model
  summarization before review.
- Observation: Dogfooding `scafld review --print-context` showed forbidden
  private/local paths from the spec's out-of-scope prose entering derived review
  scope.
- Decision: derived review scope must hard-exclude private/local runtime paths
  such as `.git`, `.priv`, `.env*`, `.scafld/config.local.yaml`, and
  `.scafld/reviews` even when a spec mentions them as exclusions.
