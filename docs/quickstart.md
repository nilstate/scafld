---
title: Quickstart
description: From zero to first spec in five minutes
---

# Quickstart

## Initialize a workspace

```bash
cd your-project
scafld init
```

`scafld init` will suggest starter commands in `.ai/config.local.yaml` when it recognizes common Node or Python repo markers. Review them before relying on them.

## Create a spec

```bash
scafld new add-auth -t "Add JWT authentication" -s medium -r high
```

This generates `.ai/specs/drafts/add-auth.yaml` from the default template. Open it and define the task, phases, and acceptance criteria:

```yaml
spec_version: "1.1"
task_id: add-auth
created: "2026-04-16T10:00:00Z"
updated: "2026-04-16T10:00:00Z"
status: draft

task:
  title: "Add JWT authentication"
  summary: "Add token-based auth to the API layer"
  size: medium
  risk_level: high
  context:
    packages: ["src/auth", "src/middleware"]
    invariants: [public_api_stable, config_from_env]
  objectives:
    - "Issue JWT tokens on successful login"
    - "Validate tokens on protected routes"
  touchpoints:
    - area: "src/middleware"
      description: "New auth middleware"

  acceptance:
    definition_of_done:
      - id: dod1
        description: "All auth endpoints return standard error envelope"
        status: pending

phases:
  - id: phase1
    name: "Token generation"
    objective: "Create JWT signing and verification"
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
    objective: "Request validation pipeline"
    dependencies: [phase1]
    changes:
      - file: src/middleware/auth.ts
        action: create
        content_spec: "Extract and validate JWT from Authorization header"
    acceptance_criteria:
      - id: ac2_1
        type: test
        description: "Middleware blocks unauthenticated requests"
        command: "npm test -- --grep 'auth middleware'"
        expected: "exit code 0"
    status: pending

planning_log:
  - timestamp: "2026-04-16T10:00:00Z"
    actor: agent
    summary: "Initial spec draft"
```

## Move through the lifecycle

```bash
# Validate against the schema
scafld validate add-auth

# Approve (moves drafts/ → approved/)
scafld approve add-auth

# Start execution (moves approved/ → active/)
scafld start add-auth

# Hand the spec to your agent...

# Run acceptance criteria
scafld exec add-auth

# Check for scope drift against git
scafld audit add-auth -b main

# Run the review (automated + adversarial scaffold)
scafld review add-auth

# Archive as completed (requires passing review)
scafld complete add-auth
```

## Check your history

```bash
scafld report
```

Prints aggregate statistics: pass rates, self-eval scores, size/risk distributions, monthly activity.

The report now also includes triage sections for stale drafts, approved specs waiting to start, active specs with no exec evidence, and review drift.

## Verify the workflow

```bash
bash tests/lifecycle_smoke.sh
```

This end-to-end smoke test exercises the happy path from `init` through `complete` and verifies that `report` still surfaces actionable triage.

## Next steps

- [Lifecycle](lifecycle) -- understand each state transition
- [Spec Schema](spec-schema) -- full reference for every field
- [CLI Reference](cli-reference) -- complete command documentation
