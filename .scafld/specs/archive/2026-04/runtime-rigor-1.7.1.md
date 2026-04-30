---
spec_version: '2.0'
task_id: runtime-rigor-1.7.1
created: '2026-04-28T17:30:00Z'
updated: '2026-04-28T11:54:20Z'
status: completed
harden_status: in_progress
size: small
risk_level: medium
---

# 1.7.1: scope-drift + severity tails + code quality + SIGINT + stream-aware review transport + schema strict-mode compliance

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

1.7.0 shipped the four structural changes. 1.7.1 closes the deferred tails surfaced during the 1.7.0 review rounds, fixes the multi-repo scope_drift bug surfaced during runx dogfooding, cleans the three code-quality flags from the 1.7.0 ship audit, and rebuilds the external-review transport so reviews don't false- timeout on legitimately long runs.
Seven phases, ordered so each is independently reviewable but they ship together as one tag:

  1. Scope-drift path normalization. Multi-repo specs that declare
     sibling-repo paths (`../cloud/...`) currently fail
     `scope_drift` because path comparison normalizes one side and
     not the other. Fix in audit_scope.py + the audit dispatch.

  2. Severity-gate tails. Two pieces deferred from 1.7.0:
       a. cmd_complete body cross-check verifies blocking and
          non-blocking COUNTS but not per-finding severity, so
          `gate_severity=medium` operators can be bypassed by
          re-classifying a finding's severity in the body.
       b. derive_task_guidance has no branch for the
          threshold-blocked `pass_with_issues` case; next_action
          currently falls through to "rerun review" which doesn't
          help (rerunning the same review hits the same gate).

  3. Code-quality cleanups (the "clean core" of the version):
       a. Replace `apply_shared_ownership` text-line regex
          rewriter with a YAML-aware injector. Slim spec output
          is structured; we can parse, mutate, dump.
       b. Unify plan-time and review-time overlap logic.
          `lifecycle_runtime.new_spec_snapshot` reimplements the
          overlap classification inline because
          `classify_active_overlap` is bilateral (both sides must
          declare shared) and plan-time is unilateral. Refactor
          the helper to accept `plan_time=True` and reuse it.
       c. Drop the `simple_yaml_load` hand-rolled fallback in
          `scafld/config.py`. PyYAML is install_requires; every
          supported runtime path has it. The fallback silently
          drops list values, which has caused a smoke flake
          already. Use `yaml.safe_load` unconditionally.

  4. SIGINT handler investigation + fix. `case_review_cancel_signal_handler`
     flakes on macOS local (CI Linux passes). The handler at
     review_runner.py:1344 sets a cancel flag and calls
     `_terminate_provider_process_group`, but on macOS the
     subprocess group sometimes survives the signal and the
     parent stays in `proc.communicate()`. Investigate the actual
     signal flow, fix the root cause, confirm fix on macOS +
     Linux.

  5. Provider transport refactor. `_run_provider_subprocess` uses
     `proc.communicate()` which blocks until end-of-process, and
     `_provider_watchdog` is wall-clock-only. A long-but-
     productive review (large spec, many tool calls) is
     indistinguishable from a hung one and gets SIGTERM'd at the
     wall deadline with no recovery. Replace communicate() with
     a reader-thread pattern that drains stdout/stderr into
     bounded buffers and advances `last_activity_at` on each
     chunk. Replace the watchdog with activity-aware kill: idle
     timeout (no new bytes for N seconds) OR absolute max wall-
     clock. Both knobs land in `review.external` config; legacy
     `timeout_seconds` aliases to `absolute_max_seconds` so
     existing configs keep working. Provider-agnostic; both
     claude and codex benefit.

  6. Claude streaming output. Phase 5 makes the watchdog activity-
     aware, but claude with `--output-format json` buffers the
     entire response and emits zero stdout bytes until end-of-
     process. Switch to `--output-format stream-json` so claude
     emits NDJSON events (assistant deltas, tool_use, system,
     result) as they happen. Rewrite `_extract_claude_stdout` to
     consume NDJSON: scan for the final `type: "result"` event,
     extract the structured packet from `result` (or
     `structured_output` when --json-schema is attached); pull
     session id and model from the `system.init` event. Codex is
     already streaming-friendly via `codex exec` (stdout streams
     human-readable progress; structured payload via
     `--output-last-message`); no codex argv change required.

  7. Liveness diagnostic on watchdog kill. Today's diagnostic
     records `raw_output: empty`, `stdout: empty`, exit 143 with
     no clue why. Extend the diagnostic dump (and the
     _write_external_diagnostic call sites) so when the watchdog
     fires the file records: kill_reason ("idle_timeout" or
     "absolute_max"), time_since_last_byte, idle_timeout_seconds
     and absolute_max_seconds in effect, last 16KB of stdout, and
     (for claude stream-json) the last N parsed event types. The
     operator gets an actionable trail instead of staring at
     empty fields.

  8. ReviewPacket schema strict-mode compliance. The new
     transport surfaced a pre-existing schema bug that the old
     broken watchdog was hiding: codex's `--output-schema` runs
     in strict mode, which rejects any object where `properties`
     lists keys that are not in `required` (when
     `additionalProperties: false`). Affected fields:
     `checked_surfaces[].limitations`,
     `findings[].spec_update_suggestions`, and the
     `spec_update_suggestions[]` item's optional `reason`,
     `phase_id`, `validation_command`. Fix: list every property
     in `required` and switch the optional ones to nullable
     (`["array", "null"]` / `["string", "null"]`). Python
     normalizers already default missing values to empty; extend
     them to treat `null` the same. Update the prompt example so
     the demonstrated shape matches the strict-mode schema.

