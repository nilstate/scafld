# scafld — Operator Cheat Sheet

The short version:

- `spec` is the contract
- `session` is the ledger
- `review` is the provider/model gate; `finalize` is the deterministic final gate
- `scafld verify` is the CI merge wall

## Default Agent And CI Path

Agents should work normally, call `review`, then call `finalize`. A passing
finalize returns a signed receipt. CI should verify that receipt with:

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
scafld build my-task --criterion <id> --disposition pass --evidence-digest <sha256> --actor <actor> --reason "<what was verified>"
scafld review my-task
scafld finalize my-task
scafld status my-task
scafld handoff my-task
scafld report
```

## When To Use What

- `review`: run the provider/model adversarial review and record accepted evidence
- `finalize`: consume accepted review evidence, run deterministic acceptance, and
  sign the receipt while archiving the canonical spec
- `verify`: recompute and enforce the signed receipt in CI
- `plan`: create the draft
- `harden`: stress-test the draft before approval
- `approve`: human ratifies the contract
- `build`: start approved work and drive validation to the next handoff or block
- `build`: attach a digest-bound operator disposition to a manual acceptance criterion and re-evaluate acceptance in the same invocation
- `complete`: legacy completion transition for older workflows

Use `scafld config` after init or when project policy changes. It proposes
config from cited repo evidence; it is not part of the normal task lifecycle.

Prompt ownership:

- embedded prompts are the runtime default
- `.scafld/core/prompts/*` is the managed visible copy refreshed by update
- `.scafld/prompts/*` overrides runtime only when the file contains
  `scafld:prompt-owner=project`

`scafld update` refreshes managed core assets, installs optional lifecycle
helper scripts, and updates unmarked prompt copies. Marker-bearing project
prompts are skipped. It also refreshes root agent docs and renders generated
`.scafld/config.yaml` into the current strict runtime shape.

## Review Providers

Real review should use an external challenger:

```bash
scafld review my-task --provider codex
scafld review my-task --provider claude
scafld review my-task --provider gemini
scafld review my-task --provider command --provider-command "./reviewer"
```

`--provider local` is for development smoke tests, not production review, and
local verdicts cannot satisfy `scafld finalize`.

## Metrics

Use `scafld report` to track:

- first-attempt pass rate
- recovery convergence rate
- challenge override rate

If those do not move, the value layer is not helping enough.
