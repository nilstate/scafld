# scafld

[![Review Gate Smoke](https://github.com/nilstate/scafld/actions/workflows/review-gate-smoke.yml/badge.svg)](https://github.com/nilstate/scafld/actions/workflows/review-gate-smoke.yml)

An opinionated orchestration layer for AI coding agents.

Canonical repo: `https://github.com/nilstate/scafld`. Default branch: `main`.

Most AI coding tools let agents jump straight into your codebase and start writing. No plan. No review. No audit trail. Just vibes and a prayer.

The result is predictable: code that looks right, passes the tests, and slowly rots from the inside. Duplicated blocks. Architectural drift. Changes nobody asked for buried in changes somebody did. The agent ships fast and you spend the next week figuring out what it actually did.

scafld enforces a simple constraint: **think before you type**.

Every non-trivial task becomes a YAML specification before a single line of code changes. The spec defines what will change, in what order, with what acceptance criteria, and how to roll it back if it breaks. A human reviews and approves the spec. Only then does the agent execute - phase by phase, validated at every checkpoint, auditable after the fact.

This isn't a wrapper around a prompt. It's a development methodology - the same separation of planning from execution that every serious engineering discipline has always required, applied to the one context where people have decided to skip it entirely.

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
 REVIEW             Adversarial self-review finds what
                    execution missed (ideally fresh session)
    |
    v
 Archive            Completed spec + audit trail
```

## Why This Exists

We built scafld because every AI coding workflow we used was broken in the same way. The agent would receive a task, immediately start modifying files, and produce something that was technically functional but architecturally thoughtless. Ask it to add a feature and it might refactor three other things along the way. Ask it to fix a bug and it might introduce a dependency you didn't want. There was no contract between what was requested and what was delivered, and no way to verify the difference after the fact.

The spec is the contract. It forces the planning to happen explicitly, in a format that a human can review and a machine can validate. It creates an audit trail that answers "what changed, why, and did it match what was agreed." It makes AI-assisted development reproducible instead of hopeful.

## Install

```bash
git clone https://github.com/nilstate/scafld.git ~/.scafld && ~/.scafld/install.sh
curl -fsSL https://raw.githubusercontent.com/nilstate/scafld/main/install.sh | sh
```

This clones scafld to `~/.scafld` and symlinks the `scafld` command to `~/.local/bin/`.

To update: `cd ~/.scafld && git pull origin main`

## Setup

```bash
cd your-project
scafld init
```

This scaffolds the full structure into your project:

```text
your-project/
  .ai/
    config.yaml            # Validation rules, rubric, safety controls
    config.local.yaml      # Your overrides (build/test/lint commands)
    prompts/               # Plan + exec mode instructions
    schemas/               # Spec validation schema
    specs/
      drafts/              # Planning in progress
      approved/            # Ready for execution
      active/              # Currently executing
      archive/             # Completed work
    logs/                  # Execution logs (gitignored)
  AGENTS.md                # Your project's invariants and policies
  CLAUDE.md                # Project overview, essential commands
  CONVENTIONS.md           # Tech stack, patterns, coding standards
```

### Make It Yours

1. **AGENTS.md** - Your architectural invariants, domain rules, forbidden actions
2. **CONVENTIONS.md** - Your tech stack, naming conventions, testing patterns
3. **CLAUDE.md** - Project overview, essential commands, agent-specific tips
4. **`.ai/config.local.yaml`** - Your build/test/lint commands (merges on top of config.yaml)

## Project Structure

scafld is opinionated about how your project should be organised, because the structure is what gives the AI agent visibility over your entire codebase.

### Single Repo

For a single codebase, just run `scafld init` at the root. The agent sees everything.

### Multi-Repo Workspace

For projects with multiple codebases - an API, a frontend, an SDK, an MCP server - the workspace pattern gives the agent visibility across all of them from a single root.

Create a root repo that acts as the orchestration layer. Add your codebases as git submodules underneath. Run `scafld init` at the root. Now the agent can see your specs, your conventions, your architectural invariants, AND all your code - in one context.

```bash
mkdir my-project && cd my-project
git init
git submodule add git@github.com:org/api.git api
git submodule add git@github.com:org/app.git app
git submodule add git@github.com:org/sdk.git sdk
scafld init
```

```text
my-project/                # Root workspace repo
  .ai/                     # scafld config and specs
  AGENTS.md                # Cross-project invariants
  CLAUDE.md                # Agent overview of the whole system
  CONVENTIONS.md           # Shared coding standards
  api/                     # Submodule: your API
  app/                     # Submodule: your frontend
  sdk/                     # Submodule: your SDK
```

The root repo is lightweight - it holds the orchestration layer (scafld files, agent docs) and pointers to the real code. Each submodule is still its own repo with its own history. But the agent sees the whole picture from the root, which means it can plan changes that span multiple codebases and understand how they connect.

This is how we work. It's not the only way, but if you're running AI agents across multiple repos without a unified root, you're asking the agent to plan with half the context.

## CLI

```bash
scafld new <task-id> [-t title] [-s size] [-r risk]     # Scaffold a new spec
scafld list [filter]                                    # List all specs
scafld status <task-id>                                 # Show spec details
scafld validate <task-id>                               # Validate against schema
scafld approve <task-id>                                # Validate + move to approved
scafld start <task-id>                                  # Move to active
scafld exec <task-id> [-p phase] [-r]                    # Run acceptance criteria (-r = resume)
scafld audit <task-id> [-b base-ref]                    # Spec vs actual git diff
scafld diff <task-id>                                   # Git history for a spec
scafld review <task-id>                                 # Run configured automated passes + scaffold Review Artifact v3
scafld complete <task-id>                                # Read review, record verdict, archive (requires passing review)
scafld complete <task-id> --human-reviewed --reason "manual audit"
                                                          # Exceptional audited override when the gate is blocked
scafld fail <task-id>                                   # Archive as failed
scafld cancel <task-id>                                 # Archive as cancelled
scafld report                                           # Aggregate stats
```

### Per-Criterion Working Directory

In monorepo/workspace setups, different acceptance criteria may target different submodules. Use the optional `cwd` field to set the working directory for a command, relative to the workspace root:

```yaml
acceptance_criteria:
  - id: ac1
    type: test
    cwd: api
    command: "bundle exec rspec spec/services/"
    expected: "0 failures"
  - id: ac2
    type: test
    cwd: app
    command: "yarn test"
    expected: "0 failures"
```

Commands without `cwd` run from the workspace root. The path must be relative and must resolve within the workspace — paths that escape the root are rejected.

You can also set a **spec-level default** under `task.context.cwd` so you don't repeat it on every criterion:

```yaml
task:
  context:
    cwd: api
    packages:
      - app/services
```

Individual criteria can still override with their own `cwd`.

### Per-Criterion Timeout

Acceptance criteria default to a 600 second timeout. Long-running checks can override that with `timeout_seconds`:

```yaml
acceptance_criteria:
  - id: ac3
    type: test
    cwd: api
    command: "bundle exec rspec"
    expected: "0 failures"
    timeout_seconds: 900
```

Use specific expectations like `0 failures` or `exit code 0` when possible. Generic phrases like `All pass` are accepted, but the explicit forms are easier for scafld to verify and for humans to audit.

## Usage

Tell your AI agent: *"Let's plan [feature]. Create a task spec."*

The agent enters read-only planning mode, explores your codebase, and produces a YAML spec with objectives, phases, acceptance criteria, and rollback commands. You review it, approve it, and the agent executes autonomously within those bounds.

## What It Actually Does

- **Spec-driven** - Every task is a versioned, schema-validated YAML artifact. Not a prompt. Not a conversation. A machine-readable contract.
- **Approval gate** - No code changes until a human reviews the plan. The agent thinks; you decide.
- **Phase-by-phase execution** - Acceptance criteria at every checkpoint, not just at the end.
- **Scope audit** - `scafld audit` compares what the spec declared against what actually changed in git. Undeclared changes get flagged.
- **Adversarial review** - Before archiving, `scafld review` runs the configured `spec_compliance` and `scope_drift` passes, scaffolds Review Artifact v3, and prepares the adversarial `regression_hunt`, `convention_check`, and `dark_patterns` sections. `scafld complete` requires a structurally valid latest review or an exceptional human-reviewed override with an audited reason.
- **Self-evaluation** - Agents score their own work against a configurable rubric. Below 7/10 triggers a second pass.
- **Rollback commands** - Per-phase rollback for safe failure recovery. Every phase declares how to undo itself.
- **Resume protocol** - Interrupted executions pick up where they left off.
- **Validation profiles** - Light, standard, or strict, configured per-task or derived from risk level.
- **Reporting** - `scafld report` aggregates pass rates, self-eval scores, and scope drift across your entire spec history.
- **Agent-agnostic** - Works with Claude, Cursor, Copilot, Windsurf, or any AI coding agent.

## Review Pipeline

The default review model is a five-pass pipeline declared in [.ai/config.yaml](.ai/config.yaml):

- `spec_compliance`
- `scope_drift`
- `regression_hunt`
- `convention_check`
- `dark_patterns`

Pass ordering is explicit through per-pass `order` fields, so the review pipeline does not depend on YAML mapping order. `scafld review` scaffolds Review Artifact v3 with per-pass `pass_results` and `round_status: "in_progress"`. The reviewer fills the configured adversarial sections, updates the metadata to `round_status: "completed"`, and sets final pass results before `scafld complete` archives the spec.

## Trust Boundary

scafld now enforces a materially stronger local review workflow, but local CLI checks are still not the whole trust boundary.

For best-in-class review governance, add the next layer outside the agent session:

- **CI or merge gate** validates the latest review artifact before code lands
- **Diff or commit binding** ties the review artifact to the exact reviewed diff or commit
- **External reviewer driver** runs the adversarial review from a configurable tool or service instead of trusting the executor path alone
- **Out-of-band approval** moves human override out of the terminal session and into a separate approval surface

## Documentation

| File | Audience | Purpose |
| ---- | -------- | ------- |
| [AGENTS.md](AGENTS.md) | AI agents | Invariants, modes, validation, conventions |
| [CLAUDE.md](CLAUDE.md) | Claude Code | Claude-specific tool tips |
| [CONVENTIONS.md](CONVENTIONS.md) | AI agents | Coding standards template |
| [.ai/config.yaml](.ai/config.yaml) | Both | All configuration in one place |
| [.ai/OPERATORS.md](.ai/OPERATORS.md) | Developers | Human cheat sheet for working with specs |

## License

MIT

## Contributing

Contributions welcome. Follow the spec-driven approach - practice what we preach.

---

Built by [Sourcey](https://sourcey.com). We build AI infrastructure that works in production, not in pitch decks.
