---
spec_version: '2.0'
task_id: exec-surface-hard-cut
created: '2026-04-25T09:00:28Z'
updated: '2026-04-25T09:05:19Z'
status: completed
harden_status: in_progress
size: small
risk_level: medium
---

# Make exec a first-class CLI surface

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

Hard-cut the CLI surface so the default help, the docs, and the actual command registry tell one story. Today `scafld exec` is real and documented, but it is hidden from the normal help surface in `scafld/commands/surface.py`. At the same time, a hidden `execute` alias still exists only for compatibility, and the smoke suite is split between protecting the hidden surface and the documented one. That is not a product I would want to use. The clean model is: `exec` is a first-class public command, `execute` is removed, and the help/package/runx surface tests all assert the same contract.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`
- `docs`

Files impacted:
- `scafld/commands/surface.py` (task-selected) - The command registry currently hides `exec` and still exposes the `execute` compatibility alias.
- `tests/package_smoke.sh` (task-selected) - Installed-wheel help expectations currently assert that default help hides exec.
- `tests/agent_surface_smoke.sh` (task-selected) - Agent-surface help expectations still assert that exec is hidden.
- `tests/runx_surface_smoke.sh` (task-selected) - Runx surface smoke still protects the deprecated execute alias.
- `docs/cli-reference.md` (task-selected) - Docs already present exec as public; verify they stay aligned after the hard cut.

Invariants:
- `default_help_must_match_the_real_public_surface`
- `one_canonical_command_per_user_facing_action`
- `no_compatibility_alias_without_a_live_product_reason`

Related docs:
- `README.md`
- `docs/cli-reference.md`
- `docs/lifecycle.md`

## Objectives

- Make `scafld exec` visible on the default help surface.
- Remove the `execute` compatibility alias from the CLI registry.
- Update help/package/runx smokes to assert the same public contract.
- Keep the docs aligned to the hard-cut surface.

## Scope



## Dependencies

- None.

## Assumptions

- The docs are already describing the desired surface more accurately than the current help output.
- No supported wrapper should still depend on the `execute` spelling once the hard cut lands.

## Touchpoints

- public CLI surface: Default help should advertise the commands a user is actually expected to run.
- compatibility cleanup: The old alias should disappear instead of lingering behind hidden switches and stale tests.
- surface verification: Package, agent, and runx smokes should all assert the same public command contract.

## Risks

- Removing the alias could break an internal smoke or wrapper that still shells out to `scafld execute`.
- Making exec public without updating tests would leave the product surface split-brained.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` Default help exposes exec as a public command.
- [ ] `dod2` The execute alias is removed from the live CLI registry.
- [ ] `dod3` Package, agent, and runx surface smokes all assert the new hard-cut surface.
- [ ] `dod4` Docs remain aligned to the public exec surface after the cut.

Validation:
- [ ] `v1` compile - Compile the Python sources after the CLI surface change.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Package smoke reflects the public exec surface.
  - Command: `bash tests/package_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - Agent surface smoke reflects the public exec surface.
  - Command: `bash tests/agent_surface_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` integration - Runx surface smoke uses the canonical exec command only.
  - Command: `bash tests/runx_surface_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Make exec public and remove the alias

Goal: Align the live command registry with the documented public surface.

Status: completed
Dependencies: none

Changes:
- `scafld/commands/surface.py` (all) - Promote exec onto the public help surface and remove the execute compatibility alias from the command registry.
- `docs/cli-reference.md` (all) - Verify and adjust any wording that still implies exec is hidden or legacy.

Acceptance:
- [x] `ac1_1` integration - Default help exposes exec and the execute alias no longer resolves.
  - Command: `bash -lc "./cli/scafld --help | grep -q 'exec' && ! ./cli/scafld --help | grep -q 'execute' && ! ./cli/scafld execute --help >/dev/null 2>&1"`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Lock the public surface with smokes

Goal: Make the help/package/runx tests assert the hard-cut surface consistently.

Status: completed
Dependencies: phase1

Changes:
- `tests/package_smoke.sh` (all) - Update the installed-wheel help assertion to require exec on the default surface.
- `tests/agent_surface_smoke.sh` (all) - Update default and advanced help assertions to treat exec as a public command and keep truly internal commands hidden.
- `tests/runx_surface_smoke.sh` (all) - Replace the execute alias check with a canonical exec-only expectation, and assert the old alias is gone.

Acceptance:
- [x] `ac2_1` integration - Package smoke reflects the public exec surface.
  - Command: `bash tests/package_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` integration - Agent surface smoke reflects the public exec surface.
  - Command: `bash tests/agent_surface_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_3` integration - Runx surface smoke uses the canonical exec command only.
  - Command: `bash tests/runx_surface_smoke.sh`
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
Timestamp: '2026-04-25T09:05:40Z'
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

- 2026-04-25T09:00:28Z - user - Spec created via scafld plan
- 2026-04-25T09:02:40Z - assistant - Replaced the placeholder draft with a hard-cut spec for making exec public and removing the lingering execute compatibility alias.
- 2026-04-25T09:01:51Z - cli - Spec approved
- 2026-04-25T09:03:25Z - assistant - Tightened phase1 acceptance so it proves the real help surface changes instead of only compiling the registry edits.
- 2026-04-25T09:02:26Z - cli - Execution started
- 2026-04-25T09:05:19Z - cli - Spec completed
