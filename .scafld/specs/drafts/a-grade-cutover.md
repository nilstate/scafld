---
spec_version: '2.0'
task_id: a-grade-cutover
created: '2026-06-10T02:55:38Z'
updated: '2026-06-10T02:55:38Z'
status: draft
harden_status: not_run
size: large
risk_level: high
---

# Cut scafld over to A-grade trust and scale

## Current State

Status: draft
Current phase: none
Next: approve
Reason: draft created
Blockers: none
Allowed follow-up command: `scafld approve a-grade-cutover`
Latest runner update: none
Review gate: not_started

## Summary

The 2026-06 deep audit rated scafld 8/10: the architecture, fail-closed
correctness culture, and crypto core are sound, and the performance and
hygiene findings from that audit have landed. What separates the current
state from an A is trust-model depth and operational maturity, namely four
gaps. Receipt trust is only as deep as a host-held key file and a
trust-on-first-commit trusted-keys.json: keys have no expiry and ledger
replay accepts receipts from revoked keys even though the verify gate
rejects them. Cross-process ledger locking is a no-op on Windows, and no
test exercises two finalize processes racing on one task. Ledger replay is
O(n) from genesis on every append and `scafld list` loads and replays every
task ledger in full, so evidence cost grows with history instead of with
the work at hand. Finally, the trust model is enforced in code but not
stated anywhere a verifier can read, so operators cannot tell which
guarantees are cryptographic and which are heuristic. This spec closes
those four gaps. The end state: a receipt chain whose keys can expire and
be revoked with replay honoring both, ledger operations that are safe under
concurrent hosts on every supported OS, evidence stores that scale with
task count, and a written threat model that says exactly what a verified
receipt does and does not prove.

## Objectives

- Honor key revocation and expiry everywhere receipts are accepted, not
  only at the verify gate.
- Make ledger appends safe under concurrent scafld processes on all
  supported platforms, proven by tests.
- Make ledger replay and task listing scale with task count, not total
  history.
- Publish the threat model so the limits of a signed receipt are explicit.

## Scope

- internal/core/trust
- internal/core/session
- internal/adapters/jsonstore
- internal/platform/filelock
- internal/adapters/cli/verify
- internal/adapters/corebundle
- docs

## Dependencies

- The 2026-06 audit fixes already on main: filelock (unix), atomic receipt
  writes, cat-file batch digests, snapshot reuse in finalize.

## Assumptions

- trusted-keys.json remains the committed trust anchor; no external PKI.
- The session ledger JSON format may gain fields but must stay readable by
  existing ledgers without migration.
- Windows remains a supported install target (winget packaging ships).

## Touchpoints

- trust.TrustedKeys schema and parsing (additive fields: expiry,
  revocation metadata).
- session.Replay and jsonstore.SessionStore.Append/Load/List.
- filelock platform package (windows implementation).
- verify CLI policy and self-check report.
- docs/invariants.md, docs/operational-contracts.md, new threat-model doc.

## Risks

- Replay-time revocation checks cross the core/adapter boundary; the trust
  data must be passed in through a port, not imported, or the arch tests
  fail the build.
- Incremental replay that trusts a cached head weakens the chain if the
  cache is not itself validated; the cached head must be recomputable and
  cross-checked.
- Stricter key policy can brick existing workspaces whose keys lack expiry
  metadata; absent fields must default to current behavior.

## Acceptance

Profile: standard

Validation:
- `go test ./...` green, including new revocation, expiry, concurrency,
  and replay-scale tests.
- `go vet ./...` and the internal/arch boundary tests stay green.
- `GOOS=windows go build ./...` compiles the windows lock.

## Phase 1: Key lifecycle in the chain

Status: pending
Dependencies: none

Objective: Receipts signed by revoked or expired keys are rejected wherever
the chain is read, with replay receiving trust data through a port so core
stays pure.

Changes:
- Add optional `expires_at` and `revoked_at` fields to trusted-keys
  entries; absent fields keep current semantics.
- Thread trusted-key facts into session replay through a use-case-owned
  port so replay can mark entries signed by revoked or expired keys.
- Verify gate rejects receipts whose key was revoked or expired at
  minted_at, with a distinct failure reason.
- `verify --self-check` reports key expiry state and signing-key file
  permissions.

