---
spec_version: '2.0'
task_id: release-readiness-no-drift-252-final
created: '2026-07-21T04:52:16Z'
updated: '2026-07-21T11:02:22Z'
status: completed
harden_status: passed
size: medium
risk_level: medium
---

# Release Readiness No Drift 2.5.2 Final

## Current State

Status: completed
Current phase: final
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-07-21T11:02:22Z
Review gate: pass

## Summary

Prepare the scafld 2.5.2 patch release from the actual current codebase with no contract, version, model, package, or gate-state drift.

## Objectives

- Harden and review prompts reject overengineering, marginal-surface bookkeeping, and compliance-matrix churn unless the finding proves a real defect, violated invariant, or broken architectural boundary.
- Harden gate state is projected from one shared core model consumed by harden, approve, and status; approving over incomplete, stale, failed, or needs-revision harden evidence requires an explicit `--reason` and records a `harden_override`.
- Provider review and provider harden attempts cannot strand tasks in `running` or `in_progress` when the caller context is canceled or the post-provider write window fails; terminal evidence recording is bounded, detached, and covered by tests.
- Provider harden setup failures after the initial `in_progress` save, including source reload cancellation, close the round as a durable `error`.
- Review success records the accepted attempt and review dossier in one ledger transaction, so a failed write cannot leave an accepted-without-review half-state.
- Acceptance evidence is only projected when the current criterion command, expected kind, type, and phase still match the evidence event; editing an already-passed acceptance command reopens the affected build phase.
- Acceptance evidence without recorded command, expected kind, criterion type, or phase identity is legacy/incomplete and does not project as current pass/fail evidence.
- Review-repair builds rerun the declared acceptance surface before returning to review, so repairing code after a failed review cannot skip completed phase criteria.
- Passed harden rounds without `spec_digest` are treated as stale/unauthoritative and require fresh hardening or a reasoned operator override before approval.
- Provider and human-reviewed review entry points reconcile session-backed acceptance evidence before accepting review state, so direct review cannot bypass stale build evidence.
- Legacy top-level `harden_status: passed` without a harden round is treated as stale and requires fresh hardening or a reasoned override.
- Explicit `scafld build` in review state refreshes acceptance evidence instead of refusing as already review-ready, so agents can update build evidence after local changes without a new CLI flag.
- All active package, installer, verifier, GitHub action, workflow, and managed initwire release defaults are synchronized on unreleased patch version `2.5.2`; no active workspace scafld config pins old model names.
- GitHub verifier workflows split pull-request verification into independent `pull_request_target` lanes owned by trusted base workflow code: a material lane verifies signed PR-head Git data without executing the head worktree, an acceptance lane reruns signed commands from a clean secretless head worktree with no persisted credentials, and an aggregate gate requires both.
- Managed workspace core assets are refreshed from the same bundled assets that ship in the release, including `.scafld/core/schemas/spec.json`.
- Local machine dogfood entry points resolve to the current workspace build before release readiness is claimed.

## Scope

- In scope: `internal/core/hardengate`, `internal/core/lifecycle`, and gate-state consumers in `internal/app/{approve,harden,review,status}`.
- In scope: acceptance evidence contract projection in `internal/core/{session,reconcile}` and build phase selection in `internal/app/build`.
- In scope: CLI output/help, provider/MCP submit descriptions, harden/review prompt assets, spec schema, markdown parser/renderer support, and docs that describe the lifecycle and review/harden contracts.
- In scope: root `AGENTS.md` source-checkout guidance and managed `.scafld/core` refresh proof.
- In scope: public spec schema parity for runtime-supported acceptance kinds, harden shape, and harden observation fields.
- In scope: public spec schema parity for runtime-rendered review provenance and detailed review finding fields.
- In scope: Markdown render/parse round-trip preservation for detailed review finding fields exposed by the public schema.
- In scope: safe verifier workflow behavior for fork pull requests, including active and bundled initwire `pull_request_target` workflows, split material/acceptance lanes, verifier script receipt extraction, and exact material-to-acceptance binding.
- In scope: release metadata and packaging surfaces for Go, npm, PyPI, Homebrew, Scoop, WinGet, GitHub verifier action/workflow, managed initwire verifier scripts, and release parity tests.
- In scope: package artifact build, installer smoke, size check, package-manager manifest rendering, full checks, and whitespace diff validation.
- Out of scope: publishing GitHub, npm, PyPI, Homebrew, Scoop, WinGet, or OCI releases.
- Out of scope: rewriting historical receipts or archived dogfood runs that record old model names or old release evidence.

## Dependencies

