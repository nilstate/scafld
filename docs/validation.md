---
title: Validation
description: Validation profiles and acceptance criteria
---

# Validation

scafld scales validation proportionate to risk. Not every change deserves the same scrutiny.

## Profiles

| Profile | Per phase | Pre commit |
|---------|-----------|------------|
| **light** | Compile check, acceptance criteria | Self-eval |
| **standard** | + targeted tests | + full test suite, linter, typecheck, security scan |
| **strict** | + boundary checks | Same as standard |

The profile is set explicitly in `task.acceptance.validation_profile` or derived from `task.risk_level`:

- `low` → light
- `medium` → standard
- `high` → strict

## Acceptance criteria types

| Type | Description | Automated? |
|------|-------------|-----------|
| `compile` | Build succeeds | Yes |
| `test` | Tests pass | Yes |
| `boundary` | Cross-module side effects checked | Yes |
| `integration` | End-to-end validation | Yes |
| `security` | Security scan passes | Yes |
| `documentation` | Docs updated | Manual |
| `custom` | Anything else | Depends on `command` |

Automated types require a `command` field. Types without a command are marked as skipped by `scafld exec` and must be checked manually.

## Timeout

Default timeout per criterion is 600 seconds. Override per-criterion:

```yaml
acceptance_criteria:
  - id: ac1_1
    command: "npm run e2e"
    timeout_seconds: 900
```

Must be a positive integer. A timeout is recorded as a failure.

## Self-evaluation

After all phases complete, the agent scores its work:

| Dimension | Max | What it measures |
|-----------|-----|-----------------|
| completeness | 3 | Does it fully meet the ask? |
| architecture_fidelity | 3 | Does it respect boundaries and patterns? |
| spec_alignment | 2 | Does execution match the spec? |
| validation_depth | 2 | How thorough is the testing? |

Total max is 10. Below the threshold (default 7) triggers a mandatory second pass. Scores of 8+ with zero noted deviations are flagged as rubber stamps.
