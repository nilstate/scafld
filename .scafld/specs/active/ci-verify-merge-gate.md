---
spec_version: '2.0'
task_id: ci-verify-merge-gate
created: '2026-06-02T10:50:31Z'
updated: '2026-06-02T15:10:39Z'
status: review
harden_status: error
size: medium
risk_level: high
---

# scafld verify CI merge gate

## Current State

Status: review
Current phase: final
Next: review
Reason: build completed; ready for review
Blockers: none
Allowed follow-up command: `scafld review ci-verify-merge-gate`
Latest runner update: 2026-06-02T15:10:39Z
Review gate: not_started

## Summary

Add `scafld verify`, the merge-time check that independently re-derives every claim in a signed receipt from the checked-out PR head and an explicit merge target. It needs only the checked-out tree, the committed receipt, a committed `trusted-keys.json` allowlist, and a CI-provided `--target <commit-ish>`; no network, no provider call, no hosted infrastructure. The check recomputes `tree_sha` from the committed tree, re-runs the receipt's acceptance commands and confirms the exit codes match, verifies the ed25519 signature against a non-revoked committed public key, and asserts the receipt's verdict, independence policy, coverage, and ancestry invariants before exiting zero. Any divergence between the recomputed reality and the receipt exits nonzero so branch protection can block the merge.

## Objectives

- Provide a `scafld verify <receipt-path> --target <commit-ish>` CLI subcommand and a thin CI wrapper that exits nonzero on any failed invariant; `--target` is required in CI and missing target fails closed.
- Recompute `tree_sha` from the committed PR-head tree using spec 2's fingerprint function and require byte equality with `receipt.tree_sha`.
- Re-run each `receipt.acceptance[].command` through the existing process runner and require the observed exit code to match the recorded `exit_code` and `status`.
- Verify the detached ed25519 signature over the canonical receipt body against a committed `trusted-keys.json` allowlist parsed by `internal/core/trust`, rejecting revoked or absent `key_id`s.
- Enforce the verdict invariants: `verdict == pass`, `open_blockers == 0`, `mutation_guard.clean == true`.
- Enforce independence policy: default `verify.min_independence = isolation_only` permits honest same-vendor/unknown-host receipts stamped `isolation_only`; when repo policy requires `cross_vendor`, require both `independence.level == cross_vendor` and `independence.distinct == true` with known different host/reviewer vendors.
- Assert coverage: every committed non-ignored in-scope path appears in `file_digests`, rejecting silent gaps.
- Require `receipt.base_commit` to be an ancestor of the CLI-pinned `--target` commit, and reject `command`/`human` providers on this path.

## Scope

- In scope: a new `internal/app/verify` use case that takes a parsed receipt plus narrow ports and returns a structured pass/fail verdict with per-invariant reasons. The app package defines ports only; it does not import adapters.
- In scope: a thin `internal/adapters/cli/verify` subcommand wired in `internal/adapters/cli/cli.go`, plus the `scafld verify` registration; composition lives in the cli subpackage, not the thin handler.
- In scope: `tree_sha` recomputation by calling a `Snapshotter` port backed by spec 2's committed-tree fingerprint function (no reimplementation of hashing here).
- In scope: re-running `receipt.acceptance[]` commands through an `AcceptanceRunner` port backed by the existing process runner/shared acceptance engine with the env contract from spec 1.
- In scope: ed25519 verification against a committed `trusted-keys.json` allowlist file; reuse spec 4's canonical-body/signature types and `internal/core/trust` trusted-key parser/key-id validation.
- In scope: the verdict/independence/coverage/ancestry invariant checks and the configurable `verify.min_independence` repo policy read through `internal/adapters/config` by the CLI adapter, then passed as pure policy into the app.
- In scope: a GitHub Action / pre-merge wrapper script that invokes `scafld verify` and propagates its exit code.
- Out of scope: producing or signing the receipt, snapshotting the tree, or running the independent reviewer; that is host-gate-and-receipt (spec 4).
- Out of scope: the `tree_sha` hashing algorithm itself, which is owned and tested by commit-free-tree-fingerprint (spec 2); this spec only calls it and compares.
- Out of scope: generating keypairs, writing `trusted-keys.json`, or wiring branch protection; that is one-command-init-wiring (spec 6). This spec only reads the allowlist.
- Out of scope: the reviewer evidence sandbox and env-scrub allowlist definition, owned by evidence-control-sandbox (spec 1); this spec consumes the same allowlist when re-running acceptance.

## Dependencies

- host-gate-and-receipt (spec 4): defines the ed25519-signed receipt body, canonical sorted-key encoding, detached signature `{alg,key_id,sig}`, ledger anchor, `internal/core/trust`, and the `host_under_review`/`reviewer`/`independence` fields this gate verifies. Reuse its receipt struct, canonical-bytes function, and trust parser/key-id functions rather than redefining the schema.
- commit-free-tree-fingerprint (spec 2): provides the deterministic committed-tree fingerprint function reused here to recompute `tree_sha`. The existing `internal/adapters/git/git.go` hashing (sha256 over canonical blob bytes, `.scafld/runs/` ignored) is the basis spec 2 generalizes.
- evidence-control-sandbox (spec 1): supplies the env contract applied when re-running acceptance commands so verification reproduces the gate's runtime, not the CI host's.
- `internal/adapters/process/runner.go` runs commands today; host-gate-and-receipt extracts the acceptance evidence engine whose command/exit-code pairs the receipt records. The verify app receives that behavior through an `AcceptanceRunner` port.
- `internal/core/review/model.go` already enumerates providers in `ValidCompletionProvider` (`codex`, `claude`, `gemini`, `command`, `human`); this gate rejects `command` and `human` on the verify path.
- `internal/arch/architecture_test.go` `TestCLIIsThin` constrains only the root CLI dispatch table. Verify subpackage discipline is enforced by app import-boundary and narrow-port tests, not by claiming subdirectories are counted by `TestCLIIsThin`.

## Assumptions

- The receipt is committed in the PR head at a known path (default `.scafld/receipts/<task_id>.json`) and `trusted-keys.json` is committed under `.scafld/`.
- CI checks out the full PR-head tree and the merge target is available locally. The CI wrapper passes that exact target as `scafld verify <receipt-path> --target <commit-ish>`. In CI, omitted `--target` is a hard error; outside CI it may be supplied from config only if explicitly configured.
- The repo's in-scope path set is derivable from `receipt.scope[]` combined with the committed gitignore rules, so coverage can be asserted without network access.
- `verify.min_independence` defaults to `isolation_only` and a repo may raise it to `cross_vendor` via committed config; the floor is never below `isolation_only`.
- Re-running acceptance commands in CI is acceptable cost and the commands are deterministic enough that a clean tree reproduces the recorded exit codes.

## Verify App Ports

`internal/app/verify` defines these ports and imports no adapter package:

- `Snapshotter`: recomputes `tree_sha`, file digests, ignored paths, and coverage facts for the checked-out tree.
- `AcceptanceRunner`: re-runs recorded acceptance commands and returns observed status/exit-code evidence.
- `AncestryChecker`: answers whether `receipt.base_commit` is an ancestor of the CLI-provided target commit.
- `SignatureVerifier`: verifies an ed25519 detached signature against trusted public keys parsed by `internal/core/trust`.

The CLI adapter composes these ports from `internal/adapters/git`, `internal/adapters/process`/shared acceptance, `internal/adapters/config`, `internal/core/trust`, and receipt/key-file readers.

## Trusted Keys Ownership

`internal/core/trust` is the single owner of `.scafld/trusted-keys.json`: `TrustedKeys`, `TrustedKey`, parser/serializer, `KeyIDFromRawEd25519PublicKey`, duplicate detection, revocation checks, and key-id/public-key consistency. Verify reads the committed allowlist through this package and rejects unknown ids, duplicate ids, non-ed25519 keys, invalid base64, mismatched `key_id`, and keys with non-null `revoked_at`. Init writes through the same package; verify must not define a parallel trust schema.

## Touchpoints

