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

Use `scafld review <task-id>` as the default challenger entrypoint. When the
workspace includes them, the wrappers remain optional provider-specific handoff
adapters:

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
- Prefer `scafld review <task-id>` first. Use `scripts/scafld-claude-review.sh <task-id>`
  only when you explicitly want the claude handoff adapter path.
- Use `scafld handoff <task-id>` when you need the current executor or challenger brief without moving lifecycle state.
- `build` starts approved work, then advances active work through validation on later calls.
- `status` is the canonical next-step surface; read `next_action` and
  `current_handoff` instead of inferring lifecycle state yourself.
- `complete` is expected to fail when the challenger blocks. That is normal; fix the issues or use the audited human override path only after a completed challenger review round when justified.

## Review iteration: single-round-by-default

`scafld complete` blocks only on blocking (high / critical) findings.
Verdict `pass` or `pass_with_issues` ships in one review round.
Medium and low findings are advisory output, not iteration triggers.

To opt into strict iteration, set `review.gate_severity: medium` (or
`low`) in `.ai/config.yaml`; non-blocking findings at or above that
severity will start gating `complete` too. See
`docs/configuration.md#review-gate-severity`.

When iteration is bounded by blocking findings, the agent may still
choose to address advisory findings — fix the cheap ones inline,
defer marginal ones to follow-up specs. Record skipped findings +
rationale in the spec's `planning_log` before `scafld complete` so
the audit trail names what was deferred and why.
