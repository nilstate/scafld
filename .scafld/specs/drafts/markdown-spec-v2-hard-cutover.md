---
spec_version: '2.0'
task_id: markdown-spec-v2-hard-cutover
created: '2026-04-29T11:59:33Z'
updated: '2026-04-30T12:25:52Z'
status: draft
harden_status: not_run
size: large
risk_level: high
---

# Cut scafld specs and workspace over to living Markdown

## Current State

Status: draft
Current phase: none
Next: none
Reason: none
Blockers: none
Allowed follow-up command: none
Latest runner update: none
Review gate: not_started

## Summary

Replace the current YAML-only scafld spec format with a hard-cut living Markdown format that keeps the full scafld form while making the artifact useful to read as a living execution document.
The same hard cutover moves the workspace control plane from `.ai/` to `.scafld/`. Project-owned runtime artifacts live under `.scafld/`, and the framework-managed reset copy lives under `.scafld/core/`. The old `.ai/scafld/` managed bundle shape is removed, not aliased.
This is not a compatibility layer. After the cutover, `.scafld/specs/**/*.md` is the only supported task-spec format. Existing YAML spec readers, regex YAML field helpers, YAML path discovery, YAML templates, YAML fixtures, YAML docs, and `.ai/` workspace detection are removed or rewritten. `.yaml` files under specs directories are legacy artifacts and must not be loaded as specs.
The new shape is a literate execution contract: Markdown prose is the native authoring surface, while scafld-owned runner sections carry the complete structured contract and mutable runner state. The runner keeps the spec current by updating front matter and runner sections for status, planning log, hardening, acceptance results, phase progress, review summary, self-eval, deviations, origin binding, and sync metadata. `session.json` remains the raw append-only ledger under `.scafld/runs/`, but the spec is the human-readable current truth.
The authority model is explicit: session entries are the durable event ledger, and Markdown runner sections are the readable projection of that ledger plus the approved task contract. Runner writes append the session event first, patch the relevant runner section second, then validate that the on-disk spec can be reconciled from session state and preserved human-owned prose.
Agent execution is intentionally narrow: the spec is the work order and live dashboard, session is the evidence ledger, and scafld commands are the only supported way to advance runner-owned state. Acceptance and phase checkmarks are evidence-derived projections, not agent-authored Markdown. Agents ask scafld for the next action, edit code, run the instructed command or record structured manual evidence, and let scafld update the living spec.
The surface format is document-native first. Prose, short lists, current state, acceptance checklists, phase changes, planning log entries, and review summaries render as strict Markdown, not YAML wrapped in comments. Fenced code blocks are allowed for prose examples; scafld-owned data renders as headings, labels, bullets, and checklists. The mutation API must make the safe path the only practical path: runtime modules patch front matter and runner sections through `spec_markdown.py` writer helpers, not ad hoc string rewrites. Command expectations use `expected_kind` because evidence-derived checkoff needs machine-checkable semantics; free-text `expected` is human context, not a pass/fail contract.
Section ownership is heading-based: `spec_markdown.py` locates exact `##` sections with a fence-aware scanner, replaces runner-owned section bodies, preserves human-owned sections unless model data changed, and fails closed when phase headings on disk do not match the model being written.
Phase identity has one clear split: `## Phase N: Name` is the only identity surface. The phase number is the canonical machine id (`phaseN`), and the text after the colon is the canonical human-visible name. Runtime writes preserve that heading text unless an explicit rename operation occurs; validation fails on missing, duplicate, or malformed phase headings, not on a human-edited phase name.

## Context

CWD: `.`

Packages:
- `scafld`
- `scafld/commands`
- `tests`
- `docs`
- `.scafld`

