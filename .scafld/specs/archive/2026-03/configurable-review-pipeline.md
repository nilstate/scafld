---
spec_version: '2.0'
task_id: configurable-review-pipeline
created: '2026-03-30T02:32:37Z'
updated: '2026-03-30T03:29:00Z'
status: completed
harden_status: not_run
size: large
risk_level: high
---

# Configurable multi-pass review pipeline

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

Scafld already has the right review decomposition in spirit: automated checks, scope drift, regression hunt, convention check, and a dark-pattern / defect scan. The problem is that only two automated passes are first-class in the CLI, while the adversarial layers are hardcoded as fixed headings and a single aggregate verdict. Evolve the review system so the five-layer model is configurable, machine-readable, and enforced through a hard cutover to Review Artifact v3 and configured pass topology.

## Context

CWD: `.`

Packages:
- `cli`
- `.ai`
- `tests`

Files impacted:
- `cli/scafld` (all) - Review pass configuration, metadata rendering/parsing, review scaffolding, completion gating, and init-time templates all live in the CLI.
- `.ai/config.yaml` (review section) - The default review topology and pass descriptions should become first-class config, not hardcoded CLI constants plus prompt text.
- `.ai/prompts/review.md` (all) - The review prompt currently hardcodes three adversarial sections and needs to align with configurable pass definitions and Review Artifact v3 expectations.
- `README.md` (review workflow sections) - Public docs need to explain the new review topology, pass results, and hard-cutover expectations.
- `AGENTS.md` (review mode guidance) - Agent instructions should describe the configured pass model instead of a partially hardcoded review workflow.
- `.ai/README.md` (review overview) - Framework docs should reflect the new review contract.
- `.ai/OPERATORS.md` (review workflow) - Human operator docs need the updated mental model and CLI behavior.
- `tests/review_gate_smoke.sh` (all) - The smoke suite is the repo's main regression net for review-gate behavior and needs new coverage for configurable passes, scaffold output, and completion gating.

Invariants:
- `domain_boundaries`
- `no_legacy_code`
- `public_api_stable`
- `config_from_env`

Related docs:
- `README.md`
- `AGENTS.md`
- `.ai/config.yaml`
- `.ai/prompts/review.md`
- `.ai/OPERATORS.md`

## Objectives

- Make the review topology explicit and configurable in `.ai/config.yaml`
- Make pass execution order explicit rather than relying on YAML mapping luck
- Promote the current five review layers to first-class pass records, not just prose headings
- Upgrade the review artifact so automated and adversarial pass results are both machine-readable
- Back the new behavior with smoke coverage instead of hand-tested CLI paths

## Scope



## Dependencies

- Existing review-gate smoke workflow remains the primary regression harness

## Assumptions

- Review topology will be represented as nested pass mappings with explicit `order` scalars, which fits Scafld' current config loader better than YAML lists
- Built-in pass ids remain the execution boundary; this task does not open a generic arbitrary-command review engine
- Phase 1 may need to adjust config-loading and normalization helpers if the current loader cannot parse the final nested pass shape cleanly
- The default pass set should map to five layers: `spec_compliance`, `scope_drift`, `regression_hunt`, `convention_check`, and `dark_patterns`
- Hard cutover is acceptable: existing legacy review files or partially-completed review rounds must be regenerated under the new CLI

## Touchpoints

- Review topology config: `.ai/config.yaml` should declare the ordered built-in automated and adversarial passes that Scafld scaffolds and validates.
- Review artifact metadata: Review Artifact v3 should record per-pass results for all configured layers instead of only two automated pass states plus one aggregate adversarial verdict.
- CLI review flow: `scafld review` should load pass definitions from config, run built-in automated passes, and scaffold the adversarial sections from the same source of truth.
- CLI completion gate: `scafld complete` should validate the configured adversarial sections and write truthful per-pass results into the spec review block for the v3 artifact shape.
- Templates and prompts: `scafld init`, README, AGENTS, operator docs, and review prompt template should all reflect the same pass model. The init touchpoint is the updated source templates, not a new `cmd_init` workflow.
- Smoke coverage: The shell smoke suite should exercise dynamic pass scaffolding, completion validation, and configured review topology behavior.

## Risks

- Review behavior is a public Scafld contract; a sloppy migration could break pass accounting or gate semantics across review and completion.
- The current config loader only supports nested mappings and scalar values; phase 1 could discover that the planned nested pass structure needs normalization changes in the CLI.
- Pass execution order is semantically important, but relying on mapping insertion order alone is too subtle for a review pipeline.
- Per-pass results can become inconsistent with overall verdict if the CLI does not define one canonical derivation path.

## Acceptance

Profile: light

