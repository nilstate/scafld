---
spec_version: '2.0'
task_id: browser-evidence-acceptance
created: '2026-05-14T14:08:47Z'
updated: '2026-05-14T14:17:41Z'
status: review
harden_status: passed
size: medium
risk_level: medium
---

# Add browser evidence acceptance

## Current State

Status: review
Current phase: final
Next: review
Reason: build completed; ready for review
Blockers: none
Allowed follow-up command: `scafld review browser-evidence-acceptance`
Latest runner update: 2026-05-14T14:17:41Z
Review gate: not_started

## Summary

Add typed browser acceptance criteria that let frontend work attach deterministic browser evidence without adding a separate lifecycle command.

## Objectives

- Add first-class browser acceptance criteria to the existing `plan -> harden -> approve -> build -> review -> complete` lifecycle.
- Keep browser execution project-owned: scafld should validate evidence produced by Playwright, Cypress, browserless scripts, or any other frontend runner instead of owning auth/server automation itself.
- Record browser evidence in the same session ledger as command criteria so handoff, review, and reports can cite the exact diagnostic artifact.
- Avoid a new lifecycle command, alias, compatibility path, or scafld-managed credential flow.

## Scope

- In: `browser` acceptance criteria and a `browser_evidence` expected kind.
- In: validation of structured browser evidence emitted by the criterion command.
- In: build/session recording that preserves the browser evidence summary and the full diagnostic path.
- In: Playwright-shaped browser command failure hints that tell the agent how to install project dependencies or browser binaries.
- In: markdown parser/renderer defaults so `browser` criteria round-trip cleanly.
- In: focused tests and docs that explain the contract, including how auth should be handled by project-owned commands.
- Out: embedding Playwright/Cypress in scafld, launching dev servers, storing browser credentials, image diffing, and new CLI lifecycle commands.

## Dependencies

- Existing acceptance criterion model in `internal/core/spec`.
- Existing command runner and diagnostic capture in `internal/app/build` and `internal/adapters/process`.
- Existing markdown parser/renderer in `internal/adapters/markdown`.
- No new runtime dependency; browser tools remain project dependencies.

## Assumptions

- Frontend projects already know how to start their app, authenticate, and run browser checks better than scafld can infer generically.
- A browser criterion command can emit one JSON evidence object on stdout while writing screenshots/traces to project or `.scafld/runs` paths.
- Auth evidence should be declarative and redacted: record auth mode/artifact, never secrets.
- A browser evidence criterion passes only when the command exits successfully and the evidence object is structurally valid with no recorded console or network errors.
- If a browser criterion uses Playwright and fails before evidence is produced because Playwright or its browser binaries are missing, scafld should surface install help in the criterion failure reason.

## Touchpoints

- `internal/core/acceptance/model.go`: expected-kind support.
- `internal/core/acceptance/browser.go`: browser evidence validation and Playwright install hints.
- `internal/core/spec/model.go`: enforce the type/expected-kind pairing.
- `internal/app/build/build.go`: use stdout for browser evidence while retaining full diagnostics.
- `internal/adapters/markdown/parser.go`: default `browser` criteria to `browser_evidence`.
- `internal/adapters/markdown/renderer.go`: preserve the new expected kind.
- `docs/execution.md`: document browser evidence criteria and auth responsibility.
- `docs/configuration.md`: clarify project-owned toolchain/auth commands for browser checks.

## Risks

- Risk: scafld accidentally becomes a weak browser automation wrapper. Mitigation: require an explicit command and only validate emitted evidence.
- Risk: auth handling leaks secrets into the ledger. Mitigation: evidence schema records mode/artifact only; docs explicitly forbid tokens/passwords.
- Risk: the evidence schema is too strict for real tools. Mitigation: keep the required fields small and allow optional artifacts.
- Risk: browser criteria become indistinguishable from normal commands in handoff. Mitigation: use a distinct `browser` type and `browser_evidence` reason text.
- Risk: install help becomes npm-only advice in pnpm/yarn/bun projects. Mitigation: keep the message generic and mention the project-owned command/dependency path first.

