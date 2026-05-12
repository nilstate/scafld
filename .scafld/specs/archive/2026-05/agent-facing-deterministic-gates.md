---
spec_version: '2.0'
task_id: agent-facing-deterministic-gates
created: '2026-05-07T14:29:39Z'
updated: '2026-05-12T14:43:54Z'
status: completed
harden_status: passed
size: large
risk_level: high
---

# Agent-facing deterministic gates

## Current State

Status: completed
Current phase: final
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-05-12T14:43:54Z
Review gate: pass

## Summary

Make scafld's deterministic gates intuitive for agents by documenting the concept and standardizing failure, status, handoff, harden, configure, review, and report surfaces around trusted state, evidence, repair guidance, and next commands.

## Objectives

- Establish "agent-facing deterministic gates" as a core scafld concept in README and docs.
- Make every blocking gate expose the same repair contract: trusted state, failure reason, evidence path, expected shape, and allowed next command.
- Make `status --json` and `handoff` the canonical surfaces for agents after build, harden, configure, review, and complete failures.
- Remove any workflow path where an agent has to infer what happened by scraping diagnostics, stale spec prose, or unrelated workspace dirt.
- Preserve strict deterministic gates while making the operator and agent experience frictionless when the next action is legitimate.
- Add regression fixtures from real agent failure reports so the product hardens against observed misuse, not imagined edge cases.

## Scope

- In scope: README and docs concept work for deterministic gates as an agent-facing contract.
- In scope: shared terminology for gate, state, evidence, projection, next command, and repair contract.
- In scope: CLI output, JSON output, and handoff improvements for blocked build, harden validation, configure proposals, review failures, stale review state, and complete rejection.
- In scope: `status --json` schema additions where agents need machine-readable blockers and repair hints.
- In scope: prompt updates so executor, recovery, harden, review, and configure prompts point agents at the correct surfaces.
- In scope: tests that reproduce real failure reports: stale review snapshot confusion, broad dirty-monorepo review, missing harden citation shape guidance, pre-implementation acceptance failures, and poor handoff after blocked build.
- In scope: docs showing concrete sample spec, status JSON, ReviewPacket, blocked handoff, and gate failure output.
- In scope: review-provider progress output that reports liveness without streaming raw exploratory model stderr into the outer agent.
- In scope: a source-checkout `./bin/scafld` wrapper that executes current Go source instead of stale copied binaries during dogfood.
- Out of scope: weakening review, complete, harden, or validation gates to reduce friction.
- Out of scope: adding a hosted control plane or external dashboard.
- Out of scope: supporting any previous workspace protocol home.
- Out of scope: making agents modify `.scafld/config.yaml` automatically without explicit operator approval.

## Dependencies

- Current Go runtime and Markdown spec grammar.
- Existing session-first projection model.
- Existing `status --json`, `handoff`, `review --json`, and `report --json` surfaces.
- Current docs site structure under `docs/` and README positioning.
- Real agent failure reports from Nitrosend, Sourcey, and scafld dogfood runs.

## Assumptions

- Determinism is still the core product promise; friendliness means better state explanation, not permissive gates.
- Agents will misuse or ignore docs unless the CLI and handoff surfaces put the next action directly in front of them.
- A blocked gate should never require the agent to guess which diagnostic file matters.
- A dirty monorepo or submodule workspace is normal; scafld must distinguish baseline dirt from task-introduced drift.
- `status --json` is the stable integration surface for wrappers and external agents.
- Handoff is transport only and must not become another state source.
- Docs should teach the concept once, then every command page should refer back to the same gate contract.

## Touchpoints

- `README.md`
- `AGENTS.md`
- `CLAUDE.md`
- `docs/introduction.md`
- `docs/installation.md`
- `docs/quickstart.md`
- `docs/lifecycle.md`
- `docs/execution.md`
- `docs/review.md`
- `docs/run-artifacts.md`
- `docs/configuration.md`
- `docs/cli-reference.md`
- `docs/integrations.md`
- `.scafld/core/prompts/*.md`
- `.scafld/prompts/*.md`
- `internal/app/status`
- `internal/app/handoff`
- `internal/app/approve`
- `internal/app/build`
- `internal/app/configure`
- `internal/app/harden`
- `internal/app/review`
- `internal/app/complete`
- `internal/app/envelope`
- `internal/app/report`
- `internal/adapters/cli`
- `internal/adapters/cli/output`
- `internal/adapters/config`
- `internal/adapters/corebundle`
- `internal/adapters/filesystem`
- `internal/adapters/git`
- `internal/adapters/jsonstore`
- `internal/adapters/markdown`
- `internal/adapters/process`
- `internal/adapters/providers`
- `internal/core/gate`
- `internal/core/session`
- `internal/core/review`
- `internal/core/execution`
- `internal/core/workspace`
- `bin/scafld`
- `Makefile`
- `test/e2e`
- `test/parity`

