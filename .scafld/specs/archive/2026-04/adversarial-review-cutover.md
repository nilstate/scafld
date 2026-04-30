---
spec_version: '2.0'
task_id: adversarial-review-cutover
created: '2026-04-23T14:03:11Z'
updated: '2026-04-24T01:11:28Z'
status: cancelled
harden_status: passed
size: large
risk_level: high
---

# Adversarial Review Cutover

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

Re-center scafld around adversarial review as the product identity. Preserve `spec` and `session` as the only primitives, demote `handoff` to transport, concentrate challenge at the `review` gate, and slim the default agent-facing surface to the workflow verbs that matter. The implementation should leave the code shape cleaner than the current cutover by keeping review challenge in the existing handoff/session substrate rather than spawning parallel subsystems.

## Context

CWD: `.`

Packages:
- `scafld/`
- `docs/`
- `.ai/`
- `tests/`
- `plans/`

Files impacted:
- `plans/llm-performance-cutover.md` (all) - Canonical cutover plan must reflect the final adversarial-review model.
- `README.md` (1-260) - Product identity, lifecycle story, and default command surface must change.
- `docs/review.md` (all) - Review becomes the challenger hero gate and must be documented that way.
- `docs/execution.md` (all) - Execution docs should present build as the long-running path into review, not the hero feature.
- `docs/cli-reference.md` (all) - Default help and documented command surface need the 10-verb model plus advanced legacy coverage.
- `docs/configuration.md` (all) - Config docs should stay minimal while naming challenge-specific metrics and constraints.
- `docs/run-artifacts.md` (all) - Transport and session responsibilities change from kind-centric to role×gate.
- `AGENTS.md` (1-260) - Agent guidance must teach the slim surface and adversarial review identity.
- `CLAUDE.md` (1-220) - Claude-specific integration notes should mirror the new workflow vocabulary.
- `.ai/README.md` (all) - Workspace guide must explain spec/session primitives and handoff transport.
- `.ai/OPERATORS.md` (all) - Operator guide must teach the new verbs and review gate semantics.
- `.ai/prompts/plan.md` (all) - Plan template should teach the new wrapper surface.
- `.ai/prompts/exec.md` (all) - Execution template should support build-to-review flow without overclaiming challenge.
- `.ai/prompts/review.md` (all) - Review template must become explicitly adversarial.
- `scafld/runtime_contracts.py` (all) - Own role×gate transport contracts, schema versions, and archive pathing.
- `scafld/handoff_renderer.py` (all) - One renderer should emit sibling md/json handoffs with role×gate tags.
- `scafld/session_store.py` (all) - Session must record typed entries for attempts, summaries, challenge verdicts, approvals, and overrides.
- `scafld/review_workflow.py` (all) - Review orchestration owns challenger handoff generation and verdict capture.
- `scafld/reviewing.py` (all) - Review artifacts must reference challenge history without breaking Review Artifact v3.
- `scafld/commands/review.py` (all) - review and complete must gate on challenger verdicts and audited overrides.
- `scafld/commands/reporting.py` (all) - report should headline first_attempt_pass_rate, recovery_convergence_rate, and challenge_override_rate.
- `scafld/commands/surface.py` (all) - CLI surface should default to the slim agent-facing verb set and move legacy commands to advanced help.
- `scafld/commands/workflow.py` (all) - New wrapper command module for plan/build keeps orchestration out of the low-level command modules.
- `scafld/commands/execution.py` (all) - build wrappers and review handoff expectations still depend on clean execution/session behavior.
- `tests/run_contracts_smoke.sh` (all) - Transport tests should assert role×gate metadata and sibling artifacts.
- `tests/session_recovery_smoke.sh` (all) - Session smoke should cover typed event recording and recovery counters.
- `tests/review_handoff_smoke.sh` (all) - Review handoff tests should become challenger-handoff tests.
- `tests/review_gate_smoke.sh` (all) - Review gate semantics must stay compatible while using challenger verdicts.
- `tests/report_metrics_smoke.sh` (all) - Report metrics must include challenge_override_rate.
- `tests/agent_surface_smoke.sh` (all) - New smoke test for the agent-facing verb surface and help output.
- `tests/package_smoke.sh` (all) - New modules, prompts, and docs must remain packaged.
- `tests/update_smoke.sh` (all) - Managed bundle refresh must keep the slim prompt and help surface synchronized.

Invariants:
- `domain_boundaries`
- `public_api_stable`
- `config_from_env`
- `no_test_logic_in_production`

Related docs:
- `plans/llm-performance-cutover.md`
- `README.md`
- `docs/review.md`
- `docs/run-artifacts.md`
- `AGENTS.md`

## Objectives

- Reposition scafld as long-running AI coding work under adversarial review.
- Keep `spec` and `session` as the only primitives.
- Make `handoff` transport only, tagged by `role × gate`, emitted as sibling `.md/.json` files.
- Fire challenge at `review` only in v1 and record challenger verdicts plus human overrides in session.
- Expose the compact agent-facing command surface: plan, approve, build, review, complete, status, list, report, handoff, update.
- Keep legacy commands working and discoverable through advanced help and compatibility paths.
- Keep v1 at zero spec schema changes.

## Scope



## Dependencies

- Builds on the existing handoff/session runtime already landed in the workspace.

## Assumptions

