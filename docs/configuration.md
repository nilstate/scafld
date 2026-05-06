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
contract. Config controls the invariant catalog, harden prompt limits, review
provider, model, timeouts, and review focus. Runtime-critical gates are still
enforced by the Markdown spec, acceptance criteria, review packet validation,
and lifecycle commands.

## Managed vs Project-Owned

Project-owned:

- `.scafld/config.yaml`
- `.scafld/config.local.yaml`
- `.scafld/prompts/*`
- `.scafld/specs/**`
- `.scafld/runs/**`

Managed by scafld:

- `.scafld/core/config.yaml`
- `.scafld/core/prompts/*`
- `.scafld/core/schemas/*`
- `.scafld/core/scripts/*`
- `.scafld/core/specs/examples/*`

`scafld update` refreshes `.scafld/core/` and safely refreshes project prompt
copies that are still known defaults. Customized project prompts are skipped.
Specs, sessions, reviews, and local config are never overwritten.

## Prompt Overrides

Prompt lookup uses project files first:

```text
.scafld/prompts/harden.md
.scafld/core/prompts/harden.md
built-in fallback
```

Use project prompts for local voice and policy. Keep core prompts as the reset
copy that package upgrades can refresh. If you have not customized a project
prompt, `scafld update` keeps it aligned with the bundled prompt contract.

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

## Convention Surface

scafld does not require a `CONVENTIONS.md` file. Convention adherence is
surfaced through protocol artifacts that the runtime and reviewers already use:

- `.scafld/config.yaml` names canonical invariant IDs and review passes.
- Specs select the invariants that apply to the task and declare scope,
  out-of-scope work, touchpoints, risks, and acceptance criteria.
- `AGENTS.md` and `CLAUDE.md` give agents the short operating contract at the
  root discovery surface.
- Optional project docs can explain local style, but they are only binding when
  a spec, invariant, or review pass explicitly cites them.

This keeps conventions close to enforcement. Prose can help, but config and
specs are the contract.

Invariant IDs live in config:

```yaml
invariants:
  canonical:
    domain_boundaries: "Respect layer separation and ownership boundaries."
    tenant_isolation: "Do not leak data across tenants."
```

Specs select the relevant IDs for a task. Harden prints the configured catalog
so the agent can choose the right constraint while tightening the draft. Review
prompts include the invariants selected by the spec.

## Configure Proposals

`scafld init` installs a truthful default config. It does not ask an agent to
guess project policy.

Use `scafld configure` when a repo needs project-specific tightening:

```bash
scafld configure
```

The command scans recognizable project surfaces, writes
`.scafld/config.proposed.yaml`, and prints CONFIGURE MODE instructions. The
proposal is evidence-backed: every suggested command or invariant cites the
file that implied it. Open questions are explicit when scafld cannot infer a
safe answer. If an existing config contains old keys the Go runtime does not
read, the proposal includes a `legacy_ignored_config_keys` warning so cleanup is
explicit rather than silent.

The proposal is not automatically applied. An operator or agent should inspect
the cited sources, then copy only verified invariant IDs or local review
defaults into `.scafld/config.yaml`. Commands and review focus are guidance for
future specs or real `review.automated_passes` / `review.adversarial_passes`
entries. If scafld does not read a field, do not add it to config as if it were
enforced.

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
review and cannot satisfy `scafld complete`.

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
verifies the cited code or archive references and refuses to close the round
when they do not resolve.
