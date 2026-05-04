---
title: Review
description: Adversarial review gate behavior
---

# Review

`review` is the load-bearing gate in scafld.

Execution tries to finish the job. Review tries to break confidence in the job.
The implementation agent should not grade its own work.

## Run Review

```bash
scafld review <task-id>
```

By default, review uses the provider configured in `.scafld/config.yaml` at
`review.external.provider`. Fresh workspaces use `auto`: scafld looks for an
installed external challenger and chooses `codex` first, then `claude`. If
neither is available, review fails closed. That is intentional; a missing
challenger should not silently become a clean review.

Provider-specific model defaults also come from config:

```yaml
review:
  external:
    provider: "auto"
    codex:
      model: "gpt-5.5"
    claude:
      model: "claude-opus-4-7"
```

Use `.scafld/config.local.yaml` for local-only provider or model overrides.

Explicit providers:

```bash
scafld review <task-id> --provider codex
scafld review <task-id> --provider claude
scafld review <task-id> --provider command --provider-command "./reviewer"
scafld review <task-id> --provider local
scafld review <task-id> --provider codex --model gpt-5
```

Provider meanings:

- `codex`: read-only ephemeral Codex review using a structured output schema.
- `claude`: Claude review with restricted read-only tools and stream-json
  output.
- `command`: custom reviewer command. It receives the review prompt on stdin and
  must emit a ReviewPacket-compatible response.
- `local`: deterministic local pass-through provider for development and smoke
  tests. It is not an adversarial review.

## What scafld Sends

The reviewer receives a task contract, acceptance evidence, and a clear
read-only instruction. It also receives the configured review agenda from
`review.automated_passes` and `review.adversarial_passes`, ordered by each
pass's `order` field. The prompt tells the challenger not to mutate the
workspace and to return a ReviewPacket verdict.

The packet is the provider content contract:

```json
{
  "verdict": "pass",
  "findings": []
}
```

Findings require:

- `id`
- `severity`: `blocking` or `non_blocking`
- `summary`

Any blocking finding forces verdict `fail`.

## What scafld Trusts

scafld validates the packet, checks that the workspace did not mutate during
review, records the review event in session, then projects the verdict back into
the spec.

The authority order stays the same:

- session stores evidence
- spec shows the readable current projection
- provider output is accepted only after validation

Invalid packet output fails review. Workspace mutation during review becomes a
blocking finding, even if the provider returned `pass`.

## Complete Gate

```bash
scafld complete <task-id>
```

`complete` refuses unless:

- review has completed
- the review verdict is `pass`

If review fails, repair the work, rerun acceptance as needed, rerun review, then
complete only after the challenger clears the gate.

## Challenger Stance

A useful adversarial review:

- attacks the diff, not just the prose
- attacks the spec contract and acceptance evidence
- cites concrete files, commands, or spec sections
- separates blocking findings from advisory findings
- says `pass` only when the evidence holds

Generic clean notes are not useful. A clean review should still explain what was
checked and why that was enough.

## Diagnostics

External providers run through the process runner with timeout and idle-timeout
protection. Provider failures and timeouts write diagnostics under:

```text
.scafld/runs/<task-id>/diagnostics/
```

Use those diagnostics to inspect paid model output that could not be accepted
as a valid review packet.
