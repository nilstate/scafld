# Trellis

A spec-driven development framework for AI coding agents. Every task becomes a machine-readable YAML specification before any code changes happen - structured, auditable, and validated at every step.

## How It Works

```text
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

## Setup

### Add Trellis as a submodule

```bash
cd your-project
git submodule add https://github.com/sourcey/trellis.git .trellis
```

### Bootstrap your workspace

```bash
.trellis/cli/trellis init
```

This sets up the workspace and installs `trellis` to `~/.local/bin/` so you can use it from anywhere:

```text
your-project/
  .trellis/                 # Trellis submodule (don't edit directly)
  .ai/
    config.yaml            -> .trellis/.ai/config.yaml (symlink, copy to customize)
    prompts/               -> .trellis/.ai/prompts/ (symlink)
    schemas/               -> .trellis/.ai/schemas/ (symlink)
    specs/
      drafts/              # Planning in progress
      approved/            # Ready for execution
      active/              # Currently executing
      archive/             # Completed work
    logs/                  # Execution logs (optional)
  AGENTS.md                # Customize: your project's invariants and policies
  CLAUDE.md                # Customize: project overview, essential commands
  CONVENTIONS.md           # Customize: tech stack, patterns, coding standards
```

### Customize for your project

The init command copies template files to your project root. These are yours to customize:

1. **AGENTS.md** - Add your project's architectural invariants, domain rules, and forbidden actions
2. **CONVENTIONS.md** - Add your tech stack, naming conventions, testing patterns, and commands
3. **CLAUDE.md** - Add project overview, essential commands, and agent-specific tips
4. **`.ai/config.yaml`** - Replace the symlink with a copy and update validation commands for your build/test/lint tools

### CLI

```bash
trellis new <task-id> [-t title] [-s size] [-r risk]   # Scaffold a new spec
trellis list [filter]                                    # List all specs
trellis status <task-id>                                 # Show spec details
trellis validate <task-id>                               # Validate against schema
trellis approve <task-id>                                # drafts/ -> approved/
trellis start <task-id>                                  # approved/ -> active/
trellis complete <task-id>                               # active/ -> archive/
trellis fail <task-id>                                   # active/ -> archive/ (failed)
trellis cancel <task-id>                                 # active/ -> archive/ (cancelled)
```

**Tip:** Add `.trellis/cli` to your PATH, or alias `trellis` to `.trellis/cli/trellis`.

### Start using it

Tell your AI agent: *"Let's plan [feature]. Create a task spec."*

## Workspace Architecture

Trellis is designed to be added as a **submodule** so your AI agent has a complete scaffold alongside your project code. The agent reads the prompts, schemas, and config from Trellis, while your project-specific policies live in the root-level AGENTS.md, CONVENTIONS.md, and CLAUDE.md.

```text
.trellis/                    # Framework (shared, versioned)
  .ai/config.yaml            # Default validation config
  .ai/prompts/               # Plan + exec mode instructions
  .ai/schemas/spec.json      # Spec validation schema
  cli/trellis                # CLI tool

your-project/                # Your code + customizations
  AGENTS.md                  # YOUR invariants, YOUR domain rules
  CONVENTIONS.md             # YOUR tech stack, YOUR patterns
  CLAUDE.md                  # YOUR commands, YOUR project overview
  .ai/specs/                 # YOUR task specs (gitignored or committed)
```

The submodule stays clean and updatable. Your customizations live in your repo.

## Key Features

- **Spec-driven** - every task is a versioned, schema-validated YAML artifact
- **Approval gate** - human reviews the plan before any code changes
- **Phase-by-phase execution** - with acceptance criteria at every checkpoint
- **Validation profiles** - light, standard, strict (configured in `config.yaml`)
- **Self-evaluation** - agents score work against a rubric (7/10 threshold)
- **Resume protocol** - interrupted executions can pick up where they left off
- **Rollback commands** - per-phase rollback for safe failure recovery
- **Agent-agnostic** - works with Claude, Cursor, Copilot, Windsurf, or any AI agent

## Trellis Structure

```text
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

cli/trellis             # CLI tool (Python 3, no deps)
AGENTS.md               # Canonical agent guide (all AI agents)
CLAUDE.md               # Claude Code specific notes
CONVENTIONS.md          # Coding standards template
```

## Documentation

| File | Audience | Purpose |
| ---- | -------- | ------- |
| [AGENTS.md](AGENTS.md) | AI agents | Canonical guide - invariants, modes, validation, conventions |
| [CLAUDE.md](CLAUDE.md) | Claude Code | Claude-specific tool tips |
| [CONVENTIONS.md](CONVENTIONS.md) | AI agents | Coding standards template |
| [.ai/config.yaml](.ai/config.yaml) | Both | All configuration in one place |
| [.ai/OPERATORS.md](.ai/OPERATORS.md) | Developers | Human cheat sheet for working with specs |

## License

MIT License - free to use, modify, and distribute.

## Contributing

Contributions welcome. Follow the spec-driven approach and document changes in commit messages.
