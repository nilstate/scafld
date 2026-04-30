# Task Specifications

This directory contains living Markdown task specifications organized by
lifecycle status.

---

## Directory Structure

```
specs/
├── drafts/              # Planning in progress
│   └── *.md          (status: draft | under_review)
├── approved/            # Ready for execution
│   └── *.md          (status: approved)
├── active/              # Currently executing
│   └── *.md          (status: in_progress)
└── archive/             # Completed work
    └── YYYY-MM/
        └── *.md      (status: completed | failed | cancelled)
```

---

## File Naming

**Convention:** `{task-id}.md` using kebab-case, descriptive names.

Good: `add-user-metrics.md`, `refactor-auth-module.md`, `fix-chunk-dedup.md`
Bad: `task-123.md` (not descriptive), `AddMetrics.md` (not kebab-case)

---

## Workflow

### 1. Planning

AI generates spec in `drafts/` with `status: "draft"`. If blocked, set `status: "under_review"`.

### 2. Review & Approval

Developer reviews, then approves:

```bash
scafld approve my-task
```

### 3. Execution

AI moves spec to `active/`, sets `status: "in_progress"`, and executes phases.

### 4. Review

Run adversarial review before completing:

```bash
scafld review my-task
# Fill in findings in .scafld/reviews/my-task.md
```

### 5. Completion

Mark complete (reads review, records verdict, moves to `archive/YYYY-MM/`):

```bash
scafld complete my-task
```

---

## Spec Anatomy

Each spec validated by `.scafld/core/schemas/spec.json` includes:

- YAML front matter with task identity, lifecycle status, size, risk, and timestamps
- human-owned Markdown prose for summary, objectives, scope, context, risks, and notes
- readable runner sections for current state, executable acceptance, rollback,
  review, deviations, and audit metadata
- phase headings in the form `## Phase N: Name`, where `phaseN` is the
  machine-stable id and `Name` is the human-readable phase name
- phase labels for `Goal`, `Status`, `Dependencies`, `Changes`, and
  `Acceptance`

---

## Finding Work

```bash
scafld list                  # All specs
scafld list active           # Currently executing
scafld list approved         # Awaiting execution
scafld list drafts           # Planning in progress
scafld list archive          # Completed work
```

---

## See Also

- [AGENTS.md](../../AGENTS.md) - Status lifecycle and agent policies
- [config.yaml](../config.yaml) - Validation profiles and size/risk tiers
- [core/schemas/spec.json](../core/schemas/spec.json) - Spec validation schema