Hard cutover: no behavior changes that break 1.7.0-shipped contracts. All eight phases are additive/corrective only. Phase 5+6 together replace the broken transport; phase 7 is observability; phase 8 is the schema fix the working transport surfaced.
Final dogfood: the auto-runner review of 1.7.1 itself runs under the new transport. If the watchdog/streaming code has a bug, this review will surface it before tag.

## Context

CWD: `.`

Packages:
- none

Files impacted:
- `scafld/audit_scope.py` (all) - Phase 1 path normalization. Phase 3a YAML-aware apply_shared_ownership. Phase 3b classify_active_overlap unification.
- `scafld/commands/audit.py` (all) - Phase 1: audit command consumes the normalized paths.
- `scafld/review_workflow.py` (all) - Phase 1: scope_drift diagnostic surface.
- `scafld/commands/review.py` (all, shared) - Phase 2a: cmd_complete body cross-check adds per-severity multiset check on findings.
- `scafld/runtime_guidance.py` (all) - Phase 2b: derive_task_guidance gains a threshold-blocked branch.
- `scafld/lifecycle_runtime.py` (all) - Phase 3b: switch from inline overlap reimpl to classify_active_overlap(plan_time=True).
- `scafld/config.py` (all) - Phase 3c: drop simple_yaml_load fallback; use yaml.safe_load directly.
- `scafld/review_runner.py` (all, shared) - Phase 4: SIGINT handler. Phase 5: reader-thread transport + activity watchdog. Phase 6: claude stream-json + NDJSON parser. Phase 7: liveness diagnostic dump. Phase 8: prompt example update (build_external_review_prompt).
- `.ai/schemas/review_packet.json` (all) - Phase 8: every property in required; optional fields use ["<type>", "null"] for codex strict-mode acceptance.
- `scafld/review_packet.py` (all) - Phase 8: normalizers handle null values as the empty default.
- `.ai/config.yaml` (all, shared) - Migrate scafld's own review config from legacy `timeout_seconds: 600` to the new `idle_timeout_seconds` + `absolute_max_seconds` keys so its review actually benefits from the activity-aware watchdog (180s idle catches hangs; 1800s ceiling won't false-kill substantive reviews).
- `tests/test_audit_scope.py` (all) - Coverage for path normalization and the unified classify_active_overlap.
- `tests/test_review_packet.py` (all, shared) - Coverage for per-severity body cross-check (gate-integrity tail).
- `tests/test_review_gate_severity.py` (all) - Coverage for derive_task_guidance threshold branch.
- `tests/test_review_runner.py` (all, shared) - Coverage for the SIGINT handler fix.
- `tests/review_gate_smoke.sh` (all, shared) - Smokes for sibling-repo scope_drift path, body per-severity tamper, threshold-block guidance.
- `tests/lifecycle_smoke.sh` (all, shared) - Smoke if needed for plan-time classify_active_overlap unified path.
- `docs/configuration.md` (all, shared) - Document path normalization caveat and any user-visible change to gate_severity guidance.
- `docs/scope-auditing.md` (all) - Operator docs for sibling-repo scope_drift behavior.

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
- [x] `dod1` scope_drift correctly handles sibling-repo declared paths (`../cloud/foo.py`); declared paths don't show as undeclared when they actually exist in the changed set.
- [x] `dod2` cmd_complete body cross-check rejects body that re-classifies a finding's severity without changing count.
- [x] `dod3` derive_task_guidance returns a threshold-blocked next_action that names the threshold and suggests fix-or-relax (instead of "rerun review").
- [x] `dod4` apply_shared_ownership uses YAML round-trip (or equivalent structural mutation) and stays idempotent.
- [x] `dod5` lifecycle_runtime.new_spec_snapshot calls classify_active_overlap(plan_time=True); inline overlap loop is gone.
- [x] `dod6` scafld/config.py uses yaml.safe_load unconditionally; simple_yaml_load is removed.
- [x] `dod7` scafld review handles SIGINT promptly on macOS (case_review_cancel_signal_handler smoke passes locally on macOS within the existing 5-second wait window).
- [x] `dod8` Provider watchdog kills on idle_timeout (no new stdout bytes for N seconds) OR absolute_max wall-clock; existing wall-only behaviour gone. Legacy `timeout_seconds` config aliases to `absolute_max_seconds` with a deprecation note logged once.
- [x] `dod9` claude review streams stdout incrementally via `--output-format stream-json`; `_extract_claude_stdout` consumes NDJSON events and returns the final `result` packet, session id, and model with parity to today's batch-mode behaviour.
- [x] `dod10` When the watchdog fires, the diagnostic dump records kill_reason (idle_timeout|absolute_max), time_since_last_byte, the threshold values in effect, and the tail of stdout (last 16KB / last 200 lines). Today's empty-fields diagnostic shape is gone.
- [x] `dod11` Auto-runner external review of `runtime-rigor-1.7.1` itself completes successfully under the new transport (final dogfood; failure here means phases 5-7 regress something).
- [x] `dod12` ReviewPacket schema is accepted by codex's structured-output strict mode (every key in `properties` listed in `required`; optional fields use `["<type>", "null"]`); python normalizer treats null as the empty default; both codex and claude reviews complete with the same schema.

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
  - Command: `python3 -m unittest tests.test_acceptance tests.test_audit_scope tests.test_review_packet tests.test_review_gate_severity tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Scope-drift path normalization

Goal: Multi-repo specs declaring sibling-repo paths no longer fail scope_drift.

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
  - Command: `python3 -m unittest tests.test_audit_scope`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Severity-gate tails (per-severity body cross-check + threshold-blocked guidance)

Goal: Close the two non-blocking tails deferred from 1.7.0 gate-integrity and severity-gates.

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
  - Command: `python3 -m unittest tests.test_review_packet tests.test_review_gate_severity`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Code-quality cleanups (apply_shared_ownership + classify_active_overlap unification + simple_yaml_load removal)

Goal: Replace text-rewriter and inline overlap with structural code; remove the lossy YAML fallback.

Status: completed
Dependencies: phase2

Changes:
- none

Acceptance:
- [x] `ac3_1` compile
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` test
  - Command: `python3 -m unittest tests.test_audit_scope tests.test_clean_plan tests.test_acceptance`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_3` boundary - simple_yaml_load is gone.
  - Command: `sh -c '! grep -q "def simple_yaml_load" scafld/config.py'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 4: SIGINT handler fix (macOS subprocess termination)

