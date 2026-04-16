---
title: Execution
description: Running specs and validating results
---

# Execution

Execution is what happens after planning. The spec is approved, moved to active, and an agent works against it. scafld's role during execution is validation and auditing.

## Starting work

```bash
scafld start add-auth
```

Moves the spec from `approved/` to `active/` and sets the status to `in_progress`. Hand the spec to your agent -- the spec file itself is the prompt.

## Running acceptance criteria

```bash
# Run all acceptance criteria
scafld exec add-auth

# Run a specific phase only
scafld exec add-auth -p phase1

# Resume from where you left off (skip already-passed criteria)
scafld exec add-auth --resume
```

For each criterion, scafld:

1. Resolves the working directory (criterion `cwd`, or `task.context.cwd`, or project root)
2. Runs the `command` with the configured timeout (default 600 seconds)
3. Compares the result against `expected`
4. Records pass/fail with timestamp and output snippet directly in the spec

```yaml
acceptance_criteria:
  - id: ac1_1
    type: test
    description: "Middleware rejects missing tokens"
    command: "npm test -- --grep 'auth middleware'"
    expected: "exit code 0"
    timeout_seconds: 30
    result: pass              # filled in by scafld exec
    executed_at: "2026-04-16T10:05:00Z"
    result_output: "3 passing"
```

## Expected value matching

The `expected` field supports several patterns:

| Pattern | Meaning |
|---------|---------|
| `exit code 0` | Command exits with code 0 |
| `0 failures` | Exit 0 and no "N failures" in output |
| `All pass` | Exit 0, no failures reported |
| `no matches` | Pass if exit != 0 or output is empty |
| Any other string | Substring match (case-insensitive) + exit 0 |

## Working directory

Commands execute relative to the project root by default. Override per-criterion or per-spec:

```yaml
task:
  context:
    cwd: "api"              # default for all criteria

phases:
  - acceptance_criteria:
      - id: ac1_1
        command: "cargo test"
        cwd: "crates/auth"  # overrides task-level cwd
```

Paths must be relative and cannot escape the workspace root.

## Phase ordering

Phases execute in order. If phase 2 depends on phase 1, phase 1's criteria must all pass first. To run a specific phase regardless:

```bash
scafld exec add-auth -p phase2
```

## Handling failures

When a criterion fails, fix the issue and resume:

```bash
scafld exec add-auth --resume
```

The `--resume` flag skips criteria already recorded as pass, resuming from the first pending or failed criterion.

## Watching progress

```bash
scafld status add-auth
scafld status add-auth --json
```

Shows phase progress, criteria results, and current status.

## After execution

Once all criteria pass, run the review:

```bash
scafld review add-auth
```

See [Review](review) for the automated and adversarial review process.
