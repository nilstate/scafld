---
title: Invariants
description: Non-negotiable architectural rules
---

# Invariants

Invariants are architectural constraints the agent cannot violate regardless of
the task. They are named in `.scafld/config.yaml`, selected by each spec, and
summarized in root agent guidance so the executor sees them before acting.

## Default invariants

| Invariant | Rule |
|-----------|------|
| `domain_boundaries` | Services stay in their layers, no circular dependencies |
| `error_envelope` | Consistent error format across all endpoints |
| `no_legacy_code` | No dual-reads, dual-writes, or runtime shims. Migrate immediately. |
| `no_test_logic_in_production` | Fixtures and mocks stay in test files |
| `public_api_stable` | HTTP contracts and event schemas don't change without explicit approval |
| `config_from_env` | Never hardcoded secrets or configuration |

## Contract Hierarchy

Convention enforcement has one hierarchy:

1. `.scafld/config.yaml` names the canonical invariant IDs and review agenda.
2. Each spec selects the invariants that apply to the task.
3. `AGENTS.md`, `CLAUDE.md`, and `.claude/rules` carry agent-facing operating
   guidance and project rule context.
4. Optional project docs can explain local style, but they are context, not a
   required scafld control file.

## How they're used

Every spec declares which invariants must be preserved:

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

Add invariant IDs to `.scafld/config.yaml`:

```yaml
invariants:
  canonical:
    domain_boundaries: "Respect layer separation and ownership boundaries."
    my_custom_invariant: "Explain the rule in one concrete sentence."
```

Then make the spec select the IDs it must preserve. Keep the root agent docs
short; they should tell agents to obey declared scope and invariants, not become
an exhaustive style guide.

## Deviations

If a task genuinely requires violating an invariant, the spec records a deviation:

```yaml
deviations:
  - rule: "public_api_stable"
    reason: "Signature change required for type safety"
    mitigation: "Added deprecation warning on old signature"
```

Deviations are visible in the review and audit trail. They're not violations -- they're acknowledged exceptions with documented rationale.
