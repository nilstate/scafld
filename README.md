# Trellis

A spec-driven development framework for AI coding agents. Structured, auditable, and deterministic AI-assisted development.

## Overview

Trellis provides:

- **Planning Mode** — AI explores codebase and generates machine-readable task specifications
- **Execution Mode** — AI executes approved specs with validation at every checkpoint
- **Self-Evaluation** — AI scores its work against a rubric to maintain quality
- **Audit Trail** — Full logging and versioned specs for transparency

## Quick Start

### 1. Set Up Your Project

Copy the `.ai/` directory to your project root and customize:

```bash
# Copy the engine
cp -r .ai/ /path/to/your/project/.ai/

# Customize configuration
edit /path/to/your/project/.ai/config.yaml
```

### 2. Customize Configuration

Edit `.ai/config.yaml` to match your project:

```yaml
# Update tech stack
tech_stack:
  backend:
    language: "Your language"
    framework: "Your framework"

# Update validation commands
validation:
  per_phase:
    - id: compile_check
      command: "your-compile-command"
    - id: targeted_tests
      command: "your-test-command {spec_pattern}"
```

### 3. Use the System

**For AI Agents:**

1. Read `.ai/prompts/plan.md` to enter planning mode
2. Generate spec in `.ai/specs/drafts/{task-id}.yaml`
3. After approval, read `.ai/prompts/exec.md` to enter execution mode
4. Execute phases, validate, and archive

**For Developers:**

1. Review generated specs before approval
2. Move approved specs: `drafts/` → `approved/`
3. Monitor execution: `tail -f .ai/logs/{task-id}.log`
4. Archive completed specs to `archive/YYYY-MM/`

## Directory Structure

```
.ai/
├── README.md              # System overview
├── OPERATORS.md           # Human cheat sheet
├── config.yaml            # Configuration and validation rules
├── prompts/
│   ├── plan.md           # Planning mode instructions
│   └── exec.md           # Execution mode instructions
├── schemas/
│   └── spec.json         # JSON schema for specifications
├── playbooks/            # Reusable workflow templates (optional)
├── specs/                # Task specifications
│   ├── README.md         # Spec workflow documentation
│   ├── drafts/           # Planning in progress
│   ├── approved/         # Ready for execution
│   ├── active/           # Currently executing
│   └── archive/          # Completed work
└── logs/                 # Execution logs
```

## Key Concepts

### Spec-Driven Development

Every task becomes a machine-readable YAML specification:
- Task context, objectives, and scope
- Acceptance criteria as runnable commands
- Phased execution with validation checkpoints
- Rollback commands for safety

### ReAct Pattern

AI agents follow the ReAct (Reasoning + Acting) pattern:
1. **THOUGHT** — Analyze the task
2. **ACTION** — Search, read, or modify
3. **OBSERVATION** — Capture results
4. **THOUGHT** — Decide next step

All reasoning is logged for transparency.

### Mode Isolation

- **Planning Mode** — Read-only exploration, outputs spec file
- **Execution Mode** — Follows approved spec, modifies code

Modes are mutually exclusive; the approval gate separates them.

### Self-Evaluation

AI scores work on a 0-10 scale:
- Completeness (0-3)
- Architecture Fidelity (0-3)
- Spec Alignment (0-2)
- Validation Depth (0-2)

Threshold of 7/10 required for completion.

## Documentation

- [AGENTS.md](AGENTS.md) — AI agent policies and invariants
- [CONVENTIONS.md](CONVENTIONS.md) — Coding standards template
- [CLAUDE.md](CLAUDE.md) — Claude-specific integration guide
- [.ai/README.md](.ai/README.md) — Detailed system documentation
- [.ai/OPERATORS.md](.ai/OPERATORS.md) — Quick reference for operators
- [.ai/specs/README.md](.ai/specs/README.md) — Spec workflow documentation

## Workflow

### Task Lifecycle

```
User Request
    ↓
┌─────────────────┐
│  PLANNING MODE  │  AI generates spec
│  (read-only)    │  ← .ai/prompts/plan.md
└────────┬────────┘
         ↓
    Draft Spec (.ai/specs/drafts/)
         ↓
    Human Review
         ↓
    Approved Spec (.ai/specs/approved/)
         ↓
┌─────────────────┐
│ EXECUTION MODE  │  AI executes spec
│ (autonomous)    │  ← .ai/prompts/exec.md
└────────┬────────┘
         ↓
    Phase 1 → Validate → Phase 2 → ... → Final Validation
         ↓
    Completed Spec (.ai/specs/archive/)
```

### Spec Status Lifecycle

```
draft → under_review → approved → in_progress → completed
                                      ↓           ↓
                                   (blocked)   failed
                                                  ↓
                                              cancelled
```

## Validation Profiles

| Profile | Use Case | Checks |
|---------|----------|--------|
| `light` | micro/small, low-risk | compile, acceptance items |
| `standard` | typical features | compile, tests, lint, typecheck, security |
| `strict` | high-impact changes | all standard + broader coverage |

## Best Practices

### For AI Agents

1. Always start in PLAN mode for non-trivial tasks
2. Read code before editing
3. Follow the spec exactly (deviations need approval)
4. Run acceptance criteria after every phase
5. Log all reasoning transparently
6. Self-evaluate honestly

### For Developers

1. Review specs thoroughly before approval
2. Monitor logs during execution
3. Validate specs against the JSON schema
4. Commit specs for audit trail
5. Iterate on config as team learns

## Customization

### Adding Invariants

Edit `.ai/config.yaml`:

```yaml
invariants:
  canonical:
    - domain_boundaries
    - your_custom_invariant
```

### Adding Validation Checks

```yaml
validation:
  per_phase:
    - id: your_check
      type: command
      command: "your-validation-command"
      required: true
```

### Adjusting Rubric

```yaml
rubric:
  completeness:
    weight: 4  # increase importance
  threshold: 8  # raise quality bar
```

## License

MIT License - Free to use, modify, and distribute.

---

## Contributing

Contributions welcome! Please:

1. Follow the spec-driven approach
2. Document changes in commit messages
3. Update relevant documentation
4. Test your changes

## Support

For issues and feature requests, please open a GitHub issue.
