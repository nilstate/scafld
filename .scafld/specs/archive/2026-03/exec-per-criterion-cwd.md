---
spec_version: '2.0'
task_id: exec-per-criterion-cwd
created: '2026-03-25T05:00:00Z'
updated: '2026-03-25T04:37:54Z'
status: completed
harden_status: not_run
size: small
risk_level: low
---

# Per-criterion working directory for scafld exec

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

In monorepo/workspace setups, scafld exec runs all commands from the workspace root. Backend commands need `cd api &&` prefixes, frontend commands need `cd app &&`, etc. This is fragile and verbose. Add an optional `cwd` field on acceptance criteria so each command can declare its working directory relative to the workspace root. Commands without `cwd` continue to run from root (backward-compatible).

## Context

CWD: `.`

Packages:
- `cli`
- `.ai/schemas`

Files impacted:
- `cli/scafld` (920-925) - parse_acceptance_criteria must extract cwd field from criteria
- `cli/scafld` (1030-1040) - cmd_exec must resolve and validate cwd before running subprocess
- `.ai/schemas/spec.json` (acceptance_criteria items) - Add optional cwd property to criterion schema
- `README.md` (CLI section) - Document the cwd field in the spec format

Invariants:
- `domain_boundaries`

Related docs:
- `README.md`

## Objectives

- Acceptance criteria can declare cwd relative to workspace root
- Commands without cwd run from workspace root (backward-compatible)
- cwd paths that escape the workspace root are rejected
- Missing cwd directories produce a clear error, not a Python traceback
- The resolved directory is shown in exec output for clarity

## Scope



## Dependencies

- None.

## Assumptions

- cwd is always relative to workspace root
- Absolute paths are not supported (could escape sandbox)

## Touchpoints

- cli/scafld: CLI exec command and criteria parser
- .ai/schemas/spec.json: Spec validation schema

## Risks

- Symlinks in cwd could escape workspace root

## Acceptance

Profile: light

Definition of done:
- [ ] `dod1` cwd field is parsed from acceptance criteria
- [ ] `dod2` Commands run from the specified cwd directory
- [ ] `dod3` Missing cwd produces a clear error (not a traceback)
- [ ] `dod4` cwd escaping workspace root is rejected
- [ ] `dod5` Commands without cwd still run from workspace root

Validation:
- none

## Phase 1: Add cwd to criteria parser and exec runner

Goal: Parse cwd from spec YAML and use it when running commands

Status: pending
Dependencies: none

Changes:
- none

Acceptance:
- [x] `ac1` compile - CLI passes Python syntax check
  - Command: `python3 -c "import py_compile; py_compile.compile('cli/scafld', doraise=True)"`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Update schema and docs

Goal: Document the cwd field in the spec schema and README

Status: pending
Dependencies: phase1

Changes:
- `.ai/schemas/spec.json` (all) - Add optional cwd string property to acceptance_criteria items
- `README.md` (all) - Add a note about per-criterion cwd in the CLI/spec documentation

Acceptance:
- [x] `ac2` compile - Schema is valid JSON
  - Command: `python3 -c "import json; json.load(open('.ai/schemas/spec.json'))"`
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

- 2026-03-25T05:00:00Z - agent - Identified the issue: scafld exec hardcodes cwd=root for all subprocess calls. In monorepo workspaces, specs must prefix every command with cd <submodule> &&. Per-criterion cwd is the minimal, backward-compatible fix. A global spec-level cwd was considered but rejected because multi-repo specs may have criteria targeting different submodules.
- 2026-03-25T04:36:15Z - cli - Spec approved
- 2026-03-25T04:36:15Z - cli - Execution started
- 2026-03-25T04:37:54Z - cli - Spec completed
