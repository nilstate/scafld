---
spec_version: '2.0'
task_id: native-lifecycle-json-contracts
created: '2026-04-21T06:41:00Z'
updated: '2026-04-21T08:50:20Z'
status: completed
harden_status: not_run
size: large
risk_level: high
---

# Make scafld lifecycle commands natively machine-readable

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

Turn scafld into a machine-first kernel for automation by adding native `--json` contracts to every automation-relevant command and documenting the result shapes. Today runx still reconstructs or normalizes parts of scafld output. That means scafld does not yet feel like the engineering system itself. This work makes lifecycle state, review state, audit findings, and failure reasons available as stable structured contracts instead of terminal heuristics.

## Context

CWD: `/home/kam/dev/scafld`

Packages:
- `cli`
- `scafld`
- `tests`
- `docs`

Files impacted:
- `cli/scafld` (all) - Lifecycle commands need shared `--json` handling and stable structured payloads.
- `scafld/reviewing.py` (all) - Review data already contains the machine-safe details that should become native CLI output.
- `scafld/command_runtime.py` (all) - Structured workspace-not-found errors now flow through the shared machine envelope.
- `scafld/spec_store.py` (all) - Structured spec lookup and transition errors now need stable machine-readable codes.
- `scafld/output.py` (all) - The shared output layer should own JSON envelopes and error serialization.
- `docs/cli-reference.md` (all) - The CLI reference must document each command's structured contract.
- `docs/lifecycle.md` (all) - Lifecycle docs should explain the machine-facing state model and transition payloads.
- `docs/review.md` (all) - Review docs should explain native structured review-open and complete outputs.
- `tests/review_gate_smoke.sh` (all) - Review smoke should assert the native JSON contract instead of relying on wrapper behavior.
- `tests/harden_smoke.sh` (all) - Harden smoke should assert JSON output for harden and mark-passed flows.
- `tests/json_contract_smoke.sh` (all) - A new smoke test should cover the full command matrix in JSON mode.

Invariants:
- `human_output_remains_default`
- `json_mode_uses_stable_envelopes`
- `error_conditions_have_parseable_codes`
- `machine_callers_never_need_terminal_scraping`
- `documented_contracts_match_real_output`

Related docs:
- `README.md`
- `docs/cli-reference.md`
- `docs/lifecycle.md`
- `docs/review.md`
- `docs/validation.md`

## Objectives

- Add native `--json` support for init, new, validate, approve, start, status, exec, audit, harden, review, complete, fail, cancel, and report.
- Define a shared JSON envelope with command identity, task identity, current state, warnings, and structured errors.
- Return command-specific payloads for review, complete, audit, exec, harden, and report rather than forcing wrappers to infer them.
- Emit machine-readable error codes, failure reasons, and next-action hints in JSON mode.
- Document the native contracts so runx and third parties can consume them directly.
- Prove the contracts with smoke coverage that runs the real packaged CLI.

## Scope



## Dependencies

- command-core-integrity-hardening

## Assumptions

- Default CLI usage should remain human-oriented; JSON mode is opt-in.
- A stable envelope is more important than preserving every current ad hoc line of terminal text in JSON mode.
- runx should become a thin consumer of these contracts rather than continuing to normalize them.

## Touchpoints

- command output: Add and document stable JSON envelopes and command payloads.
- review and harden payloads: Expose review-open, review-complete, and harden state as native structured data.
- error model: Define parseable error codes and failure reasons for machine callers.
- smoke coverage: Verify the command matrix in JSON mode against the real CLI.

## Risks

- JSON payload drift could create a second undocumented API if contracts are not explicitly documented and tested.
- Interactive users could see degraded terminal UX if JSON support leaks into default output paths.
- Error payloads could stay inconsistent if command failures keep raising ad hoc exceptions.

## Acceptance

Profile: strict

