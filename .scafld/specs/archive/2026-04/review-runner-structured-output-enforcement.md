---
spec_version: '2.0'
task_id: review-runner-structured-output-enforcement
created: '2026-04-27T13:57:09Z'
updated: '2026-04-27T15:42:58Z'
status: completed
harden_status: in_progress
size: small
risk_level: medium
---

# Enforce ReviewPacket structure via provider CLI flags

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

External review verdicts come back as the `result` field of a provider wrapper JSON. Today scafld leans on prompt engineering to coax claude and codex into emitting one ReviewPacket JSON object and nothing else. In practice both providers preface JSON with prose ~50% of the time, breaking `_strip_json_fence` (which uses `re.fullmatch`) and producing the cryptic "external reviewer returned invalid ReviewPacket JSON: Expecting value: line 1 column 1 (char 0)" failure. Operators see the failure, retry, and hope the model behaves on the next attempt.
Both provider CLIs expose generation-time structured-output enforcement: claude has `--json-schema <inline>` and codex has `--output-schema <path>`. These constrain the model at the generation step — no prose preface possible. That's structural enforcement, not best-effort compliance.
This spec adds a JSON Schema for the ReviewPacket, follows scafld's existing `.ai/schemas/` file convention, and wires both provider CLIs to enforce the schema. Python-side `normalize_review_packet` stays as defense-in-depth (and as insurance against the known codex bug where `--output-schema` is silently dropped when `--json` is combined with MCP servers). The prompt's "JSON only, no markdown" language is dropped because the CLI now enforces it.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`
- `.ai/schemas`

Files impacted:
- `.ai/schemas/review_packet.json` (task-selected) - New static JSON Schema describing the ReviewPacket shape; lives next to spec.json; bundled by scafld init/update.
- `scafld/runtime_bundle.py` (task-selected) - Add `resolve_review_packet_schema_path(root)` mirroring `resolve_schema_path`; declare the new asset in the bundle list.
- `scafld/review_runner.py` (task-selected) - Load the schema at prompt-build time, parameterize pass_ids per topology, pass to claude via `--json-schema` (inline) and codex via `--output-schema` (temp file).
- `scafld/review_packet.py` (task-selected) - No behavior change required; review_packet_from_text stays as the runtime contract checker (defense-in-depth against codex CLI bug 15451).
- `scafld/reviewing.py` (task-selected) - Drop re.IGNORECASE from FINDING_TARGET_RE so Python and the JSON Schema target pattern stay in lockstep (JSON Schema Draft-07 ECMA-262 regex does not support inline (?i)).
- `tests/test_review_runner.py` (task-selected) - Unit coverage for schema loading, pass_id parameterization, and per-provider arg shape.
- `tests/review_gate_smoke.sh` (task-selected) - Smoke that proves the schema is passed to the provider arg list (assertable via stub provider that captures argv).
- `docs/configuration.md` (task-selected) - Document the new schema file and the per-provider enforcement flags.

Invariants:
- `review_packet_shape_must_remain_in_lockstep_with_normalize_review_packet`
- `python_side_validation_must_remain_authoritative_at_runtime`
- `schema_must_be_parameterized_with_topology_pass_ids_per_spec`
- `provider_cli_flag_shape_differences_must_be_invisible_to_callers`

Related docs:
- `.ai/schemas/spec.json`
- `scafld/review_packet.py`
- `https://github.com/openai/codex/issues/15451`
- `https://github.com/openai/codex/issues/4181`

## Objectives

- Make the model's output structurally valid by construction, not by prompt persuasion.
- Reuse scafld's existing `.ai/schemas/` file convention rather than introducing a new schema engine.
- Keep `normalize_review_packet` authoritative at runtime so a CLI bug or a future model regression cannot silently demote the contract.

## Scope



## Dependencies

- None.

## Assumptions

