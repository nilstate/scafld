---
title: Validation
description: Spec validation and acceptance matchers
---

# Validation

`scafld validate <task-id>` checks the Markdown spec shape before execution:

- `spec_version` is `2.0`
- `task_id` is stable and machine-safe
- lifecycle status is valid
- harden status is valid
- phase ids are unique
- criteria ids are unique
- executable criteria use a known expected kind

## Acceptance Criteria

Executable criteria are Markdown checklist items with structured child lines:

```markdown
Acceptance:
- [ ] `ac1_1` test: Unit tests pass.
  - Command: `go test ./...`
  - Expected kind: `exit_code_zero`
```

Supported expected kinds:

- `exit_code_zero`: command exits 0.
- `exit_code_nonzero`: command exits non-zero.
- `no_matches`: command produces no output or returns a no-match status.

Criteria without a known expected kind are rejected before execution. This is
intentional: free-form `Expected:` prose is documentation, not an executable
contract.

## Build-Time Evaluation

After a phase is open and implemented, `scafld build` runs that phase's command
criteria and evaluates the result against `Expected kind`. The full command
output is captured in the session/diagnostic surface; the spec keeps only
concise projected evidence.

## Manual Evidence

Manual work belongs in prose, harden rounds, review findings, or session notes
produced by commands. The executable gate remains explicit command evidence.
