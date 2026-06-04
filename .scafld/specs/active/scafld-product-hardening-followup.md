---
spec_version: '2.0'
task_id: scafld-product-hardening-followup
created: '2026-05-04T12:04:38Z'
updated: '2026-05-18T09:30:18Z'
status: blocked
harden_status: passed
size: large
risk_level: high
---

# Bring scafld to release-quality product shape

## Current State

Status: blocked
Current phase: phase3
Next: repair
Reason: phase phase3 acceptance failed
Blockers: phase phase3 acceptance failed
Allowed follow-up command: `scafld handoff scafld-product-hardening-followup`
Latest runner update: 2026-05-18T09:30:18Z
Review finalize: not_started

## Summary

scafld has crossed the hard cutover to the Go runtime and regained the important product shape: living Markdown specs, hardening, external adversarial review, session-derived evidence, package wrappers, and release automation. The remaining work is not another rewrite. It is the discipline pass that makes the product release-quality: the Markdown spec must not lose information when scafld mutates it, every configurable claim must either drive runtime behavior or be clearly labeled as guidance, every review verdict must be bound to durable evidence and the reviewed workspace state, every provider path must survive real streaming and interruption failures, package releases must be repeatable across the supported install channels, and the README/docs must preserve the scafld voice without exaggerating what the runtime enforces.
This spec is the follow-up program to turn scafld from "working and promising" into "boringly trustworthy." The end state is a project where a developer can install scafld from npm, PyPI, GitHub releases, Homebrew, Go, or another supported channel, initialize a workspace, create and harden a plan, execute acceptance, pass adversarial review, complete the task, and audit the evidence without knowing any implementation history.

## Objectives

- Make every runtime-read config key explicit, tested, and documented.
- Make Markdown parse/render round trips preserve every documented living-spec section before relying on the spec as the readable source of truth.
- Remove, relocate, or label config that is guidance rather than runtime behavior.
- Make validation profiles and configured validation commands either executable or explicitly out of the runtime contract.
- Seal review verdicts to durable packets, provider provenance, reviewed git state, and session evidence.
- Bring provider transport to production-grade behavior for streaming, timeouts, diagnostics, mutation detection, and interrupt cleanup.
- Make packaging and release repeatable across Go, GitHub releases, npm, PyPI, and at least one OS package manager path.
- Preserve the scafld product voice in README/docs while keeping claims tied to enforceable behavior.
- Prove the product by dogfooding scafld on this follow-up work before release.

## Scope

- In scope: runtime config audit and enforcement.
- In scope: Markdown spec grammar, parser, renderer, golden tests, and no-loss mutation discipline.
- In scope: validation pipeline design and implementation if it remains in config.
- In scope: review packet persistence, canonical hashing, provenance, and complete-gate freshness checks.
- In scope: provider transport resilience for Codex, Claude, command, and local review paths.
- In scope: package wrapper consistency for npm and PyPI.
- In scope: Go install, GitHub release assets, and OS package distribution documentation or automation.
- In scope: session-derived reporting for the product metrics scafld claims.
- In scope: README/docs rewrite where needed to keep the scafld voice and harden/review philosophy.
- In scope: dogfood proof using scafld itself.
- Out of scope: supporting `.ai/` workspaces.
- Out of scope: supporting YAML task specs.
- Out of scope: preserving Python runtime internals as production fallback logic.
- Out of scope: adding a hosted service.

## Dependencies

- Current Go runtime remains the primary implementation.
- Existing npm and PyPI package names remain `scafld`.
- GitHub repository identity remains `nilstate/scafld` while the product name remains 0state scafld.
- Go module path remains `github.com/nilstate/scafld/v2` for semantic import versioning.
- External provider CLIs are optional for automated CI, but real external dogfood must be run before release.
- Current `.scafld/specs/**/*.md` grammar remains the supported task-spec format.

## Assumptions