Definition of done:
- [x] `dod1` Review topology is loaded from config as ordered named built-in automated and adversarial passes
- [x] `dod2` Review Artifact v3 records per-pass results for all configured review layers
- [x] `dod3` `scafld review` scaffolds the configured adversarial sections and pass metadata without hardcoded section assumptions
- [x] `dod4` `scafld complete` validates configured pass sections and writes truthful per-pass review results into the spec
- [x] `dod5` The final nested pass definition shape is explicitly supported by config loading and normalization helpers
- [x] `dod6` Repo docs and init templates describe the five-layer review model consistently
- [x] `dod7` Smoke coverage exercises configurable review topology, scaffold output, and completion gating

Validation:
- [ ] `dod1` compile - CLI remains syntactically valid after review-topology refactor
  - Command: `python3 -m py_compile cli/scafld`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `dod2` test - Smoke case verifies ordered configurable review topology, config loading, and v3 pass metadata
  - Command: `bash tests/review_gate_smoke.sh review-pass-topology`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `dod3` test - Smoke case verifies review scaffolding matches the configured pass topology
  - Command: `bash tests/review_gate_smoke.sh review-scaffold-topology`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `dod4` test - Smoke case verifies completion enforces configured pass sections/results
  - Command: `bash tests/review_gate_smoke.sh review-complete-topology`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `dod6` documentation - Each reviewed doc and template mentions the same five-layer review pipeline
  - Command: `bash -lc 'for f in README.md AGENTS.md .ai/README.md .ai/OPERATORS.md .ai/prompts/review.md .ai/config.yaml; do for term in spec_compliance scope_drift regression_hunt convention_check dark_patterns; do rg -q "$term" "$f" || { echo "$f missing $term"; exit 1; }; done; done; rg -q "Review Artifact v3" README.md; rg -q "Review Artifact v3" .ai/prompts/review.md'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `dod7` test - Full review-gate smoke suite remains green
  - Command: `bash tests/review_gate_smoke.sh all`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Define review topology and artifact model

Goal: Move review pass definitions, explicit ordering, and per-pass result semantics into first-class config and metadata helpers

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [ ] `ac1_1` compile - CLI still compiles after review-topology helper refactor
  - Command: `python3 -m py_compile cli/scafld`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_2` test - Smoke case verifies configurable pass topology and v3 metadata normalization
  - Command: `bash tests/review_gate_smoke.sh review-pass-topology`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Drive review and complete from configured passes

Goal: Eliminate hardcoded review headings and pass assumptions from the CLI review flow

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [ ] `ac2_1` test - Review scaffolding follows the configured pass topology
  - Command: `bash tests/review_gate_smoke.sh review-scaffold-topology`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac2_2` test - Completion gate enforces configured pass sections and results
  - Command: `bash tests/review_gate_smoke.sh review-complete-topology`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Propagate docs, prompts, and smoke coverage

Goal: Make templates, guidance, and regression tests match the new review contract

Status: completed
Dependencies: none

Changes:
- none

Acceptance:
- [ ] `ac3_1` documentation - Docs mention the configurable five-layer review model
  - Command: `rg -n 'spec_compliance|scope_drift|regression_hunt|convention_check|dark_patterns|Review Artifact v3' README.md AGENTS.md .ai/README.md .ai/OPERATORS.md .ai/prompts/review.md .ai/config.yaml`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac3_2` test - Full smoke suite stays green after docs/tests updates
  - Command: `bash tests/review_gate_smoke.sh all`
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
Verdict: pass
Timestamp: '2026-03-30T03:27:02Z'
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

- 2026-03-30T02:32:37Z - user - Spec created via scafld new
- 2026-03-30T02:37:00Z - agent - Reviewed Scafld planning and review docs plus the CLI's current review implementation. Confirmed that the conceptual five-layer model already exists in docs, but only two automated passes and three fixed headings are first-class in code.
- 2026-03-30T02:41:00Z - agent - Chose a conservative evolution path: configurable built-in review passes, Review Artifact v3 with per-pass results, dynamic scaffolding/validation, and explicit ordered pass definitions.
- 2026-03-30T02:47:35Z - user - Requested a hard cutover instead of legacy compatibility. Tightened the draft to remove v2 migration work, require explicit pass ordering, add a dedicated scaffold validation case, and clarify that init impact is via updated source templates.
- 2026-03-30T03:02:01Z - cli - Spec approved
- 2026-03-30T03:02:01Z - cli - Execution started
- 2026-03-30T03:23:22Z - agent - Implemented the configurable five-pass review pipeline with Review Artifact v3, updated the default review topology/config/docs, and expanded the smoke suite to cover ordered topology, dynamic scaffolding, completion enforcement, and the existing override/exec paths.
- 2026-03-30T03:29:00Z - cli - Spec completed