Files impacted:
- `scafld/spec_markdown.py` (all) - New strict living Markdown parser, renderer, safe section writer API, and normalized model bridge.
- `scafld/spec_reconcile.py` (all) - New session-to-spec projection and drift reconciliation helpers for living Markdown specs.
- `scafld/commands/spec.py` (all) - New spec maintenance command surface for reconcile check/repair operations.
- `scafld/spec_store.py` (all) - Hard-cut store APIs from YAML files and regex mutations to Markdown .md specs and model mutations.
- `scafld/spec_templates.py` (all) - Generate full and slim living Markdown draft specs instead of YAML templates.
- `scafld/spec_model.py` (all) - New normalized-model accessor module replacing legacy YAML text scanner helpers.
- `scafld/spec_parsing.py` (all) - Remove the misleading legacy parser module after callers move to spec_model.py and spec_markdown.py.
- `scafld/lifecycle_runtime.py` (task-selected) - Plan, validate, approve, start, status, and new-spec paths must use .md specs and Markdown validation.
- `scafld/command_runtime.py` (task-selected) - Workspace detection must use `.scafld/` and `.scafld/core/manifest.json`, with no `.ai/` fallback.
- `scafld/config.py` (task-selected) - Add configurable harden policy by risk/size and workspace defaults.
- `scafld/runtime_contracts.py` (task-selected) - Run, handoff, diagnostic, review-packet, and session paths move from `.ai/runs/` to `.scafld/runs/`.
- `scafld/runtime_guidance.py` (task-selected) - Centralize the agent next-action resolver so execution is driven by evidence-derived spec/session state.
- `scafld/execution_runtime.py` (task-selected) - Harden and exec paths mutate Markdown runner sections for runner-owned living state and evidence-derived phase/acceptance progress.
- `scafld/acceptance.py` (task-selected) - Acceptance result writes must update Markdown acceptance checklists instead of YAML fields.
- `scafld/audit_scope.py` (task-selected) - Scope auditing must collect declared changes from the living Markdown normalized model and active .md specs.
- `scafld/hardening.py` (task-selected) - Promote harden from prompt/status machinery to a structured spec-quality protocol with rubric findings and pass/block semantics.
- `scafld/harden_protocol.py` (all) - New deterministic harden rubric engine for authority, ownership, recovery, invariants, boundaries, examples, phase splits, operations, dogfood, and complexity containment.
- `scafld/handoff_renderer.py` (task-selected) - Handoff rendering must load Markdown specs and cite stable Markdown anchors.
- `scafld/review_workflow.py` (task-selected) - Review header, automated spec compliance, review block upsert, and self-eval checks must read/write Markdown specs.
- `scafld/review_runtime.py` (task-selected) - Review gate status checks must read Markdown spec metadata.
- `scafld/workflow_runtime.py` (task-selected) - Plan/build wrappers must stop using YAML scalar helpers and use Markdown spec metadata.
- `scafld/adapter_runtime.py` (task-selected) - Provider adapters must load Markdown specs for current phase and status.
- `scafld/commands/lifecycle.py` (task-selected) - Lifecycle CLI output, list, branch, sync, cancel, fail, validate, and harden approval policy paths must use Markdown specs.
- `scafld/commands/execution.py` (task-selected) - Harden CLI output must expose structured findings, suggested spec patches, unresolved blockers, and pass/block state.
- `scafld/commands/handoff.py` (task-selected) - Handoff CLI must read Markdown status and normalized spec data.
- `scafld/commands/projections.py` (task-selected) - Summary, checks, and PR-body projections must build from Markdown specs.
- `scafld/commands/reporting.py` (task-selected) - Report metrics must aggregate Markdown specs, harden rounds, self-eval, phase counts, and acceptance results.
- `scafld/commands/review.py` (task-selected) - Complete flow must write review/current-state truth into Markdown specs before archival.
- `scafld/commands/surface.py` (task-selected) - Register `scafld spec reconcile` or equivalent spec maintenance command surface.
- `scafld/commands/audit.py` (task-selected) - Audit command must consume Markdown declared-change maps.
- `scafld/runtime_bundle.py` (task-selected) - Managed bundle constants move from `.ai/scafld/` to `.scafld/core/`; Markdown examples and core README content are shipped there.
- `scafld/commands/workspace.py` (task-selected) - `init`, `update`, workspace messages, scan/update discovery, and JSON payloads must create and report `.scafld/` paths.
- `scafld/review_packet.py` (task-selected) - Schema and artifact path references must use `.scafld/core/schemas/` and `.scafld/runs/` where applicable.
- `scafld/review_runner.py` (task-selected) - External review prompt/path metadata must refer to `.scafld/` artifacts and Markdown spec anchors.
- `setup.py` (task-selected) - Python package data must ship Markdown spec assets and new tests/docs references.
- `package.json` (task-selected) - npm package file list must ship Markdown spec assets and no YAML examples.
- `.scafld/core/schemas/spec.json` (all) - Managed core schema becomes the normalized living Markdown spec model; required spec_version is required.
- `.scafld/specs/README.md` (all) - Document .md lifecycle directories, hard cutover, and authoring conventions.
- `.ai/` (all) - Remove the legacy workspace tree after assets move to `.scafld/`; no runtime code should require `.ai/`.
- `.scafld/core/specs/examples/add-error-codes.yaml` (all) - Remove YAML example from the managed bundle.
- `.scafld/core/specs/examples/add-error-codes.md` (all) - Add full-fidelity living Markdown example showing every schema field and runner section.
- `.scafld/core/specs/examples/markdown-2.0-skeleton.md` (all) - Add the minimal canonical living Markdown skeleton used by docs, tests, and format repair.
- `.scafld/core/scripts/` (all) - Managed provider adapter scripts move into the disposable core bundle; there is no project-owned script override directory in v2.
- `.scafld/prompts/plan.md` (task-selected) - Planning prompt must instruct agents to edit Markdown specs and runner sections.
- `.scafld/prompts/harden.md` (task-selected) - Hardening prompt must reference Markdown harden-round regions.
- `.scafld/prompts/exec.md` (task-selected) - Executor prompt must refer to Markdown spec anchors and living current-state blocks.
- `.scafld/prompts/review.md` (task-selected) - Review prompt must allow stable Markdown spec anchors and no YAML-specific wording.
- `.scafld/README.md` (task-selected) - Workspace runtime docs must describe Markdown specs.
- `README.md` (task-selected) - Product description, install notes, and runtime primitive claims must reflect living Markdown specs.
- `AGENTS.md` (task-selected) - Agent rules must say the Markdown spec is the living source of truth and must not hand-edit runner sections incorrectly.
- `CONVENTIONS.md` (task-selected) - Add implementation review checklist for living-spec changes.
- `docs/spec-schema.md` (all) - Replace YAML schema reference with living Markdown grammar, normalized model reference, and structured harden findings.
- `docs/quickstart.md` (task-selected) - Quickstart must show .md specs and Markdown authoring.
- `docs/planning.md` (task-selected) - Planning guidance must explain prose-owned versus scafld-owned regions.
- `docs/lifecycle.md` (task-selected) - Lifecycle examples must use .md paths and living document state.
- `docs/execution.md` (task-selected) - Execution docs must state the runner records concise current truth in the Markdown spec.
- `docs/run-artifacts.md` (task-selected) - Artifact responsibilities must define spec as living current state and session as raw ledger.
- `docs/installation.md` (task-selected) - Install/editor setup must remove YAML language-server instructions and document living Markdown.
- `docs/configuration.md` (task-selected) - Slim spec and review topology examples must use the new Markdown shape where they mention specs.
- `docs/integrations.md` (task-selected) - Adapter integration docs must reference managed provider scripts under `.scafld/core/scripts/`.
- `docs/invariants.md` (task-selected) - Invariant examples must show Markdown spec sections.
- `docs/scope-auditing.md` (task-selected) - Audit docs must describe Markdown phase change extraction.
- `docs/github-flow.md` (task-selected) - Issue/branch/PR flow docs must use .md spec paths.
- `docs/workspaces.md` (task-selected) - Workspace examples must use Markdown acceptance criteria and cwd fields.
- `tests/test_spec_markdown.py` (all) - New parser/renderer/section unit coverage.
- `tests/test_spec_reconcile.py` (all) - New write-order, replay, and section drift coverage.
- `tests/test_runtime_guidance.py` (all) - New next-action resolver coverage for effortless agent execution, evidence-derived AC state, and computed phase progress.
- `tests/test_spec_command.py` (all) - New reconcile command check/repair coverage.
- `tests/test_harden_protocol.py` (all) - New deterministic harden rubric, finding severity, suggested patch, and pass/block behavior coverage.
- `tests/golden/specs/` (all) - Golden Markdown fixture inputs and expected outputs for parser, renderer, and targeted mutation diffs.
- `tests/test_spec_store.py` (all) - Rewrite store and lifecycle file-discovery tests around .md specs; remove YAML helper assumptions.
- `tests/test_clean_plan.py` (task-selected) - Plan and slim plan tests must assert Markdown output and no .yaml leaks.
- `tests/test_audit_scope.py` (task-selected) - Declared change parsing and shared ownership tests must use Markdown specs.
- `tests/test_acceptance.py` (task-selected) - Result-write tests must assert Markdown acceptance checklists.
- `tests/test_review_packet.py` (task-selected) - Spec-path fixtures and repair handoff paths must use .md specs.
- `tests/test_review_gate_severity.py` (task-selected) - Review gate fixtures must use .md spec paths where a spec path is needed.
- `tests/test_git_state.py` (task-selected) - Spec file fixtures must use .md paths.
- `tests/test_review_anchor_exclusion.py` (task-selected) - Review anchor exclusion tests must use .md control-plane paths.
- `tests/invalid_yaml_spec_smoke.sh` (all) - Remove or rename to invalid_markdown_spec_smoke.sh; YAML-specific invalid paths are no longer product behavior.
- `tests/invalid_markdown_spec_smoke.sh` (all) - New malformed Markdown/front-matter/section smoke coverage.
- `tests/acceptance_result_write_smoke.sh` (all) - Rewrite fixture and assertions for Markdown acceptance result updates.
- `tests/agent_surface_smoke.sh` (task-selected) - Agent surface smoke must assert .md plan output.
- `tests/build_happy_path_smoke.sh` (all) - Approved spec fixture and lifecycle assertions must use Markdown specs.
- `tests/challenge_override_smoke.sh` (task-selected) - Review/complete fixtures must use Markdown specs.
- `tests/claude_handoff_adapter_smoke.sh` (task-selected) - Adapter fixtures must use Markdown specs.
- `tests/codex_handoff_adapter_smoke.sh` (task-selected) - Adapter fixtures must use Markdown specs.
- `tests/completed_progress_truth_smoke.sh` (all) - Completed-progress truth assertions must inspect Markdown runner sections.
- `tests/git_origin_smoke.sh` (all) - Origin binding smoke fixtures must use Markdown specs.
- `tests/harden_smoke.sh` (all) - Hardening smokes must prove structured findings, unresolved blocker behavior, suggested patches, and Markdown harden-round mutation.
- `tests/json_contract_smoke.sh` (task-selected) - JSON contract path assertions must expect .md specs.
- `tests/lifecycle_smoke.sh` (all) - Core lifecycle smoke fixtures and archive assertions must use .md specs.
- `tests/list_command_smoke.sh` (all) - List fixtures must use .md specs and Markdown phase status.
- `tests/mixed_repo_detection_smoke.sh` (task-selected) - Repo detection plan assertions must expect Markdown spec text.
- `tests/package_smoke.sh` (task-selected) - Package contents must include .md example and exclude .yaml example.
- `tests/phase_boundary_smoke.sh` (all) - Phase boundary fixtures must use Markdown specs.
- `tests/projection_surface_smoke.sh` (task-selected) - Projection fixtures and path assertions must use Markdown specs.
- `tests/prompt_precedence_smoke.sh` (task-selected) - Prompt precedence fixture must use Markdown spec.
- `tests/real_standard_cohort_smoke.sh` (task-selected) - Archived standard-cohort fixture must use Markdown spec.
- `tests/recovery_cap_smoke.sh` (task-selected) - Recovery cap fixture must use Markdown spec.
- `tests/report_metrics_smoke.sh` (task-selected) - Report fixtures must use Markdown spec metrics.
- `tests/retire_approved_smoke.sh` (task-selected) - Retirement fixtures and archive paths must use .md specs.
- `tests/review_baseline_smoke.sh` (task-selected) - Review baseline fixture must use Markdown spec.
- `tests/review_gate_smoke.sh` (all) - Large review gate suite must be cut over from YAML fixtures and string assertions to Markdown runner sections.
- `tests/review_handoff_smoke.sh` (task-selected) - Review handoff fixture must use Markdown spec.
- `tests/review_packet_smoke.sh` (task-selected) - Review packet anchor examples must use Markdown anchors.
- `tests/review_prompt_contract_smoke.sh` (task-selected) - Review prompt contract fixture must use Markdown spec.
- `tests/run_contracts_smoke.sh` (task-selected) - Contract parser smoke must read Markdown acceptance blocks.
- `tests/runx_surface_smoke.sh` (task-selected) - Runx surface path assertions must expect .md specs.
- `tests/session_recovery_smoke.sh` (task-selected) - Session recovery fixture must use Markdown spec.
- `tests/status_next_action_smoke.sh` (task-selected) - Status/next-action fixture must use Markdown spec and assert agent next-action JSON is derived from session-backed acceptance state.
- `tests/update_smoke.sh` (task-selected) - Managed bundle update assertions must expect Markdown specs and no YAML examples.
- `.scafld/specs/` (all) - Convert the repository's own current specs and examples to .md or remove legacy YAML from the active managed surface.
- `.scafld/specs/drafts/markdown-spec-v2-hard-cutover.md` (all) - Dogfood the new format by converting this implementation spec itself to living Markdown during the migration.