Goal: scafld review exits promptly after SIGINT on macOS as well as Linux.

Status: completed
Dependencies: phase3

Changes:
- none

Acceptance:
- [x] `ac4_1` compile
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac4_2` test
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac4_3` boundary - Cancel-signal smoke registered.
  - Command: `grep -q 'case_review_cancel_signal_handler' tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 5: Provider transport refactor (reader-thread architecture + activity-aware watchdog)

Goal: Replace the wall-clock-only watchdog with an activity-aware one fed by stdout/stderr reader threads. Provider-agnostic; both claude and codex benefit.

Status: completed
Dependencies: phase4

Changes:
- none

Acceptance:
- [x] `ac5_1` compile
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac5_2` test
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac5_3` boundary - Reader-thread transport is in place (_StreamPump class + activity-aware watchdog).
  - Command: `sh -c 'grep -q "class _StreamPump" scafld/review_runner.py && grep -q "kill_state\[.reason.\] = .idle_timeout." scafld/review_runner.py && grep -q "kill_state\[.reason.\] = .absolute_max." scafld/review_runner.py'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 6: Claude stream-json output + NDJSON parser

Goal: Claude emits NDJSON events incrementally so the activity watchdog can see real liveness signals; parser extracts the structured packet from the final `result` event.

