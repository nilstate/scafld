---
spec_version: '2.0'
task_id: real-work-dogfood-proof
created: '2026-04-24T06:10:00Z'
updated: '2026-04-24T01:06:46Z'
status: completed
harden_status: passed
size: large
risk_level: medium
---

# Prove The Ceremony Pays Off On Real Repo Work

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

Dogfood scafld against a curated cohort of medium brownfield tasks until the real-standard question set is no longer theoretical. The real standard is explicit: build feels easier than a raw agent loop, review catches meaningful defects often enough to matter, the handoff is consumed by default, and the extra ceremony feels justified on real repo work. This slice defines the cohort, the fixed question set, the reporting script, and the smoke coverage that keeps them honest without introducing internal-only docs.

## Context

CWD: `.`

Packages:
- `scripts/`
- `tests/`
- `.ai/specs/archive/`
- `scafld/commands/reporting.py`

Files impacted:
- `scripts/real_standard.py` (all) - A cohort runner should summarize the real-standard questions from existing sessions and review artifacts.
- `tests/real_standard_cohort_smoke.sh` (all) - New smoke should keep the cohort script and fixed question set honest.
- `scafld/commands/reporting.py` (all) - Current report JSON should be consumable by the cohort script without new primitives.
- `.ai/specs/archive/` (task-selected) - Dogfood should run against real archived or newly executed medium tasks.

Invariants:
- `public_api_stable`
- `config_from_env`
- `no_test_logic_in_production`

Related docs:
- `README.md`
- `docs/review.md`
- `docs/execution.md`

## Objectives

- Define the real-standard question set as an explicit cohort contract.
- Add a script that summarizes the real-standard signals from report/session/review data.
- Keep the fixed question set encoded in the cohort script and smoke coverage rather than internal-only docs.
- Make future release decisions depend on real repo work, not architecture confidence.

## Scope



## Dependencies

- build-happy-path-product-fit
- challenger-review-signal-lift
- default-handoff-consumption

## Assumptions

- Some parts of the real standard remain partly qualitative and should be recorded honestly as operator notes.
- The cohort script should consume existing report and session outputs rather than inventing a parallel telemetry layer.

## Touchpoints

- dogfood cohort: Medium brownfield tasks are the proving ground, not toy smoke tests.
- operator notes: Qualitative evidence should sit alongside the measurable signals, not be hand-waved away.
- existing report surface: The cohort script should stand on the current report/session/review artifacts.

## Risks

- The cohort can turn into vanity reporting if the tasks are too small or too friendly.
- Qualitative notes can become vague and non-comparable.
- A custom script can drift from current report/session contracts.

## Acceptance

Profile: standard

Definition of done:
- [ ] `dod1` A fixed cohort question set exists in code and smoke coverage.
- [ ] `dod2` A thin cohort script summarizes the real-standard signals from existing report/session/review data.
- [ ] `dod3` The cohort summary surfaces build ease, review usefulness, default handoff consumption, and ceremony payoff as explicit prompts.
- [ ] `dod4` A cohort smoke test proves the script and fixed question set stay aligned.

Validation:
- [ ] `v1` custom - Validate this draft spec.
  - Command: `python3 cli/scafld validate real-work-dogfood-proof`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test - Run the cohort smoke and report smoke.
  - Command: `bash tests/real_standard_cohort_smoke.sh && bash tests/report_metrics_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` custom - The cohort script can summarize an explicit task list.
  - Command: `python3 scripts/real_standard.py --help`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Define the cohort contract

Goal: Encode the fixed question set and representative-task contract in the cohort tooling.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` custom - Cohort smoke encodes the fixed question set.
  - Command: `bash tests/real_standard_cohort_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Summarize the cohort honestly

Goal: Add a thin script that reads existing artifacts and produces a cohort summary.

Status: completed
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` test - Cohort smoke and report smoke pass.
  - Command: `bash tests/real_standard_cohort_smoke.sh && bash tests/report_metrics_smoke.sh`
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
Timestamp: '2026-04-24T01:05:00Z'
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

- 2026-04-24T06:10:00Z - assistant - Drafted the real-work dogfood slice around a fixed cohort question set.
- 2026-04-24T01:03:21Z - cli - Spec approved
- 2026-04-24T01:03:38Z - cli - Execution started
- 2026-04-24T01:06:46Z - cli - Spec completed
