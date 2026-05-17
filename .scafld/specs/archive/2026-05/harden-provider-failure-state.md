---
spec_version: '2.0'
task_id: harden-provider-failure-state
created: '2026-05-17T13:30:40Z'
updated: '2026-05-17T13:38:12Z'
status: completed
harden_status: passed
size: small
risk_level: low
---

# Close provider harden rounds on provider failure

## Current State

Status: completed
Current phase: final
Next: done
Reason: task completed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-05-17T13:38:12Z
Review gate: pass

## Summary

When an external harden provider fails after scafld opens a round, close that round as failed with a rerun path instead of leaving draft state in progress.

## Objectives

- Preserve deterministic harden state when provider-backed hardening fails after
  opening a round.
- Record provider invocation or dossier-validation failure as a failed harden
  round with an ended timestamp and explicit rerun path.
- Preserve both errors if scafld cannot record the provider failure state.
- Cover the failure path with a focused app-layer test.

## Scope

- In: `internal/app/harden/harden.go` provider-backed harden failure handling.
- In: `internal/app/harden/harden_test.go` coverage for provider invocation
  errors after a round is opened.
- Out: provider timeout policy, provider CLI transport flags, and release
  packaging.
- Out: changing the manual harden flow.

## Dependencies

- Existing provider-backed harden implementation.
- Spec store behavior for saving an in-progress round before provider
  invocation.

## Assumptions

- A provider failure is different from a `needs_revision` dossier, but both must
  leave the draft in a deterministic state.
- The failed provider round should remain evidence; scafld should not delete it
  or silently retry.
- The operator can repair provider availability/output and rerun harden to append
  a new round.

## Touchpoints

- `internal/app/harden/harden.go`
- `internal/app/harden/harden_test.go`

## Risks

- Masking the original provider error or the failed-round save error; mitigate by
  returning both with `errors.Join`.
- Writing misleading state if another harden round appears while the provider is
  running; mitigate by checking the latest round ID before closing failure state.
- Treating provider failure as harden pass; mitigate by setting
  `harden_status: failed` and preserving a rerun command.

## Acceptance

Profile: standard

Validation:
- [x] `harden-provider-failure-test` command - Harden provider failure state tests pass
  - Command: `go test ./internal/app/harden -run 'TestRunProviderHarden(ClosesRoundOnProviderError|ClosesRoundOnInvalidDossier|ReportsFailureRecordingError)'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-9

## Phase 1: Implementation

Status: completed
Dependencies: none

Objective: Complete the requested change.

Changes:
- Add a small helper that reloads the draft, verifies the latest harden round ID, marks that round failed, stores the provider failure reason, and updates current state to a rerun path.
- Call the helper when provider invocation fails or when the returned dossier fails strict validation.
- Return a joined error if the helper cannot save the failed-round state.
- Add focused tests proving provider invocation errors and invalid dossiers close the round and leave `harden_status: failed`.
- Add a focused test proving the original provider error and failure-recording error are both surfaced.

Acceptance:
- [x] `ac1` command - Provider harden failure tests pass
  - Command: `go test ./internal/app/harden -run 'TestRunProviderHarden(ClosesRoundOnProviderError|ClosesRoundOnInvalidDossier|ReportsFailureRecordingError)'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6

## Rollback

- Revert the helper and focused test.
- Rerun `go test ./internal/app/harden` after rollback to confirm the previous
  behavior is restored.

## Review

Status: completed
Verdict: pass
Mode: discover
Provider: command:deterministic
Output: command.stdout
Summary: Focused review passed: provider harden failure paths are covered by named tests and the spec matches the implementation scope.

Attack log:
- `internal/app/harden/harden.go`: verify provider invocation failure, invalid dossier failure, and failure-recording error handling -> clean (The named acceptance tests cover the state transitions and joined error behavior.)

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

Status: failed
Started: 2026-05-17T13:31:11Z
Ended: 2026-05-17T13:31:11Z
Verdict: needs_revision
Provider: claude
Model: claude-opus-4-7
Output format: claude.mcp_submit_harden
Summary: Spec scope and risks are well-formed and the touchpoints exist, but two objectives are under-covered: the planned test only exercises provider-invocation failure, not the dossier-validation failure path the spec also commits to recording; and the helper's save errors are silently swallowed, leaving the round inconsistent on a failure-on-failure. The acceptance command also passes trivially given the helper and test already exist as uncommitted modifications, so the criterion does not prove the spec's intent.

