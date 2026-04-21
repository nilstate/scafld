---
title: Spec Schema
description: Full YAML spec reference
---

# Spec Schema

Spec version: **1.1**. Schema file: `.ai/schemas/spec.json` (JSON Schema v7).

## Top-level fields

```yaml
spec_version: "1.1"              # required
task_id: "kebab-case-id"         # required, pattern: ^[a-z0-9-]+$
created: "2026-04-16T10:00:00Z"  # required, ISO 8601
updated: "2026-04-16T10:00:00Z"  # required, ISO 8601
status: "draft"                  # required, see lifecycle
task: {}                         # required
phases: []                       # required, min 1
planning_log: []                 # required, min 1
origin: {}                       # optional, provider-neutral source + git binding metadata
```

## Status values

`draft`, `under_review`, `approved`, `in_progress`, `completed`, `failed`, `cancelled`

## task block

```yaml
task:
  title: "Human-friendly title"              # min 5 chars
  summary: "Problem statement and goal"      # min 20 chars
  size: small                                # micro | small | medium | large
  risk_level: medium                         # low | medium | high

  context:
    packages: ["src/auth"]                   # 1+ required
    files_impacted:                          # optional
      - path: "src/file.ts"
        lines: "all"                         # "all", [1,2,3], or "100-150"
        reason: "Why this file changes"
    invariants: ["public_api_stable"]        # 1+ required
    related_docs: ["docs/auth.md"]           # optional
    cwd: "api"                               # optional, default cwd for criteria

  objectives: ["Outcome 1", "Outcome 2"]     # 1+ required

  scope:                                     # optional
    in_scope: ["What changes"]
    out_of_scope: ["What doesn't"]

  touchpoints:                               # 1+ required
    - area: "src/middleware"
      description: "New auth middleware"
      owners: ["team-name"]                  # optional
      links: ["https://..."]                 # optional

  risks:                                     # optional
    - description: "Risk description"
      impact: "low"                          # optional
      mitigation: "Mitigation plan"          # optional

  acceptance:
    validation_profile: standard             # optional: light | standard | strict
    definition_of_done:                      # 1+ required
      - id: "dod1"
        description: "Checklist item"
        status: pending                      # pending | in_progress | done
        checked_at: ""                       # optional, ISO timestamp
        notes: ""                            # optional
    validation:                              # optional
      - id: "v1"
        type: compile                        # compile | test | boundary | integration | security | documentation | custom
        description: "How to verify"
        command: "npm test"                  # required for automated types
        expected: "exit code 0"
        cwd: "submodule"                     # optional
        timeout_seconds: 600                 # optional, default 600

  dependencies: ["..."]                      # optional
  assumptions: ["..."]                       # optional
  constraints: {}                            # optional
  notes: "Decisions, trade-offs"             # optional
```

## phases array

```yaml
phases:
  - id: phase1                               # pattern: ^phase[0-9]+$
    name: "Phase name"                       # min 5 chars
    objective: "What this phase accomplishes" # min 10 chars
    dependencies: [phase0]                   # optional

    changes:                                 # 1+ required
      - file: "src/path/to/file.ts"
        action: create                       # create | update | delete | move
        move_to: "src/new/path.ts"           # required if action=move
        lines: "all"                         # optional
        content_spec: |                      # narrative description
          What changes in this file.

    acceptance_criteria:                     # 1+ required
      - id: ac1_1
        type: test                           # compile | test | boundary | integration | security | documentation | custom
        description: "What this proves"
        command: "npm test -- --grep 'auth'"  # required for automated types
        expected: "exit code 0"
        cwd: "submodule"                     # optional
        timeout_seconds: 600                 # optional
        result: pass                         # filled by scafld exec
        executed_at: "2026-04-16T..."        # filled by scafld exec
        result_output: "3 passing"           # filled by scafld exec

    status: pending                          # pending | in_progress | completed | failed | skipped
```

## Optional blocks

### origin

