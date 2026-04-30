---
spec_version: '2.0'
task_id: harden-external-review-contract
created: '2026-04-25T14:44:29Z'
updated: '2026-04-26T06:43:29Z'
status: completed
harden_status: not_run
size: large
risk_level: high
---

# Harden external review into a trustworthy gate

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

The external review runner is structurally useful, but its current contract is too easy to game. It accepts shape-valid JSON with weak evidence, records model-self-reported provenance, has no subprocess timeout, treats Codex and Claude isolation as equivalent, and exposes misleading review metadata in some command paths. This task turns the runner from a rigorous-looking wrapper into a trustable review gate by making scafld own observed facts, keeping findings in prose, validating the rendered review before completion is suggested, and clearly surfacing isolation and model attribution limits.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`
- `docs`

Files impacted:
- `scafld/review_runner.py` (all) - Core boundary for runner config, provider resolution, prompt wrapping, subprocess execution, output normalization, timeout handling, hashing, provenance, and provider isolation metadata.
- `scafld/review_workflow.py` (all) - Writes completed review rounds from runner output and must validate the canonical review artifact before replacing the in-progress round.
- `scafld/reviewing.py` (all) - Parser/gate quality rules must reject filler clean sections and keep finding format validation strict after the runner contract changes.
- `scafld/commands/review.py` (all) - User-facing review command must report real runner status, fail on invalid external output, honor flag overrides in JSON mode, and avoid suggesting completion after a malformed runner result.
- `scafld/review_runtime.py` (all) - Review snapshots need runner override awareness and should expose control-plane runner metadata without claiming that JSON mode spawned a reviewer.
- `scafld/commands/surface.py` (all) - CLI flags may need timeout/isolation related help and JSON-mode override semantics documented through the command surface.
- `scafld/handoff_renderer.py` (all) - Generated challenger handoffs carry spec-derived content and may need explicit untrusted-input boundaries or structured section metadata.
- `.ai/prompts/review.md` (all) - The challenger template must teach the evidence contract, clean-section rules, and untrusted spec/diff boundary in the same language the parser enforces.
- `.ai/scafld/prompts/review.md` (all) - Managed runtime bundle copy must stay aligned with the canonical review prompt.
- `.ai/config.yaml` (all) - Review runner defaults need timeout, fallback policy, and isolation configuration fields.
- `.ai/specs/active/external-review-runner-hard-cut.yaml` (ownership metadata only) - Active-scope audit needs the predecessor runner spec to explicitly mark overlapping runner files as shared while this hardening spec supersedes and continues that work.
- `.ai/scafld/config.yaml` (all) - Managed runtime bundle copy must stay aligned when review runner config defaults change.
- `scafld/adapter_runtime.py` (all) - Optional provider adapters are the only place scafld can observe executor provider/model facts when it launches external agents.
- `scafld/session_store.py` (all) - Session entries may need typed provider/model telemetry for executor and challenger attribution.
- `scafld/execution_runtime.py` (all) - Native execution/session lifecycle may need to preserve observed executor metadata when wrapper-launched work flows through scafld.
- `scafld/commands/reporting.py` (all) - Reports should surface review trust signals, isolation downgrade counts, and known/unknown model separation instead of implying proof where none exists.
- `scafld/review_signal.py` (all) - Clean-review signal should distinguish evidence-backed clean sections from generic boilerplate.
- `docs/review.md` (all) - Operator-facing review docs must describe the hardened runner contract, evidence requirements, isolation caveats, and failure behavior.
- `docs/configuration.md` (all) - Review runner config docs must include timeout, auto fallback policy, provider model pins, and isolation metadata semantics.
- `docs/cli-reference.md` (all) - CLI docs must match review flag behavior, JSON mode, and timeout/error outcomes.
- `docs/integrations.md` (all) - Provider integration docs must be honest about Codex vs Claude isolation and wrapper attribution limits.
- `tests/review_gate_smoke.sh` (all) - Main smoke harness should prove hardened external review behavior, timeout handling, malformed output rejection, and fallback warnings.
- `tests/review_prompt_contract_smoke.sh` (all) - Prompt contract smoke should prove untrusted spec content is fenced and cannot be mistaken for runner instructions.
- `tests/review_signal_corpus_smoke.sh` (all) - Review signal fixtures should prove generic clean-section boilerplate is not counted as evidence.
- `tests/codex_handoff_adapter_smoke.sh` (all) - Optional Codex adapter smoke may need to record observed executor provider/model facts without changing the primary review runner.
- `tests/claude_handoff_adapter_smoke.sh` (all) - Optional Claude adapter smoke may need to record observed executor provider/model facts and document weaker isolation.
- `tests/package_smoke.sh` (all) - Package smoke may need fixture updates if managed prompt/config assets change.
- `tests/update_smoke.sh` (all) - Runtime bundle update smoke may need assertions for changed managed config/prompt assets.

Invariants:
- `review_artifact_stays_scafld_owned`
- `provenance_must_be_observed_not_model_claimed`
- `findings_stay_grounded_prose`
- `degraded_isolation_must_be_visible`
- `packaged_cli_is_the_truth_surface`

Related docs:
- `docs/review.md`
- `docs/configuration.md`
- `docs/cli-reference.md`
- `docs/integrations.md`

## Objectives

- Remove model-controlled provenance fields from the external review output contract; scafld must set reviewer mode, session, isolation, provider, model, timing, exit, timeout, and hash facts from observed runner state.
- Replace the broad verdict/blocking/non_blocking JSON envelope with a prose-first review body that is parsed by the same review artifact gate used for manual challenger rounds.
- Add external reviewer subprocess timeouts with configurable defaults and clear failure/fallback messaging.
- Fence spec-derived and handoff-derived prompt content as untrusted input so injected instructions in task descriptions cannot override runner rules.
- Make Codex and Claude isolation differences explicit in args, provenance, warnings, docs, and reports; do not silently treat Claude fallback as equivalent to the Codex sandbox.
- Ensure scafld validates the rendered completed review round before replacing the latest round or printing the next complete command.
- Record executor/challenger provider and model telemetry only when observed, and report unknown values as unknown rather than proof of model separation.

## Scope



## Dependencies

- None.

## Assumptions

- Codex remains the strongest supported external review provider because its non-interactive CLI supports read-only sandboxing and ephemeral execution.
- Claude can be made fresher with session and MCP/settings controls, but it still lacks a visible Codex-equivalent sandbox in the installed CLI surface.
- The existing markdown review parser is the right canonical gate; runner output should feed that parser instead of becoming a second trusted format.
- Shape validation can catch generic filler and malformed findings, but it cannot prove that a model performed a real review. The product should surface evidence limits honestly.

## Touchpoints

- review runner trust boundary: External provider output is untrusted. Scafld should parse it, render a canonical review round, validate that round, then persist only if the gate accepts the structure and evidence contract.
- review provenance: Provenance fields must be observed runner facts. Model-supplied strings must not become reviewer_mode, reviewer_session, provider, model, or isolation facts.
- provider isolation: Codex and Claude have different isolation properties. The CLI and reports should make that visible, especially when provider=auto falls back from Codex to Claude.
- prompt construction: Runner instructions need to be outside the untrusted handoff/spec block, with explicit boundary markers and instructions to treat all task content as data.
- telemetry and reporting: Scafld should report what it knows about executor/challenger model separation, and explicitly report unknown when it cannot observe it.

## Risks

- Replacing the JSON envelope with prose-first output may make provider parsing more complex and break the current smoke stubs.
- Tightening clean-section validation may reject legitimate but concise clean reviews.
- Claude hardening flags may vary by installed Claude Code version.
- Capturing executor model telemetry may overpromise if work was performed outside scafld wrappers.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` External review provenance fields are set by scafld from observed subprocess facts, and malicious model-supplied reviewer_mode or session strings are ignored.
- [ ] `dod2` External review output is prose-first and must pass the canonical markdown review parser before `scafld review` reports success or prints `scafld complete`.
- [ ] `dod3` External review subprocesses obey a configurable timeout with a default of 600 seconds and fail cleanly with local/manual fallback guidance.
- [ ] `dod4` Prompt injection inside task/spec/handoff content is fenced as untrusted input in the external review prompt.
- [ ] `dod5` Codex and Claude provenance records include an isolation level and auto fallback from Codex to Claude is visible to users and reports.
- [ ] `dod6` Generic clean-section filler such as "No issues found - checked everything" no longer counts as evidence-backed clean review signal.
- [ ] `dod7` Docs and CLI help describe the hardened review contract, timeout config, JSON mode semantics, and provider isolation limits.

