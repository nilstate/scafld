---
title: Invariants
description: Non-negotiable architectural rules
---

# Invariants

Invariants are architectural constraints the agent cannot violate regardless of the task. They're defined in `config.yaml` and documented in `AGENTS.md`.

## Default invariants

| Invariant | Rule |
|-----------|------|
| `domain_boundaries` | Services stay in their layers, no circular dependencies |
| `error_envelope` | Consistent error format across all endpoints |
| `no_legacy_code` | No dual-reads, dual-writes, or runtime shims. Migrate immediately. |
| `no_test_logic_in_production` | Fixtures and mocks stay in test files |
| `public_api_stable` | HTTP contracts and event schemas don't change without explicit approval |
| `config_from_env` | Never hardcoded secrets or configuration |

## How they're used

Every spec declares which invariants must be preserved in `task.context.invariants`:

```yaml
task:
  context:
    invariants:
      - public_api_stable
      - config_from_env
      - error_envelope
```

During execution, if the agent's changes would violate a declared invariant, it must pause and ask. This is non-optional.

## Defining your own

Add invariants to `AGENTS.md` and reference them by name in `config.yaml`:

```yaml
# config.yaml
invariants:
  canonical:
    - domain_boundaries
    - error_envelope
    - my_custom_invariant
```

Document the rule clearly in `AGENTS.md` so the agent understands what to enforce.

## Deviations

If a task genuinely requires violating an invariant, the spec records a deviation:

```yaml
deviations:
  - rule: "public_api_stable"
    reason: "Signature change required for type safety"
    mitigation: "Added deprecation warning on old signature"
```

Deviations are visible in the review and audit trail. They're not violations -- they're acknowledged exceptions with documented rationale.