Checks:
- path audit
  - Grounded in: code:internal/app/harden/harden.go:171
  - Result: passed
  - Evidence: Both touchpoints exist: harden.go already defines closeProviderHardenFailure at lines 171-190, and harden_test.go contains TestRunProviderHardenClosesRoundOnProviderError at line 277. No undeclared new files.
- command audit
  - Grounded in: code:internal/app/harden/harden_test.go:1
  - Result: passed
  - Evidence: `go test ./internal/app/harden` is a standard Go test invocation runnable from the repo root; the package exists with a valid test file.
- scope/migration audit
  - Grounded in: spec_gap:Scope
  - Result: passed
  - Evidence: Spec scopes In to harden.go and harden_test.go and explicitly excludes timeout policy, transport flags, and the manual flow. Helper writes new CurrentState.Reason 'external hardening provider failed' and AllowedFollowUp 'fix provider availability/output...' (harden.go:186,188) which are user-visible CLI/JSON strings, but no archived contract pins those exact strings, so no migration is required.
- acceptance timing audit
  - Grounded in: spec_gap:Acceptance
  - Result: failed
  - Evidence: Acceptance is `go test ./internal/app/harden`. The helper at harden.go:171 and the test at harden_test.go:277 are already present in the working tree (per starting git status `M internal/app/harden/harden.go` and `M internal/app/harden/harden_test.go`), so the criterion already passes before any further build work. As written it cannot distinguish a successful implementation of this spec from a no-op build.
- rollback/repair audit
  - Grounded in: code:internal/app/harden/harden.go:123
  - Result: failed
  - Evidence: Rollback only says revert helper and test, but closeProviderHardenFailure's own return value is ignored at harden.go:123 and :127 (`_ = closeProviderHardenFailure(...)`). If Save inside the helper fails (disk, concurrent rename, validation), the round stays Status: in_progress and harden_status remains in_progress while Run returns only the original provider error — the very state this spec promises to eliminate. Repair path for failure-on-failure is undefined.
- design challenge
  - Grounded in: spec_gap:Phase 1
  - Result: failed
  - Evidence: Phase 1 commits to covering 'provider invocation or dossier-validation failure', but the planned test only covers provider invocation (fakeHardenProvider{err: ...} at harden_test.go:283). The dossier-validation branch at harden.go:126-129 has no focused test, so the failed-round-close behavior on an invalid dossier is asserted only by inspection. This leaves half the stated objective unverified.

Questions:
- Should the acceptance test be a new named test (e.g. TestRunProviderHardenClosesRoundOnInvalidDossier) that exercises the dossier-validation failure path, in addition to the existing provider-invocation test?
  - Grounded in: code:internal/app/harden/harden.go:126
  - Recommended answer: Yes. Add a second focused test in harden_test.go that returns a dossier failing coreharden.ValidateDossier and asserts the round closes with Status: failed, EndedAt set, and Summary containing 'invalid provider dossier'. Without it the spec's stated objective ('Record provider invocation or dossier-validation failure as a failed harden round') is not actually verified.
  - If unanswered: Default to narrowing the spec objective to provider-invocation failure only and remove the dossier-validation claim from Objectives and Phase 1.
- How should closeProviderHardenFailure's own Save error be surfaced when it itself fails after a provider error?
  - Grounded in: code:internal/app/harden/harden.go:123
  - Recommended answer: Replace `_ = closeProviderHardenFailure(...)` with a path that joins the close-failure into the returned error (e.g. errors.Join) and logs/returns enough context for the operator to see both. Silently dropping the close error leaves harden_status: in_progress on disk while Run reports only the provider error — exactly the inconsistent state this spec is supposed to remove.
  - If unanswered: Explicitly document in Risks and Rollback that a Save failure inside the helper leaves the round in_progress and that the operator must manually edit the spec or rerun harden to recover.
- Because the implementation is already present as uncommitted modifications, should the acceptance criterion be strengthened to assert the new behaviors (round Status=failed, EndedAt set, CurrentState.AllowedFollowUp contains '--provider <provider>') rather than just `go test ./internal/app/harden`?
  - Grounded in: spec_gap:Acceptance
  - Recommended answer: Yes. The current criterion already passes on the working tree, so it does not prove the spec's intent. Either name the specific failing test that must turn green (e.g. -run TestRunProviderHardenClosesRoundOnProviderError) or describe the exact post-conditions the test must assert, so reviewers can distinguish a real build from a no-op.
  - If unanswered: Default to naming the specific test functions in the acceptance command so the criterion fails before the work exists and passes only after the named tests are added.

Design objections:
- `objection-1` high - Dossier-validation failure branch is in scope per Objectives but has no planned test.
  - Grounded in: code:internal/app/harden/harden.go:126
  - Evidence: Objectives line 'Record provider invocation or dossier-validation failure as a failed harden round' and Phase 1 'Call the helper when provider invocation fails or when the returned dossier fails strict validation' cover both branches at harden.go:122-124 and :126-129. Phase 1's test bullet only mentions 'provider invocation errors', and harden_test.go's only failure test uses fakeHardenProvider{err: ...} (line 283). The strict-validation branch is unverified.
  - Recommendation: Add an explicit Phase 1 change item and acceptance assertion for a dossier-validation failure test that returns an invalid dossier from the fake provider and verifies Status=failed, EndedAt set, and Summary containing 'invalid provider dossier'.
- `objection-2` high - Failure-on-failure is silently swallowed, undermining the spec's deterministic-state objective.
  - Grounded in: code:internal/app/harden/harden.go:123
  - Evidence: harden.go:123 and :127 call `_ = closeProviderHardenFailure(...)`. If the helper's Save fails (e.g. fs error after the in-progress round was written), the round remains Status: in_progress and harden_status: in_progress while Run returns only the upstream provider error. That is the exact state the spec exists to eliminate.
  - Recommendation: Either propagate the close error via errors.Join with the original failure, or add a Risk + Rollback bullet acknowledging the silent-drop and document the manual recovery (operator edits the spec or reruns harden). Add a test that injects a Save failure inside the helper to lock the chosen behavior in.
- `objection-3` medium - Acceptance command passes trivially because the implementation is already in the working tree.
  - Grounded in: spec_gap:Acceptance
  - Evidence: Starting git status lists `M internal/app/harden/harden.go` and `M internal/app/harden/harden_test.go`; the helper and TestRunProviderHardenClosesRoundOnProviderError already exist. `go test ./internal/app/harden` will pass without any build-phase work, so the acceptance criterion does not gate the spec's intent.
  - Recommendation: Narrow the acceptance command to the specific test names the spec promises to add (for example `go test ./internal/app/harden -run 'TestRunProviderHardenClosesRoundOn(ProviderError|InvalidDossier)'`) or state the explicit post-conditions the test(s) must assert.
- `objection-4` low - User-visible CurrentState.Reason and AllowedFollowUp strings are not pinned anywhere the spec acknowledges.
  - Grounded in: code:internal/app/harden/harden.go:188
  - Evidence: Helper writes 'external hardening provider failed' and 'fix provider availability/output, then run scafld harden <id> --provider <provider>'. These flow into CLI output and `scafld status --json`. The spec's Scope and Risks do not mention CLI/JSON surface, so a future renamer could break operator muscle memory without tripping any test.
  - Recommendation: Either add a focused assertion on the AllowedFollowUp prefix in the new test (the existing test already does substring '--provider <provider>'), or note in Scope that these CLI strings are part of the contract this spec sets.

Recommended edits:
- Objectives / Phase 1
  - Grounded in: code:internal/app/harden/harden.go:126
  - Recommendation: Add an explicit test bullet for the dossier-validation failure path (e.g. TestRunProviderHardenClosesRoundOnInvalidDossier) so both branches that call closeProviderHardenFailure are covered.
- Risks / Rollback
  - Grounded in: code:internal/app/harden/harden.go:123
  - Recommendation: Document the failure-on-failure case: if closeProviderHardenFailure's Save fails, the round remains in_progress. Either propagate the error via errors.Join (preferred — it preserves both signals) or describe the manual recovery path.
- Acceptance
  - Grounded in: spec_gap:Acceptance
  - Recommendation: Replace `go test ./internal/app/harden` with a command that names the specific new tests, or add explicit post-condition assertions (Status=failed, EndedAt set, harden_status=failed, AllowedFollowUp contains '--provider <provider>'), so the criterion fails before the build and passes only after the spec's work is in place.