Invariants:
- `no_yaml_spec_backwards_compatibility`
- `no_ai_workspace_backwards_compatibility`
- `workspace_control_plane_lives_in_dot_scafld`
- `core_bundle_lives_in_dot_scafld_core`
- `core_is_managed_and_disposable`
- `markdown_spec_is_the_only_supported_task_spec_format`
- `full_scafld_form_is_preserved`
- `spec_is_living_current state`
- `session_json_remains_raw_append_only_ledger`
- `session_json_is_authoritative_event_ledger`
- `spec_runner_sections_are_replayable_projection`
- `session_write_precedes_spec_projection_write`
- `runner_section_drift_is_rebuildable_from_session`
- `reconcile_command_can_check_and_repair_runner_sections`
- `acceptance_state_is_evidence_derived`
- `phase state_is_computed_from_acceptance_state`
- `agents_cannot_directly_check_off_runner_criteria`
- `state_advancement_requires_scafld_commands`
- `agent_next_action_is_machine_readable`
- `markdown_native_runner_payloads_are_the_default`
- `fenced_code_blocks_are_prose_examples_only`
- `no_normalized_field_is_dropped_for_readability`
- `runner_section_writer_api_is_the_only_runtime_mutation_path`
- `expected_kind_is_required_for_machine_checkable_acceptance`
- `phase_identity_is_not_duplicated_across_heading_heading_and_payload`
- `phase_heading_text_is_canonical_human_name`
- `phase_number_is_canonical_machine_id`
- `phase_heading_grammar_is_explicit`
- `slim_mode_keeps_required_regions_with_compact_content`
- `runner_only_rewrites_scafld_owned_regions`
- `human_prose_regions_survive_runner_mutations`
- `no_fenced_yaml_outside_scafld_runner_sections`
- `markdown_is_known_only_by_spec_markdown`
- `projection_replay_is_known_only_by_spec_reconcile`
- `v2_spec_is_dogfooded_on_scafld_itself`
- `implementation_prs_use_v2_review_checklist`
- `validation_errors_name_markdown_section_and_normalized_path`
- `diffs_are_small_for_runner_mutations`
- `handoffs_and_projections_read_normalized_model_not_raw_markdown_scrapes`