- The current lifecycle remains the only underlying state machine.
- Review is the one meaningful adversarial gate in v1.
- The challenger verdict can be recorded in session and surfaced through Review Artifact v3 without a schema migration.
- Wrapper verbs can orchestrate existing low-level commands instead of replacing them immediately.

## Touchpoints

- Identity: README, plans, prompts, and operator docs must tell one adversarial-review story.
- Transport: handoff metadata and file layout must be role×gate-based and remain immutable.
- Session ledger: session must become the only durable source for challenge verdicts and overrides.
- Review gate: review must become the hero gate while complete remains the audited closeout path.
- CLI surface: default help and prompts must teach the slim verb surface without breaking scripts.
- Reporting: report must surface challenge_override_rate next to the existing quality metrics.

## Risks

- A weak review prompt could make challenge ceremonial rather than adversarial.
- CLI slimming can confuse existing users if advanced help is not obvious.
- Role×gate transport can drift from existing kind-based behavior.
- Challenge_override_rate can overclaim causality.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` The cutover plan and repo docs consistently frame scafld as adversarial review over long-running AI coding work.
- [ ] `dod2` handoff transport is role×gate tagged, emitted as sibling `.md/.json` pairs, and remains immutable.
- [ ] `dod3` session records typed entries for attempts, phase summaries, challenger verdicts, approvals, and human overrides.
- [ ] `dod4` review emits a challenger-role handoff and complete gates on the challenger verdict unless a human override is recorded.
- [ ] `dod5` The default agent-facing help and prompt surface teaches the 10 workflow verbs while legacy commands remain available.
- [ ] `dod6` report shows first_attempt_pass_rate, recovery_convergence_rate, and challenge_override_rate.

Validation:
- [ ] `v1` custom - Validate this draft spec.
  - Command: `python3 cli/scafld validate adversarial-review-cutover`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` compile - Compile the scafld package after the refactor.
  - Command: `python3 -m compileall scafld`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test - Run transport and session smoke coverage.
  - Command: `bash tests/run_contracts_smoke.sh && bash tests/session_recovery_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` test - Run review gate and challenger handoff coverage.
  - Command: `bash tests/review_handoff_smoke.sh && bash tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v5` test - Run report and agent-surface coverage.
  - Command: `bash tests/report_metrics_smoke.sh && bash tests/agent_surface_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v6` test - Run package and managed-bundle coverage.
  - Command: `bash tests/package_smoke.sh && bash tests/update_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Identity and docs

Goal: Rewrite the plan, docs, prompts, and operator guidance around adversarial review as the core identity.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [ ] `ac1_1` documentation - Cutover plan and docs consistently use adversarial-review framing.
  - Command: `bash -lc 'rg -qi "adversarial review|unchallenged|challenge_override_rate" plans/llm-performance-cutover.md README.md docs/review.md AGENTS.md .ai/README.md .ai/OPERATORS.md'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_2` documentation - Prompt docs teach the plan/build/review vocabulary and treat review as adversarial.
  - Command: `bash -lc 'rg -qi "plan|build|review" .ai/prompts/plan.md .ai/prompts/exec.md && rg -qi "adversarial|challenger|falsify" .ai/prompts/review.md'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Transport and ledger cleanup

Goal: Refactor the current handoff/session implementation into clean role×gate transport plus typed session entries.

Status: pending
Dependencies: phase1

Changes:
- none

Acceptance:
- [ ] `ac2_1` compile - scafld compiles after the transport and ledger refactor.
  - Command: `python3 -m compileall scafld`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac2_2` test - Transport and session smoke tests pass.
  - Command: `bash tests/run_contracts_smoke.sh && bash tests/session_recovery_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Review challenger gate

Goal: Make review the single challenge gate and record its disposition in session.

Status: pending
Dependencies: phase2

Changes:
- none

Acceptance:
- [ ] `ac3_1` test - Review handoff and review gate smoke tests pass.
  - Command: `bash tests/review_handoff_smoke.sh && bash tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 4: Agent surface and compatibility

Goal: Expose the compact workflow verbs while preserving the existing command surface for scripts and advanced users.

Status: pending
Dependencies: phase3

Changes:
- none

Acceptance:
- [ ] `ac4_1` test - Agent-surface and managed-bundle smoke tests pass.
  - Command: `bash tests/agent_surface_smoke.sh && bash tests/update_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 5: Reporting and packaging

Goal: Expose the final metrics and keep the repo distributable after the cutover.

Status: pending
Dependencies: phase4

Changes:
- none

Acceptance:
- [ ] `ac5_1` test - Report, package, and lifecycle smoke tests pass.
  - Command: `bash tests/report_metrics_smoke.sh && bash tests/package_smoke.sh && bash tests/lifecycle_smoke.sh`
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

Estimated effort hours: 20.0
Actual effort hours: none
AI model: unspecified
React cycles: none

Tags:
- adversarial-review
- session
- transport
- review-gate
- cli

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

- 2026-04-23T14:01:58Z - user - Spec created via scafld new
- 2026-04-23T14:03:11Z - agent - Replaced the placeholder draft with the final adversarial-review cutover shape.
- 2026-04-24T01:10:44Z - cli - Spec approved
- 2026-04-24T01:11:28Z - cli - Execution started
- 2026-04-24T01:11:28Z - cli - Spec cancelled
