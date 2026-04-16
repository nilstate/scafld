---
title: Configuration
description: Config files and customization
---

# Configuration

scafld uses two config files in `.ai/`:

- `config.yaml` -- base configuration (committed to version control)
- `config.local.yaml` -- project-specific overrides (build/test/lint commands)

The local config deep-merges on top of the base.

## Invariants

Architectural constraints the agent cannot violate:

```yaml
invariants:
  canonical:
    - domain_boundaries
    - error_envelope
    - no_legacy_code
    - no_test_logic_in_production
    - public_api_stable
    - config_from_env
```

Specs reference these by name in `task.context.invariants`. If a task requires violating an invariant, the agent must pause and ask.

Define your own in `AGENTS.md` and reference them here.

## Validation profiles

```yaml
validation:
  profiles:
    light:
      per_phase: [compile_check, acceptance_item_check]
      pre_commit: [perf_eval]

    standard:
      per_phase: [compile_check, targeted_tests, acceptance_item_check]
      pre_commit: [full_test_suite, linter_suite, typecheck, security_scan, perf_eval]

    strict:
      per_phase: [compile_check, targeted_tests, boundary_check, acceptance_item_check]
      pre_commit: [full_test_suite, linter_suite, typecheck, security_scan, perf_eval]
```

Profile selection: explicit via `task.acceptance.validation_profile`, or derived from `task.risk_level` (low=light, medium=standard, high=strict).

## Review passes

```yaml
review:
  automated_passes:
    spec_compliance:
      order: 10
      title: "Spec Compliance"
    scope_drift:
      order: 20
      title: "Scope Drift"

  adversarial_passes:
    regression_hunt:
      order: 30
      title: "Regression Hunt"
    convention_check:
      order: 40
      title: "Convention Check"
    dark_patterns:
      order: 50
      title: "Dark Patterns"
```

Passes execute in `order` sequence. All configured sections are required in the review file for `scafld complete` to accept.

## Self-evaluation rubric

```yaml
rubric:
  completeness:
    weight: 3
    description: "0=partial, 1=meets ask, 2=edge cases, 3=edge cases + conventions"
  architecture_fidelity:
    weight: 3
  spec_alignment:
    weight: 2
  validation_depth:
    weight: 2
  threshold: 7
  on_below_threshold: "perform_second_pass"
```

Max score is 10. Below threshold triggers a second pass.

## Local overrides

`config.local.yaml` merges on top of the base. Use it for project-specific commands:

```yaml
validation:
  per_phase:
    compile_check: "npm run build"
    targeted_tests: "npm test -- {spec_pattern}"
```