## Acceptance

Profile: standard

Validation:
- [x] `final-tests` command - Focused implementation tests pass
  - Command: `go test ./internal/core/acceptance ./internal/app/build ./internal/adapters/markdown ./internal/core/spec`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-10
- [x] `dogfood-browser-evidence` browser - scafld accepts a browser evidence packet
  - Command: `printf '{"url":"https://example.test/dashboard","viewport":"1440x900","auth":{"mode":"none"},"screenshots":[{"path":".scafld/runs/browser-evidence-acceptance/browser-dashboard.png","description":"dashboard loaded"}],"console_errors":[],"network_errors":[]}'`
  - Expected kind: `browser_evidence`
  - Status: pass
  - Evidence: browser evidence accepted for https://example.test/dashboard at 1440x900
  - Source event: entry-11

## Phase 1: Implementation

Status: completed
Dependencies: none

Objective: Add the browser evidence contract without changing the lifecycle surface.

Changes:
- Extend acceptance evaluation with `browser_evidence`.
- Add a small browser evidence validator using only the standard library.
- Teach build to evaluate browser criteria from stdout while preserving diagnostic files for review.
- Teach markdown parsing that `browser` criteria default to `browser_evidence`.
- Add focused tests for valid evidence, missing evidence, console/network errors, command failures, and markdown round-trip.
- Add focused tests for Playwright missing-install diagnostics on browser command failure.
- Update docs to explain browser evidence, auth responsibility, and why there is no new CLI command.

Acceptance:
- [x] `ac1` command - Primary validation command
  - Command: `go test ./internal/core/acceptance ./internal/app/build ./internal/adapters/markdown ./internal/core/spec`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6
- [x] `ac2` command - Full scafld check stays green
  - Command: `make check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-7

## Rollback

- Revert the acceptance evaluator, build, parser, docs, and tests changed by this task.
- Re-run `go test ./internal/core/acceptance ./internal/app/build ./internal/adapters/markdown` to confirm the old command-only behavior still passes after rollback.

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

Status: passed
Started: 2026-05-14T14:16:42Z
Ended: 2026-05-14T14:17:11Z

Checks:
- Path audit
  - Grounded in: code:internal/core/acceptance/model.go:18
  - Result: passed
  - Evidence: Existing acceptance model owns expected kinds; new browser validator file and docs paths are explicit task touchpoints.
- Command audit
  - Grounded in: code:Makefile:36
  - Result: passed
  - Evidence: Focused `go test` command has already run green; `make check` is the repository's full validation target.
- Scope/migration audit
  - Grounded in: spec_gap:Scope
  - Result: passed
  - Evidence: The plan adds one criterion type/expected kind and docs; no lifecycle alias, compatibility path, migration, or browser framework dependency is introduced.
- Acceptance timing audit
  - Grounded in: spec_gap:Acceptance
  - Result: passed
  - Evidence: Phase criteria run after implementation; final dogfood browser evidence runs after the evaluator exists.
- Rollback/repair audit
  - Grounded in: spec_gap:Rollback
  - Result: passed
  - Evidence: Rollback is a normal code/docs revert plus focused tests; no persisted data migration or release artifact is required.
- Design challenge
  - Grounded in: code:internal/app/build/build.go:322
  - Result: passed
  - Evidence: The plan exists to make frontend evidence deterministic inside the existing build gate; it solves the root lifecycle gap without adding a scafld-owned browser/auth harness or separate CLI command.

Questions:
- none


## Planning Log

- Observation: acceptance criteria already carry a `Type`, so browser review belongs in the existing build criteria model rather than as a new CLI command.
- Decision: browser auth stays in the project-owned runner command; scafld validates redacted evidence instead of storing or producing credentials.
- Decision: the dogfood criterion is a generated evidence packet, not a live Playwright dependency, so scafld can prove the contract deterministically in its own test suite.
