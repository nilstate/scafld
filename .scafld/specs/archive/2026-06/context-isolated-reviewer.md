---
spec_version: '2.0'
task_id: context-isolated-reviewer
created: '2026-06-02T10:50:31Z'
updated: '2026-06-04T09:08:41Z'
status: completed
harden_status: needs_revision
size: medium
risk_level: high
---

# Context-isolated independent reviewer, no fail-closed stall

## Current State

Status: completed
Current phase: final
Next: done
Reason: finalization receipt passed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-06-04T09:06:49Z
Review gate: not_started

## Summary

The finalize needs an independent reviewer that is always available, even when the only installed CLI is the same vendor that is driving the task. Today `AutoProviderInfo` in `internal/adapters/providers/provider.go` fails closed: when `fallback_policy` is `disable` (the default) and only the host provider is present, it returns an error and the run dead-ends. This spec adds a gate-path selector that classifies independence into the canonical `independence{level,distinct}` struct, always degrades to an `isolation_only` reviewer (same vendor or unknown host, fresh context, adversarial persona, running in the evidence sandbox) rather than stalling, and upgrades to `cross_vendor` only when both the detected host vendor and selected reviewer vendor are known and different. The existing CLI human review and harden paths keep their fail-closed behavior unchanged.

## Objectives

- Add a gate-path reviewer selector that never returns "no independent provider"; it always yields a runnable reviewer plus a stamped `independence{level,distinct}` classification.
- Define `independence.level` as the canonical two-value enum: `isolation_only` (the always-available floor, including unknown host identity) and `cross_vendor` (the upgrade when a known different vendor CLI exists).
- Reuse the existing `AutoProviderInfo` ordering, `autoProviderAvailable`, `DetectHostAgent`, and provider transport rather than reimplementing CLI detection or invocation, but add a provider/vendor normalizer that recognizes every supported reviewer provider (`codex`, `claude`, `gemini`).
- Keep `AutoProviderInfo`'s fail-closed stall intact for the CLI review and harden selectors; the finalize selector is the only caller that degrades gracefully.
- Make the `isolation_only` reviewer adversarial: select the host vendor's CLI when known, otherwise the first available supported CLI, request a fresh context, and run it through the evidence sandbox owned by `evidence-control-sandbox`.

## Scope

- In scope: a new gate-path selection function (in the providers adapter) that returns a provider selection plus `independence` and never errors when at least one supported CLI exists.
- In scope: the `independence{level,distinct}` value type with its enum and a classifier that maps normalized host vendor plus selected reviewer vendor to `isolation_only` or `cross_vendor`.
- In scope: removing the fail-closed STALL from the GATE path only, replacing it with degradation to `isolation_only`; the default `fallback_policy "disable"` stays the configured default and the CLI/harden paths still honor it.
- In scope: documenting, in code-doc and spec, what `isolation_only` catches (context contamination, self-congratulation, drift, forgotten acceptance criteria, claimed-but-not-done) and forfeits (correlated blind spots, same-model-wrong-twice).
- In scope: an explicit unknown-host rule: empty or unrecognized host identity can never prove cross-vendor independence and must stamp `isolation_only` with `distinct=false`.
- In scope: an explicit supported-reviewer-vendor rule: every provider in `autoProviderOrder`, including Gemini, normalizes to a known reviewer vendor for independence classification.
- Out of scope: the evidence scratch dir, fenced untrusted-data prompt, pinned binary, and env scrub. Those are the reviewer runtime owned by `evidence-control-sandbox`; this spec selects the reviewer and passes it through that sandbox, it does not build the sandbox.
- Out of scope: the diff definition and canonical file bytes. Owned by `commit-free-tree-fingerprint`.
- Out of scope: receipt assembly, the `independence` field serialization into the signed body, and ed25519 signing. Owned by `host-gate-and-receipt`; this spec exports the selector consumed by that future finalize use case.
- Out of scope: the CLI human `scafld review` path and the harden path. They keep fail-closed selection unchanged.

## Dependencies

