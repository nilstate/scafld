---
spec_version: '2.0'
task_id: external-review-attribution-precision
created: '2026-04-26T13:38:45Z'
updated: '2026-04-26T14:05:43Z'
status: completed
harden_status: passed
size: small
risk_level: medium
---

# External review attribution precision

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

The external review runner now records observed model ids, but the attribution surface still conflates structured provider facts with unstructured regex hints. Claude model ids come from a real JSON envelope; Codex model ids are inferred from stdout/stderr text and have a false-positive risk. Tighten that distinction, preserve useful diagnostics, and tune the remaining review-format surfaces without weakening the strict review gate.

## Context

CWD: `.`

Packages:
- `scafld`
- `tests`
- `docs`

Files impacted:
- `scafld/review_runner.py` (task-selected) - Provider model extraction, Claude session echo checks, prompt diagnostics, and review prompt rules live here.
- `scafld/session_store.py` (task-selected) - Provider telemetry confidence and model-separation aggregation must distinguish observed from inferred attribution.
- `scafld/commands/review.py` (task-selected) - Review command output should label inferred model ids honestly.
- `scafld/commands/reporting.py` (task-selected) - Report output and JSON should show inferred model counts separately from observed model counts.
- `scafld/reviewing.py` (task-selected) - Finding citation validation should accept stable YAML/Markdown anchors in addition to numeric code lines.
- `scafld/review_workflow.py` (task-selected) - Manual challenger handoff instructions should match the accepted finding citation format.
- `scafld/review_signal.py` (task-selected) - The legacy clean-review alias should be explicitly deprecated rather than presented as an evidence claim.
- `scripts/real_standard.py` (task-selected) - Cohort reporting should continue using format-compliant wording and avoid the deprecated clean-review alias where possible.
- `tests/review_gate_smoke.sh` (task-selected) - External runner smoke coverage should prove inferred-vs-observed model source semantics and prompt diagnostic truncation.
- `tests/report_metrics_smoke.sh` (task-selected) - Provider telemetry aggregation should prove inferred model counts and trusted model separation.
- `tests/review_signal_corpus_smoke.sh` (task-selected) - Review parser corpus should protect YAML/Markdown citation support and clean-review signal wording.
- `docs/review.md` (task-selected) - Operator docs should explain observed vs inferred attribution and deprecated clean-review alias semantics.

Invariants:
- `provider_telemetry_must_not_overclaim`
- `review_gate_verdict_semantics_do_not_change`
- `diagnostics_must_remain_debuggable`
- `packaged_cli_is_the_truth_surface`

Related docs:
- none

## Objectives

- Treat Claude JSON envelope model ids as structurally observed, but treat Codex stdout/stderr model hints as inferred.
- Keep inferred model ids out of trusted same-model/separated model-separation decisions.
- Restrict unstructured model hints enough to avoid obvious `model: User` and `model_id: legacy` false positives.
- Check Claude's reported session id against scafld's requested UUID and record a warning when they diverge.
- Make diagnostic prompt previews keep both trusted header and untrusted task tail.
- Accept stable YAML/Markdown anchor citations for document/config findings while preserving strict numeric line citations for code.
- Mark `clean_reviews_with_evidence` as a deprecated compatibility alias; keep human-facing and preferred JSON surfaces on format-compliant wording.

## Scope



## Dependencies

- None.

## Assumptions

- None.

## Touchpoints

- external review attribution: Provider provenance and session telemetry must separate structured observations from unstructured hints.
- review parser strictness: The review gate should allow non-code anchors where line numbers are not the stable citation form, without accepting loose prose bullets.
- diagnostics and reports: Debug artifacts and aggregate reports should expose attribution precision and avoid overstating clean-review evidence.

## Risks

- None.

## Acceptance

Profile: strict

Definition of done:
- [ ] `dod1` Codex unstructured model hints record `model_source: inferred` and confidence `inferred`, while Claude JSON envelope model ids remain observed.
- [ ] `dod2` Model-separation telemetry only treats observed model ids as trusted; inferred ids are counted separately.
- [ ] `dod3` Claude session id mismatches emit telemetry warnings and prompt diagnostics keep both the header and the handoff tail.
- [ ] `dod4` Review findings can cite YAML/Markdown anchors such as `config.yaml#review.external` while unanchored prose findings remain invalid.
- [ ] `dod5` Clean-review report/docs use format-compliant wording, with `clean_reviews_with_evidence` documented and emitted only as a deprecated compatibility alias.

