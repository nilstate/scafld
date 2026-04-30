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

`scafld init` will suggest starter commands in `.scafld/config.local.yaml` when it recognizes common Node or Python repo markers. Review them before relying on them.

## Create a spec

```bash
scafld plan add-auth -t "Add JWT authentication" -s medium -r high
```

This generates `.scafld/specs/drafts/add-auth.md` from the default template and
immediately opens a harden round. When scafld already knows the repo shape, the
draft also inherits the suggested compile and test commands plus a short
repo-context header. Open it and fill in the task-specific fields:

````md
---
spec_version: "2.0"
task_id: add-auth
status: draft
created: "2026-04-16T10:00:00Z"
updated: "2026-04-16T10:00:00Z"
size: medium
risk_level: high
---

# Add JWT authentication

Add token-based auth to the API layer.

## Current State

Status: draft
Current phase: none
Review gate: not_started

## Objectives

- Issue JWT tokens on successful login.
- Validate tokens on protected routes.

## Scope

### In Scope

- Token creation and verification.
- API request authentication middleware.

### Out of Scope

- User model or database schema changes.
- Frontend authentication flow.

## Phase 1: Token generation

Goal: Create JWT signing and verification.

Status: pending
Dependencies: none

Changes:
- `src/auth/token.ts` — create signing and verification helpers.

Acceptance:
- [ ] `ac1_1` test: Token round-trips correctly.
  - Command: `npm test -- --grep 'token'`
  - Expected kind: `exit_code_zero`
  - Status: `pending`

## Phase 2: Auth middleware

Goal: Request validation pipeline.

Status: pending
Dependencies: phase1

Changes:
- `src/middleware/auth.ts` — extract and validate JWT from `Authorization`.

Acceptance:
- [ ] `ac2_1` test: Middleware blocks unauthenticated requests.
  - Command: `npm test -- --grep 'auth middleware'`
  - Expected kind: `exit_code_zero`
  - Status: `pending`
````

## Move through the lifecycle

```bash
# Bind the task to a working branch
scafld branch add-auth

# Validate against the schema
scafld validate add-auth

# Approve (moves drafts/ → approved/)
scafld approve add-auth

# Hand the spec to your agent...

# Start approved work and run acceptance criteria
scafld build add-auth

# Check for scope drift against git
scafld audit add-auth -b main

# Render deterministic engineering surfaces
scafld summary add-auth
scafld checks add-auth --json
scafld pr-body add-auth

# Run the review (automated + adversarial scaffold)
scafld review add-auth

# Archive as completed (requires passing review)
scafld complete add-auth
```

That sequence is the intended issue-to-branch-to-PR flow: the spec stays the
source of truth, `scafld branch` binds it to the working branch, and
`summary`/`checks`/`pr-body` project the same task state into human and CI
surfaces without wrappers reconstructing it.

## Check your history

```bash
scafld report
```

Prints aggregate statistics: pass rates, review outcomes, size/risk
distributions, monthly activity, and the session-derived runtime metrics.

The report also includes triage sections for stale drafts, approved specs
waiting to start, active specs with no exec evidence, and review drift.

## Verify the workflow

```bash
bash tests/lifecycle_smoke.sh
```

This end-to-end smoke test exercises the happy path from `init` through `complete` and verifies that `report` still surfaces actionable triage.

## Next steps

- [Lifecycle](lifecycle) -- understand each state transition
- [Spec Schema](spec-schema) -- full reference for every field
- [CLI Reference](cli-reference) -- complete command documentation
- [GitHub Flow](github-flow) -- project task state onto PR, issue, and CI surfaces