- `evidence-control-sandbox` (spec 1): provides the reviewer runtime (evidence scratch dir, memory-autoload off, env scrub, pinned binary) that the selected `isolation_only` reviewer runs inside. This spec hands its chosen provider to that sandbox.
- `host-gate-and-receipt` (spec 4): consumes the `independence{level,distinct}` value this spec returns and the chosen `reviewer{vendor,...}` and stamps them into the signed receipt body.
- Repo fact: `AutoProviderInfo` (internal/adapters/providers/provider.go:181-198) already encodes host-aware provider ordering and the fail-closed stall; reuse its ordering and availability helpers.
- Repo fact: `DetectHostAgent` (provider.go:270) infers the host vendor from environ; reuse it for distinctness classification.
- Repo fact: default `FallbackPolicy` is `disable` (config.go Default(), Review.External and Harden.External); the finalize selector is independent of this value.
- Repo fact: `normalizeHostAgent` currently recognizes host identities, not every supported reviewer provider. This spec adds a separate reviewer-provider normalizer so `gemini` can qualify as a known distinct reviewer.

## Assumptions

- "Same vendor, fresh context" is achievable per provider: Claude via `--no-session-persistence` and a fresh `--session-id` (already in `ClaudeArgs`), Codex via `--ephemeral --ignore-user-config` (already in `CodexArgs`), Gemini via an isolated settings path (already in `GeminiProvider`). No new provider flags are required for context freshness.
- The adversarial persona is delivered through the reviewer prompt, which the `evidence-control-sandbox` spec assembles; this spec only marks the selection as adversarial/isolation_only so the prompt builder can branch.
- A reviewer is normally runnable on the finalize path because the host itself is expected to be a supported vendor CLI on PATH; worst case the finalize reviews same-vendor or unknown-host at `isolation_only`. If no supported CLI is available at all, the selector returns an error.

## Independence Contract

`classifyIndependence(hostVendor, reviewerVendor string) Independence` normalizes host identity with host normalization and reviewer identity with a separate supported-provider normalizer. The reviewer normalizer recognizes every provider in `autoProviderOrder`: `codex`, `claude`, and `gemini`. It returns `cross_vendor` and `distinct=true` only when both normalized values are non-empty and different. It returns `isolation_only` and `distinct=false` when either value is empty/unknown or when the vendors match. Receipt assembly must use this value directly; it must not infer distinctness from raw strings.

## Gate Consumer Contract

This spec is provider-adapter scope only. It exports the selector for `host-gate-and-receipt`; it does not wire the existing `scafld review` app path and does not remove the current fail-closed stall from ordinary review. The selector returns a gate-facing provider selection, not an `internal/app/review.Provider` implementation:

```go
type GateReviewerSelection struct {
    Provider string
    Binary string
    Model string
    Independence Independence
}
```

`host-gate-and-receipt` composes this selection with the receipt-grade invocation path from `evidence-control-sandbox`. Ordinary `scafld review` and `scafld harden` keep using `AutoProviderInfo`/`providers.Select` and keep their current fail-closed behavior.

## Touchpoints

- internal/adapters/providers/provider.go
- internal/adapters/config/config.go
- internal/adapters/cli/review/selection.go
- internal/adapters/cli/harden/selection.go
- internal/app/review/
- internal/adapters/providers/provider_test.go

## Risks

- Misclassifying independence (reporting `cross_vendor` when host identity is unknown or both CLIs are the same vendor, or `isolation_only` when a distinct known vendor is present) would weaken the receipt's truth. Mitigated by deriving `distinct` strictly from normalized known host vendor vs normalized selected provider vendor and asserting unknown-host cases in tests.
- Accidentally relaxing the CLI/harden fail-closed stall while editing shared helpers. Mitigated by leaving `AutoProviderInfo` and its callers byte-unchanged in behavior and adding a regression test that the stall error still fires for those paths.

## Acceptance

Profile: strict

Validation:
- [x] `v1` validate spec - Spec validates under scafld.
  - Command: `go run ./cmd/scafld validate context-isolated-reviewer`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-39

## Phase 1: Independence classification value and classifier

Status: pass
Dependencies: none

Objective: Introduce the canonical `independence{level,distinct}` value type and a pure classifier that maps the normalized host vendor and selected reviewer vendor to a level. The classifier reuses host detection for host identity but uses a supported reviewer-provider normalizer for reviewer identity so Gemini is treated as known when selected. `level` is the two-value enum `isolation_only` and `cross_vendor`; `distinct` is true only when both selected reviewer vendor and host vendor are known, non-empty, normalized, and different. Empty or unknown host identity is always `isolation_only` with `distinct=false`. This phase adds the type and classifier with no call-site changes, keeping it single-responsibility and independently testable.