- Existing scafld release scripts and package-manager templates.
- Installed Go, Node, npm, and Python toolchains used by `make check` and release smoke scripts.
- Local Git workspace state for `git diff --check`, review material projection, and local dogfood binary version identity.

## Assumptions

- `2.5.1` is already tagged/published, so this patch readiness lane prepares `2.5.2`.
- Historical receipts are immutable audit evidence and are not active configuration drift.
- Legacy model strings in upgrade tests are compatibility fixtures; active configs should use rolling/latest defaults unless explicitly overridden.
- Local provider harden is acceptable for smoke-hardening this release-readiness contract, but the final review gate must examine the actual changed material rather than the earlier poisoned baseline.

## Touchpoints

- `.github/actions/scafld-verify/action.yml`
- `.github/workflows/scafld-verify.yml`
- `README.md`
- `AGENTS.md`
- `.scafld/core/schemas/spec.json`
- `docs/**`
- `internal/adapters/cli/**`
- `internal/adapters/corebundle/assets/agentdocs/AGENTS.md`
- `internal/adapters/corebundle/**`
- `internal/adapters/corebundle/assets/core/prompts/{harden,review}.md`
- `internal/adapters/corebundle/assets/core/schemas/spec.json`
- `internal/adapters/corebundle/assets/initwire/**`
- `internal/adapters/{markdown,mcp,providers}/**`
- `internal/app/{approve,harden,review,status}/**`
- `internal/app/build/**`
- `internal/core/{session,reconcile}/**`
- `internal/core/{hardengate,lifecycle,spec}/**`
- `package/npm/package.json`
- `package/pypi/**`
- `scripts/{set-release-version,scafld-verify,build-release-artifacts,smoke-release-installers,check-release-size}.sh`
- `test/release/packaging_test.go`

## Risks

- Over-tightening harden/review prompts could hide legitimate defects; wording must still block verified invariant, boundary, security, data, or correctness failures.
- Release metadata can drift when bump scripts miss a managed verifier or package-manager surface; parity tests must fail closed on active release defaults.
- Provider cancellation can leave stale gate evidence if terminal writes use the caller context; review and harden must use a bounded detached evidence context after provider return.
- A failed review success write can create misleading half-state if accepted attempts and review dossiers are not recorded atomically.
- Acceptance evidence can drift when a criterion id is reused after editing the command or expected kind; replay must fail closed and rerun the affected build phase instead of inheriting old pass state.
- A single-lane or PR-authored verifier workflow can execute or alter fork verification logic in the same authority context that verifies signed material; the release verifier must use trusted base workflow code, keep material verification and head acceptance isolated, and require both results.
- Direct `go install ./cmd/scafld` embeds tag-derived dirty version metadata in a local checkout; dogfood binaries need explicit version identity when used as machine defaults.

## Acceptance

Profile: standard

## Phase 1: Release Readiness No Drift 2.5.2 Final

Status: completed
Dependencies: none

Objective: Prepare the scafld 2.5.2 patch release from the actual current codebase with no contract, version, model, package, or gate-state drift.