- claude `--json-schema` accepts an inline JSON string and constrains the response content to match.
- codex `--output-schema` accepts a file path and constrains the response content for the gpt-5.x family (we default to gpt-5.5).
- Hand-writing the schema is one-time work; it's bounded in size and changes rarely.
- Topology adversarial pass_ids are stable within a single review run; per-call composition is fine.
- Provider CLI flag may silently no-op on edge cases (codex#15451, codex#4181); Python-side validation catches that.

## Touchpoints

- schema asset: New static schema file alongside existing spec.json, distributed via the bundle.
- runtime bundle resolver: Mirror the spec.json resolver shape so operators can override locally if needed.
- external review subprocess: Per-provider arg builders gain the schema flag; codex needs a temp file lifecycle.

## Risks

- Hand-written schema drifts from `normalize_review_packet`'s actual acceptance shape, causing the provider to refuse valid content or accept invalid content.
- codex bug 15451 silently drops the schema constraint when MCP servers are loaded.
- claude `--json-schema` may not accept very large schema strings on some platforms.
- Operator overrides `gpt-5.5` to a non-gpt-5 model; codex `--output-schema` becomes a no-op (bug 4181).

## Acceptance

Profile: standard

Definition of done:
- [x] `dod1` `.ai/schemas/review_packet.json` exists and accepts the ReviewPacket shape that `normalize_review_packet` accepts.
- [x] `dod2` claude review subprocess includes `--json-schema <inline-json>` in its argv; codex review subprocess includes `--output-schema <path>` in its argv with a real existing file at that path.
- [x] `dod3` The composed schema's `pass_results` properties are exactly the topology's adversarial pass_ids; extra or missing pass_ids are rejected by the schema.
- [x] `dod4` `build_external_review_prompt` no longer instructs the model with 'JSON only, no markdown' style negations; runtime contract relies on the CLI flags.
- [x] `dod5` A pinned ReviewPacket fixture passes both `normalize_review_packet` and the JSON Schema check, guarding against contract drift.
- [x] `dod6` Smoke registers schema-arg presence with a stub provider that captures argv.

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
- [ ] `v2` test - Unit tests cover schema load, parameterization, and contract-drift fixture.
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` boundary - Schema file is valid JSON.
  - Command: `python3 -c 'import json; json.loads(open(".ai/schemas/review_packet.json").read())'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` boundary - Smoke harness has valid bash syntax.
  - Command: `bash -n tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v5` boundary - Schema-arg smoke is registered.
  - Command: `grep -q 'case_review_runner_schema_arg_passed_to_provider' tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Static schema file and resolver

Goal: Add `.ai/schemas/review_packet.json` and a runtime-bundle resolver mirroring the spec.json convention.

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1_1` compile - Sources compile.
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` boundary - Schema file is valid JSON.
  - Command: `python3 -c 'import json; json.loads(open(".ai/schemas/review_packet.json").read())'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_3` test - Schema contract test passes.
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Per-provider CLI enforcement

Goal: Pass the topology-parameterized schema to claude via `--json-schema` and to codex via `--output-schema <temp-file>`. Drop the redundant 'JSON only' prompt language.

Status: completed
Dependencies: phase1

Changes:
- none

Acceptance:
- [x] `ac2_1` compile - Sources compile after CLI wiring.
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` test - Provider arg and composition tests pass.
  - Command: `python3 -m unittest tests.test_review_runner`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Smoke and operator docs

Goal: Smoke-prove the per-provider arg passing with a stub that captures argv; document the schema and flags.

Status: completed
Dependencies: phase2

Changes:
- none

Acceptance:
- [x] `ac3_1` boundary - Smoke harness has valid bash syntax.
  - Command: `bash -n tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` boundary - Schema-arg smoke is registered.
  - Command: `grep -q 'case_review_runner_schema_arg_passed_to_provider' tests/review_gate_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_3` boundary - Operator docs mention the new enforcement subsection.
  - Command: `grep -qiE 'json-schema|output-schema|structured output enforcement' docs/configuration.md`
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
Timestamp: '2026-04-27T15:42:30Z'
Review rounds: 8
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

- 2026-04-27T13:57:09Z - user - Spec created via scafld plan after the operator asked whether the existing schema engine could be reused; review of provider CLIs found claude `--json-schema` and codex `--output-schema` as the right primitives.
- 2026-04-27T13:59:03Z - cli - Spec approved
- 2026-04-27T13:59:09Z - cli - Execution started
- 2026-04-27T15:42:58Z - cli - Spec completed
