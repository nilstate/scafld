# Claude Code Integration Guide

This guide explains how to use Claude Code effectively with Trellis.

## Overview

Claude Code works best with Trellis when you understand and leverage the spec-driven development workflow. This guide shows you how.

---

## The Trellis System

Trellis uses a **spec-driven approach** where tasks are planned, documented, and executed through machine-readable YAML specifications.

### Key Files to Know

| File | Purpose | When to Read |
|------|---------|--------------|
| `.ai/prompts/plan.md` | Planning mode instructions | Starting any non-trivial task |
| `.ai/prompts/exec.md` | Execution mode instructions | After a spec is approved |
| `.ai/config.yaml` | Validation rules and settings | Understanding project constraints |
| `.ai/schemas/spec.json` | Spec format definition | When creating or validating specs |
| `CONVENTIONS.md` | Coding standards | Before writing any code |
| `AGENTS.md` | AI policies and invariants | Understanding boundaries |

---

## How to Work with Claude Code

### For Quick Tasks (Trivial Changes)

For small, obvious changes (typos, simple fixes), you can work directly:

```
User: Fix the typo in README.md line 42
Claude: [Makes the fix directly]
```

### For Non-Trivial Tasks (Features, Refactors, Bugs)

Use the spec-driven workflow:

#### Step 1: Enter Planning Mode

Ask Claude to create a task spec:

```
User: I want to add user authentication to the API. Let's plan this.

Claude: I'll enter planning mode and create a task specification.
[Reads .ai/prompts/plan.md]
[Explores codebase]
[Generates spec in .ai/specs/drafts/add-user-auth.yaml]
```

#### Step 2: Review and Approve

Review the generated spec, then approve:

```
User: The spec looks good. Approve it.

Claude: [Moves spec to .ai/specs/approved/]
[Sets status to "approved"]
```

#### Step 3: Execute

Ask Claude to execute the approved spec:

```
User: Execute the add-user-auth spec.

Claude: [Reads .ai/prompts/exec.md]
[Moves spec to .ai/specs/active/]
[Executes phase by phase with validation]
```

---

## Effective Prompts

### Starting a Task

```
"Let's plan adding [feature]. Create a task spec."

"I need to refactor [module]. Start with a planning spec."

"There's a bug in [area]. Let's create a spec to fix it."
```

### During Planning

```
"What files would this change affect?"

"What are the risks with this approach?"

"Add an acceptance criterion for [specific behavior]."

"The scope should exclude [thing]."
```

### During Execution

```
"Execute the [task-id] spec."

"Show me the current phase status."

"What's blocking this phase?"

"Skip this phase and continue."
```

### Reviewing Work

```
"Show me what changed in phase 2."

"What's the self-evaluation score?"

"Were there any deviations from the spec?"
```

---

## Understanding the Spec Format

A task spec has these key sections:

```yaml
spec_version: "1.1"
task_id: "add-feature-name"
status: "draft"  # or approved, in_progress, completed, etc.

task:
  title: "Short description"
  summary: "What problem we're solving"
  size: "small"  # micro, small, medium, large
  risk_level: "low"  # low, medium, high
  context:
    packages: ["src/module/"]
    invariants: ["domain_boundaries"]
  objectives:
    - "What we want to achieve"
  acceptance:
    definition_of_done:
      - id: dod1
        description: "Checklist item"
        status: pending

phases:
  - id: phase1
    name: "Phase Name"
    objective: "What this phase accomplishes"
    changes:
      - file: "path/to/file"
        action: update
        content_spec: "Description of changes"
    acceptance_criteria:
      - id: ac1_1
        type: test
        command: "npm test"
        expected: "PASS"
```

---

## Validation Profiles

The system uses three validation profiles:

| Profile | When | Checks |
|---------|------|--------|
| `light` | Low-risk, micro/small tasks | Compile, acceptance items |
| `standard` | Medium-risk, typical features | + Tests, lint, typecheck, security |
| `strict` | High-risk, large changes | All checks with broader coverage |