Related docs:
- `docs/spec-schema.md`
- `docs/run-artifacts.md`
- `.scafld/core/schemas/spec.json`
- `scafld/spec_store.py`
- `scafld/spec_model.py`
- `scafld/spec_templates.py`
- `scafld/acceptance.py`
- `scafld/execution_runtime.py`
- `scafld/runtime_guidance.py`
- `scafld/review_workflow.py`

## Objectives

- Define the living Markdown spec grammar as the sole scafld task-spec format.
- Move the workspace control plane from `.ai/` to `.scafld/`, with framework-managed assets under `.scafld/core/`.
- Preserve every current scafld spec field in the normalized model; no reduction of the form.
- Make specs readable as living documents with summary, current state, context, scope, phases, evidence, review, self-eval, and deviations visible in one file.
- Define a reconcile contract where session.json is the durable event ledger and Markdown runner sections are its readable projection.
- Make agent execution effortless by exposing one machine-readable next-action contract derived from spec state, session evidence, and current blockers.
- Make the spec materially more readable than YAML by rendering prose, short lists, current state, acceptance, phase changes, review summaries, and planning log entries as native Markdown instead of encoded data blocks.
- Make section mutation physically hard to misuse by routing runtime writes through narrow `spec_markdown.py` writer APIs.
- Upgrade harden into a structured spec-quality protocol that surfaces pivotal questions, records decisions, suggests spec patches, and blocks only on unresolved hardening blockers.
- Replace YAML text regex mutation with deterministic Markdown section mutation.
- Cut all runtime readers, handoffs, projections, audit, reporting, tests, examples, package assets, and docs over to `.md` specs.
- Fail loudly on legacy `.ai/` workspaces and `.yaml` specs instead of silently loading or migrating them at runtime.

## Scope



## Dependencies

- PyYAML remains available for front matter.
- No new Markdown parser dependency is required; the format is intentionally strict and can be parsed with a section scanner.
- Current in-flight scafld specs in this repository must be converted as part of the implementation commit because YAML task specs stop working.

## Assumptions

- A hard cutover is acceptable even if older workspaces with `.yaml` specs fail until their specs are rewritten as `.md`.
- A hard workspace namespace cutover is acceptable even if older workspaces with `.ai/` fail until initialized or rewritten as `.scafld/`.
- Fenced code blocks are prose examples only; scafld runner data renders as labels, bullets, and checklists.
- The normalized schema remains the source of truth for semantics; Markdown is a serialization and authoring surface.
- Session telemetry is authoritative for replayable runner events, but humans should not need to read session.json to know the current task state.
- Agents should not need to infer execution order from prose; the public next-action payload is the narrow execution path.
- Manual acceptance criteria still require structured evidence events before they can affect acceptance or phase status.

## Touchpoints

- spec grammar: living Markdown uses YAML front matter, canonical headings, section-specific Markdown-native section payloads, and fenced code blocks only for prose examples.
- workspace namespace: Project runtime state lives under `.scafld/`; managed framework reset assets live under `.scafld/core/`.
- workspace layout: Project-owned config, prompts, specs, reviews, and runs live directly under `.scafld/`; managed schemas, default prompts, examples, and provider adapter scripts live under `.scafld/core/`.
- store: Spec discovery, require, move, read, write, validation, and status metadata become Markdown-only.
- runner mutation: The runner appends the session event first, then updates only front matter and runner sections for living current-state truth.
- agent execution loop: Agents ask scafld for the next action, edit code, run the indicated command or record structured evidence, and rely on scafld to advance acceptance and phase state.
- reconcile: Runner sections can be rebuilt from session state plus preserved human prose; drift detection compares replayed runner sections to the on-disk spec.
- repair command: `scafld spec reconcile <task-id>` checks or repairs section drift using the session ledger and preserved prose.
- execution: Acceptance results are evidence-derived from scafld-recorded events, and phase status is computed from required acceptance state before being projected into Markdown runner sections.
- review: Review summary, pass results, finding counts, self-eval, and deviations are reflected in the Markdown spec while detailed review artifacts remain separate.
- handoffs: Generated handoffs consume the normalized model and cite stable Markdown anchors.
- audit: Declared changes come from phase runner sections in Markdown specs; active overlap scans `.md` files only.
- docs and packaging: All user-visible examples, package assets, and managed bundle paths switch to `.scafld/` and Markdown specs.
- tests: All fixtures and assertions that currently create or inspect YAML specs are rewritten.
- golden fixtures: Golden Markdown fixtures pin exact canonical output for skeleton, full example, targeted region replacement, and reconcile repair.
- acceptance schema: `expected_kind` is the explicit machine contract for command criteria; legacy `expected` text is not sufficient.
- harden protocol: Harden runs deterministic rubric checks, produces structured findings, asks only grounded questions, and writes accepted decisions back into the living spec.
- dogfood: The scafld implementation spec itself is converted to living Markdown and used by the cutover tests.
- review checklist: Implementation PRs are checked against normalized-model access, session-first writes, prose preservation, legacy rejection, and focused tests.

