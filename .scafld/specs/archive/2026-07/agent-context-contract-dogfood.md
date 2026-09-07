---
spec_version: '2.0'
task_id: agent-context-contract-dogfood
created: '2026-07-19T23:51:04Z'
updated: '2026-07-20T00:00:13Z'
status: completed
harden_status: passed
size: medium
risk_level: medium
---

# Agent Context Contract Dogfood

## Current State

Status: completed
Current phase: final
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-07-20T00:00:13Z
Review gate: pass

## Summary

Make scafld deliver canonical source Markdown and role contracts to harden and review agents, with required instruction sections and honest context budgets.

## Objectives

- Deliver source-backed agent context as a first-class runtime contract, not a stale internal packet.
- Make harden and review agents receive the canonical spec Markdown, role contract, and exactly one output contract.
- Keep CLI/API/MCP/provider surfaces as light adapters over shared core/app contracts.
- Use this spec as dogfood evidence and apply harden/review findings as patch improvements before commit.

## Scope

- `internal/core/agentcontract`: typed role contract model, role list, digest provenance, required context section conversion.
- `internal/adapters/cli/agentcontract`: workspace override, managed core copy, embedded prompt resolution.
- `internal/core/reviewcontext`: required/discretionary context section budgeting and strict required-budget validation.
- `internal/app/harden`, `internal/app/review`, `internal/app/status`, `internal/app/handoff`: agent-facing packet assembly from exact spec source and shared contract sections.
- `internal/core/diagnostics`: shared provider failure reason compaction and diagnostic-path preservation for harden/review lifecycle state.
- `internal/adapters/markdown`: exact source loading, harden shape/triplet round-trip, and removal of empty top-level validation boilerplate.
- `internal/core/harden`, `internal/core/spec`, provider/MCP harden schema surfaces: shape gate and observation triplet fields.
- `internal/adapters/corebundle/assets/core/prompts/*.md`, `.scafld/core/prompts/*.md`, `.scafld/core/schemas/harden_dossier.json`: managed contract assets.
- `AGENTS.md`, `CLAUDE.md`, and managed agent-doc assets: agent context hierarchy and stale-packet rule.

## Dependencies

- Existing scafld review-gate, workspace baseline, and managed core-bundle update machinery.
- Go toolchain, npm wrapper syntax check, Python launcher compile check, shell syntax checks, and race tests through `make check`.

## Assumptions

- The Markdown spec remains source of truth for the task contract.
- Structured JSON remains source of truth for lifecycle/gate state.
- Review/harden provider output must keep one structured output contract per packet.
- The dirty diff already exists before this dogfood spec is approved, so the late approval baseline cannot honestly classify the full existing diff as task changes. Harden findings remain actionable; review packet checks must disclose this limitation.

## Touchpoints

- `internal/core/agentcontract/model.go`
- `internal/adapters/cli/agentcontract/loader.go`
- `internal/adapters/corebundle/assets.go`
- `internal/core/reviewcontext/model.go`
- `internal/app/harden/context.go`
- `internal/app/harden/harden.go`
- `internal/app/review/context.go`
- `internal/app/review/review.go`
- `internal/adapters/markdown/renderer.go`
- `internal/adapters/markdown/parser.go`
- `internal/adapters/markdown/spec_store.go`
- `internal/core/diagnostics/diagnostics.go`
- `internal/core/harden/model.go`
- `internal/core/harden/schema.go`
- `internal/core/spec/model.go`
- `internal/adapters/corebundle/assets/core/prompts/harden.md`
- `internal/adapters/corebundle/assets/core/prompts/review.md`
- `internal/adapters/corebundle/assets/core/prompts/plan.md`
- `internal/adapters/corebundle/assets/core/config.yaml`
- `internal/adapters/corebundle/assets/core/schemas/harden_dossier.json`
- `internal/adapters/providers/invoke.go`
- `internal/arch/architecture_test.go`
- `AGENTS.md`
- `CLAUDE.md`

## Risks

- Required source/contract sections could exceed provider context if no separate required budget exists.
- Provider instruction sections could be silently dropped if left discretionary.
- Harden could still invent adapter boundaries for no-surface tasks if schema/prompt validation forces a non-empty list.
- A late dogfood spec can overclaim review coverage if baseline ownership is not disclosed.
- Prompt contract drift can reappear if role names and embedded prompt assets are not structurally checked.

## Acceptance

Profile: standard

## Phase 1: Agent Context Contract Dogfood

Status: completed
Dependencies: none

Objective: Make scafld deliver canonical source Markdown and role contracts to harden and review agents, with required instruction sections and honest context budgets.

Changes:
- Add the shared agent contract model and CLI loader.
- Render exact `Source Spec Markdown` as required context before derived sections.
- Render harden/review role contracts and exactly one output contract per packet.
- Split required context budgets from discretionary derived-section budgets.
- Mark provider instructions required so read-only and untrusted-data rules cannot drop.
- Preserve harden shape and observation triplets through model, schema, parser, renderer, provider, and MCP surfaces.
- Preserve failed provider diagnostics as compact control-state reasons plus first-class diagnostic pointers across harden and review.
- Remove duplicated prompt fallback packages and enforce role-to-asset sync in arch tests.
- Tighten prompt templates against boilerplate, weak review findings, empty adapter-boundary invention, and stale-packet context drift.
- Dogfood this spec through harden and review packet checks, then apply real findings as improvements.