## Risks

- Risk: Product docs become abstract philosophy again. Mitigation: require each docs section to include one concrete artifact or command output.
- Risk: Adding fields to JSON output breaks consumers. Mitigation: only add fields; do not rename or remove existing keys.
- Risk: Handoff becomes too verbose for agent context. Mitigation: use structured sections and keep raw diagnostics behind paths unless needed.
- Risk: Gate friendliness gets interpreted as softer gates. Mitigation: explicitly document "strict in what scafld trusts, generous in what scafld explains."
- Risk: The spec overlaps with `scafld-product-hardening-followup`. Mitigation: keep this spec focused on the agent-facing gate contract; leave packaging, full review sealing, and broad product hardening to the existing follow-up.

## Acceptance

Profile: strict

Validation:
- [x] `v1` command - Full repository validation passes.
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-461
- [x] `v2` command - This spec validates.
  - Command: `go run ./cmd/scafld validate agent-facing-deterministic-gates`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-462
- [x] `v3` command - Docs contain the core concept and repair-contract language.
  - Command: `rg -n "agent-facing deterministic gates|repair contract|trusted state|allowed next command|strict in what scafld trusts" README.md docs`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-463
- [x] `v4` command - Runtime and prompts stop referring agents to stale review storage paths.
  - Command: `rg -n "\\.scafld/reviews" README.md docs .scafld/core/prompts .scafld/prompts internal`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-464
- [x] `v5` command - Review-provider progress stays summary-oriented while raw stderr remains captured.
  - Command: `go test ./internal/adapters/process ./internal/adapters/providers ./internal/adapters/cli/review`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-465

## Phase 1: Core concept and docs vocabulary

Status: completed
Dependencies: none

Objective: Make agent-facing deterministic gates a named scafld concept with concrete examples and consistent language across the public docs.

Changes:
- Add a docs section that defines the gate contract: every gate must expose trusted state, failure reason, evidence path, expected shape, and allowed next command.
- Update the README to frame determinism as the product center and auditability as the consequence.
- Update lifecycle, execution, review, run-artifacts, configuration, and integrations docs to use the same vocabulary.
- Add concrete examples for blocked build, harden citation failure, review failure, stale review rejection, and configure proposal output.
- Remove or rewrite any docs that imply the agent should inspect undocumented storage paths before using `status --json` or `handoff`.

Acceptance:
- [x] `ac1_1` command - Core concept appears in public docs.
  - Command: `rg -n "agent-facing deterministic gates|repair contract|trusted state|failure reason|evidence path|expected shape|allowed next command" README.md docs`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-333
- [x] `ac1_2` command - Docs position auditability as a consequence, not the primary claim.
  - Command: `rg -n "auditable protocol|audit log for agent work|auditability is the core" README.md docs`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-334
- [x] `ac1_3` command - Docs no longer point review agents at the old review directory.
  - Command: `rg -n "\\.scafld/reviews" README.md docs .scafld/core/prompts .scafld/prompts`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-335

## Phase 2: Shared gate failure contract

Status: completed
Dependencies: phase1

Objective: Standardize how CLI commands explain deterministic failures so agents can repair without guessing.

Changes:
- Define a shared failure payload for gate failures with fields for `gate`, `status`, `reason`, `evidence`, `expected`, `actual`, `blockers`, and `next`.
- Apply the payload to human output and JSON output for harden citation failures, build acceptance failures, review failures, complete rejections, and configure validation errors.
- Preserve existing exit codes and add tests that assert failure output names the allowed follow-up command.
- Make error messages teach exact expected shapes when strict parsers reject harden questions, ReviewPackets, or config snippets.

Acceptance:
- [x] `ac2_1` command - CLI/app tests cover gate failure output.
  - Command: `go test ./internal/adapters/cli ./internal/app/build ./internal/app/harden ./internal/app/review ./internal/app/complete ./internal/adapters/config`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-336
- [x] `ac2_2` command - Harden citation failure output includes the expected Markdown shape.
  - Command: `go test ./internal/app/harden ./internal/adapters/markdown -run 'Citation|Harden|Question'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-337
- [x] `ac2_3` command - Complete rejection output includes evidence and next action.
  - Command: `go test ./internal/app/complete ./test/e2e -run 'Complete|Review'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-338

## Phase 3: Status and handoff as repair surfaces

Status: completed
Dependencies: phase2

Objective: Make `status --json` and `handoff` sufficient for the next agent after any blocked gate.

Changes:
- Extend `status --json` with additive fields for current gate, blocker list, evidence references, latest diagnostic references, and repair command.
- Make blocked build handoff include the first failed or pending criterion, command, expected kind, phase dependency context, and relevant diagnostics.
- Make review handoff include the latest accepted review findings from session state, not stale in-progress provider output.
- Make configure handoff/proposal output distinguish copyable runtime config patch from spec guidance and docs references.
- Ensure handoff text says which state is trusted and which artifacts are only supporting evidence.

Acceptance:
- [x] `ac3_1` command - Status and handoff tests pass.
  - Command: `go test ./internal/app/status ./internal/app/handoff ./test/e2e -run 'Status|Handoff|Blocked|Review'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-339
- [x] `ac3_2` command - `status --json` remains additive and valid JSON.
  - Command: `go test ./internal/adapters/cli -run 'StatusJSON|Handoff'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-340
- [x] `ac3_3` command - Handoff no longer forces agents to inspect diagnostics before seeing the blocker summary.
  - Command: `rg -n "inspect diagnostics first|read diagnostics before|\\.scafld/runs/.*/diagnostics" internal/app/handoff .scafld/core/prompts .scafld/prompts`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-341

## Phase 4: Plan, harden, and configure guidance

Status: completed
Dependencies: phase2

Objective: Make early gates self-teaching so agents produce valid specs and config proposals on the first pass more often.

Changes:
- Update `plan` prompts/docs to require executable acceptance only after the implementation phase owns the files needed by those commands.
- Update `harden` prompt/docs/errors to produce one canonical Markdown question format and to normalize old-but-recognizable shapes without data loss.
- Update `configure` docs/output to explain sparse project config, full core example config, local overrides, detected tooling, and operator approval boundaries.
- Add real-world detector fixtures for rbenv/asdf/pnpm/workspaces/submodules/dirty monorepos where missing environment setup caused false build failures.

Acceptance:
- [x] `ac4_1` command - Plan/harden/configure tests pass.
  - Command: `go test ./internal/app/plan ./internal/app/harden ./internal/adapters/cli/configure ./internal/adapters/config`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-342
- [x] `ac4_2` command - Configure detects local environment setup hints.
  - Command: `go test ./internal/adapters/cli/configure -run 'Detect|Proposal|Ruby|Node|Workspace|Env'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-343
- [x] `ac4_3` command - Harden parser preserves question data across canonicalization.
  - Command: `go test ./internal/adapters/markdown -run 'Harden|Canonical|Preserve'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-344

## Phase 5: Review observability and scope confidence

Status: completed
Dependencies: phase3

Objective: Prevent review from looking stale, silent, or unrelated to the task when an agent is waiting on it.

Changes:
- Stream provider lifecycle updates in the outer CLI so the calling agent can see review progress during long-running providers.
- Ensure `review --json` and `status --json` distinguish in-progress provider output from the latest accepted verdict.
- Make review scope/baseline explanations visible in status and handoff when dirty workspaces or submodules are present.
- Make review findings, provider diagnostics, and mutation-guard failures available through a stable evidence reference instead of a guessed file path.
- Add tests for stale snapshot confusion and broad dirty-monorepo review behavior.

Acceptance:
- [x] `ac5_1` command - Review status/progress tests pass.
  - Command: `go test ./internal/app/review ./internal/adapters/providers ./internal/adapters/process ./test/e2e -run 'Review|Provider|Baseline|Scope|Progress'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-345
- [x] `ac5_2` command - Review prompt and docs do not reference stale review storage.
  - Command: `rg -n "\\.scafld/reviews|latest review markdown file|review packet under reviews" README.md docs .scafld/core/prompts .scafld/prompts internal`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-346
- [x] `ac5_3` command - Provider progress remains visible without corrupting structured output.
  - Command: `go test ./internal/adapters/providers ./internal/adapters/process -run 'Progress|NDJSON|Structured|Diagnostic'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-347

## Phase 6: Report guidance and product metrics

Status: completed
Dependencies: phase3

Objective: Make `scafld report` express the product metrics discussed in docs and tell agents/operators what to improve next.

Changes:
- Ensure report metrics are derived from session ledgers, not static spec prose.
- Include first-attempt pass rate, recovery convergence, challenge override rate, review pass rate, baseline coverage, and blocked-gate distribution.
- Add human-readable guidance for low-confidence metrics, zero denominators, and next improvement areas.
- Update report docs and README examples to match actual output.