- 2.1.0 is allowed to be the first release where the Go runtime is the primary implementation.
- Package wrappers should download or locate released Go binaries rather than reimplement runtime behavior.
- Local review remains a development/smoke escape hatch only; it is not a valid substitute for adversarial review in release proof.
- If a config section is useful only as agent guidance, docs and comments must say that plainly.
- Release automation can start with one OS package manager path if adding every registry in one task would dilute quality.
- Work should be split into release blockers and follow-up polish; parser/render fidelity, review sealing, and package release integrity block the next release, while expanded metrics and additional registries can land afterward.

## Touchpoints

- Config loader, config schema, and core config comments.
- Markdown spec parser, renderer, examples, and golden fixtures.
- Hardening prompt composition and project/core prompt override behavior.
- Review provider selection and review prompt agenda composition.
- Review packet schema, provider output parsing, and session review entries.
- Complete finalize verification and reviewed-state freshness checks.
- Process runner capture limits, diagnostics, timeout behavior, and signal cleanup.
- Git workspace state hashing used for mutation detection.
- Package wrappers and release artifact naming.
- README badges, install sections, and high-level product explanation.
- E2E, release, parity, architecture, provider, config, session, and review tests.

## Risks

- none

## Acceptance

Profile: strict

Validation:
- [ ] `v1` test - Full check passes.
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `v2` release - Snapshot release artifacts build.
  - Command: `make release-snapshot`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `v3` boundary - Legacy workspace and YAML task-spec logic are absent from production code and docs.
  - Command: `rg -n '\\.ai/|\\.ai/config|\\.yaml task spec|YAML task spec|legacy Python runtime|python runtime fallback' README.md docs internal cmd package .scafld/core`
  - Expected kind: `no_matches`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `v4` contract - This spec validates under the current Markdown runtime.
  - Command: `go run ./cmd/scafld validate scafld-product-hardening-followup`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `v5` manual - Real external review is run with at least one installed external challenger before release.
  - Expected kind: `manual`
  - Status: pending
  - Evidence: none

## Phase 1: Markdown grammar, config truth, and no-loss persistence

Status: completed
Dependencies: none

Objective: Make the living spec safe to mutate and make config an honest operator surface. scafld must preserve documented Markdown sections when it opens harden/build/review/complete rounds, and runtime-read config keys must be typed, tested, documented, and clearly separated from guidance-only policy.

Changes:
- `internal/adapters/markdown/**` - Parse and render every documented spec section that scafld ships in examples and docs, including context files, invariants, related docs, risks, definition of done, review details, origin, harden rounds, and planning log.
- `internal/adapters/markdown/**` - Add no-loss golden tests that parse, render, save, reload, and compare real core examples plus this follow-up spec.
- `.scafld/core/specs/examples/*.md` - Keep examples aligned with the parser grammar and byte-stable under round trip.
- `internal/adapters/config/**` - Define the full typed config surface that the runtime reads today. Include base config, local overlay, defaults, validation errors, and tests for provider/model/timeouts/harden/review passes.
- `internal/adapters/cli/harden/**` - Keep harden config composition outside thin CLI command handlers and test prompt injection for `harden.max_questions_per_round`.
- `internal/adapters/cli/review/**` - Keep review provider/config composition outside thin CLI command handlers and test config, local overlay, CLI flag precedence, fallback policy, and review pass agenda.
- `.scafld/config.yaml` - Remove dead runtime keys. Clearly label guidance-only project policy sections.
- `.scafld/core/config.yaml` - Match project config comments and keep it suitable for new workspace initialization.
- `internal/adapters/corebundle/assets/core/config.yaml` - Keep embedded managed config byte-aligned with `.scafld/core/config.yaml`.
- `docs/configuration.md` - Document config precedence, runtime-read fields, local overrides, guidance-only sections, and examples.

Acceptance:
- [x] `ac1_1` test - Markdown, config, harden, review selection, and CLI tests pass.
  - Command: `go test ./internal/adapters/markdown ./internal/testkit/golden ./test/parity ./internal/adapters/config ./internal/adapters/cli/harden ./internal/adapters/cli/review ./internal/adapters/cli`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6