## Risks

- Markdown becomes ambiguous if humans edit outside the strict section grammar.
- Runner rewrites can destroy human prose if the renderer owns too much of the document.
- Full-document rendering creates noisy diffs on every acceptance result write.
- Removing YAML compatibility strands existing task files.
- Removing `.ai/` workspace detection strands existing workspaces.
- Tests currently depend on simple YAML fixture generation and direct PyYAML assertions.
- Review and report metrics may silently change if old regex helpers are left in place.
- Complex schema fields such as origin sync, harden rounds, and nested results can be hard to make readable.
- Spec runner sections and session entries can drift if a write fails halfway or a human edits runner-owned bytes.
- Humans may paste code examples into prose, weakening the ownership boundary.
- Harden can remain performative if it only opens a prompt and lets `--mark-passed` close empty rounds.
- Agents may treat the Markdown spec as editable checklist state and check off acceptance criteria without evidence.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` The living Markdown grammar is documented with full field mapping from every current schema field to one canonical Markdown location.
- [ ] `dod2` `.scafld/specs/**/*.md` is the only supported spec discovery surface; `.yaml` task specs are not loaded, moved, validated, listed, executed, or archived.
- [ ] `dod3` Plan/new/slim-plan generate readable living Markdown specs with the full scafld form, Markdown-native current-state/checklist sections, and every required runner section.
- [ ] `dod4` Lifecycle transitions mutate front matter, current-state, and planning log runner sections without rewriting human prose.
- [ ] `dod5` Execution writes acceptance results and phase status into Markdown-native runner sections; resume and current-phase selection read the normalized model.
- [ ] `dod6` Review completion writes review summary, pass results, DoD status, self-eval/deviation visibility, and completion truth into the Markdown living spec.
- [ ] `dod7` Audit, report, handoff, projection, workflow, and adapter paths consume the normalized Markdown spec model and do not scrape YAML text.
- [ ] `dod8` All bundled examples, package assets, docs, and smoke fixtures use `.scafld/` and `.md` specs; no user-facing doc claims specs are YAML or the workspace root is `.ai/`.
- [ ] `dod9` Validation errors include Markdown section/anchor and normalized path; malformed front matter and section errors are covered.
- [ ] `dod10` The repository's own control-plane artifacts are moved from `.ai/` to `.scafld/`, and task specs are converted to living Markdown or removed from the supported spec surface before the cutover lands.
- [ ] `dod11` `.scafld/core/` is treated as managed/disposable by init/update, while `.scafld/config.yaml`, `.scafld/config.local.yaml`, `.scafld/prompts/`, and task artifacts are project-owned.
- [ ] `dod12` The reconcile contract is implemented: session writes precede spec projection writes, and runner sections can be rebuilt from session without changing human prose.
- [ ] `dod13` Task specs reject malformed Markdown code fences and use Markdown-native section payloads for simple sections.
- [ ] `dod14` All command criteria declare `expected_kind`; legacy `expected:` alone is invalid in living Markdown specs.
- [ ] `dod15` `scafld spec reconcile <task-id>` can check and repair section drift from session without changing human prose.
- [ ] `dod16` Golden fixtures pin exact parse/render/mutation/reconcile output.
- [ ] `dod17` The Markdown implementation spec itself is converted to living Markdown before completion.
- [ ] `dod18` The implementation review checklist is documented and referenced by agent guidance.
- [ ] `dod19` Harden produces structured findings and cannot pass with unresolved blocking spec-quality gaps.
- [ ] `dod20` Harden can suggest or apply spec-only patches for accepted decisions without touching implementation code.
- [ ] `dod21` Acceptance and phase status are evidence-derived: direct section edits cannot mark work complete unless session-backed events reconcile.
- [ ] `dod22` A public agent next-action payload tells the agent the current phase, criterion, command or manual evidence requirement, reason, blockers, and allowed follow-up command.

Validation:
- [ ] `v1` compile - Python sources and tests compile.
  - Command: `python3 -m compileall scafld tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` test - Unit tests pass after living Markdown cutover.
  - Command: `python3 -m unittest discover tests`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` test - All smoke tests pass after fixture migration.
  - Command: `bash -lc 'for t in tests/*_smoke.sh; do bash "$t"; done'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` boundary - No runtime code imports or defines YAML-specific spec helper APIs or the legacy spec_parsing module.
  - Command: `bash -lc '! rg -n "from scafld\.spec_parsing|import scafld\.spec_parsing|yaml_read_field|yaml_set_field|yaml_read_nested|append_planning_log|glob\(\"\*\\.yaml\"\)|safe_load\(text\)|safe_dump\(data" scafld'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v5` boundary - No bundled or source-visible task spec remains as `.yaml` under `.scafld/specs`.
  - Command: `bash -lc '! find .scafld/specs -type f -name "*.yaml" | grep .'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v6` boundary - Plan creates `.md` draft specs and no `.yaml` draft leak remains.
  - Command: `bash -lc 'tmp=$(mktemp -d); cd "$tmp" && SCAFLD_SOURCE_ROOT=/Users/kam/dev/0state/scafld /Users/kam/dev/0state/scafld/cli/scafld init >/dev/null && SCAFLD_SOURCE_ROOT=/Users/kam/dev/0state/scafld /Users/kam/dev/0state/scafld/cli/scafld plan md-cutover-smoke --command "python3 -V" --files README.md >/dev/null && test -f .scafld/specs/drafts/md-cutover-smoke.md && test ! -e .scafld/specs/drafts/md-cutover-smoke.yaml && test ! -d .ai'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v7` boundary - No runtime or user-facing source refers to `.ai/` as the active workspace control plane.
  - Command: `bash -lc '! rg -n "\.ai/(specs|runs|reviews|prompts|schemas|config|scafld)|no \.ai|AI_DIR = \"\.ai\"" scafld README.md AGENTS.md .scafld docs tests setup.py package.json'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v8` boundary - Markdown task specs reject malformed Markdown code fences.
  - Command: `python3 -m unittest tests.test_spec_markdown -k unclosed_code_fence`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v9` test - Session-to-spec reconcile tests prove replay, drift detection, and human prose preservation.
  - Command: `python3 -m unittest tests.test_spec_reconcile`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v10` boundary - living Markdown fixtures and docs no longer rely on legacy expected-only command criteria.
  - Command: `bash -lc '! rg -n "expected: \"(exit code|0 failures|all tests pass|No matches|Exit code)" .scafld docs tests README.md AGENTS.md'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v11` test - Harden protocol unit tests prove rubric findings, unresolved-blocker handling, suggested patches, and pass/block semantics.
  - Command: `python3 -m unittest tests.test_harden_protocol`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v12` test - Agent next-action and evidence-derived phase/acceptance state tests pass.
  - Command: `python3 -m unittest tests.test_runtime_guidance tests.test_spec_reconcile tests.test_acceptance`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v13` test - Status next-action smoke proves the agent execution path is machine-readable and session-backed.
  - Command: `bash tests/status_next_action_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Define living Markdown contract

Goal: Specify the living Markdown grammar, normalized model, and ownership rules before touching runtime behavior.

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [ ] `ac1_1` test - Markdown parser/renderer unit tests pass.
  - Command: `python3 -m unittest tests.test_spec_markdown`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_2` test - The Markdown example parses into a required spec_version normalized model that validates against `.scafld/core/schemas/spec.json`.
  - Command: `python3 -m unittest tests.test_spec_markdown -k example`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_3` boundary - The schema docs include an explicit no-backcompat statement.
  - Command: `grep -q 'YAML task specs are unsupported' docs/spec-schema.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_4` boundary - The canonical skeleton fixture exists, names the core runner sections, and keeps simple sections Markdown-native instead of encoded data blocks.
  - Command: `bash -lc 'test -f .scafld/core/specs/examples/markdown-2.0-skeleton.md && rg -q "^## Current State$" .scafld/core/specs/examples/markdown-2.0-skeleton.md && rg -q "Allowed follow-up command" .scafld/core/specs/examples/markdown-2.0-skeleton.md && rg -q "Source event" .scafld/core/specs/examples/markdown-2.0-skeleton.md && rg -q "^## Acceptance$" .scafld/core/specs/examples/markdown-2.0-skeleton.md && rg -q "^## Phase 1: Example phase$" .scafld/core/specs/examples/markdown-2.0-skeleton.md && rg -q "^## Rollback$" .scafld/core/specs/examples/markdown-2.0-skeleton.md && rg -q "^## Harden Rounds$" .scafld/core/specs/examples/markdown-2.0-skeleton.md && ! rg -q "^```yaml" .scafld/core/specs/examples/markdown-2.0-skeleton.md'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_5` test - Reconcile contract unit tests pass for session-first writes, replay, drift detection, and prose preservation.
  - Command: `python3 -m unittest tests.test_spec_reconcile`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_6` boundary - The Markdown parser rejects malformed Markdown code fences.
  - Command: `python3 -m unittest tests.test_spec_markdown -k unclosed_code_fence`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_7` test - Golden Markdown fixture tests pass for canonical render, targeted mutation diffs, and reconcile output.
  - Command: `python3 -m unittest tests.test_spec_markdown tests.test_spec_reconcile -k golden`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_8` boundary - Markdown syntax handling is isolated to `spec_markdown.py` and session replay semantics are isolated to `spec_reconcile.py`.
  - Command: `bash -lc '! rg -n "markdown heading|ATX|encoded data|runner section" scafld --glob "!spec_markdown.py" --glob "!spec_reconcile.py"'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_9` test - Section writer tests prove heading-based updates preserve surrounding bytes and fail closed on missing, duplicate, mismatched, or malformed sections.
  - Command: `python3 -m unittest tests.test_spec_markdown -k update_spec_markdown`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_10` test - No-loss rendering tests prove every normalized schema field round-trips through either Markdown-native payloads or declared plain Markdown fields.
  - Command: `python3 -m unittest tests.test_spec_markdown -k no_loss`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac1_11` test - Phase identity tests prove heading text is parsed as the human-visible name, the phase number is the machine id, headings must match `[a-z][a-z0-9_-]*`, heading renames are preserved, and heading drift is rejected.
  - Command: `python3 -m unittest tests.test_spec_markdown -k phase_identity`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Hard cut workspace store and lifecycle

