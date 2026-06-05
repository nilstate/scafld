---
spec_version: '2.0'
task_id: base-delta-seal
created: '2026-06-05T03:52:41Z'
updated: '2026-06-05T04:30:17Z'
status: completed
harden_status: not_run
size: medium
risk_level: high
---

# Seal committed work with verifiable base_delta receipts

## Current State

Status: completed
Current phase: final
Next: done
Reason: finalization receipt passed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-06-05T04:28:39Z
Review gate: not_started

## Summary

`scafld finalize` seals on the uncommitted working tree and, by default, mints a `working_tree` receipt where `base_commit == head_commit == HEAD-at-mint`. The receipt's `tree_sha` is computed over the working tree (`git.Adapter.Snapshot` -> `writeTemporaryTree`), but `base_commit` is pinned to the HEAD that existed when the seal ran. After the operator commits the work, HEAD moves, and `scafld verify` re-derives `base_commit` as the *current* HEAD, so a committed `working_tree` receipt fails with `base_commit mismatch` (confirmed: `verify failed: base_commit mismatch`). This is why the dogfood `scafld verify` CI workflow is red on direct-to-main seals. A second, independent failure stacks on top: the workflow installs scafld with `go install .../scafld@v2.4.7`, which fails because 2.4.7 is unreleased.

The fix is to mint `base_delta` receipts at seal time by passing `base_ref = HEAD` (the parent commit). In `base_delta` mode the snapshot records `base_commit = merge-base(base_ref, HEAD)`, which stays equal to the parent across the later commit. Because `tree_sha` is taken over the working tree (and runtime receipts are excluded from the fingerprint), the committed head's scoped bytes match the sealed bytes, so `scafld verify --target <parent>` re-derives the same `base_commit` and `tree_sha` and passes. The seal still happens on the uncommitted tree (no flow inversion); only the recorded mode and `base_commit` change. Separately, the scafld repo's own verify workflow must obtain scafld without depending on an unreleased proxy version.

## Objectives

