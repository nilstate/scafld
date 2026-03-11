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

## Workflow

### 1. Planning Phase

**AI generates spec in `drafts/`:**

```bash
.ai/specs/drafts/add-feature-metrics.yaml
```

**Initial status:**
```yaml
status: draft
```

---

### 2. Review & Approval

**User reviews and approves:**

1. Review the spec file
2. Update status and move file:
   ```bash
   # Update status in file
   # status: approved
   # Then move to approved/
   mv .ai/specs/drafts/add-feature-metrics.yaml \
      .ai/specs/approved/add-feature-metrics.yaml
   ```

**Or, if changes needed:**

1. Edit spec in `drafts/`
2. Set `status: under_review`
3. Iterate until ready

---

### 3. Execution Phase

**AI starts execution:**

1. Move spec to active:
   ```bash
   mv .ai/specs/approved/add-feature-metrics.yaml \
      .ai/specs/active/add-feature-metrics.yaml
   ```
2. Set `status: "in_progress"` in the spec
3. Execute phases, update `phases[].status` as work progresses

---

### 4. Completion

**Task completes (or fails):**

1. Determine archive month (from `updated` or `created` timestamp)
2. Create archive folder if needed:
   ```bash
   mkdir -p .ai/specs/archive/$(date +%Y-%m)
   ```
3. Move spec from `active/` to `archive/YYYY-MM/`
4. Update `status` to `"completed"`, `"failed"`, or `"cancelled"`

---

## Spec Anatomy

Each spec validated by `.ai/schemas/spec.json` includes the following high-level sections:

- **`task` block:** Title, summary, `task.context` (packages/files/invariants), objectives, scope, dependencies, assumptions, touchpoints, risks, acceptance checklist, constraints, and info sources.
- **`planning_log`:** Chronological entries summarizing the conversational planning steps (timestamps + what changed).
- **`phases`:** Ordered execution units with `changes[].content_spec`, acceptance criteria, and per-phase status fields.
- **`rollback`:** Strategy + per-phase commands for safe reversions.
- **`self_eval` / `deviations` / `metadata`:** Populated during execution, but stubs must exist during planning.

---

## Finding Work

**Current active tasks:**
```bash
ls .ai/specs/active/
```

**Awaiting approval:**
```bash
ls .ai/specs/approved/
```

**Work in progress (planning):**
```bash
ls .ai/specs/drafts/
```

**Recent completions:**
```bash
ls .ai/specs/archive/$(date +%Y-%m)/
```

---

## Cleanup

**Archive old completed specs:**

Periodically review `archive/` folders older than 3-6 months. Consider:
- Compressing old archives: `tar -czf archive-2024.tar.gz archive/2024-*`
- Moving to long-term storage
- Deleting if no longer needed (ensure git history preserved)

**Keep git history:**

Even if files are deleted, git retains full history:
```bash
# View deleted spec
git log --all --full-history -- .ai/specs/archive/2024-12/old-task.yaml
```

---

## Status Transitions

```
draft → under_review → approved → in_progress → completed
  ↓                                    ↓           ↓
(edit)                             (blocked)   failed
                                      ↓           ↓
                                  (resume)   cancelled
```

**Valid transitions:**
- `draft` → `under_review` (user requests review)
- `under_review` → `approved` (user approves)
- `under_review` → `draft` (changes requested)
- `approved` → `in_progress` (AI starts execution)
- `in_progress` → `completed` (all phases done, success)
- `in_progress` → `failed` (unrecoverable error)
- `in_progress` → `cancelled` (user cancels)
- `in_progress` → `in_progress` (can stay here if blocked; explain in logs)

---

## File Naming

**Convention:** `{task-id}.yaml`

**Task ID format:** Kebab-case, descriptive

**Good examples:**
- `add-user-metrics.yaml`
- `refactor-auth-module.yaml`
- `fix-chunk-deduplication.yaml`

**Bad examples:**
- `task-123.yaml` (not descriptive)
- `AddMetrics.yaml` (not kebab-case)
- `fix_bug.yaml` (underscore instead of dash)

---

## Metadata

Each spec includes metadata for tracking:

```yaml
created: "2025-01-17T10:30:00Z"   # When spec was generated
updated: "2025-01-17T14:20:00Z"   # Last modification
metadata:
  estimated_effort_hours: 3
  actual_effort_hours: 2.5       # Filled during/after execution
  tags:
    - "refactor"
    - "backend"
```

Use `updated` timestamp to determine archive month.

---

## Size & Risk Tiers

Use `task.size` and `task.risk_level` to tune planning and validation:

- `task.size`: `micro | small | medium | large`
  - `micro/small` tasks can use fewer phases and lighter validation
  - `medium/large` tasks should have more detailed phases and broader checks
- `task.risk_level`: `low | medium | high`
  - Guides which validation profile to apply (`light | standard | strict`)
  - By default: `low` → `light`, `medium` → `standard`, `high` → `strict`

For each spec, you can optionally set `task.acceptance.validation_profile` to
override the risk-based default. Profiles are defined in `.ai/config.yaml`.

---

## See Also

- [../.ai/README.md](../README.md) — Overview of the AI agent system
- [../.ai/prompts/plan.md](../prompts/plan.md) — Planning mode instructions
- [../.ai/prompts/exec.md](../prompts/exec.md) — Execution mode instructions
- [../.ai/schemas/spec.json](../schemas/spec.json) — Spec validation schema