The profile is determined by:
1. Explicit `task.acceptance.validation_profile` setting
2. Or derived from `task.risk_level`: low→light, medium→standard, high→strict

---

## Self-Evaluation

Claude scores work on a 0-10 rubric:

| Dimension | Max | What It Measures |
|-----------|-----|------------------|
| Completeness | 3 | Requirements met, edge cases, conventions |
| Architecture | 3 | Respects boundaries, uses patterns |
| Spec Alignment | 2 | Matches approved plan |
| Validation | 2 | Has tests, runs checks |

**Threshold: 7/10** — If below, Claude does a second pass.

---

## When Things Go Wrong

### Acceptance Criteria Fails

Claude will:
1. Attempt self-healing (1 retry)
2. If still failing, rollback the phase
3. Report the issue and ask for guidance

### Blocked Execution

Claude will report:
- What's blocking
- Recommendations
- Options to proceed

### Deviation Needed

If the spec needs to change during execution:
1. Claude will pause
2. Document the proposed deviation
3. Ask for approval before continuing

---

## Tips for Best Results

### 1. Be Specific About Scope

```
Good: "Add a logout button to the header that clears the session"
Bad: "Add logout"
```

### 2. Let Planning Happen

Don't rush to implementation. The planning phase:
- Identifies risks early
- Catches missing requirements
- Creates clear acceptance criteria

### 3. Review Specs Carefully

Before approving, check:
- [ ] Objectives match your intent
- [ ] Scope is correct (in/out)
- [ ] Acceptance criteria are complete
- [ ] Phases make sense

### 4. Trust the Process

The spec-driven approach may feel slower initially, but it:
- Reduces rework
- Catches errors early
- Creates audit trail
- Maintains code quality

### 5. Iterate on Config

If validation checks don't fit your project:
- Update `.ai/config.yaml`
- Add project-specific criteria
- Adjust the rubric weights

---

## Quick Commands

```bash
# View current draft specs
ls .ai/specs/drafts/

# View approved specs
ls .ai/specs/approved/

# Monitor execution
tail -f .ai/logs/{task-id}.log

# View active task
cat .ai/specs/active/{task-id}.yaml

# Recent completions
ls .ai/specs/archive/$(date +%Y-%m)/
```

---

## Example Session

```
User: I want to add rate limiting to the API. Let's plan this properly.

Claude: I'll enter planning mode to create a comprehensive task specification
for adding rate limiting.

[Reads .ai/prompts/plan.md]
[Explores codebase for existing patterns]
[Analyzes middleware structure]

Based on my analysis, I'll create a spec with:
- Context: API middleware layer
- Objectives: Add configurable rate limiting
- Phases: 1) Add rate limiter, 2) Configure routes, 3) Add tests

[Writes spec to .ai/specs/drafts/add-rate-limiting.yaml]

✓ Plan generated: .ai/specs/drafts/add-rate-limiting.yaml
  Status: draft
  Size: medium
  Risk: medium
  Phases: 3

Please review the spec and let me know if you'd like any changes,
or say "approve" to proceed to execution.

User: The spec looks good. Approve and execute.

Claude: [Approves spec, moves to approved/]
[Enters execution mode, reads .ai/prompts/exec.md]
[Moves to active/, sets status: in_progress]

→ Phase 1/3: Add rate limiter middleware
  Objective: Create configurable rate limiting middleware
  Changes: 2 files

[Applies changes]
[Runs acceptance criteria]

✓ Phase 1: Add rate limiter | 3/3 criteria ✓ | Next: Phase 2

→ Phase 2/3: Configure routes
...

✓ Task complete: add-rate-limiting
  Phases: 3/3 completed
  Acceptance: 8/8 passed
  PERF-EVAL: 8/10
  Files changed: 4
```

---

## Summary

1. **Use specs** for non-trivial work
2. **Review before approving** — the spec is the contract
3. **Trust the validation** — acceptance criteria catch errors
4. **Check the logs** — all reasoning is recorded
5. **Iterate** — adjust config as you learn what works

Trellis keeps AI-assisted development structured, auditable, and high-quality.
