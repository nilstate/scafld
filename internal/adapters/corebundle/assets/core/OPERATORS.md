# scafld — Operator Cheat Sheet

The short version:

- `spec` is the contract
- `session` is the ledger
- `finalize` is the default agent gate
- `scafld verify` is the CI merge wall

## Default Agent And CI Path

Agents should work normally, then call `finalize`. A passing finalize returns a
signed receipt. CI should verify that receipt with:

```bash
scafld verify .scafld/receipts/latest.json --target <commit-ish>
```

## Optional Operator Lifecycle

The full lifecycle is still useful for operators, debugging, and direct
human-controlled work:

```bash
scafld plan my-task --title "My task" --size small --risk low
scafld harden my-task
scafld harden my-task --mark-passed
scafld approve my-task
scafld build my-task
scafld review my-task
scafld complete my-task
scafld status my-task
scafld handoff my-task
scafld report
```

## When To Use What

- `finalize`: default agent-facing acceptance and review receipt
- `verify`: recompute and enforce the signed receipt in CI
- `plan`: create the draft
- `harden`: stress-test the draft before approval
- `approve`: human ratifies the contract
- `build`: start approved work and drive validation to the next handoff or block
- `review`: run the adversarial review gate
- `complete`: archive only after the review finalize passes

Use `scafld config` after init or when project policy changes. It proposes
config from cited repo evidence; it is not part of the normal task lifecycle.

Prompt ownership:

- `.scafld/prompts/*` is the active template layer
- `.scafld/core/prompts/*` is the managed reset copy

`scafld update` refreshes managed core assets, installs optional lifecycle
helper scripts, and updates existing manifest-backed prompt copies. Customized
project prompts are skipped. It also refreshes root agent docs and renders
generated `.scafld/config.yaml` into the current strict runtime shape.

## Review Providers

Real review should use an external challenger:

```bash
scafld review my-task --provider codex
scafld review my-task --provider claude
scafld review my-task --provider gemini
scafld review my-task --provider command --provider-command "./reviewer"
```

`--provider local` is for development smoke tests, not production review, and
local verdicts cannot satisfy `scafld complete`.

## Metrics

Use `scafld report` to track:

- first-attempt pass rate
- recovery convergence rate
- challenge override rate

If those do not move, the value layer is not helping enough.
