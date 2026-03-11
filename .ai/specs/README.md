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

Developer reviews, then approves:

```bash
trellis approve my-task
```

### 3. Execution

AI moves spec to `active/`, sets `status: "in_progress"`, and executes phases.

### 4. Completion

Mark complete (moves to `archive/YYYY-MM/`):

```bash
trellis complete my-task
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
trellis list                  # All specs
trellis list active           # Currently executing
trellis list approved         # Awaiting execution
trellis list drafts           # Planning in progress
trellis list archive          # Completed work
```

---

## See Also

- [AGENTS.md](../../AGENTS.md) - Status lifecycle and agent policies
- [config.yaml](../config.yaml) - Validation profiles and size/risk tiers
- [schemas/spec.json](../schemas/spec.json) - Spec validation schema