Changes:
- Add shared harden gate projection and wire harden/status/approve to consume it.
- Add reasoned harden override approval receipts and human status `next action` output.
- Strengthen harden and review contracts against overengineering and marginal-surface bookkeeping while preserving real blocker enforcement.
- Add `internal/core/lifecycle` terminal evidence context and use it for post-provider review/harden closure.
- Record accepted review attempts and review dossiers in one session transaction; close unrecorded success failures as failed attempts.
- Close provider harden terminal recording failures as durable harden `error` state, with local-storage follow-up instead of leaving `in_progress`.
- Close provider harden source reload/setup failures after the initial save as durable harden `error` state.
- Extend the bundled spec schema to cover runtime-supported `manual` and `browser_evidence` acceptance kinds plus harden `shape`, `question`, `recommended`, and `if_unanswered` fields.
- Extend the bundled spec schema to cover runtime-rendered review provider/model/output/normalization fields plus detailed review finding location, evidence, impact, validation, status, confidence, and review pass fields.
- Preserve detailed review finding fields through Markdown render/parse projection, including category, confidence, reproducer, suggested fix, related spec, review pass, and status.
- Record expected kind and criterion type on new criterion evidence events, project criterion pass/fail only when the current criterion contract still matches that event, and reopen completed phases whose criteria become stale.
- Fail closed on incomplete legacy criterion evidence by requiring recorded expected kind, criterion type, and matching phase identity before projection.
- Reconcile build input before deciding a review-state task is already ready for review, and prevent `final` from masking an earlier reopened phase.
- Add a review-repair build path that reruns all declared phase and final acceptance criteria before recording new review-ready build evidence.
- Let explicit review-state builds rerun all acceptance, keeping `status` as the normal review guidance while preserving a frictionless evidence refresh path.
- Make status consume the reconciled ledger-backed model for lifecycle and next-action decisions while still exposing the exact raw Markdown under `spec_source`.
- Update app-level reconcile tests to use contract-bound criterion evidence instead of legacy wildcard evidence.
- Treat digestless passed harden rounds as stale in hardengate projection so approve requires `--reason` instead of silently trusting unverifiable harden evidence.
- Reconcile the review model before provider or human-reviewed review gating, update provider context to use the reconciled model, and return a structured build-gate error when reconciliation leaves the task non-reviewable.
- Treat no-round legacy `harden_status: passed` as stale in hardengate projection, with approve/status tests proving it cannot silently approve.
- Keep active and bundled verifier workflows on trusted-base `pull_request_target` workflow code and protected-base checkouts, fetch pull-request head commits without checking them out into the verifier root, run independent material and acceptance lanes, pass `SCAFLD_VERIFY_HEAD` plus `SCAFLD_VERIFY_MODE`, and bind acceptance execution to the clean verified material commit.
- Add release packaging tests that fail on unsafe verifier checkout patterns, missing separate-head verifier support, and version drift across active release surfaces.
- Update initwire bundle tests to assert the trusted-base workflow and separate-head verifier script contract instead of the removed PR-head checkout wrapper.
- Extend cancellation and half-state tests for review and harden provider attempts.
- Synchronize all active `2.5.2` release metadata and model/latest defaults.
- Refresh managed `.scafld/core` assets and root `AGENTS.md` from the bundled release assets.
- Rebuild local dev binaries and local release artifacts without publishing.