Goal: Move the workspace control plane to `.scafld/` and replace spec discovery, templates, validation, movement, and lifecycle metadata with Markdown-only behavior.

Status: pending
Dependencies: phase1

Changes:
- none

Acceptance:
- [ ] `ac2_1` test - Spec store and command runtime tests pass with `.scafld/` Markdown-only discovery, movement, and legacy `.ai`/YAML rejection.
  - Command: `python3 -m unittest tests.test_spec_store tests.test_command_runtime`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac2_2` test - Clean plan tests pass and assert `.md` output for verbose and slim specs.
  - Command: `python3 -m unittest tests.test_clean_plan`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac2_3` boundary - Init creates `.scafld/`, managed core scripts, no project schema/script override dirs, and plan creates `.md`, not `.yaml`, in the new workspace.
  - Command: `bash -lc 'tmp=$(mktemp -d); cd "$tmp" && SCAFLD_SOURCE_ROOT=/Users/kam/dev/0state/scafld /Users/kam/dev/0state/scafld/cli/scafld init >/dev/null && test -d .scafld/core && test -d .scafld/core/scripts && test ! -d .scafld/schemas && test ! -d .scafld/scripts && test ! -d .ai && SCAFLD_SOURCE_ROOT=/Users/kam/dev/0state/scafld /Users/kam/dev/0state/scafld/cli/scafld plan hard-cut-plan --command "python3 -V" --files README.md >/dev/null && test -f .scafld/specs/drafts/hard-cut-plan.md && test ! -f .scafld/specs/drafts/hard-cut-plan.yaml'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac2_4` boundary - No runtime spec discovery code searches for `*.yaml` task specs.
  - Command: `bash -lc '! rg -n "glob\(\"\*\\.yaml\"\)|f\"\{task_id\}\.yaml\"|\.yaml\".*spec" scafld/spec_store.py scafld/lifecycle_runtime.py scafld/audit_scope.py scafld/hardening.py'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac2_5` boundary - Workspace runtime constants no longer define `.ai` as the control-plane root.
  - Command: `bash -lc '! rg -n "AI_DIR = \"\.ai\"|FRAMEWORK_DIR = f\"\{AI_DIR\}/scafld\"|\.ai/scafld|no \.ai" scafld/command_runtime.py scafld/runtime_bundle.py scafld/runtime_contracts.py scafld/commands/workspace.py'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Replace readers with model access

Goal: Move every read surface off YAML text scanning and onto the normalized Markdown spec model.

Status: pending
Dependencies: phase2

Changes:
- none

Acceptance:
- [ ] `ac3_1` compile - No stale YAML helper imports or legacy spec_parsing imports remain in runtime modules.
  - Command: `bash -lc '! rg -n "from scafld\.spec_parsing|import scafld\.spec_parsing|yaml_read_field|yaml_read_nested|yaml_set_field|append_planning_log" scafld tests'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac3_2` test - Audit scope tests pass with Markdown specs.
  - Command: `python3 -m unittest tests.test_audit_scope`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac3_3` test - Review packet, review gate severity, git state, and review anchor unit tests pass with `.md` spec paths.
  - Command: `python3 -m unittest tests.test_review_packet tests.test_review_gate_severity tests.test_git_state tests.test_review_anchor_exclusion`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac3_4` test - Projection and report smoke tests pass with Markdown specs.
  - Command: `bash -lc 'bash tests/projection_surface_smoke.sh && bash tests/report_metrics_smoke.sh && bash tests/list_command_smoke.sh'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac3_5` boundary - Audit/review/handoff/report paths use `.scafld/` artifact roots.
  - Command: `bash -lc '! rg -n "\.ai/(specs|runs|reviews|schemas|prompts)" scafld/audit_scope.py scafld/review_workflow.py scafld/review_runtime.py scafld/review_packet.py scafld/review_runner.py scafld/handoff_renderer.py scafld/commands/reporting.py scafld/commands/projections.py'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 4: Implement harden protocol

