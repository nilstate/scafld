---
spec_version: '2.0'
task_id: runtime-rigor-gate-integrity
created: '2026-04-27T15:43:46Z'
updated: '2026-04-28T04:50:24Z'
status: completed
harden_status: in_progress
size: small
risk_level: medium
---

# Verify review packet sha256 on complete (hand-edit detection)

## Current State

Status: completed
Current phase: none
Next: none
Reason: none
Blockers: none
Allowed follow-up command: none
Latest runner update: none
Review gate: not_started

## Summary

`scafld complete` reads the review markdown but doesn't verify the packet came from a real challenger. An operator (or agent) can edit `.ai/reviews/<task>.md` to flip a verdict between `scafld review` and `scafld complete` and the CLI accepts it.
Most of the seal is already in place:

  - `canonical_response_sha256` is computed in
    `review_runner.py:1808,1841` over the full canonical
    `{"packet": ..., "projection": ...}` shape.
  - The hash is already persisted into the review markdown's
    metadata block via `build_review_metadata` /
    `review_provenance` (review_workflow.py:611-629). Existing
    external-reviewed files in `.ai/reviews/` already carry it.
  - The canonical packet body is already written to
    `.ai/runs/<task>/review-packets/review-<n>.json` by
    `write_review_packet_artifact` (review_packet.py:496-507).

What's missing is the verify-on-complete step. Add a `verify_review_seal(metadata, packet) -> (ok, reason)` helper that mirrors the writer's hash computation; in `cmd_complete`, load the packet artifact, recompute, refuse on mismatch with `EC.REVIEW_GATE_BLOCKED`. Hand-edits to the review markdown metadata or to the recorded findings now fail loudly.
Hard cutover: review files written before 1.7.0 (no `canonical_response_sha256` in metadata) fail with a clear "review file predates 1.7.0 seal" error. Operator re-runs `scafld review` under 1.7.0 to refresh the seal.

## Context

CWD: `.`

Packages:
- none

Files impacted:
- `scafld/review_packet.py` (all) - Add `compute_canonical_response_sha256(packet)` and `verify_review_seal(metadata, packet)` helpers; mirrors review_runner.py:1808,1841.
- `scafld/commands/review.py` (all) - cmd_complete loads the packet artifact, calls verify_review_seal, blocks on mismatch.
- `tests/test_review_packet.py` (all) - Cover hash computation determinism, metadata-tamper rejection, packet-tamper rejection, missing-seal rejection.
- `tests/review_gate_smoke.sh` (all, shared) - Smoke that proves complete rejects a hand-edited review file and accepts an unmodified one.
- `docs/configuration.md` (all) - Document the seal + hard cutover for pre-1.7.0 review files.

Invariants:
- none

Related docs:
- none

## Objectives

- None.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- None.

## Risks

- None.

## Acceptance

Profile: standard

Definition of done:
- [x] `dod1` `compute_canonical_response_sha256(packet)` returns deterministic hex sha256 matching the writer in review_runner.py.
- [x] `dod2` `verify_review_seal(metadata, packet)` returns (True, "") on match; (False, reason) on hash mismatch; (False, "missing_seal") when metadata lacks `canonical_response_sha256`.
- [x] `dod3` `scafld complete` blocks with `EC.REVIEW_GATE_BLOCKED` and a named gate_reason when the packet artifact's recomputed hash doesn't match metadata.
- [x] `dod4` Pre-1.7.0 review files (no canonical_response_sha256 in metadata) fail complete with a clear hard-cutover error.
- [x] `dod5` Smoke proves a hand-edited review file fails complete; an unmodified one passes.

Validation:
- [ ] `v1` compile
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test
  - Command: `python3 -m unittest tests.test_review_packet`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` boundary - Tamper-evidence smoke is registered.
  - Command: `grep -q 'case_review_complete_rejects_tampered_review_file' tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: verify_review_seal helper + canonical hash recomputation

Goal: Add the helpers needed to verify a parsed packet against the metadata hash.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` compile
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` test
  - Command: `python3 -m unittest tests.test_review_packet`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: cmd_complete verifies the seal

Goal: complete loads the packet artifact, calls verify_review_seal, blocks on mismatch or missing seal.

Status: completed
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` compile
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` test
  - Command: `python3 -m unittest tests.test_review_packet`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Smoke + docs

Goal: Smoke-prove tamper rejection end-to-end. Document the seal + hard cutover.

Status: completed
Dependencies: phase2

Changes:
- none

Acceptance:
- [x] `ac3_1` boundary
  - Command: `bash -n tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` boundary - Tamper-evidence smoke is registered.
  - Command: `grep -q 'case_review_complete_rejects_tampered_review_file' tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_3` boundary - Docs cover the seal.
  - Command: `grep -qiE 'canonical_response_sha256|Review seal' docs/configuration.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Rollback

Strategy: per_phase

Commands:
- none

## Review

Status: not_started
Verdict: pass_with_issues
Timestamp: '2026-04-28T04:50:02Z'
Review rounds: 7
Reviewer mode: none
Reviewer session: none
Round status: none
Override applied: none
Override reason: none
Override confirmed at: none
Reviewed head: none
Reviewed dirty: none
Reviewed diff: none
Blocking count: 0
Non-blocking count: none

Findings:
- none

Passes:
- `id`: spec_compliance
- `id`: scope_drift
- `id`: regression_hunt
- `id`: convention_check
- `id`: dark_patterns

## Self Eval

Status: not_started
Completeness: none
Architecture fidelity: none
Spec alignment: none
Validation depth: none
Total: none
Second pass performed: none

Notes:
none

Improvements:
- none

## Deviations

- none

## Metadata

Estimated effort hours: none
Actual effort hours: none
AI model: none
React cycles: none

Tags:
- none

## Origin

Source:
- none

Repo:
- none

Git:
- none

Sync:
- none

Supersession:
- none

## Harden Rounds

- none

## Planning Log

- 2026-04-27T15:43:46Z - user - Spec drafted as cutover #2 (per plans/1.7.0-cutover.md).
- 2026-04-28T13:30:00Z - agent - Slimmed against verified code: hash + metadata persistence + canonical artifact already exist; spec narrows to verify_review_seal helper + cmd_complete recompute. Dropped seal_review_packet, sealed.json sibling, sealing_party enum, grace pass (hard cutover means pre-1.7 files fail).
- 2026-04-28T19:00:00Z - agent - Review 6 verdict=fail. Resolved 1 blocking + 1 non-blocking
- 2026-04-28T18:30:00Z - agent - Review 5 verdict=fail (2 high blocking + 3 medium non-blocking).
- 2026-04-28T17:30:00Z - agent - Review 4 verdict=fail. Resolved 1 blocking finding:
- 2026-04-28T17:00:00Z - agent - Review 3 verdict=fail. Resolved 1 blocking + 1 non-blocking medium:
- 2026-04-28T16:30:00Z - agent - Review 2 verdict=fail. Resolved both blocking findings:
- 2026-04-28T15:50:00Z - agent - Review 1 verdict=fail. Resolved both blocking findings:
