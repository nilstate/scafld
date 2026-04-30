---
spec_version: '2.0'
task_id: structured-review-packets
created: '2026-04-26T14:51:57Z'
updated: '2026-04-26T16:04:21Z'
status: completed
harden_status: passed
size: large
risk_level: high
---

# Structured review packets

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

External adversarial review should become a structured LLM-to-LLM repair transport without returning to verdict-only JSON theater. The provider must return a rich ReviewPacket that carries pass results, checked surfaces, findings, evidence, fix guidance, tests, and spec update suggestions. Scafld owns provenance and renders the canonical human markdown review plus an executor repair handoff from the normalized packet.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`
- `docs`

Files impacted:
- `scafld/review_packet.py` (new) - ReviewPacket schema normalization, validation, markdown projection, packet persistence, and repair-handoff rendering live here.
- `scafld/review_runner.py` (task-selected) - External provider prompt, JSON extraction, packet validation, hashes, and runner result assembly live here.
- `scafld/review_workflow.py` (task-selected) - Completed review rounds must be rendered from packets and invalid packet artifacts need diagnostics.
- `scafld/runtime_contracts.py` (task-selected) - Review packet artifacts need a stable run directory.
- `scafld/runtime_guidance.py` (task-selected) - Status/current_handoff must route failed structured reviews to the packet-derived executor repair handoff.
- `scafld/commands/review.py` (task-selected) - Review command should expose packet and repair-handoff paths after external review.
- `scafld/reviewing.py` (task-selected) - Finding target validation constants should be reusable by ReviewPacket validation.
- `tests/review_packet_smoke.sh` (new) - Focused coverage for packet validation, markdown projection, and repair handoff content.
- `tests/review_gate_smoke.sh` (task-selected) - External runner smoke stubs must return ReviewPacket JSON and assert packet artifacts.
- `docs/review.md` (task-selected) - Operator docs should explain ReviewPacket as canonical structured transport and markdown/repair handoff as projections.
- `docs/run-artifacts.md` (task-selected) - Run artifact docs should list review-packets and repair handoffs.
- `AGENTS.md` (task-selected) - The canonical agent guide must list the new executor review_repair handoff and explain when agents consume it.
- `tests/run_contracts_smoke.sh` (task-selected) - Runtime contract smoke should lock the review_repair handoff path and agent guide inventory.

Invariants:
- `scafld_owns_provenance`
- `markdown_review_gate_remains_authoritative_for_completion`
- `structured_output_must_carry_repair_context_not_just_verdicts`
- `invalid_provider_output_must_leave_diagnostics`
- `packet_artifacts_must_not_be_blindly_applied_to_specs`
- `rejected_review_packet_projections_must_not_create_normal_repair_artifacts`

Related docs:
- none

## Objectives

- Define ReviewPacket v1 as the canonical provider review content contract.
- Require rich repair context for every finding: failure mode, why it matters, evidence, suggested fix, tests to add, and spec update suggestions.
- Require checked-surface context for clean passes so executors can understand what the challenger actually attacked.
- Render the existing markdown review artifact from the normalized packet instead of parsing provider-authored markdown.
- Persist normalized packets under `.ai/runs/{task-id}/review-packets/review-N.json`.
- Generate an executor repair handoff from the packet under `.ai/runs/{task-id}/handoffs/executor-review-repair.md`.
- Expose that repair handoff through status/current_handoff when structured review blocks completion.
- Keep scafld-owned metadata and provider provenance out of the model-returned schema.
- Preserve diagnostics for invalid JSON, invalid packet shape, and invalid markdown projection.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- external challenger contract: Provider output becomes structured ReviewPacket JSON with strict machine fields and rich prose fields.
- review artifact projection: Scafld renders markdown reviews and repair handoffs from normalized packet content.
- LLM handoff ecosystem: The reviewer output must be useful to the next executor agent as a dense repair brief, including spec-update guidance.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [x] `dod1` External providers are prompted to return ReviewPacket JSON only, and scafld rejects legacy prose as invalid external output.
- [x] `dod2` ReviewPacket validation enforces exact pass ids/results, a closed top-level schema, exactly one checked surface per pass, checked surfaces for clean passes, repository-grounded file:line and Markdown/YAML anchor targets, the 10-finding cap, single-line markdown-projected and repair-handoff-rendered fields, severity, blocking status, and rich repair fields.
- [x] `dod3` Completed external review rounds are rendered from normalized ReviewPacket content and still pass the existing markdown review parser.
- [x] `dod4` Normalized packet artifacts and executor repair handoffs are persisted and referenced in review provenance/command output only after the markdown review projection passes the canonical review parser, and failed structured reviews surface the repair handoff through status/current_handoff.
- [x] `dod5` Docs explain ReviewPacket as the canonical structured transport, with markdown and repair handoffs as projections.

Validation:
- [ ] `v1` compile - Python sources compile after packet contract changes.
  - Command: `python3 -m compileall scafld cli scripts`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - Focused packet smoke validates schema, projection, persistence, and repair handoff rendering.
  - Command: `bash tests/review_packet_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - External runner smoke proves both provider stubs return ReviewPacket JSON and produce packet artifacts.
  - Command: `bash tests/review_gate_smoke.sh external-runner-structured-packet`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` integration - External runner observability smoke still persists diagnostics for invalid output.
  - Command: `bash tests/review_gate_smoke.sh external-runner-observability`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: ReviewPacket contract and projections

Goal: Add the canonical packet schema, validation, markdown projection, packet artifact persistence, and executor repair handoff rendering.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` compile - Python sources compile after packet module and workflow changes.
  - Command: `python3 -m compileall scafld cli scripts`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` integration - Focused ReviewPacket smoke passes, including duplicate checked-surface rejection, scafld-owned field rejection, finding-cap enforcement, multiline repair-field rejection, repository-grounded file and anchor target validation, and rejected-projection artifact safety.
  - Command: `bash tests/review_packet_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: External runner packet cutover

Goal: Cut external providers over to ReviewPacket JSON while preserving diagnostics and existing gate semantics.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac2_1` integration - External runner structured packet smoke passes.
  - Command: `bash tests/review_gate_smoke.sh external-runner-structured-packet`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` integration - External runner observability smoke still passes.
  - Command: `bash tests/review_gate_smoke.sh external-runner-observability`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Docs and ecosystem surfaces

Goal: Document ReviewPacket as the structured review transport and make downstream LLM repair flow explicit.

Status: completed
Dependencies: none

Changes:
- `docs/review.md` (all) - Explain ReviewPacket, markdown review projection, repair handoff projection, and scafld-owned metadata boundaries.
- `docs/run-artifacts.md` (all) - Document review-packets artifacts and executor-review-repair handoff files.
- `AGENTS.md` (all) - Document executor × review_repair as a current role×gate handoff and tell agents when to read the packet-derived repair brief.
- `tests/run_contracts_smoke.sh` (all) - Assert the runtime contract and canonical agent guide both expose executor × review_repair.
- `scafld/runtime_guidance.py` (all) - When structured external review fails and provenance has a repair handoff, expose executor × review_repair as status/current_handoff.

Acceptance:
- [x] `ac3_1` compile - Python sources still compile after docs and final integration.
  - Command: `python3 -m compileall scafld cli scripts`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` integration - Review packet, structured external runner, and run contract smokes pass together.
  - Command: `bash tests/review_packet_smoke.sh && bash tests/review_gate_smoke.sh external-runner-structured-packet && bash tests/run_contracts_smoke.sh`
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
Timestamp: '2026-04-26T16:04:07Z'
Review rounds: 5
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

- 2026-04-26T14:51:57Z - user - Spec created via scafld plan
- 2026-04-26T15:02:00Z - assistant - Converted the structured review direction into a ReviewPacket v1 plan: provider returns canonical structured content, scafld owns provenance, markdown and executor repair handoffs are projections, and the packet is explicitly optimized for LLM-to-LLM repair transfer.
- 2026-04-26T14:55:35Z - cli - Spec approved
- 2026-04-26T15:19:00Z - cli - Execution started
- 2026-04-26T15:42:00Z - assistant - Incorporated the structured review packet findings into the spec: duplicate checked surfaces are rejected, markdown-projected packet strings are single-line, repair-handoff-rendered fields cannot inject multiline markdown, and packet/repair artifacts are written only after the rendered markdown review projection passes the canonical parser.
- 2026-04-26T15:52:00Z - assistant - Incorporated the later convention finding: AGENTS.md now lists executor × review_repair as a current handoff and tells agents to read the packet-derived repair brief when structured external review finds issues.
- 2026-04-26T16:00:00Z - assistant - Incorporated the grounded-target finding: external ReviewPacket validation now checks file:line targets against repository files and line counts while preserving stable Markdown/YAML anchor citations.
- 2026-04-26T16:06:00Z - assistant - Incorporated the schema-boundary findings: packet validation now rejects unexpected or scafld-owned top-level fields and enforces the trusted 10-finding cap before projection.
- 2026-04-26T16:12:00Z - assistant - Incorporated the repair-routing and anchor-grounding findings: failed structured reviews now expose executor × review_repair through status, and packet validation rejects missing Markdown/YAML anchor files.
- 2026-04-26T16:04:21Z - cli - Spec completed