Definition of done:
- [x] `dod1` Every automation-relevant command supports native `--json`.
- [x] `dod2` JSON mode emits one stable envelope shape plus command-specific result fields.
- [x] `dod3` Machine-readable error codes and reasons are returned in JSON mode.
- [x] `dod4` Review, complete, audit, exec, and harden no longer require wrapper-side reconstruction.
- [x] `dod5` Docs and smoke tests prove the native JSON contracts.

Validation:
- [ ] `v1` integration - Lifecycle JSON contract smoke passes.
  - Command: `./tests/json_contract_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Review gate smoke passes in JSON mode.
  - Command: `./tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - Harden smoke passes in JSON mode.
  - Command: `./tests/harden_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Define the JSON contract

Goal: Establish one shared envelope and one structured error model for machine callers.

Status: completed
Dependencies: none

Changes:
- `scafld/output.py` (all) - Define JSON envelope rendering, warning emission, and machine-readable error serialization.
- `scafld/errors.py` (all) - Add stable error codes and details fields that lifecycle commands can return in JSON mode.
- `scafld/command_runtime.py` (all) - Add stable shared error codes for missing workspace discovery so JSON callers get parseable failures.
- `scafld/spec_store.py` (all) - Add stable shared error codes for missing specs, ambiguous task IDs, and invalid lifecycle transitions.
- `docs/cli-reference.md` (all) - Document the shared JSON envelope, error shape, and per-command payload conventions.

Acceptance:
- [x] `ac1_1` documentation - CLI docs publish the shared JSON contract.
  - Command: `grep -En -- "--json|error code|warning|command" docs/cli-reference.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Backfill lifecycle commands

Goal: Make each lifecycle command emit native structured payloads in JSON mode.

Status: completed
Dependencies: phase1

Changes:
- `cli/scafld` (all) - Thread `--json` through every automation-relevant command and return stable command payloads.
- `scafld/reviewing.py` (all) - Expose review-open and review-complete details as command payloads rather than wrapper-only helpers.

Acceptance:
- [x] `ac2_1` integration - Lifecycle JSON contract smoke passes.
  - Command: `./tests/json_contract_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Lock the contract with smokes

Goal: Protect the machine contracts with review, harden, and lifecycle smoke coverage.

Status: completed
Dependencies: phase2

Changes:
- `tests/json_contract_smoke.sh` (all) - Exercise the command matrix in JSON mode and assert stable fields, error codes, and payload shapes.
- `tests/review_gate_smoke.sh` (all) - Assert native JSON review-open and complete payloads instead of wrapper-reconstructed fields.
- `tests/harden_smoke.sh` (all) - Assert native JSON harden payloads, citation warnings, and mark-passed results.
- `docs/lifecycle.md` (all) - Explain the shared machine-facing lifecycle envelope and which lifecycle commands expose native transitions.
- `docs/review.md` (all) - Explain the native machine-facing review lifecycle contract.

Acceptance:
- [x] `ac3_1` integration - Review gate smoke passes after native JSON backfill.
  - Command: `./tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` integration - Harden smoke passes after native JSON backfill.
  - Command: `./tests/harden_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Rollback

Strategy: manual

Commands:
- none

## Review

Status: not_started
Verdict: incomplete
Timestamp: '2026-04-21T08:50:20Z'
Review rounds: 1
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

- 2026-04-21T06:41:00Z - codex - Mapped the machine-contract work needed so runx and other tools can consume scafld directly instead of normalizing CLI text.
- 2026-04-21T08:20:47Z - cli - Spec approved
- 2026-04-21T08:20:47Z - cli - Execution started
- 2026-04-21T10:05:00Z - codex - Backfilled native JSON envelopes across lifecycle commands, added structured error codes at shared command/runtime boundaries, updated review and harden JSON coverage, and documented the machine contract.
- 2026-04-21T08:48:55Z - codex - Recorded passing execution evidence for the lifecycle, review, and harden JSON smokes; marked the contract work complete at the spec level so the next git/PR-native slice can build on stable machine output.
- 2026-04-21T08:50:20Z - cli - Spec completed
