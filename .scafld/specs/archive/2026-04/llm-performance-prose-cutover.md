---
spec_version: '2.0'
task_id: llm-performance-prose-cutover
created: '2026-04-23T08:49:58Z'
updated: '2026-04-23T12:55:41Z'
status: completed
harden_status: not_run
size: medium
risk_level: medium
---

# LLM Performance Prose Cutover

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

Reframe scafld around the clean `spec / handoff / session` model without changing the lifecycle. This spec updates the roadmap, README, docs, operator guidance, and prompt docs so the repo consistently describes handoff as compiled model input, session as the run ledger, the handoff file pair as `*.md + *.json`, and report as the only honest attribution surface in an agent-agnostic system.

## Context

CWD: `.`

Packages:
- `README.md`
- `docs/`
- `.ai/`
- `AGENTS.md`
- `CLAUDE.md`
- `plans/`

Files impacted:
- `plans/llm-performance-cutover.md` (all) - Canonical forward model for the cutover.
- `README.md` (1-340) - Primary mission and product model need to change.
- `docs/introduction.md` (all) - Intro framing must become brownfield governed execution, not loose one-shot framing.
- `docs/execution.md` (all) - Execution docs must explain handoff generation and session compaction.
- `docs/review.md` (all) - Review docs must explain review handoff and attribution limits.
- `docs/configuration.md` (all) - Config docs must describe only the minimal v1 llm block.
- `docs/cli-reference.md` (all) - CLI docs must describe one new read-only handoff command.
- `docs/run-artifacts.md` (all) - New doc for handoff/session contracts and .ai/runs layout.
- `AGENTS.md` (1-220) - Agent guidance must explain prompt templates, handoffs, and session compaction.
- `CLAUDE.md` (1-80) - Claude-specific guidance must mirror the new model.
- `.ai/README.md` (all) - Workspace guide must explain spec, handoff, session, and review.
- `.ai/OPERATORS.md` (all) - Operators need the clean mental model and the single new handoff command.
- `.ai/prompts/plan.md` (all) - Plan prompt must be described as a renderer template.
- `.ai/prompts/exec.md` (all) - Exec prompt must be described as the phase handoff template.
- `.ai/prompts/review.md` (all) - Review prompt must be described as the review handoff template.
- `.ai/prompts/recovery.md` (all) - New recovery handoff template must be introduced in the docs.
- `.ai/prompts/harden.md` (all) - Harden should be documented as the obvious next handoff kind, not a parallel model.

Invariants:
- `domain_boundaries`
- `public_api_stable`
- `config_from_env`

Related docs:
- `plans/llm-performance-cutover.md`
- `docs/execution.md`
- `docs/review.md`
- `docs/run-artifacts.md`

## Objectives

- Lock the mission line: scafld is the control plane that gets the most out of your LLM during governed execution.
- Explain `spec / handoff / session` as the full value layer and keep the lifecycle untouched.
- Make `.ai/prompts/*.md` subordinate templates for handoff rendering.
- Name the canonical success metrics: first_attempt_pass_rate and recovery_convergence_rate.
- State v1 compatibility rules: zero spec schema changes, one-way handoffs, and archived run retention.
- State the honest agent-agnostic boundary: the handoff can be ignored, so report metrics are the only attribution surface.
- Name harden as the next natural handoff kind without building it in v1.

## Scope



## Dependencies

- None. This is the framing spec for the rest of the cutover.

## Assumptions

- The existing lifecycle must remain the only workflow users learn.
- The first implementation adds one public handoff command and keeps everything else under existing commands.

## Touchpoints

- Mission: README and intro docs explain scafld as a control plane for governed brownfield execution.
- Runtime model: Docs explain spec, handoff, session, review, report, and archived run retention without growing extra primitives.
- Prompt hierarchy: Prompt files are described as renderer templates, not parallel artifacts.
- Contract details: Docs state the sibling handoff `.md/.json` format, recovery exhaustion behavior, and zero spec schema changes.
- Attribution boundary: Docs explicitly say the external agent may ignore a handoff and report can only infer quality lift from outcomes.

## Risks

- Docs may still imply scafld is a provider runtime.
- Older wording around one-shot building may survive and muddy the mission.

## Acceptance

Profile: standard

Definition of done:
- [ ] `dod1` README, docs, operator docs, and prompt docs consistently describe the `spec / handoff / session` model.
- [ ] `dod2` Prompt files are documented as templates rendered into handoffs.
- [ ] `dod3` Docs explicitly place phase-boundary compaction in `session.phase_summaries`.
- [ ] `dod4` Docs explicitly state that the external agent may ignore the handoff and that report metrics are the attribution proxy.
- [ ] `dod5` Docs explicitly state zero spec schema changes, sibling `.md/.json` handoffs, recovery exhaustion, and archived run retention.

Validation:
- [ ] `v1` custom - Validate this draft spec.
  - Command: `python3 cli/scafld validate llm-performance-prose-cutover`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` documentation - Confirm the roadmap and docs use the locked mission line and core nouns.
  - Command: `bash -lc 'rg -qi "control plane that gets the most out of your LLM during governed execution" plans/llm-performance-cutover.md README.md && for term in "handoff" "session" "phase_summaries" "may ignore the handoff" "zero spec schema changes" "first_attempt_pass_rate" "recovery_convergence_rate" "failed_exhausted" "phase-phase1.json"; do rg -qi "$term" plans/llm-performance-cutover.md README.md docs AGENTS.md CLAUDE.md .ai/README.md .ai/OPERATORS.md || exit 1; done'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` documentation - Confirm prompt docs describe templates rather than standalone prompts.
  - Command: `bash -lc 'for term in "template" "handoff"; do rg -qi "$term" .ai/prompts/plan.md .ai/prompts/exec.md .ai/prompts/review.md .ai/prompts/harden.md || exit 1; done'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Roadmap and mission line

Goal: Rewrite the roadmap and README around the locked runtime model.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` documentation - Roadmap and README carry the new mission line.
  - Command: `bash -lc 'rg -qi "control plane that gets the most out of your LLM during governed execution" plans/llm-performance-cutover.md README.md'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Docs and operator model

Goal: Propagate the model and attribution rules through docs and operator guidance.

Status: pending
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` documentation - Docs and operator docs describe session as the only durable run ledger.
  - Command: `bash -lc 'rg -qi "only durable run ledger" docs .ai/README.md .ai/OPERATORS.md'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Prompt docs as templates

Goal: Subordinate prompt docs to handoff rendering and name harden as a future kind.

Status: pending
Dependencies: phase2

Changes:
- none

Acceptance:
- [x] `ac3_1` documentation - Prompt files are documented as templates and harden is named as a future handoff kind.
  - Command: `bash -lc 'rg -qi "template" .ai/prompts/plan.md .ai/prompts/exec.md .ai/prompts/review.md && rg -qi "next handoff kind|future handoff kind" .ai/prompts/harden.md'`
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
Timestamp: '2026-04-23T12:55:41Z'
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

Estimated effort hours: 6.0
Actual effort hours: none
AI model: unspecified
React cycles: none

Tags:
- llm-performance
- docs
- handoff-session

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

- 2026-04-23T08:49:58Z - user - Requested a clean forward model and draft specs for the end-to-end cutover.
- 2026-04-23T08:49:58Z - agent - Rewrote the prose cutover around spec, handoff, session, prompt templates, and the attribution boundary.
- 2026-04-23T12:48:27Z - cli - Spec approved
- 2026-04-23T12:48:27Z - cli - Execution started
- 2026-04-23T12:55:41Z - cli - Spec completed
