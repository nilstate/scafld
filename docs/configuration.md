---
title: Configuration
description: Minimal runtime config for model profile, context budget, and recovery cap
---

# Configuration

scafld merges:

- `.ai/config.yaml`
- `.ai/config.local.yaml`

The local file overlays the committed base file.

## Minimal LLM Surface

v1 keeps the runtime surface intentionally small:

```yaml
llm:
  model_profile: "default"
  context:
    budget_tokens: 12000
  recovery:
    max_attempts: 1
```

Meaning:

- `model_profile`: label written into handoffs and session
- `context.budget_tokens`: renderer hint for how much context to pack
- `recovery.max_attempts`: hard cap for executor recovery handoffs

If an external wrapper knows token or cost usage, it may write optional `usage`
fields into session. scafld does not require that data.

## Review Topology

Review ordering stays explicit:

```yaml
review:
  automated_passes:
    spec_compliance:
      order: 10
    scope_drift:
      order: 20
  adversarial_passes:
    regression_hunt:
      order: 30
    convention_check:
      order: 40
    dark_patterns:
      order: 50
```

Configured titles and order flow into both:

- the review scaffold under `.ai/reviews/`
- the generated challenger handoff

## Review Runner

The review runner contract stays narrow:

```yaml
review:
  runner: "external"     # external | local | manual
  external:
    provider: "auto"     # auto | codex | claude
    timeout_seconds: 600
    fallback_policy: "warn" # warn | allow | disable
    codex:
      model: ""
    claude:
      model: ""
```

Meaning:

- `runner`: default review execution mode
- `external.provider`: provider selection for external review; `auto` prefers
  `codex` first, then `claude`
- `external.timeout_seconds`: maximum provider subprocess runtime before
  `scafld review` fails with fallback guidance
- `external.fallback_policy`: behavior when `provider: auto` cannot find
  Codex but can find Claude; `warn` allows fallback with a warning, `allow`
  records the weaker isolation without warning, and `disable` requires Codex
- `external.<provider>.model`: optional provider-specific model pin

CLI overrides are explicit:

```bash
scafld review <task-id> --runner local
scafld review <task-id> --runner manual
scafld review <task-id> --provider codex --model gpt-5
```

There is no silent fallback from external review into local review. If no
external provider exists, scafld fails cleanly and tells you to opt into
`local` or `manual`.

Provider fallback is not treated as equivalent isolation. Codex provenance is
recorded as read-only and ephemeral. Claude provenance is recorded as restricted
tools plus fresh session, which is weaker than the Codex sandbox unless the
installed Claude CLI grows an equivalent control. Set
`review.external.fallback_policy: "disable"` to prevent automatic Claude
fallback.

Review provenance separates requested and observed model fields. An empty
observed model means scafld could not verify what the provider actually billed.
Provider invocation session entries include `confidence`: `observed`,
`inferred`, `requested_only`, or `unknown`.
Reports use conservative separation states: `separated`, `same_model`,
`unknown_executor`, `unknown_challenger`, and `unknown_both`.

## Why It Stays Small

The cutover goal is clean execution, not config sprawl.

Strategy lives in config. State lives in session. The spec schema stays
unchanged until more surface earns its place through measured wins.

## Repo-Aware Scaffolding

`scafld init`, `new`, and `plan` also derive suggested validation commands from
the current workspace. Mixed Python+Node repos are handled explicitly: scafld
merges both detectors, prefers concrete commands over placeholder defaults, and
combines commands with `&&` when both stacks provide real signals.