Acceptance:
- [x] `ac1` command - Full project gate
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6
- [x] `ac2` command - Contract packet tests
  - Command: `go test ./internal/app/harden ./internal/app/review ./internal/core/reviewcontext ./internal/adapters/cli/agentcontract`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-7
- [x] `ac3` command - Harden schema and asset tests
  - Command: `go test ./internal/core/harden ./internal/adapters/markdown ./internal/adapters/providers ./internal/adapters/corebundle ./internal/arch`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-8

## Rollback

- Revert the contract delivery patch and rerun `scafld update --root .` to restore managed core assets.
- If provider packet rendering regresses, restore the previous prompt renderer and fail closed before provider invocation.

## Review

Status: completed
Verdict: pass
Mode: verify
Summary: Human-reviewed override accepted: reviewed dogfood packet in /tmp/agent-context-contract-dogfood-review.txt; provider harden idle-timeout was applied as a code improvement; late approval baseline means existing dirty diff is baseline context, not post-approval task changes

Attack log:
- `review gate`: manual human audit -> clean (reviewed dogfood packet in /tmp/agent-context-contract-dogfood-review.txt; provider harden idle-timeout was applied as a code improvement; late approval baseline means existing dirty diff is baseline context, not post-approval task changes)

Findings:
- none

## Self Eval

- Focused tests passed after closing the remaining schema/packet/Markdown issues.
- `make check` must be rerun after the dogfood lifecycle closes.

## Deviations

- This dogfood spec was created after most implementation work because the earlier narrow draft was deleted. The review baseline therefore cannot prove the entire dirty diff as task-owned; this spec treats harden and review packet findings as improvement signals and states that limitation explicitly.

## Metadata

- created_by: scafld

## Origin

Created by: scafld
Source: plan

## Harden Rounds

### round-1

Status: error
Started: 2026-07-19T23:52:12Z
Ended: 2026-07-19T23:52:12Z
Summary: provider error: provider failed: process idle timeout (diagnostic: /Users/kam/dev/0state/scafld/.scafld/runs/agent-context-contract-dogfood/diagnostics/command-1784505314387933000.txt)
Diagnostic: `/Users/kam/dev/0state/scafld/.scafld/runs/agent-context-contract-dogfood/diagnostics/command-1784505314387933000.txt`
Shape decision:
True shape:
Minimal plan:
Shared owner:
Adapter boundaries:
Required spec edits:

Observations:
- none

### round-2

Status: passed
Started: 2026-07-19T23:57:36Z
Ended: 2026-07-19T23:58:33Z
Shape decision: keep
True shape: One source-backed agent-context contract pipeline: exact spec Markdown plus typed role/output/instruction contracts render as required packet sections, while derived sections remain indexes.
Minimal plan: Keep the pure role-contract model and required-context renderer, keep CLI as the resolver, keep app commands as packet assemblers, and use tests plus this dogfood round to prevent drift.
Shared owner: internal/core/reviewcontext and internal/core/agentcontract
Adapter boundaries: CLI resolves workspace/managed/embedded contracts; markdown store exposes exact source; app commands assemble packets; provider and MCP surfaces consume structured contracts
Required spec edits:

Observations:
- design
  - Result: clean
  - Anchor: code:internal/core/agentcontract/model.go:44
  - Note: The role contract is a pure core model with provenance and required-section conversion, so the design is not another prompt block bolted onto one adapter.
- scope
  - Result: clean
  - Anchor: spec_gap:Scope
  - Note: Scope names the shared core/app owner, CLI loader, packet assembly, markdown source loading, provider/MCP schema surfaces, managed assets, and agent docs touched by the patch.
- path
  - Result: clean
  - Anchor: code:internal/adapters/cli/agentcontract/loader.go:14
  - Note: The declared loader path exists and resolves project override, managed core copy, then embedded asset without adding adapter logic to core.
- command
  - Result: clean
  - Anchor: code:Makefile:42
  - Note: `make check` is the repository-native full gate and includes fmt, vet, arch, package checks, package-manager render, tests, and race tests.
- timing
  - Result: clean
  - Anchor: spec_gap:Acceptance
  - Note: Acceptance commands are deterministic build-phase tests; harden/review lifecycle dogfood is kept outside build acceptance so phase timing stays executable.
- rollback
  - Result: blocks
  - Anchor: code:internal/core/diagnostics/diagnostics.go:20
  - Note: Dogfood provider harden hit an idle timeout and initially dumped multiline provider diagnostics into the living spec; harden now records a compact reason plus a first-class diagnostic path, and review uses the same shared diagnostics helper for failed attempts.
  - Question: Should provider stderr be copied into the living Markdown contract?
  - Recommended answer: No; store a compact one-line reason plus diagnostic pointer in control state and keep full output in the diagnostic file.
  - If unanswered: Keep the compact reason, preserve the structured diagnostic path, and treat diagnostic files as the detailed evidence surface.
  - Status: fixed


## Planning Log

- none
