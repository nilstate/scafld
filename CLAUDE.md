# Claude Code Integration Notes

Read `AGENTS.md` first.

The short version:

- `spec` is the contract
- `session` is the durable run ledger
- `review` is the adversarial gate

## Default Command Surface

Prefer these commands in prompts and automation:

```bash
scafld plan <task-id>
scafld approve <task-id>
scafld build <task-id>
scafld review <task-id>
scafld complete <task-id>
scafld status <task-id>
scafld handoff <task-id>
scafld report
```

Legacy commands still work, but they are not the taught surface.

## Prompting Patterns

```text
Plan the task as a scafld spec.
Approve is done; build the task.
Run the adversarial review gate.
Show the current handoff.
Show the current task status.
```

## Tooling Notes

- Read the generated handoff before editing code.
- Use `scafld handoff <task-id>` when you need the current executor or challenger brief without moving lifecycle state.
- `build` starts approved work, then advances active work through validation on later calls.
- `complete` is expected to fail when the challenger blocks. That is normal; fix the issues or use the audited human override path only when justified.