Validation:
- [ ] `v1` compile - Compile Python sources after runner/session/parser changes.
  - Command: `python3 -m compileall scafld cli scripts`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v2` integration - External review runner smoke proves inferred attribution and diagnostics.
  - Command: `bash tests/review_gate_smoke.sh external-runner-attribution-precision`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v3` integration - Report metrics smoke proves inferred counts and trusted model separation.
  - Command: `bash tests/report_metrics_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [ ] `v4` integration - Review signal corpus proves YAML/Markdown finding citations and clean-review wording.
  - Command: `bash tests/review_signal_corpus_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pending
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Model attribution precision

Goal: Split structured observed model facts from unstructured inferred hints and keep trusted model-separation metrics honest.

Status: completed
Dependencies: none

Changes:
- `scafld/review_runner.py` (all) - Return model source/confidence with provider extraction, mark Codex regex hints as inferred, restrict unstructured model hints to plausible model prefixes, check Claude reported session ids, and preserve prompt head plus tail in diagnostics.
- `scafld/session_store.py` (all) - Add inferred provider confidence, count inferred models separately, and make model-separation states use observed-confidence models only.
- `scafld/commands/review.py` (all) - Print inferred model labels distinctly from observed model labels.
- `scafld/commands/reporting.py` (all) - Add inferred model counts to human and JSON provider telemetry.
- `tests/review_gate_smoke.sh` (all) - Add smoke coverage for Codex inferred model hints, false-positive rejection, Claude session mismatch warnings, and prompt preview head/tail truncation.
- `tests/report_metrics_smoke.sh` (all) - Assert inferred model aggregation and trusted model separation behavior.

Acceptance:
- [x] `ac1_1` integration - External-runner attribution precision smoke passes.
  - Command: `bash tests/review_gate_smoke.sh external-runner-attribution-precision`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` integration - Report metrics smoke passes with inferred attribution counts.
  - Command: `bash tests/report_metrics_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Parser and wording precision

Goal: Keep review parsing strict while accepting stable YAML/Markdown citations and retiring evidence-overclaim wording from preferred surfaces.

Status: completed
Dependencies: none

Changes:
- `scafld/reviewing.py` (all) - Extend finding citation validation to allow `*.yaml#anchor` and `*.md#heading` citations while preserving severity and bullet format.
- `scafld/review_runner.py` (all) - Update external reviewer instructions to allow YAML/Markdown anchors and continue forbidding line ranges.
- `scafld/review_workflow.py` (all) - Update manual challenger handoff instructions to allow YAML/Markdown anchors alongside numeric file lines.
- `scafld/review_signal.py` (all) - Emit preferred format-compliant clean-review keys and mark the legacy clean_reviews_with_evidence alias as deprecated compatibility metadata.
- `scripts/real_standard.py` (all) - Prefer the format-compliant clean-review key and avoid evidence wording in cohort output.
- `tests/review_signal_corpus_smoke.sh` (all) - Add corpus assertions for YAML/Markdown anchor finding citations and deprecated clean-review alias semantics.
- `docs/review.md` (all) - Document observed vs inferred attribution, YAML/Markdown anchors, and the deprecated clean-review compatibility alias.

Acceptance:
- [x] `ac2_1` integration - Review signal corpus smoke passes.
  - Command: `bash tests/review_signal_corpus_smoke.sh`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` compile - Python sources compile after parser/report changes.
  - Command: `python3 -m compileall scafld cli scripts`
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
Timestamp: '2026-04-26T14:05:37Z'
Review rounds: 2
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

- 2026-04-26T13:38:45Z - user - Spec created via scafld plan
- 2026-04-26T13:40:00Z - assistant - Converted the next adversarial feedback batch into attribution precision, citation-format, diagnostics, and clean-review wording work. Verified that the duplicate `response_sha256` item was already fixed in the previous commit.
- 2026-04-26T13:42:23Z - cli - Spec approved
- 2026-04-26T13:57:04Z - cli - Execution started
- 2026-04-26T14:05:43Z - cli - Spec completed
