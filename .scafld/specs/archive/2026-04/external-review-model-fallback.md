---
spec_version: '2.0'
task_id: external-review-model-fallback
created: '2026-04-27T09:03:46Z'
updated: '2026-04-27T10:11:59Z'
status: completed
harden_status: in_progress
size: small
risk_level: low
---

# Provider model fallback list for external review

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

`review.external.codex.model` and `review.external.claude.model` accept a single string. The pinned defaults (`gpt-5.5`, `claude-opus-4-7`) are aggressive on purpose, but if an account does not have access to the pin, `scafld review` fails the entire run with `external review runner failed via codex` and forces the operator to either pass `--model` or switch runner. The provider-binary fallback in `_auto_provider_candidates` does not help here: the binary exists, only the requested model is not available, so auto fallback never triggers.
Accept a list under `review.external.{codex,claude}.model` and have `run_external_review` retry the same provider across the list when the failure surface looks like a model-rejection. Record the per-attempt outcomes in provenance so it is obvious which model produced the verdict. Keep the existing single-string form working untouched.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`

Files impacted:
- `scafld/review_runner.py` (task-selected) - Owns config parsing for review.external and the provider invocation loop.
- `scafld/runtime_bundle.py` (task-selected) - Loads runtime config; confirm list-vs-string acceptance for the model field.
- `.ai/config.yaml` (task-selected) - Reference config that documents the model list shape; bundled with new workspaces via `scafld init`/`scafld update`.
- `tests/test_review_runner.py` (task-selected) - Pure tests for config parsing and the model-rejection classifier.
- `tests/review_gate_smoke.sh` (task-selected) - End-to-end smoke that exercises the fallback path with a stub provider.
- `docs/configuration.md` (task-selected) - Operator docs for the review configuration block.

Invariants:
- `single_string_model_must_continue_to_work`
- `fallback_only_triggers_on_model_rejection_surfaces`
- `every_attempt_must_be_recorded_in_session_ledger`
- `first_successful_attempt_is_authoritative_for_verdict`

Related docs:
- `.ai/specs/archive/2026-04/external-review-observed-model-truth.yaml`
- `.ai/specs/archive/2026-04/harden-external-review-contract.yaml`
- `docs/configuration.md`

## Objectives

- Accept a list of models per provider in `.ai/config.yaml` without breaking the existing single-string form.
- Retry the provider call against the next model only when the failure looks like model-rejection.
- Record an entry per attempted model in the session ledger and in the review provenance.
- Document the model list shape and rejection classifier in operator docs.

## Scope



## Dependencies

- None.

## Assumptions

- Model rejection from `codex exec` and `claude -p` is observable in stderr with a stable enough signature to classify reliably; if the signature is ambiguous, fallback does not trigger and behavior matches today.
- Operators who pin a single model want today's behavior; the list form is opt-in.
- It is acceptable to record a `failed_model_unavailable` provider invocation status alongside a subsequent `completed`; downstream consumers (`scafld status`, `scafld report`) already iterate the entry list rather than reading only the latest.

## Touchpoints

- review config: `_provider_model` becomes `_provider_models` returning an ordered list; existing callers consume the first element when they only want one.
- review runner: Wrap the provider call in a per-model loop with rejection classification.
- session ledger: Record one provider invocation entry per model attempt, including the new `failed_model_unavailable` status.
- operator docs: Document the list shape and how to spot fallback events in the report.

## Risks

- Rejection classifier is too aggressive and triggers fallback on transient or unrelated errors, masking real failures.
- List form silently changes behavior for operators who already configured a single model.
- Per-attempt session ledger entries swell `session.json`.

## Acceptance

Profile: standard

Definition of done:
- [x] `dod1` `review.external.{codex,claude}.model` accepts a string or a non-empty list of strings; existing single-string configs continue to work.
- [x] `dod2` When a provider call fails with a recognized model-rejection signature, the runner retries the same provider with the next model in the list.
- [x] `dod3` The session ledger records one provider invocation per attempted model, including the failure reason for skipped attempts.
- [x] `dod4` Smoke coverage proves the fallback path against a stub provider; unit coverage proves the rejection classifier.
- [x] `dod5` Docs explain the list shape, the rejection classifier, and how to observe fallback in `scafld status`.

Validation:
- [ ] `v1` compile - Python sources compile.
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test - Unit tests cover config parsing and the rejection classifier.
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - Smoke proves the fallback path.
  - Command: `bash tests/review_gate_smoke.sh external-review-model-fallback`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` boundary - Operator docs mention the model list shape.
  - Command: `rg -n 'model:.*\[' docs/configuration.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Accept a model list in review config

Goal: Parse `review.external.{codex,claude}.model` as `str | list[str]` and thread an ordered list of models down to the runner.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` compile - Sources compile after config refactor.
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` test - Config parsing tests pass.
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Per-model retry loop with rejection classifier

Goal: When a provider exits with a model-rejection signature, retry against the next configured model and record both attempts.

Status: completed
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` test - Classifier and runner unit tests pass.
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` boundary - Fallback smoke is registered in the harness dispatcher and case_all.
  - Command: `grep -q 'case_external_review_model_fallback' tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_3` boundary - Smoke harness has valid bash syntax.
  - Command: `bash -n tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_4` boundary - Docs explain the rejection classifier and list shape.
  - Command: `grep -qE 'rejection signature|model not available|list of strings' docs/configuration.md`
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
Timestamp: '2026-04-27T10:05:07Z'
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

- 2026-04-27T09:03:46Z - user - Spec created via scafld plan.
- 2026-04-27T09:10:00Z - agent - Replaced placeholder with a concrete model-fallback plan covering config, runner loop, ledger entries, and docs.
- 2026-04-27T09:09:22Z - cli - Spec approved
- 2026-04-27T09:12:58Z - cli - Execution started
- 2026-04-27T10:11:59Z - cli - Spec completed