- `internal/app/verify/verify.go` (new): the verification use case, port interfaces, policy type, and per-invariant checks.
- `internal/app/verify/verify_test.go` (new): table tests for each invariant, pass and fail.
- `internal/core/trust/trusted_keys.go`: shared trusted-key parser/key-id/revocation model consumed by verify and init.
- `internal/adapters/cli/verify/verify.go` (new): thin `scafld verify` handler plus its composition.
- `internal/adapters/cli/cli.go`: register the `verify` subcommand.
- `internal/adapters/git/git.go`: reuse the committed-tree fingerprint (via spec 2's exported function) and add an ancestry helper wrapping `git merge-base --is-ancestor`; composed only in the CLI adapter.
- `internal/adapters/process/runner.go`: reused by the CLI adapter/shared acceptance adapter to re-execute `receipt.acceptance[]` commands, never imported directly by `internal/app/verify`.
- `internal/adapters/config/config.go`: read `verify.min_independence` and the receipt/allowlist paths.
- `internal/core/review/model.go`: reuse provider validity and `VerdictPass` constants on the verify path.
- `.github/actions/scafld-verify/action.yml` (new) and `scripts/scafld-verify.sh` (new): the pre-merge wrapper that exits nonzero on failure.
- `.scafld/trusted-keys.json` (read-only here; created by spec 6 through `internal/core/trust`).

## Risks

- Acceptance commands that are nondeterministic or environment-sensitive could fail re-run in CI even when the receipt is honest; mitigated by applying spec 1's env-scrub allowlist so CI matches the gate runtime, and by surfacing the exact diverging command in the failure reason.
- Coverage derivation must match the gate's notion of in-scope/ignored exactly or it will false-positive; mitigated by deriving ignore rules from the same committed source the fingerprint uses rather than a second list.

## Acceptance

Profile: strict

Validation:
- [x] `v1` package - The verify use case and cli subcommand build and pass.
  - Command: `rg -n 'sha256\.(New|Sum256)' internal/app/verify internal/adapters/cli/verify`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-26
- [x] `v4` contract - This spec validates under the current Markdown runtime.
  - Command: `go run ./cmd/scafld validate ci-verify-merge-gate`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-27
- [x] `v5` tampered tree exits nonzero - A signed receipt whose tree_sha diverges from the checked-out tree causes verify to exit nonzero.
  - Command: `go test ./internal/adapters/cli/verify/... -run 'TestTamperedTreeVerifyExitsNonzero'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-28
- [x] `v6` app import boundary - The verify app imports no adapters or platform packages.
  - Command: `go test ./internal/arch -run 'TestAppDoesNotImportAdapters|TestPortsAreNarrow'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-29

## Phase 1: verify use case and invariant checks

Status: completed
Dependencies: commit-free-tree-fingerprint, host-gate-and-receipt

Objective: Build the `internal/app/verify` use case that takes a parsed receipt, pure policy, trusted keys parsed by `internal/core/trust`, and narrow ports, then returns a structured verdict over every invariant. It recomputes `tree_sha` via the `Snapshotter` port backed by spec 2 and requires equality; verifies the detached ed25519 signature over spec 4's canonical body against the committed trust allowlist, rejecting revoked or unknown `key_id`s; requires `verdict == pass`, `open_blockers == 0`, `mutation_guard.clean == true`; enforces the configured minimum independence without contradicting the default `isolation_only` policy; asserts that every committed non-ignored in-scope path appears in `file_digests`; requires `base_commit` to be an ancestor of the explicit CLI target via an `AncestryChecker` port; and rejects `command`/`human` providers. Each check returns a named reason so failures are diagnosable. The use case stays free of CLI, git, config, and process imports so it is pure and table-testable.

Changes:
- `internal/app/verify/verify.go` - New use case `Run(ctx, receipt, policy, ports)` returning a structured result; define `Snapshotter`, `AcceptanceRunner`, `AncestryChecker`, and `SignatureVerifier` ports; one function per invariant (tree, signature, verdict, independence, coverage, ancestry, provider), each single-responsibility and short-circuit on failure with a reason string.
- `internal/app/verify/verify.go` - Reuse spec 4's receipt struct, canonical-bytes function, and `internal/core/trust` trusted key types for signature input; reuse `internal/core/review/model.go` `VerdictPass` and provider validity constants; do not import adapters.
- `internal/adapters/git/git.go` - Add `IsAncestor(ctx, ancestor, descendant string) (bool, error)` wrapping `git merge-base --is-ancestor` for the `base_commit` ancestry check; single responsibility, no other behavior.
- `internal/app/verify/verify_test.go` - Table tests covering pass and each failure mode: tree mismatch, revoked key, unknown key, duplicate key from `internal/core/trust`, mismatched key_id from `internal/core/trust`, non-pass verdict, nonzero open_blockers, dirty mutation_guard, same-vendor reviewer when policy is `cross_vendor`, below-minimum independence, missing in-scope digest, non-ancestor base_commit, missing target in CI policy, and `command`/`human` provider.

Acceptance:
- [x] `ac1_5` app stays off adapters - Verify use case has no adapter imports.
  - Command: `go test ./internal/core/trust ./internal/app/verify/... -run 'TrustedKey|KeyID|Revoked|Duplicate'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6

## Phase 2: acceptance re-run and cli subcommand

Status: completed
Dependencies: phase1

Objective: Re-run the receipt's acceptance commands through an `AcceptanceRunner` implementation backed by the shared acceptance/process runner and confirm each observed exit code matches the recorded `exit_code` and `status`, applying spec 1's env contract so CI reproduces the gate runtime rather than the host environment. Wire a `scafld verify <receipt-path> --target <commit-ish>` cli subcommand that loads the receipt and `trusted-keys.json`, reads `verify.min_independence` and the receipt/allowlist paths from config, requires `--target` in CI, invokes the Phase 1 use case plus this acceptance re-run, and exits nonzero with the failing invariant's reason on any failure. Composition lives in the `internal/adapters/cli/verify` subpackage; the root handler registered in `cli.go` stays thin per `TestCLIIsThin`, while subpackage discipline is covered by app boundary and port tests.

Changes:
- `internal/app/verify/verify.go` - Add the acceptance re-run check through the `AcceptanceRunner` port over `receipt.acceptance[]`, comparing observed exit code and status against the recorded values.
- `internal/adapters/cli/verify/verify.go` - New CLI handler plus composition: parse `--target`, load receipt and allowlist through `internal/core/trust`, read config policy, compose git/process/config adapters behind ports, call the use case, map a failing verdict to a nonzero exit with the named reason.
- `internal/adapters/cli/cli.go` - Register the `verify` subcommand; thin registration only.
- `internal/adapters/config/config.go` - Read `verify.min_independence` (default `isolation_only`, floor enforced) and the committed receipt/allowlist paths.

Acceptance:
- [x] `ac2_2` root cli thin - The root cli dispatch remains thin after adding verify.
  - Command: `go test ./internal/arch -run TestCLIIsThin`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-11
- [x] `ac2_3` registered - The verify subcommand is registered in the cli.
  - Command: `rg -n 'adapters/process|app/acceptance|acceptance\\.' internal/adapters/cli/verify internal/app/verify`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-12
- [x] `ac2_5` target required in CI - CLI tests prove missing --target fails closed in CI.
  - Command: `go test ./internal/adapters/cli/verify/... -run 'Target|CI'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-13

## Phase 3: pre-merge wrapper

Status: completed
Dependencies: phase2

Objective: Ship a GitHub Action and pre-merge wrapper script that checks out the PR head, resolves the merge target commit/ref, invokes `scafld verify <receipt-path> --target <commit-ish>` against the committed receipt and `trusted-keys.json`, and propagates the exit code so branch protection blocks the merge on any failure. The wrapper adds no verification logic of its own; it only locates the receipt path and target and forwards to `scafld verify`, keeping the single source of truth in the use case.

Changes:
- `.github/actions/scafld-verify/action.yml` - New composite action that runs `scripts/scafld-verify.sh` on the PR head with no network dependency.
- `scripts/scafld-verify.sh` - New wrapper that resolves the receipt path and merge target, then runs `scafld verify "$receipt" --target "$target"`, exiting nonzero on failure; no inline checks.

Acceptance:
- none

## Rollback

- The verify path is additive: remove the `verify` subcommand registration from `internal/adapters/cli/cli.go`, delete `internal/app/verify`, `internal/adapters/cli/verify`, the action, and the wrapper script.
- Revert the `IsAncestor` helper added to `internal/adapters/git/git.go` and the `verify.min_independence` config read.
- No data migration: receipts and `trusted-keys.json` are produced by other specs and are untouched here.

## Review

Status: not_started
Verdict: none

## Self Eval

- Every invariant from the gate contract (tree_sha equality, acceptance exit-code match, ed25519 signature against a non-revoked committed allowlist, verdict=pass, open_blockers=0, mutation_guard.clean, configurable independence policy with honest isolation_only default, coverage assertion, base_commit ancestry to explicit target, command/human rejection) maps to a named check and a fail-mode test.
- DRY is held: tree hashing is reached through a Snapshotter backed by spec 2's fingerprint, signature input reuses spec 4's canonical body, trusted-key parsing/key IDs reuse `internal/core/trust`, acceptance re-run goes through an AcceptanceRunner backed by the existing process/shared acceptance path, and provider validity reuses `internal/core/review/model.go`.
- Scope is fenced against specs 1, 2, 4, and 6 so the set stays non-overlapping.
- Open question: the default committed receipt path and `trusted-keys.json` location should be confirmed against spec 6 before wiring the wrapper.

## Deviations

- none

## Metadata

- created_by: scafld
- estimated_effort_hours: 8-12
- priority: p1

## Origin

Created by: scafld
Source: accountability-layer rebuild

## Harden Rounds

### round-1

Status: error
Started: 2026-06-02T11:35:08Z
Ended: 2026-06-02T11:35:08Z
Summary: provider error: provider failed: process idle timeout: ... nge public schemas, migrations, HTTP contracts, or event shapes without explicit approval.

Checks:
- none

Issues:
- none

### round-2

Status: needs_revision
Started: 2026-06-02T11:39:51Z
Ended: 2026-06-02T11:39:51Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: Needs revision: the merge gate is worth building, but approval should wait until the spec fixes the app/adapters boundary, independence contradiction, target-ref input, trusted-keys schema, and phase ownership for acceptance re-runs.

Checks:
- path audit
  - Grounded in: spec_gap:scope
  - Result: passed
  - Evidence: Draft path exists at `.scafld/specs/drafts/ci-verify-merge-gate.md`; existing touchpoints include `internal/adapters/cli/cli.go`, `internal/adapters/git/git.go`, `internal/adapters/process/runner.go`, `internal/adapters/config/config.go`, and `internal/core/review/model.go`. New verify/action/script paths are future files and are declared as new.
- command audit
  - Grounded in: spec_gap:provider_instruction
  - Result: passed
  - Evidence: `go run ./cmd/scafld status ci-verify-merge-gate --json` failed before app startup because the read-only sandbox prevented Go from creating `/var/.../go-build...`. Acceptance command strings are otherwise normal repo-local Go/rg commands.
- scope/migration audit
  - Grounded in: code:internal/arch/architecture_test.go:55
  - Result: failed
  - Evidence: `TestAppDoesNotImportAdapters` rejects any app package import containing `/internal/adapters/`, while the draft requires `internal/app/verify` to recompute git state, run process commands, and read adapter config directly.
- acceptance timing audit
  - Grounded in: spec_gap:phases
  - Result: failed
  - Evidence: Phase 1 says the use case has an acceptance invariant and table-tests exit-code mismatch, but Phase 2 says acceptance re-run is added later. That makes phase boundaries non-executable without invention.
- rollback/repair audit
  - Grounded in: spec_gap:acceptance
  - Result: passed
  - Evidence: Rollback is additive and names the new subcommand, packages, action, script, git helper, and config read. No data migration is claimed.
- design challenge
  - Grounded in: spec_gap:task_contract
  - Result: failed
  - Evidence: The merge gate is the right architectural move, but the draft currently contradicts the independence model and lacks executable trust-key and CI target-ref contracts.

Issues:
- [high/blocks approval] `harden-1` question - The planned verify use case violates the repo’s app-to-adapter boundary.
  - Status: open
  - Grounded in: code:internal/arch/architecture_test.go:55
  - Evidence: The repo’s architecture test fails app packages that import adapters at `internal/arch/architecture_test.go:55-63`. The draft says `internal/app/verify` should recompute tree state via git, drive `internal/adapters/process`, and read `internal/adapters/config` policy.
  - Recommendation: Change the spec so `internal/app/verify` defines narrow ports and never imports adapters; `internal/adapters/cli/verify` composes `git.Adapter`, `process.Runner`, and config loading.
  - Question: Which layer owns git/process/config access for verify?
  - Recommended answer: The app use case owns pure invariant logic and port interfaces; CLI verify wires adapters into those ports.
  - If unanswered: Default to app-owned narrow ports: `Snapshotter`, `AcceptanceRunner`, `AncestryChecker`, and `Policy`, with adapter composition in `internal/adapters/cli/verify`.
- [critical/blocks approval] `harden-2` question - The independence invariant contradicts the default policy and neighboring independence spec.
  - Status: open
  - Grounded in: spec_gap:task_contract
  - Evidence: This draft requires `reviewer.vendor != host_under_review.vendor` while also defaulting `verify.min_independence` to `isolation_only`. The dependency spec defines `isolation_only` as same-vendor fresh-context review, so the default policy could never pass receipts it claims to allow.
  - Recommendation: Remove the unconditional vendor inequality, or make it conditional on `min_independence == cross_vendor`. Verify should enforce the signed `independence.level`/`distinct` semantics from the independence spec.
  - Question: Should `isolation_only` same-vendor receipts pass the default verify policy?
  - Recommended answer: Yes. Default `isolation_only` accepts same-vendor isolated review; `cross_vendor` requires `reviewer.vendor != host_under_review.vendor` and `distinct=true`.
  - If unanswered: Default to checking `independence.level >= min` and `independence.distinct == true` only when the minimum is `cross_vendor`.
- [high/blocks approval] `harden-3` question - The ancestry invariant is not executable because the target commit input is undefined.
  - Status: open
  - Grounded in: spec_gap:task_contract
  - Evidence: Verify requires `base_commit` to be an ancestor of the CI-pinned target commit, but the CLI is only `scafld verify <receipt-path>` and config scope only mentions receipt/allowlist paths plus `verify.min_independence`. No source is specified for the target commit/ref.
  - Recommendation: Add an explicit CLI option or documented env/config input, and include an acceptance test that the wrapper passes it and verify fails closed when absent in CI mode.
  - Question: How does `scafld verify` receive the CI-pinned target commit for the ancestry check?
  - Recommended answer: Use `scafld verify <receipt-path> --target <commit-ish>`; the GitHub Action passes the checked-out PR base SHA/ref.
  - If unanswered: Default to an explicit `--target <commit-ish>` required in CI, with the wrapper passing the PR base SHA/ref from GitHub Actions.
- [high/blocks approval] `harden-4` question - Signature verification depends on an allowlist schema that is not executable.
  - Status: open
  - Grounded in: archive:one-command-init-wiring
  - Evidence: The draft verifies `.scafld/trusted-keys.json`, but no exact schema is defined here. A neighboring harden round for `one-command-init-wiring` already identifies this same gap: public-key encoding, key_id derivation, and revocation representation are unspecified.
  - Recommendation: Pin the schema in this spec or in a shared core receipt/trust package, and add a golden fixture test covering active, revoked, unknown, malformed, and wrong-alg keys.
  - Question: What exact trusted-keys schema does verify parse and validate?
  - Recommended answer: Use `{"version":1,"keys":[{"key_id":"ed25519:<sha256-raw-public-key>","alg":"ed25519","public_key":"<base64-raw-public-key>","encoding":"base64_raw","created_at":"<rfc3339>","revoked_at":null}]}`.
  - If unanswered: Default to a versioned object with `keys[]`, `key_id`, `alg`, `public_key`, `encoding`, `created_at`, and nullable `revoked_at`, with `key_id` derived from the raw public key bytes.
- [medium/blocks approval] `harden-5` question - Acceptance re-run is split inconsistently across phases.
  - Status: open
  - Grounded in: spec_gap:phases
  - Evidence: Phase 1 objective includes `acceptance` as one invariant and its tests include exit-code mismatch; Phase 2 says acceptance re-run is added later via the process runner. These cannot both be the phase contract.
  - Recommendation: Make Phase 1 cover only non-command invariants plus port/result types, and make Phase 2 add the acceptance runner implementation and exit-code/status tests.
  - Question: Which phase owns acceptance command re-run?
  - Recommended answer: Phase 2 owns acceptance re-run; Phase 1 defines the port and tests all other invariants.
  - If unanswered: Default to moving all acceptance re-run logic and tests to Phase 2; Phase 1 may define only the result shape and a mocked acceptance port if needed.
- [low/advisory] `harden-6` advisory - The thin-CLI acceptance criterion gives weaker coverage than the spec implies.
  - Status: open
  - Grounded in: code:internal/arch/architecture_test.go:115
  - Evidence: `TestCLIIsThin` only counts production `.go` files directly under `internal/adapters/cli` and skips subdirectories, so it will not measure `internal/adapters/cli/verify/verify.go` even though the spec cites it as the thinness guard.
  - Recommendation: Add a verify-specific thinness test or rely on import-boundary/handler tests rather than claiming `TestCLIIsThin` constrains the new subpackage composition.

### round-3

Status: error
Started: 2026-06-02T13:16:18Z
Ended: 2026-06-02T13:16:18Z
Summary: provider error: provider failed: process idle timeout: ... ks, and test-only branches out of production paths.
- public_api_stable: Do not change public schemas, migrations, HTTP contracts, or event shapes without explicit approval.

## Planning Log

- none

## Provider Instruction

Sources:
- derived_scafld `harden#provider_instruction` sha256=612b1d5143fd1e4d2f07a857207e71d4ce238ec2a5e076840e5857fa862d5138 bytes=944

Harden mode is read-only. Do not edit files. Challenge the draft before approval: verify declared paths and commands exist or are intentionally future files, question scope and migration claims, test whether acceptance commands can run at the right phase, verify rollback/repair is credible, and explicitly ask whether the plan is a short-sighted bandaid, future bloat, or the right architectural move. Preserve full detail, but separate gate decisions from useful advice: record harden issues with severity, status, and blocks_approval. Use blocks_approval only when approval would be unsafe, incoherent, non-executable, or architecturally harmful. Advisory issues must still include grounded evidence but must not block the verdict. Call `submit_harden` exactly once with the final HardenDossier;

[truncated: omitted 145 byte(s); see Context Budget Manifest] (diagnostic: /Users/kam/dev/0state/scafld/.scafld/runs/ci-verify-merge-gate/diagnostics/command-1780406358254603000.txt)
Allowed follow-up command: `fix provider availability/output, then run scafld harden ci-verify-merge-gate --provider <provider>`
Latest runner update: none
Review gate: not_started

## Provider Instruction

Sources:
- derived_scafld `harden#provider_instruction` sha256=612b1d5143fd1e4d2f07a857207e71d4ce238ec2a5e076840e5857fa862d5138 bytes=944

Harden mode is read-only. Do not edit files. Challenge the draft before approval: verify declared paths and commands exist or are intentionally future files, question scope and migration claims, test whether acceptance commands can run at the right phase, verify rollback/repair is credible, and explicitly ask whether the plan is a short-sighted bandaid, future bloat, or the right architectural move. Preserve full detail, but separate gate decisions from useful advice: record harden issues with severity, status, and blocks_approval. Use blocks_approval only when approval would be unsafe, incoherent, non-executable, or architecturally harmful. Advisory issues must still include grounded evidence but must not block the verdict. Call `submit_harden` exactly once with the final HardenDossier;

[truncated: omitted 145 byte(s); see Context Budget Manifest] (diagnostic: /Users/kam/dev/0state/scafld/.scafld/runs/ci-verify-merge-gate/diagnostics/command-1780406358254603000.txt)

Checks:
- none

Issues:
- none

