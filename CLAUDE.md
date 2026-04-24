# Claude Code Integration Notes

Read `AGENTS.md` first.

The short version:

- `spec` is the contract
- `session` is the durable run ledger
- `review` is the adversarial gate

## Default Command Surface

Prefer these commands in prompts and automation:

```bash
scafld init
scafld plan <task-id>
scafld approve <task-id>
scafld build <task-id>
scafld review <task-id>
scafld complete <task-id>
scafld status <task-id>
scafld handoff <task-id>
scafld report
```

Advanced operator commands still exist behind `scafld --help --advanced`, but
they are not the taught surface.

When the workspace includes them, prefer these wrappers so the current handoff
is consumed before the model acts:

```bash
scripts/scafld-claude-build.sh <task-id>
scripts/scafld-claude-review.sh <task-id>
scripts/scafld-codex-build.sh <task-id>
scripts/scafld-codex-review.sh <task-id>
```

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
- Prefer `scripts/scafld-claude-build.sh <task-id>` when the workspace includes
  it; the wrapper resolves the current scafld handoff before Claude acts.
- Prefer `scripts/scafld-claude-review.sh <task-id>` for the adversarial review
  pass when the workspace includes it.
- Use `scafld handoff <task-id>` when you need the current executor or challenger brief without moving lifecycle state.
- `build` starts approved work, then advances active work through validation on later calls.
- `status` is the canonical next-step surface; read `next_action` and
  `current_handoff` instead of inferring lifecycle state yourself.
- `complete` is expected to fail when the challenger blocks. That is normal; fix the issues or use the audited human override path only after a completed challenger review round when justified.
