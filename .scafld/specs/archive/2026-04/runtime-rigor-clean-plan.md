---
spec_version: '2.0'
task_id: runtime-rigor-clean-plan
created: '2026-04-28T13:30:00Z'
updated: '2026-04-28T05:10:57Z'
status: completed
harden_status: in_progress
size: small
risk_level: medium
---

# Slim plan output, --command/--files flags, plan-time scope conflict detection

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

`scafld plan <id>` today produces a 50+ line scaffold filled with `TODO: replace with...` placeholders. The operator (or LLM) edits each TODO to a real value before approval. Plus review-time `scope_drift` is the only audit_scope check, so overlapping work between active specs only surfaces on first review.
1.7.0 ships the slim plan path:

  `scafld plan <id> -t "<title>" --command "<cmd>" --files "a.py,b.py"`

produces a complete ~30-line spec. No `TODO` markers (so `validate_spec` at lifecycle_runtime.py:69-134 passes cleanly without a manual fill round). One phase, one criterion with explicit `expected_kind: exit_code_zero` (so `evaluate_acceptance_criterion`'s strict-unset-reject at acceptance.py:359-379 doesn't fire).
The verbose template path (current behavior) is preserved when `--command` is omitted; multi-phase complex specs stay verbose by default.
Plus: after the scaffold writes the slim spec, `audit_scope.classify_active_overlap` (audit_scope.py:124) runs against other active specs:
  - Files declared `shared` by another spec: auto-tag the new
    spec's change entry with `ownership: "shared"`.
  - Files declared exclusive by another spec: refuse to plan
    with `EC.SCOPE_DRIFT` and a one-line conflict message
    naming the other task.

Schema-required fields not enforced by `validate_spec` (`task.context`, `task.objectives`, `task.touchpoints`, `task.acceptance`) are omitted by the slim template; renderers in handoff_renderer.py:115, output.py:269, and projections.py:109 use `or []` fallbacks so missing fields render as empty sections rather than crashes.
Hard cutover: existing 1.6.x specs (verbose, with TODO markers) keep working because the verbose template path is untouched. New slim specs are produced only when `--command` is supplied.

## Context

CWD: `.`

Packages:
- none

Files impacted:
- `scafld/spec_templates.py` (all) - Add `build_slim_spec_scaffold` (slim template) called when --command is provided; verbose `build_new_spec_scaffold` retained for the no-flag path.
- `scafld/lifecycle_runtime.py` (all) - `new_spec_snapshot` accepts `command`/`files` kwargs; routes to slim scaffold when set; calls `audit_scope.classify_active_overlap` after write and applies ownership tags or refuses.
- `scafld/workflow_runtime.py` (all) - `plan_snapshot` pipes `command`/`files` through to `new_spec_snapshot`.
- `scafld/commands/workflow.py` (all) - `cmd_plan` reads `args.command` and `args.files` and forwards to `plan_snapshot`.
- `scafld/commands/surface.py` (all) - Add `--command` and `--files` ArgumentSpecs to the `plan` and `new` CommandSpec arg tuples.
- `scafld/audit_scope.py` (all) - Add `apply_shared_ownership(spec_text, shared_paths)` that rewrites declared change entries to add `ownership: shared`. Existing classify_active_overlap is reused as-is.
- `tests/test_clean_plan.py` (all) - Cover slim template line count, --command/--files insertion, default size/risk, plan-time auto-tag, plan-time exclusive refusal.
- `tests/review_gate_smoke.sh` (all, shared) - Smoke for slim plan output and plan-time conflict refusal.
- `docs/configuration.md` (all) - Document slim plan flags and plan-time conflict detection.

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
- [x] `dod1` `scafld plan <id> --command "<cmd>"` produces a spec under 40 lines with no `TODO` placeholders that would trip `validate_spec`.
- [x] `dod2` Slim spec criterion always carries explicit `expected_kind: exit_code_zero` so `evaluate_acceptance_criterion` doesn't strict-reject.
- [x] `dod3` `--files "a.py,b.py"` produces one `changes[].file` entry per declared file; `phases[].changes` is non-empty.
- [x] `dod4` Verbose template path (no `--command`) is unchanged; existing 1.6.x specs continue to plan/approve/build as before.
- [x] `dod5` After scaffold, `classify_active_overlap` runs; overlapping shared files auto-tag `ownership: shared` in the new spec; exclusive conflicts refuse to plan with `EC.SCOPE_DRIFT`.
- [x] `dod6` Operator docs cover `--command`, `--files`, and plan-time conflict behavior; smoke proves both slim shape and conflict refusal end-to-end.

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
  - Command: `python3 -m unittest tests.test_clean_plan tests.test_acceptance`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` boundary - --command flag wired.
  - Command: `sh -c 'scafld plan --help 2>&1 | grep -q -- --command'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` boundary - --files flag wired.
  - Command: `sh -c 'scafld plan --help 2>&1 | grep -q -- --files'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Slim spec scaffold + --command/--files flags

Goal: `scafld plan <id> --command "<cmd>" --files "a.py,b.py"` produces a ~30-line slim spec ready to approve.

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
  - Command: `python3 -m unittest tests.test_clean_plan`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_3` boundary - CLI flags wired.
  - Command: `sh -c 'scafld plan --help 2>&1 | grep -q -- --command && scafld plan --help 2>&1 | grep -q -- --files'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Plan-time conflict detection

Goal: After scaffold, run audit_scope.classify_active_overlap; auto-tag shared, refuse on exclusive conflict.

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
  - Command: `python3 -m unittest tests.test_clean_plan`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Smoke + operator docs

Goal: End-to-end smoke for slim plan + conflict refusal. Document the new flags and behavior.

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
- [x] `ac3_2` boundary - Slim plan smoke registered.
  - Command: `grep -q 'case_plan_produces_slim_spec' tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_3` boundary - Conflict-at-plan smoke registered.
  - Command: `grep -q 'case_plan_refuses_on_exclusive_conflict' tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_4` boundary - Docs cover slim plan.
  - Command: `grep -qiE 'Slim plan|--command|--files' docs/configuration.md`
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
Timestamp: '2026-04-28T05:10:49Z'
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

- 2026-04-28T13:30:00Z - user - 1.7.0 spec 4: slim plan template + --command/--files flags + plan-time conflict detection. Folds in early-conflict-detection.
- 2026-04-28T13:30:00Z - agent - Designed against verified code: validate_spec only checks TODO markers and task title/summary/size/risk_level (lifecycle_runtime.py:69-134); jsonschema is not imported anywhere; renderers fall back gracefully on missing optional blocks. Slim spec passes all gates without TODO scaffolding.
- 2026-04-28T20:00:00Z - agent - Review 1 verdict=fail. Resolved 1 blocking + 1 medium:
