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

## Why It Stays Small

The cutover goal is clean execution, not config sprawl.

Strategy lives in config. State lives in session. The spec schema stays
unchanged until more surface earns its place through measured wins.

## Repo-Aware Scaffolding

`scafld init`, `new`, and `plan` also derive suggested validation commands from
the current workspace. Mixed Python+Node repos are handled explicitly: scafld
merges both detectors, prefers concrete commands over placeholder defaults, and
combines commands with `&&` when both stacks provide real signals.
