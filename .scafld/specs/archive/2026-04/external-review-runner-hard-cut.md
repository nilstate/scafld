---
spec_version: '2.0'
task_id: external-review-runner-hard-cut
created: '2026-04-25T13:38:58Z'
updated: '2026-04-26T08:17:18Z'
status: cancelled
harden_status: passed
size: medium
risk_level: medium
---

# Make adversarial review use a real external runner

## Current State

Status: cancelled
Current phase: none
Next: none
Reason: none
Blockers: none
Allowed follow-up command: none
Latest runner update: none
Review gate: not_started

## Summary

`scafld review` still only opens the challenger round and emits a handoff, while the thin provider wrappers are an optional side path. The clean model is: `scafld review` owns challenger execution. It resolves a runner explicitly, defaults to external execution, prefers `codex` then `claude`, supports explicit `local` and `manual` fallback modes, and records review provenance without introducing compatibility shims or a second review artifact format.

## Context

CWD: `.`

Packages:
- `scafld`
- `docs`
- `tests`

Files impacted:
- `scafld/commands/review.py` (all) - Review command should own runner resolution and execution instead of only printing the challenger handoff.
- `scafld/review_runtime.py` (all) - Review snapshot/result payload should expose runner-ready metadata without breaking the JSON control surface.
- `scafld/review_workflow.py` (all) - The review round writer and metadata model are the canonical place to persist completed challenger provenance.
- `scafld/reviewing.py` (all) - Review metadata validation must understand the new runner/provenance fields.
- `scafld/commands/surface.py` (all) - The public review command needs explicit runner/provider/model overrides.
- `scafld/review_runner.py` (all) - New dedicated boundary for runner selection, provider invocation, and output normalization.
- `.ai/config.yaml` (all) - Review runner defaults belong in config, not hardcoded CLI assumptions.
- `docs/review.md` (all) - The review product surface and fallback modes need to match the new command behavior.
- `docs/integrations.md` (all) - Thin wrapper docs must stop claiming wrappers are the default review path.
- `docs/configuration.md` (all) - Runner/provider/model config needs a documented contract.
- `docs/cli-reference.md` (all) - Review flags and default runner semantics are part of the taught CLI surface.
- `tests/review_gate_smoke.sh` (all) - The real review gate smoke should prove external, local, and manual review paths.
- `tests/codex_handoff_adapter_smoke.sh` (all) - Existing adapter smoke should keep proving the thin provider wrapper still works as a non-default integration surface.
- `tests/claude_handoff_adapter_smoke.sh` (all) - Claude wrapper smoke should continue to prove the optional integration path after the hard cut.

Invariants:
- `domain_boundaries`
- `no_legacy_code`
- `packaged_cli_is_the_truth_surface`
- `review_artifact_stays_scafld_owned`

Related docs:
- none

## Objectives

- Make `scafld review` own challenger runner selection and execution by default.
- {'Keep the external path clean': 'explicit provider selection, codex-first auto resolution, claude fallback, and real provenance capture.'}
- Preserve an explicit degraded fallback path via `local` or `manual` instead of silent implicit fallback.
- Keep the existing review artifact and review gate contracts intact instead of introducing a parallel format.

## Scope



## Dependencies

- None.

## Assumptions

- codex and claude non-interactive CLIs are available in at least some environments, but may be absent in others
- the JSON review surface should remain a control-plane snapshot and not mix subprocess stdout into machine-readable output
- scafld should remain provider-neutral in core even while shipping first-party codex and claude runner implementations

## Touchpoints

- review command surface: `scafld review` should be the canonical challenger entrypoint instead of wrapper-first behavior.
- review artifact contract: The latest review round should stay scafld-owned, validated, and provenance-rich.
- provider integrations: Codex and Claude should remain thin external integrations selected by the runner boundary, not leak into review core logic.

## Risks

- None.

## Acceptance

Profile: standard

Definition of done:
- [x] `dod1` `scafld review` defaults to external runner resolution and completes a challenger round through a clean subprocess path when a provider is available.
- [x] `dod2` Explicit `local` and `manual` review modes exist and are visibly degraded, not silent fallback.
- [x] `dod3` Review metadata records runner/provider provenance without changing the gate artifact shape.
- [x] `dod4` Docs and smoke tests teach and prove the new review path.

Validation:
- [ ] `v1` compile - Compile the Python sources after review runner changes.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Review gate smoke proves external, local, and manual runner behavior.
  - Command: `bash tests/review_gate_smoke.sh external-runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - Codex wrapper smoke still proves the thin review adapter path.
  - Command: `bash tests/codex_handoff_adapter_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` integration - Claude wrapper smoke still proves the thin review adapter path.
  - Command: `bash tests/claude_handoff_adapter_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Add runner config and selection boundary

