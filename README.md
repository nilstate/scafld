# Trellis

A spec-driven development framework for AI coding agents. Every task becomes a machine-readable YAML specification before any code changes happen - structured, auditable, and validated at every step.

## How It Works

```
User Request
    |
    v
 PLAN MODE          AI explores codebase, generates spec
 (read-only)        .ai/specs/drafts/{task-id}.yaml
    |
    v
 Human Review       Developer reviews and approves
    |
    v
 EXEC MODE          AI executes spec phase-by-phase
 (autonomous)       with validation at every checkpoint
    |
    v
 Archive            Completed spec + audit trail
```

**Plan** - AI generates a YAML task spec with objectives, phases, acceptance criteria, and rollback commands.

**Review** - Developer reviews the spec and approves it.

**Execute** - AI executes the approved spec autonomously, running validation after each phase.

**Archive** - Completed specs are archived with full execution results and self-evaluation.

## Quick Start

### 1. Add to your project

```bash
cp -r .ai/ /path/to/your/project/.ai/
cp AGENTS.md CONVENTIONS.md /path/to/your/project/
# For Claude Code users:
cp CLAUDE.md /path/to/your/project/
```

### 2. Customize

Edit `.ai/config.yaml` - update validation commands, tech stack, and invariants for your project.

### 3. Use it

Tell your AI agent: *"Let's plan [feature]. Create a task spec."*

## Project Structure

```
.ai/
  config.yaml           # Validation rules, rubric, safety controls
  prompts/plan.md       # Planning mode instructions
  prompts/exec.md       # Execution mode instructions
  schemas/spec.json     # Spec validation schema
  specs/                # Task specs by lifecycle status
    examples/           # Example completed spec
    drafts/             # Planning in progress
    approved/           # Ready for execution
    active/             # Currently executing
    archive/            # Completed work
  OPERATORS.md          # Human cheat sheet

AGENTS.md               # Canonical agent guide (all AI agents)
CLAUDE.md               # Claude Code specific notes
CONVENTIONS.md          # Coding standards template
```

## Key Features

- **Spec-driven** - every task is a versioned, schema-validated YAML artifact
- **Approval gate** - human reviews the plan before any code changes
- **Phase-by-phase execution** - with acceptance criteria at every checkpoint
- **Validation profiles** - light, standard, strict (configured in `config.yaml`)
- **Self-evaluation** - agents score work against a rubric (7/10 threshold)
- **Resume protocol** - interrupted executions can pick up where they left off
- **Rollback commands** - per-phase rollback for safe failure recovery
- **Agent-agnostic** - works with Claude, Cursor, Copilot, Windsurf, or any AI agent

## Documentation

| File | Audience | Purpose |
|------|----------|---------|
| [AGENTS.md](AGENTS.md) | AI agents | Canonical guide - invariants, modes, validation, conventions |
| [CLAUDE.md](CLAUDE.md) | Claude Code | Claude-specific tool tips |
| [CONVENTIONS.md](CONVENTIONS.md) | AI agents | Coding standards template |
| [.ai/config.yaml](.ai/config.yaml) | Both | All configuration in one place |
| [.ai/OPERATORS.md](.ai/OPERATORS.md) | Developers | Human cheat sheet for working with specs |

## License

MIT License - free to use, modify, and distribute.

## Contributing

Contributions welcome. Follow the spec-driven approach and document changes in commit messages.
