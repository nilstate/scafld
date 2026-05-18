---
title: Integrations
description: Thin provider adapter paths for Codex, Claude Code, and Gemini CLI
---

# Integrations

scafld stays provider-neutral in core.

The first-party integration layer is intentionally thin:

- `.scafld/core/scripts/scafld-codex-build.sh <task-id>`
- `.scafld/core/scripts/scafld-codex-review.sh <task-id>`
- `.scafld/core/scripts/scafld-claude-build.sh <task-id>`
- `.scafld/core/scripts/scafld-claude-review.sh <task-id>`

Each wrapper:

1. reads `scafld status --json`
2. resolves the current scafld handoff for the selected mode
3. pipes that handoff to the external agent runtime before the model acts

That is the whole point of the wrapper layer: expose provider-specific handoff
adapters without turning wrapper behavior into the core lifecycle contract.

## What The Wrappers Do

For build work:

- approved task -> resolve the current `build × phase` handoff
- recovery state -> resolve the current `build × recovery` handoff

For challenger review work:

- run `scafld review`
- scafld itself spawns the configured challenger provider
- Codex review runs with scafld's read-only ephemeral subprocess settings; user
  config and execpolicy rules are disabled for that review subprocess
- Claude review disables session persistence, slash commands, and browser
  integration, restricts built-in tools to `Read`, `Grep`, and `Glob`, and
  accepts the final verdict only through scafld's `submit_review` MCP tool.
- Gemini review runs in plan mode with a temporary scafld-owned settings file
  that exposes only the `submit_review` MCP tool for the final verdict
- Codex review requests `gpt-5.5` by default so review uses the strongest
  available Codex model unless configured otherwise
- Claude review requests `claude-opus-4-7` by default unless configured
  otherwise
- Gemini review uses Gemini CLI's configured default model unless
  `review.external.gemini.model` is set. It runs in plan mode with a temporary
  scafld MCP settings file and must submit through `submit_review`
- `review.external.fallback_policy: "disable"` prevents `provider: auto` from
  falling back from the preferred independent challenger to the current host
  agent; `warn` and `allow` both allow the fallback

For blocked review findings, the wrapper can pass the latest challenger handoff
back into the runtime so the executor has the exact review context in front of
it.

## What They Do Not Do

- they do not embed provider logic inside scafld core
- they do not replace `build`, `review`, or `complete`
- they do not introduce a second runtime state model

The stable contracts remain:

- `status --json`
- `review --json`
- `handoff`

## Provider Boundary

The wrappers assume the external binary can read prompt text from stdin.

Override provider, binary, and model in `.scafld/config.yaml`,
`.scafld/config.local.yaml`, or one-shot CLI flags such as `--provider`,
`--provider-binary`, and `--model`.

Everything provider-specific stays at the script layer. The default review path
now lives in `scafld review`; wrappers are optional transport.
