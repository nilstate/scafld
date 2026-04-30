---
spec_version: '2.0'
task_id: challenger-review-signal-lift
created: '2026-04-24T06:10:00Z'
updated: '2026-04-24T01:07:14Z'
status: completed
harden_status: passed
size: large
risk_level: high
---

# Make Challenger Review Catch More Meaningful Defects

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

Push the adversarial review gate past posture and into practical value. The challenger prompt, review artifact contract, and review-quality checks should work together so review catches meaningful defects often enough that you expect it to matter. This is the highest-leverage craft surface in the product: if review does not reliably produce grounded, useful criticism, scafld does not earn its identity.

## Context

CWD: `.`

Packages:
- `.ai/prompts/review.md`
- `scafld/review_workflow.py`
- `scafld/reviewing.py`
- `scafld/commands/review.py`
- `docs/review.md`
- `docs/cli-reference.md`
- `tests/`

Files impacted:
- `.ai/prompts/review.md` (all) - The challenger prompt needs stricter adversarial craft and evidence requirements.
- `scafld/review_workflow.py` (all) - Review orchestration should enforce the stronger prompt contract and artifact expectations.
- `scafld/reviewing.py` (all) - Review parsing and validation should detect low-signal or malformed challenger output.
- `scafld/commands/review.py` (all) - review output should surface evidence quality and clear blocker structure.
- `docs/review.md` (all) - Docs should explain what a good challenger review must contain.
- `docs/cli-reference.md` (all) - CLI docs should describe the stronger review artifact contract.
- `tests/review_gate_smoke.sh` (all) - Existing review gate coverage must stay compatible.
- `tests/challenge_override_smoke.sh` (all) - Override behavior should remain audited even with stricter review quality rules.
- `tests/review_prompt_contract_smoke.sh` (all) - New smoke should assert the prompt contract and expected review sections.
- `tests/review_signal_corpus_smoke.sh` (all) - New smoke should exercise clean and flawed review fixtures against the stronger contract.

Invariants:
- `domain_boundaries`
- `public_api_stable`
- `no_test_logic_in_production`

Related docs:
- `README.md`
- `docs/review.md`
- `AGENTS.md`

## Objectives

- Make the challenger prompt explicitly adversarial, evidence-based, and blocker-oriented.
- Require file/line grounding and clear blocker vs non-blocker separation in review artifacts.
- Reject or flag low-signal reviews that lack evidence or meaningful findings structure.
- Create a small clean/flawed review corpus so prompt changes can be dogfooded instead of guessed.
- Move scafld toward the real standard that review catches meaningful defects often enough that you expect it to matter.

## Scope



## Dependencies

- Assumes the review gate and challenger role are already the product center.

## Assumptions

- Stricter structure helps both humans and wrappers decide whether a review is meaningful.
- Review quality should be judged from grounded evidence, not from review length.

## Touchpoints

- challenger prompt: The prompt must attack the implementation instead of politely confirming it.
- artifact contract: Review artifacts should be parseable, grounded, and hard to game.
- override honesty: Human override remains the backstop for strong but incorrect challenger calls.
- review corpus: Prompt tuning should use explicit flawed and clean fixtures instead of intuition.

## Risks

- A stronger prompt can raise false positives if blocker criteria stay vague.
- Artifact quality checks can become ceremony if they only enforce formatting.
- Prompt tuning can sprawl into a parallel subsystem.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` The challenger prompt requires explicit evidence, severity, and blocker/non-blocker separation.
- [ ] `dod2` Review parsing detects low-signal output that lacks citations or meaningful finding structure.
- [ ] `dod3` A clean/flawed review corpus exists so prompt changes can be tested deliberately.
- [ ] `dod4` Review gate and override smokes stay green under the stronger contract.

Validation:
- [ ] `v1` custom - Validate this draft spec.
  - Command: `python3 cli/scafld validate challenger-review-signal-lift`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` compile - Compile scafld after the review-quality work.
  - Command: `python3 -m compileall scafld`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test - Run review gate and override regressions.
  - Command: `bash tests/review_gate_smoke.sh && bash tests/challenge_override_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` test - Run review prompt and review-signal corpus coverage.
  - Command: `bash tests/review_prompt_contract_smoke.sh && bash tests/review_signal_corpus_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Strengthen the challenger contract

Goal: Upgrade the review prompt and artifact expectations to demand grounded, adversarial findings.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` test - Prompt contract and review parsing smoke passes.
  - Command: `bash tests/review_prompt_contract_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Tune against explicit fixtures

Goal: Create a small clean/flawed corpus so review quality can be tested deliberately.

Status: completed
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` test - Review corpus and gate regressions stay green.
  - Command: `bash tests/review_signal_corpus_smoke.sh && bash tests/review_gate_smoke.sh && bash tests/challenge_override_smoke.sh`
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
Timestamp: '2026-04-24T01:07:14Z'
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

- 2026-04-24T06:10:00Z - assistant - Drafted the challenger-review quality slice against the real-standard bar.
- 2026-04-24T01:03:22Z - cli - Spec approved
- 2026-04-24T01:03:37Z - cli - Execution started
- 2026-04-24T01:07:14Z - cli - Spec completed
