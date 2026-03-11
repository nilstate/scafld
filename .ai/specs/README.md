# Task Specifications

This directory contains machine-readable task specifications organized by lifecycle status.

---

## Directory Structure

```
specs/
├── drafts/              # Planning in progress
│   └── *.yaml          (status: draft | under_review)
├── approved/            # Ready for execution
│   └── *.yaml          (status: approved)
├── active/              # Currently executing
│   └── *.yaml          (status: in_progress)
└── archive/             # Completed work
    └── YYYY-MM/
        └── *.yaml      (status: completed | failed | cancelled)
```

---

## File Naming

**Convention:** `{task-id}.yaml` using kebab-case, descriptive names.

Good: `add-user-metrics.yaml`, `refactor-auth-module.yaml`, `fix-chunk-dedup.yaml`
Bad: `task-123.yaml` (not descriptive), `AddMetrics.yaml` (not kebab-case)

---

## Workflow

### 1. Planning

AI generates spec in `drafts/` with `status: "draft"`. If blocked, set `status: "under_review"`.

### 2. Review & Approval

Developer reviews, then moves file to `approved/` and sets `status: "approved"`.

```bash
mv .ai/specs/drafts/my-task.yaml .ai/specs/approved/my-task.yaml
```

### 3. Execution

AI moves spec to `active/`, sets `status: "in_progress"`, and executes phases.

### 4. Completion

Move spec to `archive/YYYY-MM/` and set final status (`completed`, `failed`, or `cancelled`).

```bash
mkdir -p .ai/specs/archive/$(date +%Y-%m)
mv .ai/specs/active/my-task.yaml .ai/specs/archive/$(date +%Y-%m)/
```

---

## Spec Anatomy

Each spec validated by `.ai/schemas/spec.json` includes:

- **`task` block:** Title, summary, context, objectives, scope, touchpoints, risks, acceptance checklist, constraints
- **`planning_log`:** Chronological entries summarizing planning steps
- **`phases`:** Ordered execution units with `changes[].content_spec`, acceptance criteria, and per-phase status
- **`rollback`:** Strategy and per-phase commands for safe reversions
- **`self_eval` / `deviations` / `metadata`:** Populated during execution

---

## Finding Work

```bash
ls .ai/specs/active/               # Current active tasks
ls .ai/specs/approved/              # Awaiting execution
ls .ai/specs/drafts/                # Planning in progress
ls .ai/specs/archive/$(date +%Y-%m)/ # Recent completions
```

---

## See Also

- [AGENTS.md](../../AGENTS.md) - Status lifecycle and agent policies
- [config.yaml](../config.yaml) - Validation profiles and size/risk tiers
- [schemas/spec.json](../schemas/spec.json) - Spec validation schema
