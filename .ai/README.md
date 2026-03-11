# Trellis — Planning & Execution Framework

**Version:** 1.0
**Status:** Active
**Documentation:** This directory contains the Trellis configuration system.

---

## Quick Start

### For AI Agents

**Entering Planning Mode:**
1. Read [`.ai/prompts/plan.md`](prompts/plan.md)
2. Load configuration from [`.ai/config.yaml`](config.yaml)
3. Generate spec file in `.ai/specs/drafts/{task-id}.yaml` with `status: "draft"` (or `"under_review"`) conforming to [`.ai/schemas/spec.json`](schemas/spec.json)

**Entering Execution Mode:**
1. Read [`.ai/prompts/exec.md`](prompts/exec.md)
2. Load approved spec from `.ai/specs/approved/{task-id}.yaml` (`status: "approved"`)
3. Move to `.ai/specs/active/` and set `status: "in_progress"`
4. Execute phases, update `phases[].status`, and log to `.ai/logs/{task-id}.log`

### For Developers

**Review a Plan:**
```bash
# View generated plan (draft or under_review)
cat .ai/specs/drafts/{task-id}.yaml

# Approve for execution
# Edit status field: status: "approved"
# Then move the file into the approved folder
mv .ai/specs/drafts/{task-id}.yaml .ai/specs/approved/{task-id}.yaml
```

**Monitor Execution:**
```bash
# View ReAct cycle logs
tail -f .ai/logs/{task-id}.log

# Check current phase status
yq '.phases[].status' .ai/specs/active/{task-id}.yaml
```

**Validate Spec:**
```bash
# If you have a JSON schema validator
ajv validate -s .ai/schemas/spec.json -d .ai/specs/{task-id}.yaml
```

---

## Directory Structure

```
.ai/
├── README.md              # This file
├── config.yaml            # Global agent configuration (modes, validation, rubrics)
├── prompts/
│   ├── plan.md           # Planning mode system prompt
│   └── exec.md           # Execution mode system prompt
├── schemas/
│   └── spec.json         # JSON schema for task specifications
├── playbooks/            # Reusable workflow templates (optional)
│   └── {domain}/
│       └── *.playbook.yml
├── specs/                # Task specifications (organized by status)
│   ├── README.md         # Spec workflow documentation
│   ├── drafts/           # Planning in progress (status: draft | under_review)
│   ├── approved/         # Ready for execution (status: approved)
│   ├── active/           # Currently executing (status: in_progress)
│   └── archive/          # Completed work (status: completed | failed | cancelled)
│       └── YYYY-MM/      # Organized by completion month
└── logs/                 # Execution logs (ReAct cycles)
    └── {task-id}.log
```

---

## Core Concepts

### 1. **ReAct Pattern** (Reasoning + Acting)

AI agents alternate between:
- **THOUGHT**: Analyze task, predict outcomes
- **ACTION**: Search codebase, apply changes, run tests
- **OBSERVATION**: Capture results, check outputs
- **THOUGHT**: Evaluate success, decide next step

Logged to `.ai/logs/{task-id}.log` for transparency.

### 2. **Spec-Driven Development**

Every task becomes a **machine-readable specification** (YAML):
- Task block capturing title, summary, context, objectives, scope
- Touchpoints, assumptions, risks, and acceptance checklist
- Definition-of-done checklist items with explicit `status` fields that are checked off during EXEC
- Planning log entries for conversational alignment
- Granular phases with acceptance criteria
- Rollback commands
- Self-evaluation rubric

Specs are versioned, diffable, and executable.

### 3. **Acceptance Criteria as Code**

Every phase has **deterministic validation**:
```yaml
acceptance_criteria:
  - id: ac1_1
    type: compile
    command: "your-compile-command"
    expected: "exit code 0"
  - id: ac1_2
    type: test
    command: "your-test-command --filter SomeSpec"
    expected: "PASS"
```

**Criterion Types:**

| Type | Purpose | Validation |
|------|---------|------------|
| `compile` | Code compiles/loads without errors | Automated (exit code 0) |
| `test` | Unit/integration tests pass | Automated (PASS) |
| `boundary` | No layer violations | Automated (no matches) |
| `integration` | E2E scenarios work | Automated (exit code 0) |
| `security` | No secrets/vulnerabilities | Automated (no matches) |
| `documentation` | Docs updated/added | Manual verification |
| `custom` | Project-specific checks | Manual or scripted |

### 4. **Mode Isolation**

**Planning Mode:**
- Input: User request (natural language)
- Output: Spec file (`.ai/specs/{task-id}.yaml`)
- Actions: Search, read, analyze (NO code changes)

**Execution Mode:**
- Input: Approved spec file
- Output: Code changes, test results, validation logs
- Actions: Edit files, run tests, validate checkpoints

Modes are **mutually exclusive** (enforced by prompts).

**Autonomous Execution:**

The approval gate separates **planning** from **execution**, not individual actions within execution:

- **Before execution begins:** User must review and approve the spec (move to `.ai/specs/approved/` + set `status: "approved"`)
- **During execution:** AI operates **autonomously** through all phases:
  - Executes changes per spec
  - Runs acceptance criteria
  - Self-heals failures (1 retry)
  - Updates spec file with results
  - Archives on completion