Validation:
- [ ] `v1` compile - Compile the Python sources after runner and review changes.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Run the full review gate smoke suite.
  - Command: `bash tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - Run the review prompt contract smoke suite.
  - Command: `bash tests/review_prompt_contract_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` integration - Run review signal corpus fixtures.
  - Command: `bash tests/review_signal_corpus_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v5` integration - Prove optional Codex and Claude adapter paths still work.
  - Command: `bash tests/codex_handoff_adapter_smoke.sh && bash tests/claude_handoff_adapter_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v6` integration - Prove packaging and managed runtime bundle updates stay coherent.
  - Command: `bash tests/package_smoke.sh && bash tests/update_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v7` documentation - Documentation mentions timeout, isolation, prose contract, and JSON mode behavior.
  - Command: `bash -lc 'for term in timeout isolation "prose" "--json"; do rg -q -- "$term" docs/review.md docs/configuration.md docs/cli-reference.md; done'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Make runner facts observed and bounded

Goal: Remove model-self-reported provenance from the external output contract and add timeout/hash/config support at the runner boundary.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` integration - Malicious model provenance does not become review metadata.
  - Command: `bash tests/review_gate_smoke.sh external-runner-provenance`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` integration - External runner timeout fails cleanly with fallback guidance.
  - Command: `bash tests/review_gate_smoke.sh external-runner-timeout`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_3` compile - Python sources compile after runner config changes.
  - Command: `python3 -m compileall scafld cli`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Restore prose as the review contract

Goal: Stop treating shape-valid JSON as a trusted review and route external challenger content through the canonical markdown parser before persisting.

Status: completed
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` integration - Valid prose external review completes and passes the existing gate.
  - Command: `bash tests/review_gate_smoke.sh external-runner-prose`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` integration - Malformed external prose is rejected before the review command reports success.
  - Command: `bash tests/review_gate_smoke.sh external-runner-malformed-prose`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_3` integration - Generic clean-section filler no longer counts as clean evidence.
  - Command: `bash tests/review_signal_corpus_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Harden provider isolation and prompt boundaries

Goal: Make runner prompts resistant to spec-level instruction injection and make provider isolation differences explicit instead of hidden behind provider=auto.

Status: completed
Dependencies: phase2

Changes:
- none

Acceptance:
- [x] `ac3_1` integration - Prompt injection content from the spec is fenced as untrusted input.
  - Command: `bash tests/review_prompt_contract_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` integration - Codex and Claude provider args/provenance expose their isolation levels.
  - Command: `bash tests/review_gate_smoke.sh external-runner-isolation`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_3` documentation - Docs describe Claude as weaker isolation than Codex unless proven otherwise.
  - Command: `rg -q "weaker" docs/review.md docs/integrations.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 4: Align command surfaces docs and bundle

Goal: Make the public CLI, JSON output, docs, and managed runtime bundle tell the same truth about the hardened review runner.

Status: completed
Dependencies: phase3

Changes:
- none

Acceptance:
- [x] `ac4_1` integration - Review JSON mode honors runner/provider/model override metadata without spawning.
  - Command: `bash tests/review_gate_smoke.sh external-runner-json-overrides`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac4_2` integration - Package and update smokes pass after managed asset changes.
  - Command: `bash tests/package_smoke.sh && bash tests/update_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac4_3` documentation - CLI docs include timeout, invalid output, and JSON snapshot behavior.
  - Command: `bash -lc 'for term in timeout "invalid external" "snapshot"; do rg -q -- "$term" docs/cli-reference.md docs/review.md; done'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 5: Add observed model separation telemetry

Goal: Record executor/challenger provider and model facts where scafld observes them, while treating unknown values as unknown instead of failed or proven separation.

Status: completed
Dependencies: phase4

Changes:
- none

Acceptance:
- [x] `ac5_1` integration - Provider adapter smokes record executor attribution.
  - Command: `bash tests/codex_handoff_adapter_smoke.sh && bash tests/claude_handoff_adapter_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac5_2` integration - Report metrics distinguish known separation from unknown attribution.
  - Command: `bash tests/report_metrics_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac5_3` compile - Python sources compile after telemetry changes.
  - Command: `python3 -m compileall scafld cli`
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
Timestamp: '2026-04-26T06:43:19Z'
Review rounds: 9
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

- 2026-04-25T14:44:29Z - user - Asked to write the next scafld hardening work as a concrete scafld plan spec after adversarial review of the external review runner updates.
- 2026-04-25T14:44:29Z - assistant - Created a draft spec that turns the review feedback into executable phases: trusted provenance, prose-first review output, timeout and isolation hardening, prompt boundaries, docs, smoke coverage, and observed model telemetry.
- 2026-04-25T14:49:59Z - cli - Spec approved
- 2026-04-25T14:50:14Z - cli - Execution started
- 2026-04-26T06:43:29Z - cli - Spec completed