Goal: Turn harden from prompt/status machinery into a structured spec-quality protocol with configurable pass/block policy.

Status: pending
Dependencies: phase3

Changes:
- none

Acceptance:
- [ ] `ac4_1` test - Harden protocol tests prove deterministic findings, unresolved-blocker refusal, suggested spec patches, and structured JSON output.
  - Command: `python3 -m unittest tests.test_harden_protocol`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac4_2` test - Harden smoke proves `--mark-passed` cannot close a round with unresolved blocking findings and can close after accepted decisions are recorded.
  - Command: `bash tests/harden_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac4_3` test - Approval policy tests prove required harden blocks approve for configured risk/size and permits explicit override with a recorded reason.
  - Command: `python3 -m unittest tests.test_harden_protocol -k approval_policy`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac4_4` boundary - Canonical harden questions are defined once, documented, and exposed through structured harden output.
  - Command: `bash -lc 'rg -q "CANONICAL_HARDEN_RUBRIC" scafld/harden_protocol.py && rg -q "What is the real product goal" docs/spec-schema.md .scafld/prompts/harden.md && rg -q "complexity_containment" tests/test_harden_protocol.py'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 5: Implement living document mutations

Goal: Make runner-owned updates keep the Markdown spec current, replayable from session, safe for human-owned prose, and simple for agents to execute through next-action guidance.

Status: pending
Dependencies: phase4

Changes:
- none

Acceptance:
- [ ] `ac5_1` test - Acceptance result write tests pass and prove human prose preservation.
  - Command: `python3 -m unittest tests.test_acceptance && bash tests/acceptance_result_write_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac5_2` test - Lifecycle, build happy path, phase boundary, recovery cap, session recovery, and completed truth smokes pass.
  - Command: `bash -lc 'bash tests/lifecycle_smoke.sh && bash tests/build_happy_path_smoke.sh && bash tests/phase_boundary_smoke.sh && bash tests/recovery_cap_smoke.sh && bash tests/session_recovery_smoke.sh && bash tests/completed_progress_truth_smoke.sh'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac5_3` test - Review gate and challenge override smokes pass with Markdown spec mutations.
  - Command: `bash -lc 'bash tests/review_gate_smoke.sh && bash tests/challenge_override_smoke.sh'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac5_4` boundary - Runner mutation tests show no whole-document safe_dump rewrite or ad hoc section string surgery outside `spec_markdown.py`.
  - Command: `bash -lc '! rg -n "safe_dump\(|safe_load\(text\)|spec\.write_text\(_rewrite|record_exec_result\(text" scafld/acceptance.py scafld/execution_runtime.py scafld/review_workflow.py scafld/commands/review.py'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac5_5` test - Reconcile tests prove session-first write order, replay repair, drift detection, and human prose preservation in runtime mutation paths.
  - Command: `python3 -m unittest tests.test_spec_reconcile`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac5_6` test - Spec reconcile command checks drift, repairs from session, emits JSON, and preserves human prose.
  - Command: `python3 -m unittest tests.test_spec_command -k reconcile`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac5_7` boundary - A public reconcile repair command is registered in the command surface.
  - Command: `bash -lc 'rg -q "reconcile" scafld/commands/surface.py && rg -q "spec_file" tests/test_spec_command.py && rg -q "preserved_prose" tests/test_spec_command.py'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac5_8` test - Evidence-derived acceptance and computed phase state tests prove direct section checkmarks cannot complete work without session-backed events.
  - Command: `python3 -m unittest tests.test_runtime_guidance tests.test_spec_reconcile tests.test_acceptance`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac5_9` test - Status next-action smoke proves agents receive a machine-readable next step with phase, criterion, command or evidence requirement, reason, blockers, and follow-up.
  - Command: `bash tests/status_next_action_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 6: Cut fixtures and tests

Goal: Finish the code-facing fixture and smoke-test migration without mixing in documentation cleanup.

Status: pending
Dependencies: phase5

Changes:
- none

Acceptance:
- [ ] `ac6_1` test - Invalid Markdown spec smoke covers malformed front matter, unclosed code fences, and malformed sections.
  - Command: `bash tests/invalid_markdown_spec_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac6_2` test - Runtime and projection smokes pass with Markdown fixtures.
  - Command: `bash -lc 'bash tests/lifecycle_smoke.sh && bash tests/build_happy_path_smoke.sh && bash tests/phase_boundary_smoke.sh && bash tests/projection_surface_smoke.sh && bash tests/report_metrics_smoke.sh'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac6_3` test - Review, adapter, and surface smokes pass with Markdown fixtures.
  - Command: `bash -lc 'bash tests/review_gate_smoke.sh && bash tests/challenge_override_smoke.sh && bash tests/claude_handoff_adapter_smoke.sh && bash tests/codex_handoff_adapter_smoke.sh && bash tests/agent_surface_smoke.sh && bash tests/runx_surface_smoke.sh'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac6_4` test - Origin, harden, JSON contract, list, mixed repo detection, and recovery smokes pass with Markdown fixtures.
  - Command: `bash -lc 'bash tests/git_origin_smoke.sh && bash tests/harden_smoke.sh && bash tests/json_contract_smoke.sh && bash tests/list_command_smoke.sh && bash tests/mixed_repo_detection_smoke.sh && bash tests/recovery_cap_smoke.sh && bash tests/session_recovery_smoke.sh && bash tests/completed_progress_truth_smoke.sh'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac6_5` boundary - No task-spec YAML files remain in `.scafld/specs` after repository migration.
  - Command: `bash -lc '! find .scafld/specs -type f -name "*.yaml" | grep .'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac6_6` boundary - Test fixtures no longer rely on legacy expected-only command criteria.
  - Command: `bash -lc '! rg -n "expected: \"(exit code|0 failures|all tests pass|No matches|Exit code)" tests .scafld/specs'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac6_7` test - Full unit and smoke suite passes with Markdown fixtures.
  - Command: `bash -lc 'python3 -m unittest discover tests && for t in tests/*_smoke.sh; do bash "$t"; done'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac6_8` boundary - The implementation spec itself is dogfooded as a living Markdown draft.
  - Command: `bash -lc 'test -f .scafld/specs/drafts/markdown-spec-v2-hard-cutover.md && SCAFLD_SOURCE_ROOT=/Users/kam/dev/0state/scafld /Users/kam/dev/0state/scafld/cli/scafld validate markdown-spec-v2-hard-cutover --json | rg -q "\"valid\": true"'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 7: Finalize docs assets and package hygiene