- **Pause only when:**
  - Blocking issue prevents progress
  - Deviation from approved spec is required
  - Destructive/irreversible action not covered by spec

This ensures **deterministic execution** while maintaining human oversight at the planning boundary.

### 5. **Self-Evaluation**

AI scores its work against a rubric (0-10 scale):
- **Completeness** (0-3): Meets requirements + edge cases
- **Architecture Fidelity** (0-3): Respects boundaries
- **Spec Alignment** (0-2): Matches approved plan
- **Validation Depth** (0-2): Adequate testing

**Threshold:** ≥7/10 required for completion.

---

## Workflow Example

### User Request
```
"Add structured error codes to the document processing module"
```

### Planning Phase (AI)

1. **ReAct Exploration:**
   - Search for similar error patterns
   - Read existing implementations
   - Analyze architecture

2. **Shape Task Spec:**
   - Capture `task.context` (packages/files), objectives, and scope boundaries
   - Enumerate touchpoints, assumptions, and risks
   - Define acceptance checklist + validation commands, then outline phases

3. **Output:**
   ```
   ✓ Plan generated: .ai/specs/drafts/add-error-codes.yaml
     Status: draft
     Phases: 3
   ```

### Review & Approval (Developer)

```bash
# Review plan
cat .ai/specs/drafts/add-error-codes.yaml

# Approve
sed -i 's/status: draft/status: approved/' .ai/specs/drafts/add-error-codes.yaml
mv .ai/specs/drafts/add-error-codes.yaml .ai/specs/approved/
```

### Execution Phase (AI)

1. **Phase 1-N:** Execute changes per spec, run acceptance criteria
2. **Final Validation:** Full test suite, linters, security scan
3. **Self-Evaluation:** Score against rubric

---

## Configuration

### Global Settings (`.ai/config.yaml`)

Key sections:
- **Invariants:** Architectural rules and contracts
- **Modes:** Planning vs execution behavior
- **Validation:** Per-phase and pre-commit checks
- **Rubric:** Self-evaluation scoring weights
- **ReAct:** Reasoning pattern configuration

### Customization

**Add new validation check:**
```yaml
validation:
  per_phase:
    - id: custom_check
      type: command
      command: "make custom-lint"
      required: true
```

**Adjust rubric weights:**
```yaml
rubric:
  completeness:
    weight: 4  # increase importance
  threshold: 8  # raise quality bar
```

---

## Status Lifecycle

```
draft → under_review → approved → in_progress → completed
  ↓                                    ↓           ↓
(edit)                             (blocked)   failed
                                      ↓           ↓
                                  (resume)   cancelled
```

Tasks flow through this deterministic lifecycle, with each transition triggering specific actions.

---

## Best Practices

### For AI Agents

1. **Always start in PLAN mode** for non-trivial tasks (>10 lines, >1 file)
2. **Log all ReAct cycles** to `.ai/logs/{task-id}.log`
3. **Run acceptance criteria after every phase** (no skipping)
4. **Rollback on failure** (don't proceed with broken state)
5. **Update spec file** with execution results
6. **Self-evaluate honestly** (threshold enforcement prevents low-quality work)

### For Developers

1. **Review plans before approval** (check task block, touchpoints, phases, acceptance criteria)
2. **Monitor logs during execution** (`tail -f .ai/logs/{task-id}.log`)
3. **Validate spec files** (use JSON schema validator)
4. **Version control specs** (commit approved specs for audit trail)
5. **Iterate on config** (adjust rubrics, validation checks as team learns)

---

## Advantages Over Traditional Approaches

| Aspect | Traditional | Trellis |
|--------|-------------|---------------------------|
| Format | Freeform prose | Machine-readable YAML |
| Planning | Ad-hoc text | Conversational task artifact |
| Validation | Manual checklists | Executable acceptance criteria |
| Rollback | Ad-hoc git commands | Per-phase automated rollback |
| Audit trail | None | Full ReAct logs + spec versioning |
| Determinism | Low (ambiguous) | High (schema-validated specs) |
| Self-evaluation | Optional | Mandatory, scored rubric |

---

## Troubleshooting

### "Spec status is not approved"
**Fix:** Edit `.ai/specs/{task-id}.yaml` and set `status: "approved"`

### "Acceptance criterion failed"
**Check logs:** `.ai/logs/{task-id}.log` for error details
**Fix:** AI will attempt self-healing (1 retry) or rollback automatically

### "PERF-EVAL below threshold"
**Review:** Check `self_eval.notes` in spec file for gaps
**Action:** AI performs second pass automatically if `experimental.self_healing: true`

### "Deviation from spec"
**Review:** Check `deviations[]` array in spec file
**Approve:** Add `approved_by: "username"` to deviation entry

---

## See Also

- [CONVENTIONS.md](../CONVENTIONS.md) — Coding standards and patterns
- [AGENTS.md](../AGENTS.md) — High-level AI agent policies
- [CLAUDE.md](../CLAUDE.md) — Claude-specific integration guide

---

## License

MIT License - Free to use, modify, and distribute.