Goal: Define the explicit runner/provider/model contract and add a dedicated review-runner seam instead of overloading the current prompt-pipe helper.

Status: completed
Dependencies: none

Changes:
- `.ai/config.yaml` (all, shared) - Add review runner defaults covering runner mode and external provider/model selection.
- `scafld/commands/surface.py` (all, shared) - Add explicit review flags for runner/provider/model overrides.
- `scafld/review_runner.py` (all, shared) - Create the dedicated runner boundary for loading config and resolving runner/provider selection.

Acceptance:
- [ ] `ac1_1` integration - Review surface exposes the new runner/provider/model options without breaking help or parsing.
  - Command: `python3 - <<'PY'\nfrom scafld.commands.surface import COMMAND_SPECS\nreview = next(spec for spec in COMMAND_SPECS if spec.name == 'review')\nflags = {flag for arg in review.args for flag in arg.flags if flag.startswith('--')}\nassert '--runner' in flags\nassert '--provider' in flags\nassert '--model' in flags\nPY`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Run challenger reviews through external, local, and manual paths

Goal: Make `scafld review` default to external subprocess execution, preserve explicit degraded modes, and persist completed review data plus provenance through the existing review artifact.

Status: completed
Dependencies: phase1

Changes:
- `scafld/commands/review.py` (all, shared) - Use the new review runner to execute or emit the challenger round depending on runner mode.
- `scafld/review_runtime.py` (all, shared) - Expose runner-ready review payload fields without mixing provider subprocess output into JSON mode.
- `scafld/review_workflow.py` (all, shared) - Add helpers to complete the latest review round from structured runner output while keeping the canonical artifact shape.
- `scafld/reviewing.py` (all, shared) - Validate and surface the new runner/provider provenance metadata.
- `tests/review_gate_smoke.sh` (all, shared) - Add review-gate coverage for codex-first resolution, claude fallback, explicit local mode, and explicit manual mode.

Acceptance:
- [ ] `ac2_1` integration - Review gate smoke proves external/local/manual runner behavior.
  - Command: `bash tests/review_gate_smoke.sh external-runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Sync wrappers, docs, and integration expectations

Goal: Keep the thin provider wrapper story honest now that `scafld review` owns the default challenger path.

Status: completed
Dependencies: phase2

Changes:
- `docs/review.md` (all, shared) - Teach the new default review runner behavior, degraded modes, and provenance semantics.
- `docs/integrations.md` (all, shared) - Clarify that wrapper scripts are optional integrations, not the primary review product surface.
- `docs/configuration.md` (all, shared) - Document the new review runner config and override flags.
- `docs/cli-reference.md` (all, shared) - Document review flags and the non-JSON default external execution behavior.
- `tests/codex_handoff_adapter_smoke.sh` (all, shared) - Keep proving that the codex review wrapper still feeds the challenger handoff as an optional adapter path.
- `tests/claude_handoff_adapter_smoke.sh` (all, shared) - Keep proving that the claude review wrapper still feeds the challenger handoff as an optional adapter path.

Acceptance:
- [ ] `ac3_1` integration - Codex wrapper smoke still passes after the review hard cut.
  - Command: `bash tests/codex_handoff_adapter_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac3_2` integration - Claude wrapper smoke still passes after the review hard cut.
  - Command: `bash tests/claude_handoff_adapter_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
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
Verdict: none
Timestamp: none
Review rounds: none
Reviewer mode: none
Reviewer session: none
Round status: none
Override applied: none
Override reason: none
Override confirmed at: none
Reviewed head: none
Reviewed dirty: none
Reviewed diff: none
Blocking count: none
Non-blocking count: none

Findings:
- none

Passes:
- none

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

- 2026-04-25T13:38:58Z - user - Spec created via scafld plan
- 2026-04-25T13:46:00Z - assistant - Replaced the placeholder draft with the hard-cut external-review runner plan grounded in the current review/runtime boundary.
- 2026-04-25T13:41:14Z - cli - Spec approved
- 2026-04-25T13:41:14Z - cli - Execution started
- 2026-04-25T14:02:00Z - assistant - Completed phase 1 and phase 2: added review runner config/selection, wired external/local/manual review execution, and proved the new path through targeted review smoke.
- 2026-04-25T14:14:00Z - assistant - Completed phase 3: aligned taught docs/help to the external-default review path and revalidated the optional codex/claude adapter lanes.
- 2026-04-26T08:17:18Z - cli - Spec cancelled
- 2026-04-26T08:17:18Z - cli - Spec superseded by harden-external-review-contract: Superseded by the hardened external review contract implementation.