Acceptance:
- [ ] `ac1_1` trust and session tests pass with new key lifecycle coverage
  - Command: `go test ./internal/core/trust/ ./internal/core/session/ -count=1`
  - Expected kind: `exit_code_zero`
  - Status: pending
- [ ] `ac1_2` verify gate tests cover revoked and expired key rejection
  - Command: `go test ./internal/adapters/cli/verify/ -count=1`
  - Expected kind: `exit_code_zero`
  - Status: pending
- [ ] `ac1_3` architecture boundaries hold
  - Command: `go test ./internal/arch/ -count=1`
  - Expected kind: `exit_code_zero`
  - Status: pending

## Phase 2: Concurrency on every platform

Status: pending
Dependencies: phase1

Objective: Two scafld processes finalizing the same task cannot corrupt or
silently drop ledger entries on any supported OS, and a test proves it.

Changes:
- Implement filelock for windows via LockFileEx; keep the no-op fallback
  only for platforms with no install channel.
- Add a multi-process append test that races two writers on one session
  ledger and asserts no entry is lost and the chain stays valid.
- Add a concurrent finalize e2e test (two finalize invocations, one task):
  exactly one wins, the loser fails closed with the chain-break reason.

Acceptance:
- [ ] `ac2_1` windows build compiles the lock implementation
  - Command: `env GOOS=windows go build ./...`
  - Expected kind: `exit_code_zero`
  - Status: pending
- [ ] `ac2_2` jsonstore concurrency tests pass under the race detector
  - Command: `go test -race ./internal/adapters/jsonstore/ -count=1`
  - Expected kind: `exit_code_zero`
  - Status: pending
- [ ] `ac2_3` end-to-end suite stays green
  - Command: `go test ./test/e2e/ -count=1`
  - Expected kind: `exit_code_zero`
  - Status: pending

## Phase 3: Evidence at scale

Status: pending
Dependencies: phase2

Objective: Append cost tracks the new entry, not ledger length, and listing
tasks does not replay every ledger in the workspace.

Changes:
- Persist the validated ledger head with the session; on append, replay
  only from the persisted head and cross-check it against the prior entry
  digest, failing closed to a full replay on mismatch.
- Give List a metadata-only read path (task id, status, updated_at) that
  does not decode receipt bodies or replay chains.
- Keep full replay as the verify-time and load-time integrity path so the
  fast paths never become the source of truth.

Acceptance:
- [ ] `ac3_1` session store tests cover incremental replay and fallback
  - Command: `go test ./internal/adapters/jsonstore/ ./internal/core/session/ -count=1`
  - Expected kind: `exit_code_zero`
  - Status: pending
- [ ] `ac3_2` list and report commands stay correct on existing ledgers
  - Command: `go test ./internal/adapters/cli/ -count=1`
  - Expected kind: `exit_code_zero`
  - Status: pending

## Phase 4: Stated trust model

Status: pending
Dependencies: phase3

Objective: A verifier can read exactly what a passing `scafld verify`
proves, what it assumes, and what it cannot detect, without reading source.

Changes:
- Add docs/threat-model.md: what the signature binds, what the host key
  can forge, what independence detection can and cannot establish, and the
  trust-on-first-commit nature of trusted-keys.json.
- Update docs/invariants.md and docs/operational-contracts.md to reference
  the key lifecycle and concurrency guarantees added in phases 1 and 2.
- README links the threat model from the verify section.

Acceptance:
- [ ] `ac4_1` threat model document exists and is linked
  - Command: `test -f docs/threat-model.md && grep -q "threat-model" README.md`
  - Expected kind: `exit_code_zero`
  - Status: pending
- [ ] `ac4_2` full suite green at cutover
  - Command: `go test ./... -count=1`
  - Expected kind: `exit_code_zero`
  - Status: pending

## Rollback

- Phases 1 and 3 are additive to the ledger and trusted-keys formats;
  rolling back the binary leaves existing workspaces readable because
  absent fields default to current behavior.
- Phase 2 reverts to the unix-only lock by deleting the windows file.
- Phase 4 is documentation only.

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

- none

## Planning Log

- 2026-06-10: Drafted from the 2026-06 deep-audit findings (rated 8/10);
  scoped to the four gaps separating the current state from an A: key
  lifecycle, cross-platform concurrency, evidence scale, stated threat
  model.