`origin` is optional. It records where the task came from and how it binds to
the local git workspace. The shape stays provider-neutral: a wrapper can record
GitHub issue metadata in `origin.source`, but scafld itself only owns the repo,
branch, and sync facts.

```yaml
origin:
  source:                                # optional
    system: "github"                     # github | gitlab | linear | local | ...
    kind: "issue"                        # issue | pr | task | branch | ...
    id: "123"
    url: "https://github.com/org/repo/issues/123"
    title: "Bind scafld specs to git"

  repo:
    root: "."                            # workspace-relative repo root
    remote: "origin"
    remote_url: "git@github.com:org/repo.git"

  git:
    branch: "git-bound-task-origins"
    base_ref: "origin/main"
    upstream: "origin/git-bound-task-origins"
    mode: "created_branch"               # created_branch | checked_out_existing | bound_current
    bound_at: "2026-04-21T09:00:00Z"

  sync:
    status: "in_sync"                    # unbound | in_sync | drift | unavailable
    last_checked_at: "2026-04-21T09:03:00Z"
    reasons: []
    actual:
      branch: "git-bound-task-origins"
      head_sha: "abc123..."
      upstream: "origin/git-bound-task-origins"
      remote: "origin"
      remote_url: "git@github.com:org/repo.git"
      default_base_ref: "origin/main"
      dirty: false
      detached: false
```

`scafld branch` writes the `repo` and `git` binding. `scafld sync` refreshes the
`sync` snapshot. `scafld status --json` computes the live sync view directly so
wrappers do not need to infer branch drift themselves.

### rollback

```yaml
rollback:
  strategy: per_phase                        # per_phase | atomic | manual
  commands:
    phase1: "git checkout HEAD -- src/file.ts"
    phase2: "npm run revert-migration"
```

### self_eval

```yaml
self_eval:
  completeness: 3          # 0-3
  architecture_fidelity: 2  # 0-3
  spec_alignment: 1         # 0-2
  validation_depth: 1       # 0-2
  total: 7                  # sum, threshold is 7
  notes: ""
  second_pass_performed: false
```

Below 7/10 triggers a mandatory second pass.

### deviations

```yaml
deviations:
  - rule: "public_api_stable"
    reason: "Had to change signature for type safety"
    mitigation: "Added deprecation warning"
```

### planning_log

```yaml
planning_log:
  - timestamp: "2026-04-16T10:00:00Z"
    actor: agent                             # user | agent | cli
    summary: "What changed or was decided"
    notes: "Optional context"
```

## Harden fields (optional)

Two optional top-level fields record whether the operator has interrogated the draft with `scafld harden`:

| Field | Type | Description |
|-------|------|-------------|
| `harden_status` | `not_run` \| `in_progress` \| `passed` | Optional. Tracks interrogation state. Independent of the lifecycle `status`. Not consulted by `scafld approve`. |
| `harden_rounds` | array | Optional. One entry per `scafld harden` invocation. |

Each round:

```yaml
harden_rounds:
  - round: 1
    started_at: "2026-04-20T15:00:00Z"
    ended_at: "2026-04-20T15:12:00Z"
    outcome: "passed"                          # in_progress | passed | abandoned
    questions:
      - question: "Which module owns session cleanup?"
        grounded_in: "code:src/auth/session.ts:84"
        recommended_answer: "src/auth/session.ts:cleanupSession"
        if_unanswered: "Default to existing cleanupSession."
        answered_with: "Use existing cleanupSession."
      - question: "Are there TODOs in files_impacted?"
        grounded_in: "spec_gap:task.context.files_impacted"
        recommended_answer: "Enumerate the two unspecified entries."
      - question: "Does this follow the pattern from the pipeline rewrite?"
        grounded_in: "archive:configurable-review-pipeline"
        recommended_answer: "Yes; same cutover discipline."
```

The `grounded_in` string must match `^(spec_gap:|code:|archive:).+`. It records why the question existed: a spec gap, a verified code location, or an archived precedent. Treat it as audit trail for the question, not a license to invent citations. See [planning.md](./planning.md#hardening) for when to run harden.
