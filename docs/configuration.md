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
  prompts/   # optional overrides
  core/      # generated reset copy
```

The current Go runtime treats the spec and session as the hard behavioral
contract. Config controls the invariant catalog, harden prompt limits, review
provider, model, timeouts, and review focus. Runtime-critical gates are still
enforced by the Markdown spec, acceptance criteria, review dossier validation,
and lifecycle commands.

## Managed vs Project-Owned

Project-owned:

- `.scafld/config.yaml`
- `.scafld/prompts/*`
- `.scafld/specs/**`

Local/generated:

- `.scafld/config.local.yaml`
- `.scafld/core/**`
- `.scafld/runs/**`

`scafld update` refreshes `.scafld/core/` and safely refreshes project prompt
copies only when the prompt manifest proves they are unmodified defaults.
Customized project prompts are skipped. It also refreshes root agent docs.
Project config is left untouched; invalid config shapes fail at lifecycle or
config-load time. Specs, sessions, reviews, and local config are never
overwritten.

`.scafld/config.yaml` is intentionally sparse. Runtime defaults live in the
binary, and the full example shape lives in `.scafld/core/config.yaml`. Put only
project-specific invariant IDs, execution environment, provider defaults, or
review-pass overrides in the committed project config.

`.scafld/config.local.yaml` is narrower again: personal machine settings only.
Use it for local shim paths, provider binaries, and temporary model overrides.
Do not copy the full config shape into local config just to keep an example.

## Prompt Overrides

Prompt lookup uses project files first:

```text
.scafld/prompts/harden.md
.scafld/core/prompts/harden.md
built-in default
```

Use project prompts for local voice and policy. Keep core prompts as generated
reset copies. If you have not customized a project prompt, do not add a copy;
scafld will fall back to the generated core prompt and then the built-in prompt.

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
- `browser_evidence`

Criteria without a known expected kind are rejected before execution. Free-form
prose can explain intent, but the matcher is what scafld executes.

Use `browser_evidence` only with criteria typed as `browser`. The command is
still explicit and project-owned; scafld validates the emitted evidence packet
instead of assuming one browser framework or one authentication scheme.

## Execution Environment

Acceptance commands run in a deterministic non-login shell. scafld inherits the
process environment, prepends repo-detected version-manager shims, then applies
`execution` overrides from config. It does not source `.zshrc`, `.bashrc`,
rbenv init scripts, asdf init scripts, or other interactive shell startup files.

Toolchain files are detected from checked-in project state. `.tool-versions`
adds common asdf/mise shim directories, `mise.toml` adds common mise shim
directories, and language-specific files such as `.ruby-version`,
`.python-version`, `.node-version`, `.go-version`, and `.java-version` add the
matching common shim directory. Explicit config paths are placed before
detected shims.

Detected shims are intentionally conservative:

- `.tool-versions` -> asdf and mise shims
- `mise.toml` / `.mise.toml` -> mise shims
- `.ruby-version` -> rbenv shims
- `.python-version` -> pyenv shims
- `.node-version` / `.nvmrc` -> nodenv shims
- `.go-version` -> goenv shims
- `.java-version` -> jenv shims

Shared project command environment belongs in `.scafld/config.yaml`:

```yaml
execution:
  absolute_timeout_seconds: 300
  idle_timeout_seconds: 0
  path_prepend:
    - "$HOME/.rbenv/shims"
    - "$HOME/.rbenv/bin"
  env:
    BUNDLE_GEMFILE: "api/Gemfile"
```

`path_prepend` is added before the inherited `PATH`, after environment-variable
and `~` expansion. `env` values are also expanded with the current process
environment. Use `.scafld/config.local.yaml` for developer-local paths or model
provider binaries that should not be committed.

`absolute_timeout_seconds` caps each acceptance command. The default is 300
seconds. Raise it for legitimate slow project commands such as cold framework
typechecks; do not hide slow commands behind `true`. `idle_timeout_seconds` is
optional and defaults to `0`, which disables idle-timeout enforcement for
acceptance commands.

Task-specific requirements can still live directly in a criterion command, but
repo-wide toolchain setup should be declared once in config. That keeps build
evidence reproducible without forcing every spec to remember local shell
initialization details.

## Convention Surface

scafld does not require a `CONVENTIONS.md` file. Convention adherence is
surfaced through protocol artifacts that the runtime and reviewers already use:

- `.scafld/config.yaml` names canonical invariant IDs and review passes.
- Specs select the invariants that apply to the task and declare scope,
  out-of-scope work, touchpoints, risks, and acceptance criteria.
- `AGENTS.md` and `CLAUDE.md` give agents the short operating contract at the
  root discovery surface.
- `.claude/rules` is treated as project rule context when present. scafld can
  include rule files in review context and `scafld config` will cite them as
  `agent_guidance_alignment` evidence.
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

Agent rule files are deliberately advisory until promoted. Put durable policy in
config invariants, spec context, or review passes; use `CLAUDE.md` and
`.claude/rules` to help agents find and apply those policies consistently.

## Config Proposals

`scafld init` installs a truthful default config. It does not ask an agent to
guess project policy.

Use `scafld config` when a repo needs project-specific tightening:

```bash
scafld config
```

The command scans recognizable project surfaces, writes
`.scafld/config.proposed.yaml`, and prints CONFIG MODE instructions for the
agent. The proposal is evidence-backed: every suggested command or invariant
cites the file that implied it. It also contains `agent_instructions`, so the
agent knows what to update, what must stay out of runtime config, and which
open items need a human or a deeper repo read. If an existing config contains
old keys the Go runtime does not read, the proposal includes an
`ignored_config_keys` warning so cleanup is explicit rather than silent.

When recognizable toolchain files exist, the proposal may include
`config_patch.execution`. For example, `.ruby-version` can justify rbenv
shims, `.python-version` can justify pyenv shims, and `.tool-versions` can
justify asdf/mise shims. Copy only project-specific overrides that need to
outrank the auto-detected defaults.

Config also recognizes common validation surfaces:

- `Makefile`, `justfile`, and `Taskfile.*` check/test targets
- `package.json` scripts including `check`, `test`, `lint`, `typecheck`, and
  `build`, using npm, pnpm, yarn, or bun based on package manager metadata and
  lockfiles
- Go, Rust, Python, and Ruby manifests for language-specific test commands
- Python runners and locks: pytest, Ruff, `uv.lock`, `poetry.lock`,
  `requirements.txt`
- Ruby/Bundler and Rails: `Gemfile`, `Gemfile.lock`, `config/routes.rb`
- TypeScript config and workspace pipelines: `tsconfig.json`, `turbo.json`,
  `nx.json`
- Docker, Compose, Procfile, deployment config, and OpenAPI schemas as
  invariant evidence

The proposal is not automatically applied because the best project config has
to come from an agent or operator that has inspected the repo. The safe flow is:

1. Read `.scafld/config.proposed.yaml`.
2. Open the cited sources.
3. Copy only verified runtime policy into `.scafld/config.yaml`.
4. Put non-runtime guidance in `AGENTS.md`, `CLAUDE.md`, `.claude/rules`, or
   project prompts instead of inventing config fields.
5. Use suggested commands and review focus while drafting or hardening specs,
   unless you translate them into real `review.automated_passes` or
   `review.adversarial_passes` entries.

If scafld does not read a field, do not add it to config as if it were enforced.

## Review Provider Selection

Review defaults come from `.scafld/config.yaml`:

```yaml
review:
  external:
    provider: "auto"              # auto | codex | claude | gemini | command | local
    command: "./reviewer"         # only when provider: command
    provider_binary: "/path/bin"  # optional selected-provider binary override
    idle_timeout_seconds: 180
    absolute_max_seconds: 1800
    fallback_policy: "disable"    # fail closed instead of using the host agent
    codex:
      model: "gpt-5.5"
      binary: "codex"
    claude:
      model: "opus"
      binary: "claude"
    gemini:
      # model: ""                 # empty uses Gemini CLI's configured default
      binary: "gemini"
  context:
    # Aggregate rendered section-body budget for the provider brief.
    max_bytes: 16384
    files:
      - AGENTS.md
      - CLAUDE.md
      - .claude/rules
      - README.md
      - docs/review.md
      - docs/configuration.md
      - docs/execution.md
      - .scafld/core/schemas/review_dossier.json
  dossier:
    max_findings: 12
    min_attack_angles: 6
    review_depth: "standard"
    rerun_policy: "verify_open_blockers"
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
If a config still contains an old scafld-generated model default, scafld upgrades
that value to the current default while loading config. Custom model values are
preserved.

CLI flags override config for a single invocation:

```bash
scafld review task --provider auto
scafld review task --provider codex
scafld review task --provider claude
scafld review task --provider gemini
scafld review task --provider command --provider-command "./reviewer"
scafld review task --provider local
scafld review task --provider codex --model gpt-5
```

`auto` chooses an installed external provider. When scafld can infer the current
host agent, it prefers the other agent first: Claude for Codex-driven work,
Codex for Claude-driven work. Gemini is available as an additional external
challenger. With `fallback_policy: "disable"`, auto refuses to use the current
host as its own challenger; set `warn` or `allow` only when that fallback is an
intentional local tradeoff. Without a detected host, the default order is Codex,
then Claude, then Gemini. Provider-specific model defaults come from
`review.external.codex.model`, `review.external.claude.model`, and
`review.external.gemini.model`; `--model` overrides them.

If the host agent cannot be inferred from the environment, set
`SCAFLD_HOST_AGENT=codex` or `SCAFLD_HOST_AGENT=claude` before invoking scafld.
This affects only `provider: auto` ordering.

`local` exists for tests and smoke runs; it is not a substitute for adversarial
review and cannot satisfy `scafld complete`.

Named `automated_passes` and `adversarial_passes` are included in the review
prompt in `order` sequence. They are the configurable review agenda; they do
not create additional local execution steps or mutate the workspace.

`review.context.files` is the bounded product-contract context included in the
reviewer brief. scafld skips private/local paths such as
`.scafld/config.local.yaml`, `.priv/**`, `.git/**`, and `.env*` even if listed.

## Hardening

Hardening is operator-driven. `scafld approve` does not force
`harden_status: passed`, but a complete nontrivial plan spec should usually be
hardened before approval.

The active harden prompt asks the agent to record evidence-backed observations
under the latest `## Harden Rounds` entry. Required dimensions are `path`,
`command`, `scope`, `timing`, `rollback`, and `design`. The design dimension
asks why the plan exists, what deeper product or system problem it solves,
whether the proposed change is a short-sighted bandaid over an endemic issue,
and whether a different abstraction would remove the root cause more cleanly.

`harden.max_issues_per_round` is read from config and injected into the prompt
as a cap on useful findings, not a target. `--mark-passed` verifies dimension
coverage, observation anchors, and unresolved blocking observations before
closing the round. Advisory observations remain recorded without blocking
approval.

Provider-backed hardening uses `harden.external`. Leave
`harden.external.provider` empty for manual rounds, or set it to `auto`,
`codex`, `claude`, `gemini`, `command`, or `local` when the draft should be
challenged by a separate agent. The provider receives a bounded harden context packet and
must submit a strict `HardenDossier`: summary and observations. scafld derives
`pass` or `needs_revision` from dimension coverage and unresolved `blocks`
observations. Non-blocking advisories are rendered in `## Harden Rounds`, but do
not force another harden cycle.
