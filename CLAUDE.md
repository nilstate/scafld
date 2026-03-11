# Claude Code Integration Notes

Claude-specific tips for working with Trellis.

**MUST READ:** `AGENTS.md` - the canonical agent guide covering invariants, modes, validation, and conventions. Read it before doing any work.

## Tool Usage

- Always use `Read` before `Edit` to understand existing code and ensure correct string matching.
- Use `Grep` and `Glob` for codebase exploration instead of bash `find`/`grep`.
- Prefer `Edit` (targeted replacement) over `Write` (full file overwrite) for existing files.

## Entering Trellis Modes

- **Plan mode:** Read `.ai/prompts/plan.md`, then explore and generate a spec.
- **Exec mode:** Read `.ai/prompts/exec.md`, then load the approved spec and execute.

## Prompting Patterns

```
"Let's plan [feature]. Create a task spec."
"Execute the [task-id] spec."
"Show me the current phase status."
```
