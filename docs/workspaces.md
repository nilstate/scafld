---
title: Workspaces
description: Multi-repo and monorepo support
---

# Workspaces

For projects with multiple codebases -- an API, frontend, SDK, worker, or docs
site -- initialize scafld at the shared root:

```bash
mkdir workspace && cd workspace
git init
git submodule add git@github.com:org/api.git api
git submodule add git@github.com:org/frontend.git frontend
scafld init
```

The `.scafld/` directory lives at the workspace root. Specs can describe work
across subdirectories while acceptance commands choose the right working
directory.

## Working Directories

In Markdown specs, put the default working directory in context and make each
criterion command explicit when needed:

```markdown
## Context

Working directory: `api`
Packages:
- `api`
- `frontend`

## Phase 1: API token validation

Acceptance:
- [ ] `ac1_1` test: API tests pass.
  - Command: `cd api && npm test`
  - Expected kind: `exit_code_zero`

## Phase 2: Frontend auth state

Acceptance:
- [ ] `ac2_1` test: Frontend tests pass.
  - Command: `cd frontend && npm test`
  - Expected kind: `exit_code_zero`
```

Commands should stay relative to the workspace root and must not escape it.

## Review in Workspaces

Adversarial review should inspect the whole workspace state, not only the
subdirectory where a command ran. The spec should make cross-repo ownership
obvious enough that the challenger can tell expected multi-package work from
ambient workspace drift. Changes outside the task scope are sent as context, not
treated as a local pre-flight blocker by themselves.

## Branches

The current Go binary does not create or bind git branches. Use your normal git
workflow or wrapper tooling for branch management, then let scafld own the local
spec, session, and review gate.
