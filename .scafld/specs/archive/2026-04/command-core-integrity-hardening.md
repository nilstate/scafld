---
spec_version: '2.0'
task_id: command-core-integrity-hardening
created: '2026-04-21T06:40:00Z'
updated: '2026-04-21T08:43:27Z'
status: completed
harden_status: not_run
size: large
risk_level: high
---

# Refactor the scafld command core for integrity

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

Harden scafld's internal command architecture before adding more product surface area. The current CLI packs parsing, path resolution, spec storage, output formatting, and command behavior into one large entrypoint plus a few helper modules. Extract stable internal surfaces for command registration, spec IO, path/state lookup, JSON emission, and structured error handling so new Git, PR, and issue integrations do not compound brittleness.

## Context

CWD: `/home/kam/dev/scafld`

Packages:
- `cli`
- `scafld`
- `tests`
- `docs`

Files impacted:
- `cli/scafld` (all) - The monolithic CLI currently owns argument parsing, state transitions, filesystem IO, and user-facing output.
- `scafld/config.py` (all) - Shared config and init autodetection should plug into a cleaner command/runtime boundary.
- `scafld/git_state.py` (all) - Git helpers are already extracted and should integrate with a unified command/runtime layer.
- `scafld/reviewing.py` (all) - Review pass metadata should stay aligned with the refactored scope-audit behavior.
- `tests/review_gate_smoke.sh` (all) - Review smoke protects the most failure-prone lifecycle boundary.
- `tests/harden_smoke.sh` (all) - Harden smoke protects the newest lifecycle feature and anti-drift constraints.
- `tests/package_smoke.sh` (all) - Package smoke ensures the refactor does not break PyPI/npm distribution or bundled assets.
- `tests/test_command_runtime.py` (all) - Focused runtime coverage protects root detection and structured command context behavior.
- `tests/test_spec_store.py` (all) - Focused spec-store coverage protects lookup and move semantics during the refactor.
- `tests/test_git_state.py` (all) - Git-state coverage protects scope auditing and working-tree file discovery behavior.
- `README.md` (all) - Public docs should explain the new reliability posture at a high level.
- `.ai/config.yaml` (all) - The bundled default review topology should describe the actual scope-audit behavior.

Invariants:
- `human_cli_behavior_stays_intact`
- `command_logic_becomes_more_testable_not_more_magical`
- `filesystem_state_changes_stay_explicit`
- `new_internal_layers_reduce_duplication`
- `packaged_cli_and_framework_assets_remain_in_sync`

Related docs:
- `README.md`
- `AGENTS.md`
- `docs/cli-reference.md`
- `docs/lifecycle.md`
- `docs/review.md`
- `docs/scope-auditing.md`
- `docs/workspaces.md`

## Objectives

- Split shared command concerns into stable internal modules instead of growing `cli/scafld` indefinitely.
- Centralize spec discovery, load/update/move/archive operations behind one tested spec-store surface.
- Centralize path resolution and workspace discovery so every command uses the same rules.
- Introduce one structured error taxonomy that supports human and JSON callers without duplicating logic.
- Add a command-test harness that can exercise lifecycle transitions without relying only on end-to-end shell smoke.
- Keep packaging, docs, and existing workflow semantics intact while reducing the chance of future regressions.

## Scope



## Dependencies

- modularize-cli-core

## Assumptions

- The safest path is an internal hard cutover: new code should call the extracted helpers directly rather than keeping shadow implementations.
- The CLI entrypoint may remain as a thin adapter so package entrypoints do not change.
- The refactor should land before major Git/PR/issue features so new behavior builds on cleaner primitives.

## Touchpoints

- command runtime: Extract a stable command runtime and dispatch layer from `cli/scafld`.
- spec store: Create a single internal surface for locating, loading, updating, moving, and archiving specs.
- output and errors: Centralize human output, JSON output, warnings, and structured error codes.
- test harness: Add focused regression coverage beneath the shell smokes.
- docs and packaging: Keep README/docs/package assets aligned with the refactor.

## Risks

- Refactoring the CLI core could silently change lifecycle behavior or archive paths.
- New internal layers could become abstract ceremony instead of reducing duplication.
- Packaging could drift if the refactor changes import paths without updating build metadata.

## Acceptance

Profile: strict

Definition of done:
- [x] `dod1` Shared command concerns move out of `cli/scafld` into stable internal modules.
- [x] `dod2` Spec lookup, load, update, move, and archive behavior is centralized behind one tested surface.
- [x] `dod3` Structured error handling is defined once and reused across commands.
- [x] `dod4` Existing lifecycle smoke coverage passes unchanged after the refactor.
- [x] `dod5` Packaging and docs remain aligned with the refactored internal shape.