Acceptance:
- [x] `ac6_1` command - Report metrics tests pass.
  - Command: `go test ./internal/app/report ./internal/adapters/jsonstore ./test/e2e -run 'Report|Metrics'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-348
- [x] `ac6_2` command - Report docs match metric names emitted by JSON.
  - Command: `rg -n "first_attempt_pass_rate|recovery_convergence_rate|challenge_override_rate|review_pass_rate|workspace_baseline_coverage|blocked_gate_distribution" README.md docs internal/app/report`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-349

## Phase 7: Dogfood and regression fixtures

Status: completed
Dependencies: phase1, phase2, phase3, phase4, phase5, phase6

Objective: Prove the concept against the exact failure classes that prompted it.

Changes:
- Add e2e fixtures for dirty monorepo/submodule baseline separation, blocked build handoff, harden question canonicalization, stale review status, provider progress, and rbenv/asdf environment guidance.
- Dogfood this spec through harden, approve, build, review, and complete before release.
- Archive the dogfood evidence so future release review can inspect the session and gate outputs.
- Treat the completed scafld lifecycle as release evidence, not as a commandless manual acceptance criterion.

Acceptance:
- [x] `ac7_1` command - E2E regression suite passes.
  - Command: `go test ./test/e2e ./test/parity ./test/release`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-350
- [x] `ac7_2` command - Full check passes after dogfood.
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-351

## Rollback

- Revert docs concept pages and prompt changes.
- Revert additive status/handoff JSON fields and related tests.
- Revert shared gate failure payload helpers.
- Revert plan/harden/configure/review/report changes and e2e fixtures.
- Remove `.scafld/runs/agent-facing-deterministic-gates/` if the dogfood artifact itself must be discarded.

## Review

Status: completed
Verdict: pass
Mode: verify
Provider: codex
Output: codex.output_file
Summary: No open blockers found in static verification. The previously recorded provider-selection, status, handoff, and report issues appear fixed with targeted regression coverage. Runtime command/test execution could not be rerun in this read-only sandbox because Go needs a writable build directory.

Attack log:
- `ambient drift`: Workspace drift and declared scope comparison -> clean (Inspected `git status --short`, `git diff --stat`, and specific diffs. Broad changes are within the task’s declared runtime/docs/prompt scope; the two draft spec edits are narrow `exec.md` to `build.md` references caused by the command rename.)
- `runtime verification`: Lifecycle/status command execution -> skipped (Attempted `./bin/scafld status agent-facing-deterministic-gates --json`; blocked by the read-only sandbox because `go run` could not create its Go build work directory.)
- `review provider selection`: Previously reported review-provider selection blocker -> clean (Verified `internal/adapters/cli/review/selection.go` now wraps provider-selection failures with `output.ReviewProviderGateError`, and CLI tests assert human and JSON gate payloads.)
- `status repair output`: Previously reported status repair blocker -> clean (Verified `internal/adapters/cli/output/output.go` now renders `out.Repair` in `Status`, and status tests cover failed review attempts overriding next command to handoff.)
- `handoff after failed review attempt`: Previously reported handoff evidence blocker -> clean (Verified `internal/app/handoff/handoff.go` now prints failed review attempt reason and diagnostic path, with regression coverage in `handoff_test.go`.)
- `build gate`: Build phase sequencing and blocked handoff contract -> clean (Traced `internal/app/build/build.go` phase-open, phase-evidence, final-acceptance, and blocked repair paths. Tests cover no pre-implementation acceptance, one phase per invocation, current-phase blockers only, and env propagation.)
- `review/status/handoff state separation`: Review stale snapshot and accepted-vs-attempt state -> clean (Traced `latestReviewInfo`, `latestFailedReviewAttempt`, handoff review dossier filtering, and completion’s latest-review guard. Tests cover stale review after later build evidence and failed review attempts after a pass.)
- `report metrics`: Report zero-denominator guidance -> clean (Verified report human output now always prints the metrics block and tests assert `n/a (0/0)` guidance for empty output.)
- `harden gate`: Harden citation shape and canonicalization -> clean (Verified harden citation failures return `gate.Failure` with expected shape, and markdown parser tests cover mixed old/canonical harden question forms normalizing without duplicate questions.)

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
Started: 2026-05-07T15:39:21Z
Ended: 2026-05-07T15:40:33Z

Questions:
- none


## Planning Log

- 2026-05-08: Created after real dogfood reports showed the same product tension repeatedly: scafld was deterministic, but agents sometimes had to guess how to recover from strict gates.
