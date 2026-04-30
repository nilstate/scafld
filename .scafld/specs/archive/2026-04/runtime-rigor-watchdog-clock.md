---
spec_version: '2.0'
task_id: runtime-rigor-watchdog-clock
created: '2026-04-28T00:46:03Z'
updated: '2026-04-28T01:33:05Z'
status: cancelled
harden_status: in_progress
size: small
risk_level: low
---

# Suspend-aware watchdog clock immune to laptop sleep

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

The provider watchdog (added in `review-runner-watchdog-and-retry`) uses `time.monotonic()` for the deadline. On macOS, Python's `time.monotonic()` calls `clock_gettime(CLOCK_UPTIME_RAW)`, which explicitly excludes time spent in suspend. Symptom: laptop sleeps mid-review for two hours; on resume the watchdog's elapsed counter has barely moved, the deadline is never reached, and the provider process keeps running until manually killed. Observed during cutover when a review hung 2h10m through a sleep+resume cycle.
`time.time()` (wall clock) has the opposite failure mode: NTP backward adjustments or manual clock changes can rewind it, silently extending a deadline.
Neither clock alone is correct for "real time elapsed since I started." Together they are. A small `WallClockMonotonic` helper holds both starting timestamps and exposes one method, `elapsed()`, returning `max(wall_elapsed, monotonic_elapsed)`. Wall-clock catches suspend; monotonic catches NTP regression. The max() resolves any disagreement in favor of "more time has passed", which is exactly what a deadline cares about.
Single helper, single method, single comparison. Replaces every direct `time.monotonic()` call in `_provider_watchdog` and the transient-retry exponential backoff path.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`

Files impacted:
- `scafld/review_runner.py` (task-selected) - Adds the WallClockMonotonic helper class and rewires _provider_watchdog plus the transient-retry backoff to use it.
- `tests/test_review_runner.py` (task-selected) - Unit coverage for the helper under the suspend (wall jumps forward) and NTP-rewind (wall rewinds) failure modes.
- `docs/review.md` (task-selected) - Operator-facing note that the watchdog deadline is suspend-aware so a laptop sleep counts toward the deadline.

Invariants:
- `deadline_must_fire_after_wall_clock_elapses_even_if_monotonic_did_not`
- `deadline_must_fire_after_monotonic_elapses_even_if_wall_clock_rewound`
- `elapsed_must_only_move_forward_under_all_clock_conditions`
- `watchdog_thread_remains_the_only_authoritative_force_kill_path`

Related docs:
- `plans/1.7.0-cutover.md`
- `.ai/specs/archive/2026-04/review-runner-watchdog-and-retry.yaml`
- `scafld/review_runner.py`

## Objectives

- Make the configured timeout the real upper bound on a provider run regardless of laptop sleep or wall-clock adjustments.
- Replace direct time.monotonic() reads in the watchdog with a single helper whose docstring explains the two-clock failure modes.
- Keep the watchdog thread as the only authoritative force-kill path; no new signal handlers, no new processes.

## Scope



## Dependencies

- None.

## Assumptions

- Python's `time.time()` and `time.monotonic()` are both monotonic-relative-to-themselves except for the documented failure modes; the max() across them gives a strictly forward-moving elapsed counter.
- NTP backward steps are rare (typically slewed) and small (seconds, not hours); the wall-clock arm of the helper handles the more common suspend case.
- A single helper instance per provider call is enough; no shared global clock needed.

## Touchpoints

- watchdog timing: WallClockMonotonic is the canonical 'time elapsed since this provider run started' source.
- transient retry backoff: Same helper bounds the inter-attempt sleep so a laptop sleep doesn't deceive the retry loop into running indefinitely.

## Risks

- max() of wall and monotonic produces unexpectedly large jumps after sleep, firing the watchdog earlier than the operator expects.
- Wall-clock at process start is recorded before monotonic; clock skew could make elapsed() briefly negative on first call.
- Tests using mocked time leak side effects into other tests.

## Acceptance

Profile: standard

Definition of done:
- [ ] `dod1` WallClockMonotonic.elapsed() returns max(wall_elapsed, monotonic_elapsed) under all mocked clock combinations: synchronized, wall-jumps-forward (suspend), wall-rewinds (NTP), monotonic-stalls (suspend on macOS).
- [ ] `dod2` _provider_watchdog uses the helper exclusively; no direct time.monotonic() reads remain in the watchdog body.
- [ ] `dod3` Transient-retry backoff in run_external_review uses the helper to bound the sleep, so a suspend during backoff doesn't extend it beyond the configured cap.
- [ ] `dod4` Unit test simulating a 2-hour wall-clock jump while monotonic stays flat asserts the watchdog fires on resume (deadline already elapsed).
- [ ] `dod5` Unit test simulating a 30-second NTP rewind asserts the watchdog still fires when monotonic crosses the deadline.
- [ ] `dod6` Operator doc explains the two-clock model and that suspend counts toward the deadline.

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
- [ ] `v2` test - Watchdog-clock unit tests pass.
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` boundary - WallClockMonotonic class exists and is importable.
  - Command: `python3 -c 'from scafld.review_runner import WallClockMonotonic; c = WallClockMonotonic(); assert c.elapsed() >= 0'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: WallClockMonotonic helper

Goal: Add the helper class with elapsed() returning max(wall_elapsed, monotonic_elapsed). Pure unit-testable; no callers yet.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` compile - Sources compile after helper added.
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` test - WallClockMonotonic unit tests pass.
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_3` boundary - Class is importable and elapsed() returns a non-negative float.
  - Command: `python3 -c 'from scafld.review_runner import WallClockMonotonic; c = WallClockMonotonic(); assert c.elapsed() >= 0'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Watchdog rewired to use the helper

Goal: Replace the direct time.monotonic() reads in _provider_watchdog with the helper. Behavior under non-suspend conditions is unchanged.

Status: completed
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` compile - Sources compile after watchdog rewire.
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` test - Watchdog and helper unit tests pass.
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Transient-retry backoff respects suspend

Goal: The exponential backoff sleep in `run_external_review` between transient retries also uses the helper, so a suspend during backoff doesn't silently extend the wait.

Status: completed
Dependencies: phase2

Changes:
- none

Acceptance:
- [x] `ac3_1` test - Backoff tests pass.
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` boundary - Operator docs describe the suspend-aware deadline.
  - Command: `grep -qiE 'suspend-aware|laptop sleep|watchdog deadline' docs/review.md`
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

- 2026-04-28T00:46:03Z - user - Spec created via scafld plan after a laptop sleep mid-review revealed the macOS CLOCK_UPTIME_RAW failure mode in the existing watchdog deadline.
- 2026-04-28T00:46:03Z - agent - Replaced placeholder with the suspend-aware watchdog clock plan: WallClockMonotonic helper, rewire watchdog and retry backoff, mocked-clock tests for both failure modes.
- 2026-04-28T00:50:23Z - cli - Spec approved
- 2026-04-28T00:51:06Z - cli - Execution started
- 2026-04-28T01:33:05Z - cli - Spec cancelled
- 2026-04-28T01:33:05Z - cli - Spec cancelled: scoped down to a 6-line inline fix in _provider_watchdog; no spec ceremony needed