Changes:
- internal/adapters/providers/provider.go - Add an `Independence` struct with `Level string` and `Distinct bool`, the level constants `IndependenceIsolationOnly = "isolation_only"` and `IndependenceCrossVendor = "cross_vendor"`, a `normalizeReviewerProvider` helper that recognizes `codex`, `claude`, and `gemini`, and a pure `classifyIndependence(hostVendor, reviewerVendor string) Independence` that returns `cross_vendor`/`distinct=true` only when both normalized vendors are non-empty and different, and `isolation_only`/`distinct=false` when they match or either is unknown/empty. Reuse host normalization for host identity; do not use a two-vendor host-only normalizer for reviewer providers.

Acceptance:
- [x] `ac1_1` independence type present - The independence value type and level enum exist.
  - Command: `rg -n 'IndependenceIsolationOnly|IndependenceCrossVendor|func classifyIndependence' internal/adapters/providers/provider.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-40
- [x] `ac1_2` providers package builds and tests green - Classifier compiles and unit tests pass.
  - Command: `go test ./internal/adapters/providers`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-41
- [x] `ac1_3` unknown host classified safely - Empty or unknown host identity cannot stamp cross_vendor.
  - Command: `rg -n 'func TestClassifyIndependenceUnknownHost|func TestClassifyIndependenceEmptyHost' internal/adapters/providers/*_test.go && go test ./internal/adapters/providers -run 'TestClassifyIndependence(UnknownHost|EmptyHost)'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-42
- [x] `ac1_4` gemini is known reviewer vendor - Gemini reviewer can qualify as cross_vendor when host is a different known vendor.
  - Command: `rg -n 'func TestClassifyIndependenceGeminiReviewer' internal/adapters/providers/*_test.go && go test ./internal/adapters/providers -run 'TestClassifyIndependenceGeminiReviewer'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-43

## Phase 2: Gate-path selector with graceful degradation

Status: pass
Dependencies: phase1

Objective: Add a finalize-only selector that never dead-ends when at least one supported CLI exists. It reuses `AutoProviderInfo`'s host-aware ordering to prefer a distinct known vendor, but when only the host vendor's CLI is available, or when host detection is empty/unknown, it does NOT return the fail-closed error; it selects a runnable supported provider at `isolation_only` and returns a `GateReviewerSelection` plus a stamped `Independence`. When a second vendor CLI is present and the host vendor is known, it upgrades to `cross_vendor` automatically, including Gemini when available. `AutoProviderInfo` and its stall error remain unchanged so the CLI review and harden selectors keep failing closed. The host-gate use case is the future consumer; existing `scafld review` is not rewired here.

Changes:
- internal/adapters/providers/provider.go - Add `SelectGateReviewer(opts Selection) (GateReviewerSelection, error)` that walks `autoProviderOrder` for a distinct known available vendor and returns it classified `cross_vendor`; if none is found, or if host detection is empty/unknown, it falls back to the host vendor when known or first available supported vendor at `isolation_only` instead of erroring. It only returns an error when no supported CLI exists at all. Leave `AutoProviderInfo` (lines 181-198), `providers.Select`, `SelectHarden`, and their callers unchanged.

Acceptance:
- [x] `ac2_1` finalize selector present - The graceful gate-path selector exists.
  - Command: `rg -n 'func SelectGateReviewer' internal/adapters/providers/provider.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-44
- [x] `ac2_2` stall removed from finalize path - No finalize selector returns the host-only stall; the CLI stall string is confined to `AutoProviderInfo`.
  - Command: `rg -n 'no independent auto provider found' internal/adapters/providers/provider.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-45
- [x] `ac2_3` selector and stall behavior tested - Gate selector degrades to isolation_only when only host vendor is present, upgrades to cross_vendor with a second vendor, and the CLI stall still fires.
  - Command: `rg -n 'type GateReviewerSelection|SelectGateReviewer.*GateReviewerSelection' internal/adapters/providers/provider.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-46

## Phase 3: Wire finalize selection and preserve CLI fail-closed behavior

Status: pass
Dependencies: phase2

Objective: Ensure the CLI review and harden selectors still call `AutoProviderInfo` and still fail closed, and that the finalize selector is the only path that degrades. CLI adapters stay thin: the finalize selector lives in the providers adapter and the future host-gate composition calls `SelectGateReviewer`. This phase adds a regression test asserting the CLI/harden selectors are untouched and the arch boundary still holds, with no composition logic added to thin handlers.

Changes:
- internal/adapters/cli/review/selection.go - No behavior change; confirm it still selects through `providers.Select` and `AutoProviderInfo` (fail-closed). Add a code-doc note that the finalize path uses `SelectGateReviewer` instead.
- internal/adapters/cli/harden/selection.go - No behavior change; confirm it still fails closed via `AutoProviderInfo`.
- internal/adapters/providers/provider.go - Add a code-doc comment on `SelectGateReviewer` recording exactly what `isolation_only` catches (context contamination, self-congratulation, drift, forgotten acceptance criteria, claimed-but-not-done) and forfeits (correlated blind spots, same-model-wrong-twice).

Acceptance:
- [x] `ac3_1` isolation_only tradeoff documented - The catch/forfeit contract is recorded in code-doc.
  - Command: `rg -n 'AutoProviderInfo' internal/adapters/cli/review/selection.go internal/adapters/cli/harden/selection.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-47
- [x] `ac3_3` CLI stays thin - The CLI thinness arch boundary still holds with the providers changes.
  - Command: `go test ./internal/arch -run TestCLIIsThin`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-48
- [x] `ac3_4` providers and review packages green - Full provider and review suites pass after wiring.
  - Command: `go test ./internal/adapters/providers ./internal/app/review`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-49

## Rollback

- Remove `SelectGateReviewer`, the `Independence` type, the level constants, and `classifyIndependence` from internal/adapters/providers/provider.go. `AutoProviderInfo` and the CLI/harden selectors are untouched, so the system returns to fail-closed-everywhere selection with no other reverts needed.
- Revert the added code-doc comments. No config schema, receipt schema, or CLI surface changed in this spec, so there is nothing else to undo.

## Review

Status: not_started
Verdict: none

## Self Eval

- Reuses `AutoProviderInfo` ordering, `autoProviderAvailable`, `DetectHostAgent`, `normalizeHostAgent`, and `agentProviderFor` rather than reimplementing detection or transport; DRY against the existing providers adapter.
- Stamps cross-vendor only when both vendors are known and different; unknown host identity degrades honestly to `isolation_only`.
- Single responsibility: the classifier is pure, the finalize selector only selects and degrades, and receipt stamping plus sandbox runtime stay out of scope.
- Fences cleanly against `evidence-control-sandbox` (runtime), `commit-free-tree-fingerprint` (bytes/diff), and `host-gate-and-receipt` (receipt) so the spec set stays DRY.
- Keeps the CLI/harden fail-closed stall intact and proves it with a regression assertion.

## Deviations

- none

## Metadata

- created_by: scafld
- estimated_effort_hours: 4-6
- priority: p1

## Origin

Created by: scafld
Source: accountability-layer rebuild

## Harden Rounds

### round-1

Status: needs_revision
Started: 2026-06-02T11:30:20Z
Ended: 2026-06-02T11:30:20Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: The draft is close, but approval should wait until it defines the unknown-host independence rule. Without that, the implementation can truthfully pass its current text while falsely stamping receipt-grade review as `cross_vendor`.

Checks:
- path audit
  - Grounded in: code:/Users/kam/dev/0state/scafld
  - Result: passed
  - Evidence: `rg --files` confirmed the declared touchpoints exist: `internal/adapters/providers/provider.go`, `internal/adapters/cli/review/selection.go`, `internal/adapters/cli/harden/selection.go`, `internal/app/review/`, and `internal/arch/`.
- command audit
  - Grounded in: command:go test ./internal/adapters/providers ./internal/app/review ./internal/arch
  - Result: not_applicable
  - Evidence: `go test ./internal/adapters/providers ./internal/app/review ./internal/arch` and `go run ./cmd/scafld validate context-isolated-reviewer` could not execute in this read-only sandbox: `go: creating work dir ... operation not permitted`. This is an environment limit, not a spec command-shape problem.
- scope/migration audit
  - Grounded in: code:/Users/kam/dev/0state/scafld/.scafld/specs/drafts/host-gate-and-receipt.md:172
  - Result: passed
  - Evidence: Provider selection and transport live in `internal/adapters/providers/provider.go`; CLI review/harden selectors are adapters; the future finalize use case is explicitly owned by `host-gate-and-receipt`, which consumes this selector.
- acceptance timing audit
  - Grounded in: spec_gap:acceptance
  - Result: failed
  - Evidence: Acceptance covers the happy host-only and second-vendor selector cases, but it does not require the unknown-host classification case exposed by current `DetectHostAgent` behavior.
- rollback/repair audit
  - Grounded in: spec_gap:rollback
  - Result: passed
  - Evidence: Rollback is limited to symbols/comments introduced by this spec: `SelectGateReviewer`, `Independence`, constants, and `classifyIndependence`; no schema, CLI, or config rollback is required by the draft.
- design challenge
  - Grounded in: code:/Users/kam/dev/0state/scafld/internal/adapters/providers/provider.go:290
  - Result: failed
  - Evidence: `DetectHostAgent` can return empty at provider.go:290, while the draft says `classifyIndependence` returns cross-vendor whenever vendors differ. Empty host plus known reviewer is not proof of cross-vendor independence.

Issues:
- [high/blocks approval] `harden-1` question - Unknown host identity can be falsely stamped as cross-vendor.
  - Status: open
  - Grounded in: code:/Users/kam/dev/0state/scafld/internal/adapters/providers/provider.go:290
  - Evidence: `DetectHostAgent` returns `""` for an unrecognized host at provider.go:290, and `normalizeHostAgent` returns `""` for unrecognized values at provider.go:306. The draft says `classifyIndependence(hostVendor, reviewerVendor)` returns `cross_vendor`/`distinct=true` when vendors differ at `.scafld/specs/drafts/context-isolated-reviewer.md:95`. Implemented literally, `hostVendor == ""` and `reviewerVendor == "codex"` would be misclassified as `cross_vendor`, which weakens the receipt truth the spec is trying to protect.
  - Recommendation: Amend phase 1 and phase 2 to state that unknown host identity cannot prove cross-vendor independence. Add provider tests for `classifyIndependence("", "codex")` and selector behavior when host detection is empty.
  - Question: How should the finalize stamp independence when `DetectHostAgent` returns an unknown or empty host vendor?
  - Recommended answer: Treat unknown host as `isolation_only` with `distinct=false`; `cross_vendor` is valid only when the host vendor is known and differs from the selected reviewer vendor.
  - If unanswered: Default unknown or empty host to `isolation_only` with `distinct=false`; only stamp `cross_vendor` when both normalized vendors are non-empty and different.

### round-2

Status: needs_revision
Started: 2026-06-02T13:16:02Z
Ended: 2026-06-02T13:16:02Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: The draft needs revision before approval. The main blockers are a Gemini classification contradiction and an unresolved gate-consumer/wiring question that could leave the current fail-closed stall untouched.

Checks:
- path audit
  - Grounded in: code:internal/adapters/providers/provider.go:1
  - Result: passed
  - Evidence: rg --files found internal/adapters/providers/provider.go, internal/adapters/cli/review/selection.go, internal/adapters/cli/harden/selection.go, internal/app/review/, and internal/arch/architecture_test.go.
- command audit
  - Grounded in: command:go test ./internal/adapters/providers ./internal/arch ./internal/app/review
  - Result: failed
  - Evidence: go test ./internal/adapters/providers ./internal/arch ./internal/app/review and go run ./cmd/scafld status context-isolated-reviewer --json both failed before execution with mkdir .../T/go-build...: operation not permitted, caused by this read-only sandbox. The commands themselves reference existing packages/entrypoint.
- scope/migration audit
  - Grounded in: code:internal/adapters/cli/cli.go:324
  - Result: failed
  - Evidence: Current CLI review wiring calls reviewcli.Select, then passes selected.Provider into review.RunWithInput; internal/app/review.Provider requires Invoke(context.Context, review.Request). The proposed SelectGateReviewer(opts) (Agent, Independence, error) returns Agent, whose method is InvokeAgent, and Phase 3 says CLI review remains unchanged.
- acceptance timing audit
  - Grounded in: spec_gap:phases
  - Result: failed
  - Evidence: Phase-specific go test -run 'Independence.*Unknown|Independence.*Empty' and go test -run 'SelectGateReviewer.*Unknown|SelectGateReviewer.*Empty' can exit zero even if no test names match. ac2_2 only greps for the stall string in the whole file and does not prove it is confined to AutoProviderInfo.
- rollback/repair audit
  - Grounded in: spec:rollback
  - Result: passed
  - Evidence: Rollback removes the new selector, type, constants, classifier, and code-doc comments. Because the spec claims no config/schema/CLI changes, this is credible if the scope stays provider-only; it needs revision if finalize wiring is added in this spec.
- design challenge
  - Grounded in: code:internal/adapters/providers/provider.go:306
  - Result: failed
  - Evidence: The design goal is sound, but the draft currently under-specifies the actual consumer path and has a vendor-normalization contradiction for Gemini.

Issues:
- [high/blocks approval] `harden-1` question - Gemini cannot satisfy cross_vendor under the specified classifier.
  - Status: open
  - Grounded in: code:internal/adapters/providers/provider.go:306
  - Evidence: normalizeHostAgent returns only codex, claude, or empty. autoProviderOrder and autoProviderAvailable support gemini. A classifier that reuses normalizeHostAgent for the reviewer vendor will classify host=codex/reviewer=gemini as unknown reviewer, yielding isolation_only even though Phase 2 says a second known vendor should upgrade to cross_vendor.
  - Recommendation: Revise Phase 1 to define a normalization helper for classification that recognizes every supported provider in autoProviderOrder, including Gemini. Keep DetectHostAgent as the host detection source, but do not rely on the current two-vendor helper unchanged for reviewer classification.
  - Question: Should Gemini be a known reviewer vendor for cross_vendor, and if so should the spec replace reuse normalizeHostAgent with a normalizer that recognizes all supported providers?
  - Recommended answer: Yes. Gemini is a supported reviewer vendor and must be able to stamp cross_vendor when the host is known and different.
  - If unanswered: Default to adding a provider/vendor normalizer that recognizes codex, claude, and gemini, and use it for independence classification while leaving unknown host identity as isolation_only.
- [high/blocks approval] `harden-2` question - The proposed selector is not wired to the current review finalize and has a return type mismatch for that finalize.
  - Status: open
  - Grounded in: code:internal/adapters/cli/cli.go:340
  - Evidence: The current review finalize is wired in internal/adapters/cli/cli.go: reviewcli.Select builds an app review provider and review.RunWithInput consumes it. internal/app/review.Provider requires Invoke, while the proposed SelectGateReviewer returns Agent with InvokeAgent. Phase 3 says CLI review selection remains fail-closed and only notes that another finalize use case will call the new selector. Implementing the phases as written can leave the current scafld review finalize stall unchanged and the new selector unused.
  - Recommendation: Make the ownership explicit. If this task must remove the current finalize stall, add the exact adapter/app wiring and return type needed by internal/app/review.Provider, plus tests that scafld review no longer fails closed in the host-only CLI case. If this task only exports a selector for a later spec, narrow the goal text so approval does not imply the current finalize path is fixed.
  - Question: Which concrete finalize path consumes SelectGateReviewer in this task: the existing scafld review finalize, or a later host-gate-and-receipt use case?
  - Recommended answer: This task should expose the selector only for the future host-gate path; update the title/objectives to remove the claim that the existing review finalize is fixed, or add wiring if that claim must remain.
  - If unanswered: Default to provider-only scope and change the summary/objectives to say this spec exposes a selector for a later host finalize, not that it removes the current review-gate stall.
- [medium/advisory] `harden-3` acceptance_gap - Some acceptance commands can pass without proving the intended behavior.
  - Status: open
  - Grounded in: spec_gap:phases
  - Evidence: go test -run 'Independence.*Unknown|Independence.*Empty' exits successfully when zero tests match, so ac1_3 and ac2_4 can pass without proving unknown/empty behavior. ac2_2 only finds the stall string in provider.go; it does not assert the selector avoids returning it or that the string is confined to AutoProviderInfo.
  - Recommendation: Add concrete test names that must exist and run, for example TestClassifyIndependenceUnknownHost, TestClassifyIndependenceEmptyHost, TestSelectGateReviewerUnknownHostFallsBackIsolationOnly, and TestSelectGateReviewerHostOnlyDoesNotReturnStall. Prefer full package tests for behavior and keep rg checks as smoke checks only.


## Planning Log

- none
