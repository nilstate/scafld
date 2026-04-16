---
title: Planning
description: Writing effective specs
---

# Planning

The spec is the most important artifact in the scafld workflow. A bad spec produces bad work regardless of how capable the agent is.

## Anatomy of a good spec

A spec answers five questions:

1. **What** are we building? (objectives)
2. **Why** are we building it? (summary, context)
3. **Where** does it change? (scope, files_impacted, touchpoints)
4. **How** do we know it works? (acceptance criteria)
5. **What must not break?** (invariants, out_of_scope)

If any of these are vague, the spec isn't ready.

## The planning loop

In planning mode, the agent reads `.ai/prompts/plan.md` and runs a structured exploration cycle:

1. **THOUGHT** -- interpret the request in repo terms, identify unknowns
2. **ACTION** -- search the codebase, read files, check diffs
3. **OBSERVATION** -- capture what was learned
4. **THOUGHT** -- update the spec, ask clarifying questions
5. **REPEAT** until all required fields are filled

The agent is in read-only mode during planning. It can explore anything but change nothing outside `.ai/specs/`. Max 20 cycles. If blocked by uncertainty, it saves with `status: under_review` and documents what it needs.

## Task sizing

| Size | Scope | Phases | When to use |
|------|-------|--------|-------------|
| `micro` | Single file, mechanical change | 1 | Rename, typo fix. Rarely needs a spec. |
| `small` | 2-3 files, well-understood change | 1-2 | Add a utility, fix a bug. |
| `medium` | Multiple files, some design decisions | 2-4 | New feature, refactor. |
| `large` | Cross-cutting, architectural impact | 4+ | Migration, new subsystem. |

## Writing objectives

Objectives are outcomes, not activities:

```yaml
# Bad: describes activity
objectives:
  - "Refactor the auth module"

# Good: describes outcomes
objectives:
  - "Auth module uses stateless JWT instead of session cookies"
  - "All auth endpoints return standard error envelope on failure"
```

Each objective should be independently verifiable.

## Defining scope

Scope is a contract. In-scope items will change. Out-of-scope items will not be touched. This is what makes `scafld audit` possible.

```yaml
scope:
  in_scope:
    - "JWT middleware in src/middleware/"
    - "Login and refresh endpoints in src/routes/"
  out_of_scope:
    - "User model or database schema"
    - "Frontend authentication flow"
```

## Phasing work

Each phase has its own acceptance criteria, so partial progress is measurable. Dependencies between phases enforce ordering.

```yaml
phases:
  - id: phase1
    name: "Token generation"
    objective: "JWT creation and signing"
    dependencies: []
    changes:
      - file: src/auth/token.ts
        action: create
        content_spec: "Sign and verify JWTs using RS256"
    acceptance_criteria:
      - id: ac1_1
        type: test
        description: "Token round-trips correctly"
        command: "npm test -- --grep 'token'"
        expected: "exit code 0"
    status: pending

  - id: phase2
    name: "Auth middleware"
    dependencies: [phase1]
    ...
```

## The planning log

Records decisions made during spec creation. Uses a structured format:

```yaml
planning_log:
  - timestamp: "2026-04-16T10:00:00Z"
    actor: agent
    summary: "Chose RS256 over HS256 for token signing"
    notes: "Asymmetric keys allow verification without exposing the signing key"
```

## Risk assessment

Risk level determines the default validation profile:

| Risk | Default profile | What it runs |
|------|----------------|--------------|
| `low` | `light` | Compile check + acceptance criteria |
| `medium` | `standard` | + full test suite, linter, typecheck, security scan |
| `high` | `strict` | + per-phase boundary checks |

## Common mistakes

**Specs that are too vague.** "Improve performance" with no metrics. If the agent can interpret the objective three different ways, the spec has failed.

**Specs that are too prescriptive.** Dictating exact implementations line by line defeats the purpose. Specify the contract, not the algorithm.

**Skipping out_of_scope.** Without explicit boundaries, the agent will "helpfully" refactor adjacent code.