Goal: Finish user-facing docs, managed assets, package contents, and legacy `.ai/` removal after code-facing tests pass.

Status: pending
Dependencies: phase6

Changes:
- none

Acceptance:
- [ ] `ac7_1` test - Package and update smokes prove Markdown assets are bundled.
  - Command: `bash -lc 'bash tests/package_smoke.sh && bash tests/update_smoke.sh'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac7_2` boundary - User-facing docs no longer claim task specs are YAML, created as `.yaml`, or stored under `.ai/`.
  - Command: `bash -lc '! rg -n "specs/.+\.yaml|task specs are YAML|YAML specification|Commands that edit YAML specs|\.ai/(specs|runs|reviews|prompts|schemas|config|scafld)|\.ai/" README.md AGENTS.md .scafld docs'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac7_3` boundary - The legacy `.ai/` tree is absent after repository asset migration.
  - Command: `test ! -d .ai`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac7_4` boundary - Docs and examples no longer rely on legacy expected-only command criteria.
  - Command: `bash -lc '! rg -n "expected: \"(exit code|0 failures|all tests pass|No matches|Exit code)" README.md AGENTS.md .scafld docs'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac7_5` test - Full unit and smoke suite still passes after docs and package hygiene.
  - Command: `bash -lc 'python3 -m unittest discover tests && for t in tests/*_smoke.sh; do bash "$t"; done'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `ac7_6` boundary - Implementation review checklist is documented for living-spec changes.
  - Command: `bash -lc 'rg -q "living-spec implementation review checklist" CONVENTIONS.md && rg -q "session before spec projection" CONVENTIONS.md && rg -q "normalized model" CONVENTIONS.md'`
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

Estimated effort hours: 40.0
Actual effort hours: none
AI model: none
React cycles: none

Tags:
- spec-v2
- markdown
- hard-cutover
- living-document

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

- 2026-04-29T11:59:33Z - user - Requested a deep full-migration spec for scafld living Markdown as a hard cutover with no backwards compatibility.
- 2026-04-29T11:59:33Z - agent - Audited spec read/write/mutation surfaces across spec_store, spec_parsing, lifecycle, execution, review, handoff, audit, projections, reporting, packaging, docs, and tests before drafting scope.
- 2026-04-29T12:24:30Z - agent - Accepted review feedback by adding an explicit reconcile contract, session-first write ordering, section boundaries, and strict expected_kind migration scope.
- 2026-04-29T13:05:17Z - agent - Added the evidence-derived execution contract: agents advance state through scafld commands, acceptance checkmarks reconcile to session events, phase state is computed, and a machine-readable next-action payload drives the agent loop.
- 2026-04-29T13:11:54Z - agent - Tightened the v2 contract by requiring a safe section writer API, making `expected_kind` part of evidence-derived execution, and documenting that slim specs are compact literate contracts rather than free-form lightweight Markdown.
- 2026-04-29T13:20:34Z - agent - Reworked the v2 surface grammar so runner sections are Markdown-native by default, fenced code blocks are prose examples, phase identity is not duplicated, and the skeleton proves readability without losing the living-state/reconcile model.
- 2026-04-29T13:28:00Z - agent - Added phase heading canonicality: heading text is the human-visible phase name, the phase number is the machine id, runtime writes preserve heading text, and validation rejects heading drift rather than benign heading renames.
- 2026-04-29T13:31:21Z - agent - Specified exact phase heading parsing and heading grammar: `## Phase N: Name` derives machine id `phaseN`, while the text after the colon is `phases[].name`.
- 2026-04-29T13:37:35Z - agent - Clarified the final workspace/module layout: no project `.scafld/schemas/` or `.scafld/scripts/` override dirs in v2, managed adapter scripts live under `.scafld/core/scripts/`, prompt precedence is explicit, human review markdown stays flat, and normalized accessors move from spec_parsing.py to spec_model.py.