Validation:
- [ ] `v1` test - Core command regression suite passes.
  - Command: `python3 -m unittest tests.test_command_runtime tests.test_spec_store tests.test_git_state`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Review gate smoke still passes.
  - Command: `./tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - Harden smoke still passes.
  - Command: `./tests/harden_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` integration - Package smoke still passes.
  - Command: `./tests/package_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Extract shared runtime surfaces

Goal: Create stable internal modules for command dispatch, spec storage, output handling, and errors.

Status: completed
Dependencies: none

Changes:
- `cli/scafld` (all) - Reduce the entrypoint to argument parsing and thin dispatch into extracted runtime/helpers.
- `scafld/command_runtime.py` (all) - Define command registration, execution context construction, and shared exit/error handling.
- `scafld/spec_store.py` (all) - Own spec discovery, load/update/write, directory moves, archive lookup, and state mutations.
- `scafld/output.py` (all) - Render human output and structured output from shared command results.
- `scafld/errors.py` (all) - Define parseable error codes and exception helpers for command failures.
- `tests/__init__.py` (all) - Mark the unittest package so focused command-level regression modules run consistently.
- `tests/test_command_runtime.py` (all) - Cover workspace discovery and structured root resolution behavior.
- `tests/test_spec_store.py` (all) - Cover spec lookup ordering, ambiguity detection, and status moves.

Acceptance:
- [x] `ac1_1` test - Focused command runtime and spec-store tests pass.
  - Command: `python3 -m unittest tests.test_command_runtime tests.test_spec_store tests.test_git_state`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Adopt extracted surfaces

Goal: Move existing lifecycle commands onto the extracted runtime without changing public semantics.

Status: completed
Dependencies: phase1

Changes:
- `cli/scafld` (all) - Route lifecycle commands through the extracted runtime and stop duplicating path/error logic inline.
- `scafld/git_state.py` (all) - Align git helpers with the shared runtime and command context APIs.
- `scafld/reviewing.py` (all) - Keep built-in review pass metadata aligned with the refactored scope-audit behavior.
- `tests/test_git_state.py` (all) - Cover working-tree and base-ref changed-file collection so scope auditing stays aligned with real workspace state.

Acceptance:
- [x] `ac2_1` integration - Review gate smoke still passes after command adoption.
  - Command: `./tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` integration - Harden smoke still passes after command adoption.
  - Command: `./tests/harden_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_3` integration - Scope audit uses the live workspace cleanly after the git-state refactor.
  - Command: `python3 /home/kam/dev/scafld/cli/scafld audit command-core-integrity-hardening`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Lock in regressions

Goal: Document the internal hardening and keep packaging/docs aligned with the cleaner core.

Status: completed
Dependencies: phase2

Changes:
- `README.md` (all) - Document the improved reliability posture and the existence of stable machine-facing internals.
- `.ai/config.yaml` (all) - Keep the bundled review topology copy aligned with the live scope-audit behavior.
- `docs/cli-reference.md` (all) - Clarify CLI guarantees that are preserved by the refactor.
- `docs/scope-auditing.md` (all) - Document that scope audit defaults to the live workspace, still supports explicit base refs, and ignores scafld execution artifacts.
- `docs/review.md` (all) - Keep review pass documentation aligned with scope audit's live-workspace behavior.
- `tests/package_smoke.sh` (all) - Keep package verification aligned with the new module layout and entrypoint shape.

Acceptance:
- [x] `ac3_1` integration - Package smoke passes after the refactor.
  - Command: `./tests/package_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` documentation - Docs describe the refactored reliability posture without promising behavior that does not exist.
  - Command: `rg -n "machine|reliability|command" README.md docs/cli-reference.md`
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
Timestamp: '2026-04-21T08:43:27Z'
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

- 2026-04-21T06:40:00Z - codex - Mapped foundational core-hardening work required before native contracts and Git/PR/issue surfaces can land cleanly.
- 2026-04-21T07:51:07Z - cli - Spec approved
- 2026-04-21T07:51:07Z - cli - Execution started
- 2026-04-21T07:54:52Z - codex - Extracted command runtime, spec store, output, and structured error modules; rewired CLI transitions to use them; added focused unittest coverage and passed lifecycle smoke.
- 2026-04-21T07:56:54Z - codex - Confirmed no lifecycle regressions after the extraction by passing review-gate, harden, and direct package smoke commands; fixed the package smoke execute bit so the acceptance command matches reality.
- 2026-04-21T08:02:34Z - codex - Moved workspace-tree discovery and shared YAML helpers onto the extracted modules, adopted more lifecycle entrypoints onto structured errors, and documented the hardened core shape in the README and CLI reference.
- 2026-04-21T08:26:00Z - codex - Fixed scope audit to inspect the live working tree by default, include untracked files, ignore scafld execution artifacts, and added focused git-state regression coverage.
- 2026-04-21T08:34:00Z - codex - Extended scope-audit ignores to the local override file, aligned the built-in review pass copy and bundled config with live-workspace auditing, and reran the review gate path.
- 2026-04-21T08:43:27Z - cli - Spec completed
