# scafld — Operator Cheat Sheet

The short version:

- `spec` is the contract
- `session` is the ledger
- `review` is the adversarial gate

## Default Commands

```bash
scafld plan my-task -t "My task" -s small -r low
scafld approve my-task
scafld build my-task
scafld review my-task
scafld complete my-task
scafld status my-task
scafld handoff my-task
scafld report
```

## When To Use What

- `plan`: create the draft or reopen harden on an existing draft
- `approve`: human ratifies the contract
- `build`: start approved work and drive validation to the next handoff or block
- `review`: emit the challenger handoff and run the review gate
- `complete`: archive only after the review gate passes

Prompt ownership:

- `.scafld/prompts/*` is the active template layer
- `.scafld/core/prompts/*` is the managed reset copy

## Override

Only for a blocked review gate:

```bash
scafld complete my-task --human-reviewed --reason "manual audit"
```

This is audited in both the review artifact and the session ledger.

## Metrics

Use `scafld report` to track:

- first-attempt pass rate
- recovery convergence rate
- challenge override rate

If those do not move, the value layer is not helping enough.
