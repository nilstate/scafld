---
title: Workspaces
description: Multi-repo and monorepo support
---

# Workspaces

For projects with multiple codebases -- an API, a frontend, an SDK, an MCP server -- the workspace pattern gives the agent visibility across all of them from a single root.

## Setup

Create a root repo, add your codebases as git submodules, then initialize scafld at the root:

```bash
mkdir workspace && cd workspace
git init
git submodule add git@github.com:org/api.git api
git submodule add git@github.com:org/frontend.git frontend
scafld init
```

The `.ai/` directory lives at the workspace root. The agent sees the full picture.

## Per-criterion working directory

Specs in a workspace typically run commands in different submodules. Use the `cwd` field to target the right directory:

```yaml
task:
  context:
    cwd: "api"                  # default for all criteria

phases:
  - id: phase1
    acceptance_criteria:
      - id: ac1_1
        command: "npm test"
        cwd: "api"              # runs in api/
      - id: ac1_2
        command: "npm run build"
        cwd: "frontend"         # runs in frontend/
```

Working directory paths must be relative and cannot escape the workspace root.

## Cross-repo specs

A single spec can declare changes across multiple submodules. Scope auditing works across the full workspace, so changes in any submodule are tracked against the spec.

## Per-submodule conventions

Each submodule can have its own `CONVENTIONS.md`. The convention_check review pass will read conventions from both the workspace root and the relevant submodules.