Status: completed
Dependencies: phase5

Changes:
- none

Acceptance:
- [x] `ac6_1` compile
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac6_2` test
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac6_3` boundary - claude argv switched to stream-json + verbose; NDJSON parser in place.
  - Command: `sh -c 'grep -q "\"stream-json\"" scafld/review_runner.py && grep -q "_extract_claude_stdout_ndjson" scafld/review_runner.py && grep -q "_claude_ndjson_event_inspector" scafld/review_runner.py'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 7: Liveness diagnostic on watchdog kill

Goal: When the watchdog fires, the diagnostic dump names the trigger and shows the last activity instead of empty fields.

Status: completed
Dependencies: phase6

Changes:
- none

Acceptance:
- [x] `ac7_1` compile
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac7_2` test
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac7_3` boundary - Watchdog kill section header lands in diagnostic body.
  - Command: `grep -q 'Watchdog Kill' scafld/review_runner.py`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 8: ReviewPacket schema strict-mode compliance

Goal: Make the JSON schema acceptable to codex's structured-output strict mode, which the new transport just surfaced as a previously-hidden failure mode.

Status: completed
Dependencies: phase7

Changes:
- none

Acceptance:
- [x] `ac8_1` compile
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac8_2` test
  - Command: `python3 -m unittest tests.test_review_packet tests.test_review_runner`
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
Timestamp: '2026-04-28T11:53:46Z'
Review rounds: 4
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

- 2026-04-28T17:30:00Z - user - 1.7.1 clean-core spec: rolls in scope-drift draft, severity-gate tails, code-quality cleanups, SIGINT fix.
- 2026-04-28T18:30:00Z - user - Extend 1.7.1 with phases 5-7 (transport refactor, claude streaming, liveness diagnostic) after four straight 600s wall-watchdog kills on the original 1.7.1 review made it clear the broken transport, not the spec, was the failure mode. Roll into 1.7.1 instead of splitting to 1.7.2 to keep tag history clean.
- 2026-04-28T11:54:20Z - cli - Spec completed