- [x] `ac1_2` boundary - Hardening this spec does not drop documented sections.
  - Command: `go run ./cmd/scafld validate scafld-product-hardening-followup && go test ./internal/adapters/markdown -run 'RoundTrip|Golden|Examples'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-7
- [x] `ac1_3` boundary - Dead review runner config is absent.
  - Command: `rg -n 'review\\.runner|runner:' .scafld/config.yaml .scafld/core/config.yaml internal/adapters/corebundle/assets/core/config.yaml docs README.md internal`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-8
- [x] `ac1_4` smoke - Init installs base config, local config template, core config, and project prompt overrides.
  - Command: `go test ./test/e2e -run TestInit`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-9
- [x] `ac1_5` boundary - CLI adapter stays thin after config composition.
  - Command: `go test ./internal/arch -run TestCLIIsThin`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-10

## Phase 2: Runtime validation pipeline

Status: completed
Dependencies: phase1

Objective: Decide and implement the validation contract. If validation profiles remain in config, scafld must execute them predictably; if they are agent guidance, they must be moved or labeled so users do not confuse them with enforced gates.

Changes:
- `internal/core/acceptance/**` - Keep criterion `expected_kind` behavior explicit and covered for `exit_code_zero`, `exit_code_nonzero`, `no_matches`, and `manual`.
- `internal/app/build/**` - Resolve validation profile from spec risk/profile and execute configured per-phase validation commands only if this becomes runtime behavior.
- `internal/app/complete/**` - Run or verify configured pre-commit validation only if this becomes a release finalize.
- `internal/adapters/config/**` - Add typed validation profile fields only for runtime-enforced behavior; otherwise move profile comments to docs/prompt guidance.
- `.scafld/core/prompts/build.md` - If validation profiles remain guidance, ensure the build prompt explains how agents should use them without claiming runtime enforcement.
- `docs/validation.md` - State the exact ownership split between acceptance criteria, validation profiles, manual evidence, and complete gates.
- `test/e2e/**` - Add lifecycle smoke coverage for profile resolution and validation command failure behavior if profiles are runtime-enforced.

Acceptance:
- [x] `ac2_1` test - Acceptance expected-kind semantics are covered.
  - Command: `go test ./internal/core/acceptance ./internal/app/build`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-22
- [x] `ac2_2` test - Validation docs and runtime tests agree on whether config profiles are enforced.
  - Command: `go test ./internal/adapters/config ./test/e2e`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-23
- [x] `ac2_3` boundary - Docs do not claim validation config is runtime-enforced unless tests prove it.
  - Command: `rg -n 'runtime-enforced validation|validation profiles are enforced|complete runs pre_commit|build runs per_phase' docs .scafld/core/prompts .scafld/config.yaml .scafld/core/config.yaml`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-24
- [x] `ac2_4` boundary - Product decision is recorded in docs: validation profiles are explicit agent guidance, not hidden runtime policy.
  - Command: `rg -n 'Validation profiles are agent guidance, not hidden runtime gates' docs/validation.md`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-25

## Phase 3: Review evidence, provenance, and complete finalize sealing

Status: blocked
Dependencies: phase1

Objective: Make a review verdict auditable and fresh. A passing review must be tied to the provider output, validated packet, reviewed git state, session entry, and spec projection that complete relies on.

Changes:
- `internal/core/review/**` - Extend ReviewPacket validation for stable fields, finding severity, provider provenance, and canonical response hashing.
- `internal/app/review/**` - Persist review packet artifacts, canonical packet hash, provider name/model/session when available, reviewed head, reviewed dirty state, reviewed diff hash, and review pass agenda.
- `internal/adapters/providers/**` - Parse and expose provider provenance such as Claude `system.init` model/session fields and Codex output metadata where available.
- `internal/app/complete/**` - Refuse completion when the latest passing review packet is missing, hash-mismatched, stale against current workspace state, or not reflected in session.
- `internal/core/session/**` - Keep review events append-only and replayable without trusting spec checkmarks.
- `.scafld/core/schemas/review_packet.json` - Match the runtime packet contract.
- `docs/review.md` - Explain what a passing review proves, what it does not prove, and how stale reviews are rejected.
- `docs/run-artifacts.md` - Document review packet, hash, provenance, and complete-gate artifacts.

Acceptance:
- [x] `ac3_1` test - Review packet validation, persistence, and provenance tests pass.
  - Command: `go test ./internal/core/review ./internal/app/review ./internal/adapters/providers`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-30
- [x] `ac3_2` test - Complete rejects stale or tampered review evidence.
  - Command: `go test ./internal/app/complete ./test/e2e -run 'Review|Complete'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-31
- [ ] `ac3_3` boundary - Review sealing fields are present in code or schema.
  - Command: `rg -n 'canonical_response_sha256|reviewed_head|reviewed_dirty|reviewed_diff|provider_model|provider_session|review_packet' internal .scafld/core/schemas docs`
  - Expected kind: `exit_code_zero`
  - Status: fail
  - Evidence: exit code was 1
  - Source event: entry-32
- [ ] `ac3_4` boundary - Complete cannot rely only on spec text for review success.
  - Command: `rg -n 'Review\\.Verdict ==|Review\\.Status ==' internal/app/complete internal/app/review internal/core/session`
  - Expected kind: `exit_code_zero`
  - Status: fail
  - Evidence: exit code was 1
  - Source event: entry-33

## Phase 4: Provider transport and failure diagnostics

Status: pending
Dependencies: phase3

Objective: Prove provider execution remains boring under failure. The runner already has bounded capture, liveness diagnostics, event counts, idle/absolute timeouts, and hashed git-state mutation detection; this phase should close remaining interrupt/provenance/test gaps rather than re-implement what already exists.

Changes:
- `internal/adapters/process/**` - Keep capped capture buffer, liveness diagnostic summary, time since last byte, timeout thresholds, event counts, and kill reason covered by tests.
- `internal/platform/signal/**` and `cmd/scafld/**` - Ensure cancellation and second interrupt behavior cannot orphan provider subprocesses or skip necessary cleanup.
- `internal/adapters/providers/**` - Keep Claude streaming result extraction, system init provenance, partial message liveness, and event summaries covered by tests.
- `internal/adapters/providers/**` - Keep Codex output-file handling, schema path handling, and fallback-to-stdout behavior covered by tests.
- `internal/adapters/git/**` and `internal/app/review/**` - Keep workspace additions, deletions, and content modifications detectable through hashed changed-state fingerprints.
- `internal/testkit/providerfake/**` - Keep stream, idle, endless, mutation, invalid packet, and crash-mid-stream modes aligned with real provider contracts.
- `docs/review.md` - Document timeout meanings, mutation guard behavior, and diagnostic locations.

Acceptance:
- [ ] `ac4_1` test - Process runner timeout, capture, and signal tests pass under race.
  - Command: `go test -race ./internal/adapters/process ./internal/platform/signal`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac4_2` test - Provider adapters and provider fakes cover all failure modes.
  - Command: `go test ./internal/adapters/providers ./internal/testkit/providerfake ./internal/app/review`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac4_3` boundary - Provider diagnostics include kill reason and timeout context.
  - Command: `rg -n 'kill_reason|time_since_last_byte|idle_timeout|absolute_max|event summary|event_summary|last_activity' internal/adapters/process internal/adapters/providers docs`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac4_4` test - Mutation guard detects content modifications, additions, deletions, and untracked files.
  - Command: `go test ./internal/adapters/git ./internal/app/review -run 'Mutation|ChangedFiles|Status'`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none

## Phase 5: Package and release architecture

Status: pending
Dependencies: phase1, phase4

Objective: Make package releases modern and repeatable. All installers must resolve the same versioned Go binary, and release checks must fail before a broken npm/PyPI/GitHub release can ship.

Changes:
- `go.mod` - Keep module path `github.com/nilstate/scafld/v2` and document why semantic import versioning requires `/v2`.
- `cmd/scafld` and release build scripts - Use one source of truth for version injection.
- `.github/workflows/**` - Build matrix for supported OS/arch pairs, run `make check`, build release artifacts, and publish only after all gates pass.
- `package/npm/**` - Keep npm package as a thin installer/wrapper for the released Go binary. Add install smoke tests for platform mapping and version.
- `package/pypi/**` - Keep PyPI package as a thin installer/wrapper for the released Go binary. Add build and install smoke tests.
- `scripts/**` - Add or refine release helpers for snapshot, checksum, package smoke, tag verification, and registry publication checks.
- `docs/installation.md` - Separate install paths for Go, npm, PyPI, GitHub releases, Homebrew, and any other supported package manager.
- `docs/release.md` - Document exact release procedure, rollback, and publication verification for npm, PyPI, GitHub, and Go module discovery.
- `README.md` - Keep install commands clear and separate.

Acceptance:
- [ ] `ac5_1` release - Snapshot artifacts build for supported platforms.
  - Command: `make release-snapshot`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac5_2` test - Release package tests pass.
  - Command: `go test ./test/release ./test/ci`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac5_3` npm - npm wrapper packs cleanly.
  - Command: `npm --prefix package/npm pack --dry-run`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac5_4` pypi - PyPI wrapper builds cleanly in an isolated environment.
  - Command: `python3 -m venv /tmp/scafld-pypi-build && /tmp/scafld-pypi-build/bin/python -m pip install build >/dev/null && /tmp/scafld-pypi-build/bin/python -m build package/pypi`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac5_5` manual - Release docs name each supported registry and the verification command for that registry.
  - Expected kind: `manual`
  - Status: pending
  - Evidence: none

## Phase 6: Session-derived reporting and product metrics

Status: pending
Dependencies: phase3

Objective: Make scafld's quality claims measurable from session evidence rather than prose. Report should surface task state today and preserve a clear path to first-attempt pass rate, recovery convergence, and challenge override metrics.

Changes:
- `internal/app/report/**` - Add session-derived aggregate metrics for completed tasks, failed tasks, review verdicts, criterion pass/fail counts, first-attempt pass rate, and recovery convergence where evidence exists.
- `internal/core/session/**` - Ensure replay exposes the states report needs without duplicating parser logic outside owned replay packages.
- `internal/adapters/cli/**` - Keep report output stable in text and JSON envelopes.
- `docs/run-artifacts.md` - Explain which metrics are canonical and which evidence fields produce them.
- `README.md` - Tie product claims to measurable session-derived outcomes.
- `test/e2e/**` - Add report smoke with a synthetic session ledger.

Acceptance:
- [ ] `ac6_1` test - Report and session replay tests pass.
  - Command: `go test ./internal/app/report ./internal/core/session ./internal/core/reconcile`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac6_2` smoke - Report command works in an initialized workspace.
  - Command: `go test ./test/e2e -run Report`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac6_3` boundary - README metrics claims are session-derived and not model-magic claims.
  - Command: `rg -n 'first_attempt_pass_rate|recovery_convergence_rate|challenge_override_rate|session-derived|session derived' README.md docs internal/app/report internal/core/session`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none

## Phase 7: Documentation, examples, and product voice

Status: pending
Dependencies: phase1, phase2, phase3, phase5, phase6

Objective: Make the public surface feel like scafld again: clear, opinionated, accurate, and centered on hardening plus adversarial review rather than generic task running.

Changes:
- `README.md` - Preserve the product voice, explain the lifecycle, hardening, adversarial review, living spec, evidence ledger, and package install paths.
- `README.md` - Add only badges that can be kept green: CI, Go Report Card if clean, Go Reference, npm version, PyPI version, GitHub release.
- `docs/review.md` - Describe adversarial review, provider configuration, read-only behavior, packet validation, stale review rejection, and repair flow.
- `docs/configuration.md` - Keep runtime config and guidance-only config clearly separated.
- `docs/planning.md` and `docs/validation.md` - Explain how hardening improves plan specs and how acceptance evidence is checked off.
- `docs/installation.md` - Keep each install path in its own subsection.
- `.scafld/core/specs/examples/*.md` - Keep examples readable, sentinel-free, and byte-stable under parse/render.
- `.scafld/core/prompts/*.md` - Keep prompt context aligned with the Go runtime and docs.

Acceptance:
- [ ] `ac7_1` docs - Documentation parity and golden tests pass.
  - Command: `go test ./test/parity ./internal/testkit/golden ./internal/adapters/markdown`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac7_2` boundary - Docs do not mention old sentinel behavior, YAML task specs, or Python runtime fallback.
  - Command: `rg -n 'sentinel|scafld:|YAML task spec|\\.yaml spec|Python runtime|\\.ai/' README.md docs .scafld/core .scafld/prompts`
  - Expected kind: `no_matches`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac7_3` manual - README clearly explains why harden matters and why review is adversarial.
  - Expected kind: `manual`
  - Status: pending
  - Evidence: none
- [ ] `ac7_4` manual - Install section lists Go, npm, PyPI, GitHub release, and OS package options separately.
  - Expected kind: `manual`
  - Status: pending
  - Evidence: none

## Phase 8: Dogfood proof and release readiness

Status: pending
Dependencies: phase1, phase2, phase3, phase4, phase5, phase6, phase7

Objective: Prove the product by using scafld to complete this follow-up work. The release candidate should be self-hosted, hardened, reviewed by an external challenger, and packaged from a clean tree.

Changes:
- `.scafld/specs/drafts/scafld-product-hardening-followup.md` - Harden this spec before approval and keep questions/evidence in the living document.
- `.scafld/runs/scafld-product-hardening-followup/**` - Record build, review, diagnostics, and session evidence.
- `.scafld/reviews/scafld-product-hardening-followup.md` - Store the human-readable adversarial review projection.
- Release workflow or local release checklist - Verify git tag, GitHub release artifacts, npm, PyPI, and Go module visibility after publication.
- `docs/release.md` - Capture any release correction learned during dogfood.

Acceptance:
- [ ] `ac8_1` lifecycle - Full local check passes from a clean tree.
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac8_2` release - Snapshot release builds from the release candidate.
  - Command: `make release-snapshot`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac8_3` dogfood - This spec has passed hardening before approval.
  - Command: `go run ./cmd/scafld status scafld-product-hardening-followup`
  - Expected kind: `exit_code_zero`
  - Status: pending
  - Evidence: none
  - Source event: none
- [ ] `ac8_4` manual - External adversarial review is run with Codex or Claude, not only the local smoke provider.
  - Expected kind: `manual`
  - Status: pending
  - Evidence: none
- [ ] `ac8_5` manual - Release publication is verified for GitHub release, npm, PyPI, and Go module discovery before announcing the version.
  - Expected kind: `manual`
  - Status: pending
  - Evidence: none

## Rollback

- Keep the current passing Go runtime and package wrappers as the baseline.
- If a phase introduces package/release breakage, revert that phase before tagging.
- If review sealing blocks valid completions, keep packet persistence and disable only the stale-review check until the state model is corrected.
- If validation profiles become too broad, move them back to prompt/docs guidance rather than shipping half-enforced gates.
- Do not reintroduce `.ai/`, YAML task specs, or Python runtime fallback logic as rollback paths.

## Review

Status: not_started
Verdict: none

## Self Eval

- Completeness: targets config, validation, review, transport, packaging, reporting, docs, dogfood, and release readiness.
- Architecture fidelity: keeps composition in CLI-specific adapters, runtime policy in app/core, and platform behavior in adapters/platform packages.
- Spec alignment: preserves Markdown 2.0 shape and no legacy fallback rule.
- Validation depth: uses unit, arch, e2e, parity, release, manual external review, and package smoke gates.

## Deviations

- none

## Metadata

- ai_model: codex
- estimated_effort_hours: 80-140
- priority: release-quality
- release_target: 2.1.x

## Origin

Created by: codex
Source: operator request for release-quality follow-up shape

## Harden Rounds

### round-1

Status: passed
Started: 2026-05-04T12:09:42Z
Ended: 2026-05-04T12:13:55Z

Questions:
- Should parser/render fidelity be a release blocker before config, review, and package polish?
  - Grounded in: code:internal/adapters/markdown/spec_store.go:267
  - Recommended answer: Yes. The harden command itself saves through the Markdown renderer, and the current parser only handles selected sections before re-rendering. A living spec cannot be trusted until documented sections round-trip without loss.
  - If unanswered: Make Markdown no-loss persistence Phase 1 and require golden tests on core examples plus this spec.
  - Answered with: Make Markdown no-loss persistence part of Phase 1 and block release on it.
- Is validation config runtime policy or agent guidance?
  - Grounded in: code:internal/app/build/build.go:70
  - Recommended answer: Treat validation config as guidance unless this work adds a real validation runner. Build currently executes spec acceptance criteria, not `.scafld/config.yaml` validation profiles.
  - If unanswered: Label validation profiles guidance-only and avoid claiming runtime enforcement.
  - Answered with: Keep Phase 2 as the explicit decision/implementation point; no docs may claim runtime validation profiles until tests prove it.
- What is authoritative for completing a reviewed task?
  - Grounded in: code:internal/app/complete/complete.go:34
  - Recommended answer: Completion should trust a sealed session review event plus packet artifact and reviewed workspace state, not only the spec's projected Review fields.
  - If unanswered: Complete must reject spec-only review success.
  - Answered with: Phase 3 makes review packet persistence, canonical hash, reviewed git state, and session evidence the complete-gate authority.
- Which provider transport work is actually still missing?
  - Grounded in: code:internal/adapters/process/runner.go:18
  - Recommended answer: Do not re-add already implemented transport features. The runner already has bounded capture and diagnostics; providers already expose Claude provenance and event summaries. Focus on regression tests, second-interrupt cleanup, and documentation.
  - If unanswered: Rewrite Phase 4 as proof/regression hardening rather than implementation from scratch.
  - Answered with: Phase 4 now targets proof and remaining cleanup, not duplicate transport implementation.
- Does the mutation guard need full-state hashing?
  - Grounded in: code:internal/adapters/git/git.go:51
  - Recommended answer: The git adapter already fingerprints status plus file hash, so content modifications are visible when git reports the file changed. The remaining need is tests for modified, added, deleted, and untracked cases.
  - If unanswered: Add mutation-guard tests instead of inventing a second workspace state system.
  - Answered with: Phase 4 acceptance now requires mutation tests for modifications, additions, deletions, and untracked files.
- Are npm, PyPI, and Go separate implementations or wrappers over one binary?
  - Grounded in: code:.github/workflows/release.yml:77
  - Recommended answer: They are wrappers/installers over the same released Go binary. The release workflow should validate wrappers, publish after GitHub artifacts exist, and verify registry visibility.
  - If unanswered: Keep one Go runtime source of truth and make package checks prove wrappers resolve the same version.
  - Answered with: Phase 5 keeps all package channels as installer/wrapper paths for the same binary.
- What should block the next release versus later polish?
  - Grounded in: spec_gap:release_scope
  - Recommended answer: Block on Markdown no-loss persistence, review sealing/freshness, provider cleanup, and package integrity. Keep expanded reporting and additional package-manager automation as follow-up if needed.
  - If unanswered: Treat every phase as a release blocker and risk delaying release with noncritical polish.
  - Answered with: The assumptions now split release blockers from follow-up polish.
- How should this work be proven before publication?
  - Grounded in: spec_gap:dogfood_proof
  - Recommended answer: Dogfood this spec through harden, build, real external review, complete, `make check`, and `make release-snapshot`; local review is acceptable only for smoke tests.
  - If unanswered: Require at least one external provider review before announcing release readiness.
  - Answered with: Phase 8 requires external adversarial review and release publication verification.


## Planning Log

- none