- Scope
  - Grounded in: code:internal/app/harden/harden.go:186
  - Recommendation: Acknowledge that the helper introduces a new CurrentState.Reason and AllowedFollowUp wording that surfaces in CLI/JSON output, or assert these strings in the new test so they cannot be silently renamed.

### round-2

Status: passed
Started: 2026-05-17T13:35:12Z
Ended: 2026-05-17T13:35:12Z
Verdict: pass
Provider: claude
Model: claude-opus-4-7
Output format: claude.mcp_submit_harden
Summary: Round-2 confirms the revised draft closes every gap raised in round-1. The Objectives and Phase 1 now commit to both provider-invocation and dossier-validation failure paths, and both branches at harden.go:122-133 use errors.Join so a Save failure inside the helper surfaces with the original provider error. The acceptance command names three specific tests (TestRunProviderHardenClosesRoundOnProviderError|ClosesRoundOnInvalidDossier|ReportsFailureRecordingError) that exist at harden_test.go:277,300,320 and assert Status=failed, EndedAt set, Summary contents, and the rerun '--provider <provider>' wording. Scope, risks, rollback, and assumptions remain coherent; no failed checks, questions, design objections, or recommended edits remain.

Checks:
- path audit
  - Grounded in: code:internal/app/harden/harden.go:175
  - Result: passed
  - Evidence: Both touchpoints exist and are exactly the files named in Scope/Touchpoints: harden.go defines closeProviderHardenFailure at lines 175-194 and harden_test.go now contains TestRunProviderHardenClosesRoundOnProviderError (line 277), ClosesRoundOnInvalidDossier (line 300), and ReportsFailureRecordingError (line 320). No undeclared new files.
- command audit
  - Grounded in: code:internal/app/harden/harden_test.go:277
  - Result: passed
  - Evidence: The acceptance command `go test ./internal/app/harden -run 'TestRunProviderHarden(ClosesRoundOnProviderError|ClosesRoundOnInvalidDossier|ReportsFailureRecordingError)'` is a standard Go invocation from repo root; all three named test functions exist verbatim in harden_test.go.
- scope/migration audit
  - Grounded in: code:internal/app/harden/harden.go:190
  - Result: passed
  - Evidence: Scope is bound to harden.go and harden_test.go; the helper introduces new CurrentState.Reason ('external hardening provider failed') and AllowedFollowUp ('fix provider availability/output, then run scafld harden <id> --provider <provider>') at harden.go:190,192. These flow into CLI/JSON output but are not pinned by any archived contract, so no public-API migration is required; the new substring assertion in TestRunProviderHardenClosesRoundOnProviderError (harden_test.go:295) locks the rerun wording so silent renames will trip the test.
- acceptance timing audit
  - Grounded in: code:internal/app/harden/harden_test.go:320
  - Result: passed
  - Evidence: The acceptance criterion names three tests that must pass after Phase 1 — TestRunProviderHardenClosesRoundOnProviderError, ClosesRoundOnInvalidDossier, ReportsFailureRecordingError. Each asserts the spec's deterministic-state intent: round Status=failed, EndedAt populated, Summary contains 'provider unavailable' or 'invalid provider dossier', CurrentState contains '--provider <provider>', and the joined error includes both 'provider unavailable' and 'record provider harden failure: disk full'. The criterion fails before the helper/tests exist and passes only after they are in place.
- rollback/repair audit
  - Grounded in: spec_gap:Rollback
  - Result: passed
  - Evidence: Rollback: 'Revert the helper and focused test' then 'Rerun go test ./internal/app/harden after rollback to confirm the previous behavior is restored.' The change is fully contained in two app-layer files with no schema or storage migration, so revert is the correct recovery path. Failure-on-failure inside the helper is now propagated via errors.Join (harden.go:124,130), so an operator sees both the provider error and the save error rather than a silently in_progress round.
- design challenge
  - Grounded in: code:internal/app/harden/harden.go:103
  - Result: passed
  - Evidence: The plan fixes a real deterministic-state bug at the right layer. The alternative of delaying the in-progress save until after the provider returns would erase live observability of provider runs; closing the round on failure is the minimal change that preserves both visibility and determinism. The helper guards against concurrent round changes by re-reading and comparing the latest round ID (harden.go:180), and uses errors.Join to keep both signals — provider error and save error — visible. No new abstraction, no compatibility shim, no future bloat.

Questions:
- none


## Planning Log

- none
