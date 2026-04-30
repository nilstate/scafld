---
spec_version: '2.0'
task_id: structured-acceptance-result-writes
created: '2026-04-25T09:15:18Z'
updated: '2026-04-25T09:35:02Z'
status: completed
harden_status: in_progress
size: medium
risk_level: high
---

# Make acceptance result writes structurally safe

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

Replace the remaining regex text surgery in `scafld/acceptance.py` with a structured YAML update. Right now `record_exec_result()` rewrites criterion result fields by scanning raw lines, which breaks once YAML folds a long `result_output` string across continuation lines. The next explicit `scafld exec` pass removes only the first `result_output:` line, leaves the folded continuation lines behind, and can corrupt the spec. The clean model is: acceptance result writes walk the parsed document structure, update the matching criterion in place, and emit valid YAML no matter how many times exec reruns the same phase.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`

Files impacted:
- `scafld/acceptance.py` (task-selected) - Criterion result writes still mutate the spec with regex line surgery.
- `scafld/execution_runtime.py` (task-selected) - Exec currently depends on the legacy text-based result writer.
- `tests/acceptance_result_write_smoke.sh` (all) - A disposable-repo smoke should prove repeated explicit exec passes keep the spec valid YAML.

Invariants:
- `acceptance_result_updates_must_preserve_valid_yaml`
- `repeated_exec_on_the_same_phase_must_not_corrupt_the_spec`
- `packaged_cli_is_the_truth_surface`

Related docs:
- `docs/lifecycle.md`
- `docs/cli-reference.md`

## Objectives

- Make acceptance result writes structurally safe for long folded output strings.
- Keep repeated explicit `scafld exec --phase ...` runs from corrupting the spec.
- Prove the fix with a packaged-CLI smoke that validates the resulting spec after multiple exec passes.

## Scope



## Dependencies

- None.

## Assumptions

- The bug is caused by the raw-text writer failing to remove folded continuation lines from previous result_output values.
- Safe YAML emission is acceptable for these runtime result updates because the same spec already undergoes structured writes elsewhere.

## Touchpoints

- acceptance result recording: Criterion status/output writes should be document-aware, not regex line edits.
- explicit reruns: Re-executing an already-completed phase should stay safe and deterministic.
- spec integrity: Runtime metadata updates must never leave the active spec unparsable.

## Risks

- A structured rewrite could accidentally change the stored criterion shape for entries that already use nested `result` mappings.
- A result-writer fix that rewrites the full document could still damage user-authored YAML structure even if the spec stays parseable.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` Repeated explicit exec runs no longer corrupt the spec when result_output is long enough to fold.
- [ ] `dod2` Acceptance result writes preserve the criterion's existing flat or nested result shape.
- [ ] `dod3` A packaged-CLI smoke proves the spec still validates after repeated exec passes.
- [ ] `dod4` Repeated exec updates preserve unrelated author-authored YAML structure such as comments and literal block scalars.

Validation:
- [ ] `v1` compile - Compile the Python sources after the acceptance result write change.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Acceptance-result smoke proves repeated explicit exec passes keep the spec valid while preserving unrelated author-authored YAML structure.
  - Command: `bash tests/acceptance_result_write_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Replace regex result writes with structured criterion updates

Goal: Record acceptance results through parsed YAML mutation instead of raw line surgery.

Status: completed
Dependencies: none

Changes:
- `scafld/acceptance.py` (all) - Replace the raw-text criterion result writer with a structured update that preserves flat versus nested result shape.
- `scafld/execution_runtime.py` (all) - Keep exec using the new structured result writer without changing criterion semantics.

Acceptance:
- [x] `ac1_1` integration - Repeated direct result writes keep the YAML document parseable.
  - Command: `python3 - <<'PY'`
  - Expected kind: none
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Prove repeated exec preserves spec validity

Goal: Reproduce the old corruption path through the packaged CLI and prove it stays valid without rewriting unrelated author-authored YAML structure.

Status: completed
Dependencies: phase1

Changes:
- `tests/acceptance_result_write_smoke.sh` (all) - Create a disposable-repo smoke that runs explicit exec multiple times on long-output and nested-result criteria, then validates both spec integrity and preservation of author-authored comments and block scalars.

Acceptance:
- [x] `ac2_1` integration - Acceptance-result smoke proves repeated explicit exec keeps the spec valid while preserving unrelated author-authored YAML structure.
  - Command: `bash tests/acceptance_result_write_smoke.sh`
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
Verdict: pass
Timestamp: '2026-04-25T09:34:44Z'
Review rounds: 2
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

- 2026-04-25T09:15:18Z - user - Spec created via scafld plan
- 2026-04-25T09:17:10Z - assistant - Replaced the placeholder draft with a spec grounded in a live corruption caused by repeated explicit exec passes against folded result_output fields.
- 2026-04-25T09:16:27Z - cli - Spec approved
- 2026-04-25T09:18:40Z - assistant - Tightened phase1 acceptance so it exercises the repeated result-writer path directly instead of only compiling.
- 2026-04-25T09:17:17Z - cli - Execution started
- 2026-04-25T09:26:29Z - assistant - Review found the structured writer still rewrote the full spec and stripped comments and block-scalar authoring outside the runtime fields.
- 2026-04-25T09:35:02Z - cli - Spec completed
