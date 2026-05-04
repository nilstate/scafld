---
title: Configuration
description: Workspace config, prompts, and managed core files
---

# Configuration

scafld installs a small project-owned control plane:

```text
.scafld/
  config.yaml
  config.local.yaml
  prompts/
  core/
```

The current Go runtime treats the spec and session as the hard behavioral
contract. Config controls operator defaults such as review provider, model, and
timeouts; the runtime-critical gates are enforced by the Markdown spec,
acceptance criteria, review packet validation, and lifecycle commands.

## Managed vs Project-Owned

Project-owned:

- `.scafld/config.yaml`
- `.scafld/config.local.yaml`
- `.scafld/prompts/*`
- `.scafld/specs/**`
- `.scafld/runs/**`
- `.scafld/reviews/**`

Managed by scafld:

- `.scafld/core/config.yaml`
- `.scafld/core/prompts/*`
- `.scafld/core/schemas/*`
- `.scafld/core/scripts/*`
- `.scafld/core/specs/examples/*`

`scafld update` refreshes only `.scafld/core/`. It must not overwrite project
prompt overrides, specs, sessions, reviews, or local config.

## Prompt Overrides

Prompt lookup uses project files first:

```text
.scafld/prompts/harden.md
.scafld/core/prompts/harden.md
built-in fallback
```

Use project prompts for local voice and policy. Keep core prompts as the reset
copy that package upgrades can refresh.

## Acceptance Strictness

Every executable criterion must carry an explicit matcher:

```markdown
Acceptance:
- [ ] `ac1_1` test: Unit tests pass.
  - Command: `go test ./...`
  - Expected kind: `exit_code_zero`
```

Supported `Expected kind` values:

- `exit_code_zero`
- `exit_code_nonzero`
- `no_matches`

Criteria without a known expected kind are rejected before execution. Free-form
prose can explain intent, but the matcher is what scafld executes.

## Review Provider Selection

Review defaults come from `.scafld/config.yaml`:

```yaml
review:
  external:
    provider: "auto"              # auto | codex | claude | command | local
    command: "./reviewer"         # only when provider: command
    provider_binary: "/path/bin"  # optional selected-provider binary override
    idle_timeout_seconds: 180
    absolute_max_seconds: 1800
    fallback_policy: "warn"       # disable makes auto require codex
    codex:
      model: "gpt-5.5"
      binary: "codex"
    claude:
      model: "claude-opus-4-7"
      binary: "claude"
  automated_passes:
    spec_compliance:
      order: 10
      title: "Spec Compliance"
      description: "Verify recorded acceptance evidence against the spec."
  adversarial_passes:
    regression_hunt:
      order: 30
      title: "Regression Hunt"
      description: "Trace callers, importers, and downstream consumers."
```

`.scafld/config.local.yaml` overlays `.scafld/config.yaml`, so a developer can
pin a local provider or model without changing the committed project default.
`init` creates a commented local override file and the repository `.gitignore`
keeps it uncommitted.

CLI flags override config for a single invocation:

```bash
scafld review task --provider auto
scafld review task --provider codex
scafld review task --provider claude
scafld review task --provider command --provider-command "./reviewer"
scafld review task --provider local
scafld review task --provider codex --model gpt-5
```

`auto` chooses an installed external provider. It prefers Codex, then falls back
to Claude unless `fallback_policy: "disable"` is set. Provider-specific model
defaults come from `review.external.codex.model` and
`review.external.claude.model`; `--model` overrides either.

`local` exists for tests and smoke runs; it is not a substitute for adversarial
review.

Named `automated_passes` and `adversarial_passes` are included in the review
prompt in `order` sequence. They are the configurable review agenda; they do
not create additional local execution steps or mutate the workspace.

## Hardening

Hardening is operator-driven. `scafld approve` does not force
`harden_status: passed`, but a complete nontrivial plan spec should usually be
hardened before approval.

The active harden prompt asks the agent to record grounded questions under the
latest `## Harden Rounds` entry. `harden.max_questions_per_round` is read from
config and injected into that prompt as a cap, not a target. `--mark-passed`
warning-checks the cited code or archive references and closes the round.
