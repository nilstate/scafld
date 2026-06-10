---
spec_version: '2.0'
task_id: a-grade-cutover
created: '2026-06-10T02:55:38Z'
updated: '2026-06-10T03:11:22Z'
status: approved
harden_status: passed
size: large
risk_level: high
---

# Cut scafld over to A-grade trust and scale

## Current State

Status: approved
Current phase: none
Next: build
Reason: hardening passed
Blockers: none
Allowed follow-up command: `scafld build a-grade-cutover`
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
test exercises two finalize processes racing on one task. Session ledger
replay is O(n) from genesis on every append, and the report path loads and
replays every task ledger through `SessionStore.List`, so evidence cost
grows with total history instead of with the work at hand; `scafld list`
already stays on the spec-store metadata path and must remain cheap.
Finally, the trust model is enforced in code but not stated anywhere a
verifier can read, so operators cannot tell which guarantees are
cryptographic and which are heuristic. This spec closes those four gaps.
The end state: a receipt chain whose keys can expire and be revoked with
replay honoring both, ledger operations that are safe under concurrent
hosts on every supported OS, evidence/reporting stores that scale with task
count, and a written threat model that says exactly what a verified receipt
does and does not prove.

## Objectives

- Honor key revocation and expiry everywhere receipts are accepted, not
  only at the verify gate.
- Make ledger appends safe under concurrent scafld processes on all
  supported platforms, proven by tests.
- Make ledger append and evidence reporting scale with task count, not
  total history, while preserving `scafld list` as a metadata-only path.
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
- corebundle initwire tests and trusted-key fixture generation so committed
  trusted-keys.json examples stay aligned with the lifecycle schema.
- docs/invariants.md, docs/operational-contracts.md, new threat-model doc.

## Risks

- Replay-time revocation checks cross the core/adapter boundary; the trust
  data must be passed in through a port, not imported, or the arch tests
  fail the build. Because `session.Replay` is reached by Save, Append,
  WithEntry, Load, jsonstore, and app packages, Phase 1 must account for
  that call-site fan-out rather than treating the port as a one-line
  additive argument.
- Incremental replay that trusts a cached head weakens the chain if the
  cache is not itself validated; the cached head must be recomputable and
  cross-checked.
- Stricter key policy can brick existing workspaces whose keys lack expiry
  metadata; absent fields must default to current behavior.
- Trusted-key lifecycle fields are forward-compatible for the new binary
  but not transparently rollback-compatible with old binaries, because the
  current parser rejects unknown fields. Rollback after lifecycle fields are
  committed requires either removing those fields from trusted-keys.json or
  using a binary that understands them.

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

Objective: Append cost tracks the new entry, not ledger length, report-time
session metrics avoid full replay of every ledger in the workspace, and
`scafld list` remains on its existing metadata-only spec-store path.

Changes:
- Persist the validated ledger head with the session; on append, replay
  only from the persisted head and cross-check it against the prior entry
  digest, failing closed to a full replay on mismatch.
- Give report/session listing a metadata or summary read path that does not
  decode receipt bodies or replay full chains when only aggregate metrics
  are needed.
- Add a regression proving `scafld list` does not depend on SessionStore and
  stays metadata-only.
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

- Phases 1 and 3 are additive for the new binary: existing ledgers and
  trusted-keys files without lifecycle metadata remain readable because
  absent fields default to current behavior. Once `expires_at` or
  `revoked_at` are committed to trusted-keys.json, rollback to binaries that
  reject unknown fields is forward-only unless those lifecycle fields are
  removed from the file first.
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

### round-1

Status: passed
Started: 2026-06-10T03:00:50Z
Ended: 2026-06-10T03:11:05Z
Verdict: needs_revision
Provider: claude
Model: claude-opus-4-7
Output format: claude.mcp_submit_harden
Summary: Spec is largely executable: paths/commands resolve and timing aligns with phases. Two grounded concerns: the Summary calls out `scafld list` as a ledger-replay hotspot but `runList` only walks SpecStore (the actual offender is `report`/SessionStore.List), and Phase 1's rollback claim ignores that trust.ParseTrustedKeys uses DisallowUnknownFields, so any workspace that adopts `expires_at`/`revoked_at` will refuse to parse on the prior binary. Advisory: threading trust facts into session.Replay touches every Save/Append/Load call site (a cross-cutting refactor not just an additive port), and the `corebundle` touchpoint is listed without a concrete change.

Observations:
- path
  - Result: clean
  - Anchor: code:internal/platform/filelock/filelock_other.go:1
  - Note: Touchpoints exist: internal/core/trust, internal/core/session, internal/adapters/jsonstore, internal/platform/filelock (windows file intentionally new), internal/adapters/cli/verify, docs/. docs/threat-model.md is declared as new.
- command
  - Result: clean
  - Anchor: code:Makefile:1
  - Note: All acceptance commands (`go test`, `env GOOS=windows go build ./...`, `test -f docs/threat-model.md && grep -q ...`) run from the repo root with the standard Go toolchain.
- scope
  - Result: blocks
  - Anchor: code:internal/adapters/cli/cli.go:404
  - Note: Summary says `scafld list` 'loads and replays every task ledger in full', but runList only calls listusecase.Run on the SpecStore — no SessionStore involvement. The actual O(n) ledger-replay path is SessionStore.List consumed by report.Run (internal/app/report/report.go:70). Phase 3's fix targets SessionStore.List correctly, but the Summary mislabels the symptom and the corebundle touchpoint is listed without a stated change.
  - Default: Rewrite the Summary to cite the SessionStore.List/report path and either drop corebundle from Touchpoints or name the concrete edit.
  - Status: fixed
- timing
  - Result: clean
  - Anchor: spec_gap:Phases
  - Note: Each phase's acceptance commands target artifacts the same phase creates: Phase 2's windows build runs after the windows lock file exists; Phase 4's `test -f docs/threat-model.md` runs after the doc is added.
- rollback
  - Result: blocks
  - Anchor: code:internal/core/trust/trusted_keys.go:49
  - Note: ParseTrustedKeys calls decoder.DisallowUnknownFields(). The rollback claim 'absent fields default to current behavior' only holds for workspaces that never wrote the new fields. Once an operator populates `expires_at`/`revoked_at`, the prior binary will fail to parse trusted-keys.json — rollback is not transparent.
  - Default: Either bump TrustedKeysVersion and document forward-only adoption, or drop DisallowUnknownFields for the lifecycle fields and state the rollback window explicitly.
  - Status: fixed
- design
  - Result: advisory
  - Anchor: code:internal/core/session/model.go:84
  - Note: Phase 1 wants session.Replay to consume trust facts via a use-case port, but Replay is invoked from Save, Append, WithEntry, and Load (jsonstore + many app packages); threading trust through every call site is a cross-cutting refactor, not the additive port the Risks section implies. The plan is the right architectural direction for an A-grade trust model — call out the call-site fan-out so phase 1 doesn't get sized as a one-line change.


## Planning Log

- 2026-06-10: Drafted from the 2026-06 deep-audit findings (rated 8/10);
  scoped to the four gaps separating the current state from an A: key
  lifecycle, cross-platform concurrency, evidence scale, stated threat
  model.
