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

## Branch binding and sync

`scafld branch <task-id>` binds a spec to the workspace git branch. In a normal
repo that means "create or checkout the task branch and record it in
`origin.git`." In a workspace repo it means the root workspace repo owns the
binding; per-criterion `cwd` still controls which submodule each acceptance
command runs inside.

```bash
scafld branch add-auth
scafld branch add-auth --name feat/add-auth
scafld branch add-auth --bind-current
scafld sync add-auth
```

- `branch` is explicit and safety-first. It refuses dirty branch switches and
  detached-HEAD mutation.
- `--bind-current` is the escape hatch when you already created the branch
  yourself and want scafld to record it without switching.
- `sync` compares the recorded binding against the live git workspace and
  reports drift in both human output and `--json`.

When scafld evaluates branch drift, it ignores its own control-plane artifacts
under `.ai/specs/`, `.ai/reviews/`, `.ai/logs/`, plus `.ai/config.local.yaml`.
That keeps sync focused on engineering changes instead of flagging the planning
ledger itself.

## Per-submodule conventions

Each submodule can have its own `CONVENTIONS.md`. The convention_check review pass will read conventions from both the workspace root and the relevant submodules.