Acceptance:
- [x] `ac1` command - Configured validation command
  - Command: `make check && scripts/build-release-artifacts.sh 2.5.2 && scripts/smoke-release-installers.sh 2.5.2 && scripts/check-release-size.sh 2.5.2 && tmp="$(mktemp -d)" && CHECKSUMS_FILE=dist/checksums.txt OUT_DIR="$tmp" scripts/render-package-managers.sh 2.5.2 && rm -rf "$tmp" && git diff --check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-149

## Rollback

- Revert this patch before tagging if final dogfood review or release checks find a real blocker.
- If package metadata was bumped but release is abandoned, run `scripts/set-release-version.sh 2.5.1` only as part of an explicit rollback commit before publishing anything.

## Review

Status: completed
Verdict: pass
Mode: verify
Provider: claude:claude-opus-4-8[1m]
Output: claude.mcp_submit_review
Summary: Both open findings from the prior review are repaired and covered by tests, and no regressions were introduced by the fixes. F-025 (parent-process token leak) is now closed with layered defenses: the acceptance child environment is scrubbed and secret keys are blanked after config overrides (verify.go:301-303), scafld's own environment is destroyed by the `env -i` exec-replace in full mode (scripts/scafld-verify.sh:25-49), and processguard.DisableDumpable() makes /proc/$PPID/environ unreadable to same-uid children (verify.go:162-166, dumpable_linux.go). An adversarial test (verify_test.go:500) asserts the acceptance command cannot read scrubbed tokens or /proc/$PPID/environ. The step shell that scafld inherits from is exec-replaced with the sanitized env, so the parent-shell copy of the token is gone; no other same-uid dumpable ancestor holds the token in its process environ. F-026 (Windows lock cancellation leak) is repaired: lockContext now uses nonblocking LockFileEx with lockfileFailImmediately plus context-aware timer polling (filelock_windows.go:49-85), leaving no blocked goroutines or waiters after cancellation. Active and bundled verifier scripts/workflows match, and packaging tests lock in persist-credentials:false, split material/acceptance lanes, the always()-guarded aggregate gate, and the env -i sanitization contract. Verdict: pass.

Attack log:
- `internal/adapters/cli/verify/verify.go:294-321 isolatedAcceptanceEnv`: Attempt to leak CI tokens into the acceptance child environment via config execution.env overrides -> clean (acceptanceSecretEnvKeys (ACTIONS_ID_TOKEN_REQUEST_TOKEN, ACTIONS_RUNTIME_TOKEN, GH_TOKEN, GITHUB_TOKEN) are blanked after the overrides loop, so config-injected secrets cannot survive; HOME is an isolated temp dir. verify_test.go:459 seeds config-leak tokens and the acceptance command asserts they are empty.)
- `internal/adapters/cli/verify/verify.go:162-166 + processguard/dumpable_linux.go`: Read the scafld verifier parent environment through /proc/$PPID/environ from the acceptance sh -c child (the original F-025 reproducer) -> clean (DisableDumpable sets PR_SET_DUMPABLE=0, making /proc/<scafld>/environ owned by root and unreadable to the same-uid child; scafld's env is also wiped by env -i. verify_test.go:500 asserts the acceptance command cannot cat /proc/$PPID/environ.)
- `scripts/scafld-verify.sh:25-49 (full-mode env -i re-exec)`: Recover the token from an ancestor process (the step shell that launched scafld) via /proc/<parent>/environ -> clean (In full mode the step shell is exec-replaced by env -i with only whitelisted vars (no ACTIONS_RUNTIME_TOKEN/GITHUB_TOKEN), so scafld's parent process no longer holds the token; the only prior step-env copy of the token is destroyed. No remaining same-uid dumpable ancestor exposes the token in its process environ.)
- `internal/platform/filelock/filelock_windows.go:49-85 lockContext`: Cancel a contended Windows lock acquisition and check for leaked blocked goroutines or file handles (F-026) -> clean (Rewritten to nonblocking LockFileEx with lockfileFailImmediately and ctx-aware timer polling; cancellation stops the timer, closes the handle, and returns promptly with no background waiter goroutine.)
- `.github/workflows/scafld-verify.yml:11-90 material vs acceptance lanes`: Get untrusted head code to execute in the material lane's authority context -> clean (Material lane runs --material-only with no --acceptance-root and creates no head worktree (scafld-verify.sh:96-103,137-141), so no head code executes there; only the acceptance lane (full mode, sanitized env, isolated + non-dumpable) runs head commands.)
- `.github/workflows/scafld-verify.yml:92-104 verify-gate`: Pass the aggregate gate while one lane failed or was skipped -> clean (verify-gate runs under always(), needs both lanes, and exits nonzero unless both results equal success, preserving the F-024 repair.)
- `scripts/scafld-verify.sh vs internal/adapters/corebundle/assets/initwire/scripts/scafld-verify.sh + test/release/packaging_test.go:158-325`: Find drift between the active verifier script/workflow and the bundled initwire assets, or missing test coverage for the sanitization contract -> clean (Active and bundled scripts both carry the env -i SANITIZED re-exec; packaging_test asserts persist-credentials:false, both SCAFLD_VERIFY_MODE values, the SANITIZED re-exec, and env -i, so regressions fail closed.)
- `internal/platform/processguard/dumpable_other.go + verify.go:162-165 error handling`: Bypass or crash the dumpable hardening on non-Linux or on prctl failure -> clean (Non-Linux returns a noop cleanup (child procfs vector not applicable there); on Linux a prctl failure returns an error and Run fails closed rather than proceeding to run acceptance without protection.)

Findings:
- [high/non-blocking] `F-025` Acceptance token leak via verifier parent environment is now closed
  - Category: security
  - Confidence: high
  - Status: fixed
  - Review pass: boundary
  - Location: `internal/adapters/cli/verify/verify.go:162`
  - Evidence: The acceptance lane runs in full mode, which re-execs via `exec env -i` (scripts/scafld-verify.sh:25-49) so scafld and its parent shell no longer carry ACTIONS_RUNTIME_TOKEN/GITHUB_TOKEN. isolatedAcceptanceEnv scrubs the child env and blanks acceptanceSecretEnvKeys after applying config overrides (verify.go:294-303). processguard.DisableDumpable() (verify.go:162-166; dumpable_linux.go) sets PR_SET_DUMPABLE=0 so a same-uid child cannot read /proc/$PPID/environ. verify_test.go:500 exercises an acceptance command that fails if SECRET_TOKEN/GITHUB_TOKEN/ACTIONS_RUNTIME_TOKEN are readable or if /proc/$PPID/environ is readable, covering both the direct and parent vectors.
  - Impact: Previously untrusted fork acceptance code could recover the Actions runtime token from the verifier parent; this vector is now closed, restoring the secretless acceptance boundary.
  - Validation: go test ./internal/adapters/cli/verify -run MaterialRef (verify_test.go:500 asserts scrubbed env and unreadable /proc/$PPID/environ on Linux).
  - Related spec: pull_request_target acceptance lane must rerun commands from a clean secretless head worktree with no persisted credentials.
- [medium/non-blocking] `F-026` Windows lock cancellation no longer leaks goroutines or handles
  - Category: resource leak
  - Confidence: high
  - Status: fixed
  - Review pass: error_path
  - Location: `internal/platform/filelock/filelock_windows.go:55`
  - Evidence: lockContext (filelock_windows.go:49-85) now uses a nonblocking LockFileEx with lockfileFailImmediately inside a poll loop that checks ctx.Err() before each attempt and selects on ctx.Done() vs a 25ms timer. There is no synchronous blocking LockFileEx and no background waiter goroutine; on cancellation it stops the timer, closes the file handle, and returns ctx.Err() promptly. The previously described blocked-goroutine-plus-waiter pattern no longer exists.
  - Impact: Repeated canceled terminal-evidence writes under Windows lock contention return bounded without accumulating blocked goroutines or leaked file handles.
  - Validation: go test ./internal/platform/filelock on Windows; inspect lockContext for the nonblocking poll loop and absence of goroutines.
  - Related spec: Terminal evidence recording must be bounded after provider cancellation or write-window failure.

## Self Eval

- `make check` passes after the terminal evidence fix and final version sync.
- `scripts/build-release-artifacts.sh 2.5.2` rebuilds all six native assets.
- `scripts/smoke-release-installers.sh 2.5.2` verifies npm and PyPI launchers against local artifacts.
- `scripts/check-release-size.sh 2.5.2` stays below the binary-size budgets.
- Package-manager manifests render successfully against `dist/checksums.txt`.
- `go test -count=1 ./test/release` proves active/bundled verifier workflows use trusted-base `pull_request_target` split material/acceptance lanes, avoid unsafe PR-head checkout in the verifier root, disable persisted credentials, and support reading receipts from a separate fetched PR head.
- `go test -count=1 ./internal/app/build ./internal/app/status ./internal/core/reconcile ./internal/core/session` proves stale acceptance command evidence is not projected, edited phase acceptance reruns, review-state builds can explicitly refresh acceptance, review-repair builds rerun all acceptance, and status routes stale acceptance back through build.
- `go test -count=1 ./internal/app/reconcile ./internal/core/reconcile ./internal/app/build ./internal/app/status ./internal/core/session ./internal/core/hardengate ./internal/app/approve` proves incomplete legacy criterion evidence is rejected and digestless passed harden rounds require fresh harden or a reasoned override.
- `go test -count=1 ./internal/app/review ./internal/app/status ./internal/core/reconcile ./internal/core/hardengate ./internal/app/approve` proves review and human-reviewed override cannot bypass reconciled stale acceptance, and legacy no-round harden passes require refresh/override.
- `bash -n scripts/scafld-verify.sh internal/adapters/corebundle/assets/initwire/scripts/scafld-verify.sh` passes.
- `git diff --check` passes.
- `scafld --version` and `/Users/kam/go/bin/scafld --version` both report the current workspace dogfood build.
- `.scafld/core/schemas/spec.json` has no diff from `internal/adapters/corebundle/assets/core/schemas/spec.json`.
- `go test -count=1 ./internal/adapters/corebundle` proves the bundled spec schema covers the runtime public fields.
- `go test -count=1 ./internal/app/harden` proves provider harden cancellation and terminal-save failures do not leave `in_progress` state when the fallback close succeeds.
- `go test -count=1 ./internal/adapters/markdown ./internal/adapters/corebundle` proves public schema fields and Markdown projection stay in sync.

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
Started: 2026-07-21T04:53:00Z
Ended: 2026-07-21T04:53:00Z
Spec digest: abf2952d3b555de7de9df8d2c0ff56972020ec1f96e982bf38b20d0dabf16dbb
Verdict: pass
Provider: local
Output format: local.fixture
Summary: Local provider smoke hardening passed.
Shape decision: keep
True shape: Local smoke hardening keeps the draft shape for transport validation.
Minimal plan: Submit a schema-valid harden dossier without changing files.
Shared owner: internal/core/harden
Adapter boundaries: local provider emits the dossier; harden app derives the verdict
Required spec edits:

Observations:
- design
  - Result: clean
  - Anchor: spec_gap:Summary
  - Note: Local smoke provider records required harden observations.
- scope
  - Result: clean
  - Anchor: spec_gap:Scope
  - Note: Local smoke provider records required harden observations.
- path
  - Result: clean
  - Anchor: spec_gap:Context
  - Note: Local smoke provider records required harden observations.
- command
  - Result: clean
  - Anchor: spec_gap:Acceptance
  - Note: Local smoke provider records required harden observations.
- timing
  - Result: clean
  - Anchor: spec_gap:Acceptance
  - Note: Local smoke provider records required harden observations.
- rollback
  - Result: clean
  - Anchor: spec_gap:Rollback
  - Note: Local smoke provider records required harden observations.


## Planning Log

- none
