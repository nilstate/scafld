---
spec_version: '2.0'
task_id: default-handoff-consumption
created: '2026-04-24T06:10:00Z'
updated: '2026-04-24T01:07:26Z'
status: completed
harden_status: passed
size: large
risk_level: medium
---

# Make Handoff Consumption The Default Path

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

Close the biggest practical gap in the control-plane story: a great handoff does nothing if the agent ignores it. Ship thin first-party integration paths so Codex and Claude Code consume the current handoff by default, while keeping scafld itself provider-neutral. The work only counts if the default invocation path actually reads the handoff before the model acts.

## Context

CWD: `.`

Packages:
- `scafld/commands/workflow.py`
- `scafld/commands/handoff.py`
- `scafld/commands/lifecycle.py`
- `runtime bundle`
- `AGENTS.md`
- `CLAUDE.md`
- `docs/`
- `scripts/`
- `tests/`

Files impacted:
- `scafld/commands/workflow.py` (all) - build and review JSON should expose the canonical handoff and next action cleanly for adapters.
- `scafld/commands/handoff.py` (all) - handoff should remain the stable escape hatch adapters call directly.
- `scafld/commands/lifecycle.py` (1-260) - status may need to expose the same current handoff information adapters rely on.
- `scafld/runtime_bundle.py` (all) - Managed workspace assets may need to ship default adapter scripts or integration docs.
- `AGENTS.md` (1-220) - Codex guidance should consume handoff first by default.
- `CLAUDE.md` (1-220) - Claude Code guidance should consume handoff first by default.
- `docs/integrations.md` (all) - New integration doc should explain the default adapter paths.
- `scripts/scafld-codex-build.sh` (all) - Thin Codex adapter should fetch and pass the current handoff.
- `scripts/scafld-claude-build.sh` (all) - Thin Claude Code adapter should fetch and pass the current handoff.
- `tests/codex_handoff_adapter_smoke.sh` (all) - New smoke should prove the Codex path consumes handoff by default.
- `tests/claude_handoff_adapter_smoke.sh` (all) - New smoke should prove the Claude Code path consumes handoff by default.
- `tests/prompt_precedence_smoke.sh` (all) - Prompt precedence still matters once adapters rely on the active template layer.

Invariants:
- `domain_boundaries`
- `config_from_env`
- `public_api_stable`

Related docs:
- `AGENTS.md`
- `CLAUDE.md`
- `docs/cli-reference.md`

## Objectives

- Ship a thin Codex adapter that consumes the current handoff before acting.
- Ship a thin Claude Code adapter that consumes the current handoff before acting.
- Expose canonical handoff and next-action metadata in the existing JSON envelopes adapters rely on.
- Teach the adapter paths in AGENTS, CLAUDE, and integration docs.
- Move scafld toward the real standard that the agent actually consumes the handoff by default.

## Scope



## Dependencies

- Assumes role×gate handoffs and the slim public surface are already stable.

## Assumptions

- scafld should remain provider-neutral even while shipping first-party adapter examples.
- Adapters should be thin wrappers over existing JSON and handoff commands.

## Touchpoints

- json contracts: Adapters should not scrape human output for handoff paths or next actions.
- managed workspace: init and update should make the default integration path discoverable.
- agent guidance: Codex and Claude should both be taught to read handoff first.
- provider boundary: Core runtime stays provider-neutral even while examples are first-party.

## Risks

- Adapter scripts can sprawl into a second runtime surface.
- Provider-specific assumptions can leak into scafld core.
- Adapters can silently drift from the JSON contract.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` Thin Codex and Claude adapter paths consume the current handoff by default.
- [ ] `dod2` Core JSON envelopes expose the canonical handoff and next-action data adapters rely on.
- [ ] `dod3` Managed docs and workspace assets teach the adapter paths without changing the public command set.
- [ ] `dod4` Adapter smokes keep handoff consumption honest.

Validation:
- [ ] `v1` custom - Validate this draft spec.
  - Command: `python3 cli/scafld validate default-handoff-consumption`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` compile - Compile scafld after adapter-support changes.
  - Command: `python3 -m compileall scafld`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test - Run adapter smokes.
  - Command: `bash tests/codex_handoff_adapter_smoke.sh && bash tests/claude_handoff_adapter_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` test - Keep active prompt precedence covered.
  - Command: `bash tests/prompt_precedence_smoke.sh && bash tests/update_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Canonical adapter inputs

Goal: Expose stable handoff and next-action metadata in the existing command envelopes.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Adapter inputs are exposed through canonical JSON fields.
  - Command: `bash tests/codex_handoff_adapter_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Ship thin adapter paths

Goal: Provide default Codex and Claude paths that consume handoff before model execution.

Status: completed
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` test - Codex and Claude adapter smokes pass.
  - Command: `bash tests/codex_handoff_adapter_smoke.sh && bash tests/claude_handoff_adapter_smoke.sh`
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
Verdict: incomplete
Timestamp: '2026-04-24T01:07:25Z'
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

- 2026-04-24T06:10:00Z - assistant - Drafted the default-handoff-consumption slice against the real-standard bar.
- 2026-04-24T01:03:22Z - cli - Spec approved
- 2026-04-24T01:03:37Z - cli - Execution started
- 2026-04-24T01:07:26Z - cli - Spec completed