- Make `scafld finalize` mint a `base_delta` receipt by defaulting `base_ref` to HEAD whenever the workspace has a HEAD commit, so a committed receipt re-verifies in CI against the parent->head delta. Fall back to `working_tree` only when there is no HEAD (a repository with no commits yet).
- Keep the seal on the uncommitted working tree: the independent reviewer still reviews the `parent -> working-tree` delta (identical to today's content, since HEAD == parent at seal time). Only `snapshot_mode` and `base_commit` derivation change.
- Preserve every existing receipt invariant unchanged: ed25519 signature, review coverage, independence re-derivation, the mutation guard, runtime-path exclusion, and `tree_sha` integrity. This spec changes only how `base_commit`/`snapshot_mode` are chosen at mint, never what is signed-versus-reviewed.
- Fix the dogfood CI install so the scafld repository's `scafld verify` workflow builds scafld from the checked-out source instead of `go install`-ing an unreleased version, and goes green on a freshly sealed `base_delta` receipt.
- Do not retroactively invalidate historical `working_tree` receipts: they were valid at mint and stay archived; only future seals change shape.

## Scope

- In scope: defaulting the finalize `base_ref` to the resolved HEAD when the caller passes none and a HEAD exists, in the shared finalize request path (`internal/adapters/cli/finalize/run.go`), so both the `scafld finalize` CLI and the `finalize` MCP tool inherit it.
- In scope: a HEAD resolver usable from the finalize adapter (reuse the git adapter's existing HEAD resolution rather than a parallel path).
- In scope: the scafld repository's live `.github/workflows/scafld-verify.yml` building scafld from source (the repo is scafld) so its dogfood verify no longer depends on `go install .../scafld@<unreleased>`.
- In scope: regression coverage that a `base_delta` seal over uncommitted work, once committed, verifies against the parent; and that a no-HEAD workspace still falls back to `working_tree`.
- Out of scope: the receipt schema, canonical body, signing, the reviewer sandbox, independence derivation, and the coverage/digest invariants `scafld verify` checks. The `base_delta` snapshot and verify paths already exist (sealed earlier as `receipt-attests-base-delta`); this spec only makes the seal *caller* use them by default.
- Out of scope: re-sealing or migrating the already-committed `working_tree` receipts under `.scafld/receipts/`; they remain historical attestations.
- Out of scope: changing the installed `ci/` asset workflow template for downstream user repos beyond what the install fix strictly requires. Downstream repos install a released scafld by version, which is correct once a version is published; the unreleased-version failure is specific to the scafld repo dogfooding its own unpublished build.
- Out of scope: making `scafld verify` enforce branch protection (that stays the operator's GitHub step, per `ci-verify-opt-in`).

## Dependencies

- `receipt-attests-base-delta`: owns the `base_delta` snapshot semantics and the `scafld verify` re-derivation (`merge-base(base_ref, HEAD)`, `tree_sha` over the working tree, target/base ancestry). This spec relies on that mechanism unchanged and only defaults the seal caller into it.
- `ci-verify-merge-gate` / `headline-path-executes`: own `scafld verify` and the verify workflow/script content. This spec touches only the scafld repo's live workflow install bootstrap, not the verify logic.
- `ci-verify-opt-in`: established that CI verify is the opt-in PR/merge gate and that `verify.policy` is reporting-only. This spec makes that gate actually pass for the direct-to-main dogfood flow.

## Assumptions

- At seal time the work is uncommitted and HEAD is the parent commit, so `base_ref = HEAD` yields `base_commit = merge-base(HEAD, HEAD) = HEAD = parent`. The operator then commits the work (and the receipt) as a single child of that parent.
- `tree_sha` is computed over the working tree, and `.scafld/receipts/**` is excluded from the fingerprint, so committing the receipt alongside the work does not change the scoped `tree_sha`; the committed head's scoped bytes equal the sealed bytes.
- CI passes the parent as the verify target: `SCAFLD_VERIFY_TARGET` is `github.event.pull_request.base.sha` (PR) or `github.event.before` (push), both of which resolve to the parent of the sealed commit for a single-commit change.
- `base_delta` against HEAD reviews exactly the same bytes the current `working_tree` seal reviews (the `parent -> working-tree` delta), so reviewer behavior and findings are unchanged.

## Touchpoints

- internal/adapters/cli/finalize/run.go (default `base_ref` to the resolved HEAD when the request omits it and a HEAD exists; thread into the snapshot/seal request shared by CLI and MCP)
- internal/adapters/git (expose/reuse HEAD resolution for the finalize adapter; the `Snapshot` base_delta path itself is unchanged)
- internal/adapters/cli/finalize/run_test.go (a base_delta-default seal over uncommitted work, committed, verifies against the parent; a no-HEAD workspace falls back to working_tree)
- .github/workflows/scafld-verify.yml (build scafld from the checked-out source for the scafld repo's own dogfood verify; remove the dependency on `go install .../scafld@<unreleased>`)

## Risks

- Changing the finalize default from `working_tree` to `base_delta` is a behavior change for every seal. Mitigation: the reviewed delta and signed bytes are identical; only `base_commit`/`snapshot_mode` change, and the result is strictly more verifiable. Hardening must confirm no caller depends on the `working_tree` `base_commit == HEAD-at-mint` shape, and that a no-HEAD workspace still seals (working_tree fallback).
- A receipt committed in the same commit as the work could be thought to perturb its own `tree_sha`. Mitigation: runtime receipt paths are excluded from the fingerprint and per-file digests; assert this with a test that seals, commits work+receipt together, and verifies.
- Multi-commit or squashed work could misalign the receipt's `base_commit` with CI's target. Mitigation: scope this spec to the single-commit direct-to-main and standard PR head flows; document that a squash/rebase that rewrites the head requires re-sealing, and let hardening decide whether to detect and warn.
- The install fix could diverge the scafld repo's live workflow from the installed asset template. Mitigation: keep the change confined to building-from-source in the scafld repo's own workflow; leave the downstream asset (released-version install) untouched and call out the divergence in deviations.

## Acceptance

Profile: strict

Validation:
- [x] `v1` validate spec - This spec validates under the current Markdown runtime.
  - Command: `go run ./cmd/scafld validate base-delta-seal`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-27
- [x] `v2` seal invariants untouched - The finalize app still signs, checks coverage, and re-derives independence; this spec did not weaken the core seal.
  - Command: `go test ./internal/app/finalize/...`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-28

## Phase 1: finalize defaults to base_delta against HEAD

Status: pass
Dependencies: none

Objective: Default the finalize `base_ref` to the resolved HEAD when the caller omits it and a HEAD commit exists, so seals mint `base_delta` receipts that verify against the parent->head delta after the work is committed. Keep `working_tree` only when there is no HEAD. The reviewer still reviews the `parent -> working-tree` delta; only `snapshot_mode` and `base_commit` change.

Changes:
- internal/adapters/cli/finalize/run.go - when the finalize request has an empty `base_ref` and the workspace resolves a HEAD commit, set `base_ref` to that HEAD before snapshot/seal; leave an explicit caller-provided `base_ref` (PR flow) untouched, and fall back to `working_tree` when there is no HEAD.
- internal/adapters/git - reuse the existing HEAD resolver for the finalize adapter (no new snapshot logic).
- internal/adapters/cli/finalize/run_test.go - seal over uncommitted work in a temp git repo, commit work+receipt, and assert `scafld verify --target <parent>` passes; assert a no-HEAD workspace still produces a `working_tree` receipt.

Acceptance:
- [x] `ac1_1` committed seal verifies - A base_delta seal over uncommitted work verifies against the parent after the work is committed.
  - Command: `go test ./internal/adapters/cli/finalize/... -run 'BaseDelta|Committed|PostCommit|VerifiesAfterCommit'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-29
- [x] `ac1_2` default base_ref resolves HEAD - The finalize default-to-HEAD path exists in the seal adapter.
  - Command: `go test ./internal/adapters/cli/finalize/... -run 'NoHead|WorkingTreeFallback'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-30

## Phase 2: green the dogfood verify workflow

Status: pass
Dependencies: phase1

Objective: Make the scafld repository's own `scafld verify` CI workflow build scafld from the checked-out source instead of `go install`-ing an unreleased version, so it can verify a freshly sealed `base_delta` receipt and go green. Do not change the verify logic or the downstream asset template.

Changes:
- .github/workflows/scafld-verify.yml - build scafld from the checked-out source (`go build ./cmd/scafld`) and put it on PATH before running the verify script, so the script's `command -v scafld` check finds it and skips the `go install .../scafld@<version>` bootstrap.

Acceptance:
- [x] `ac2_1` workflow builds from source - The scafld repo verify workflow builds scafld locally rather than depending on an unreleased published version.
  - Command: `rg -n 'go build .*\./cmd/scafld' .github/workflows/scafld-verify.yml`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-31
- [x] `ac2_2` no unreleased install pin - The workflow no longer hard-installs an unreleased scafld version as its only bootstrap.
  - Command: `rg -n 'go install .*cmd/scafld@v2\.4\.7' .github/workflows/scafld-verify.yml`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-32

## Rollback

- The change is additive and default-shifting in one place. Restore `working_tree` as the default by not defaulting `base_ref` to HEAD in the finalize adapter, and revert the workflow build-from-source step to the prior `go install` bootstrap. No receipts, keys, signing, or verify logic are touched, so there is no data migration; historical receipts are unaffected either way.

## Review

Status: not_started
Verdict: none

Findings:
- none

## Self Eval

- The seal stays on the uncommitted working tree and reviews the same `parent -> working-tree` delta; only `base_commit`/`snapshot_mode` change, so the independent review and the signed bytes are unchanged.
- The result is strictly more verifiable: a committed `base_delta` receipt re-derives `base_commit = merge-base(parent, HEAD) = parent` and the working-tree `tree_sha` matches the committed scoped bytes, closing the `base_commit mismatch` gap.
- Runtime receipt paths are excluded from the fingerprint, so committing the receipt with the work does not perturb its own `tree_sha`; this must be pinned by a test, not assumed.
- The install fix is confined to the scafld repo's own workflow; downstream user repos keep installing a released scafld by version. The divergence is intentional and recorded.
- Open question for hardening: should `base_delta`-against-HEAD be the unconditional default, or gated behind a config/flag for callers who deliberately want a point-in-time `working_tree` attestation? Defaulting keeps the dogfood flow green with no per-seal ceremony; a flag preserves an escape hatch.

## Deviations

- none

## Metadata

- created_by: scafld
- estimated_effort_hours: 4-8
- priority: p1

## Origin

Created by: scafld
Source: plan

## Harden Rounds

- none

## Planning Log

- Root cause traced live: committed `working_tree` receipts fail `scafld verify` with `base_commit mismatch` because `working_tree` pins `base_commit` to HEAD-at-mint while verify re-derives it as the current (moved) HEAD; a second failure is the dogfood workflow's `go install .../scafld@v2.4.7` against an unreleased version. Snapshot math (`git.Adapter.Snapshot`: `tree_sha` over the working tree, `base_commit = merge-base(base_ref, HEAD)` only in base_delta mode) confirms that passing `base_ref = HEAD` at seal time yields a receipt that verifies post-commit. Plumbing already exists end to end: `snapshotMode(base_ref)` returns `base_delta` when `base_ref` is set, and `Request.BaseRef` flows to `SnapshotInput.BaseRef`.
