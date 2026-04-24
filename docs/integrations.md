---
title: Integrations
description: Thin first-party adapter paths for Codex and Claude Code
---

# Integrations

scafld stays provider-neutral in core.

The first-party integration layer is intentionally thin:

- `scripts/scafld-codex-build.sh <task-id>`
- `scripts/scafld-codex-review.sh <task-id>`
- `scripts/scafld-claude-build.sh <task-id>`
- `scripts/scafld-claude-review.sh <task-id>`

Each wrapper:

1. reads `scafld status --json`
2. resolves the current scafld handoff for the selected mode
3. pipes that handoff to the external agent runtime before the model acts

That is the whole point of the wrapper layer: make handoff consumption the
default path instead of an optional convention.

## What The Wrappers Do

For executor work:

- approved task -> resolve the current `executor × phase` handoff
- recovery state -> resolve the current `executor × recovery` handoff

For challenger review work:

- run `scafld review`
- resolve the `challenger × review` handoff
- pass that handoff to the reviewer runtime

For blocked review findings, the wrapper can pass the latest challenger handoff
back into the runtime so the executor has the exact review context in front of
it.

## What They Do Not Do

- they do not embed provider logic inside scafld core
- they do not replace `build`, `review`, or `complete`
- they do not introduce a second runtime state model

The stable contracts remain:

- `status --json`
- `handoff --json`
- `review --json`

## Provider Boundary

The wrappers assume the external binary can read prompt text from stdin.

Override the binary name with:

- `SCAFLD_CODEX_BIN`
- `SCAFLD_CLAUDE_BIN`

Everything provider-specific stays at the script layer.
