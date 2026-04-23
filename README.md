# scafld

[![Review Gate Smoke](https://github.com/nilstate/scafld/actions/workflows/review-gate-smoke.yml/badge.svg)](https://github.com/nilstate/scafld/actions/workflows/review-gate-smoke.yml)

scafld builds long-running AI coding work under adversarial review, so your agent stays coherent across the whole job.

Canonical repo: `https://github.com/nilstate/scafld`. Default branch: `main`.

Most AI coding tools optimize for speed inside one turn. scafld optimizes for correctness across the whole run:

- the work starts from a reviewed spec
- execution stays phase-bounded and measurable
- completion is gated by an independent challenger at review

The differentiator is simple: **the agent does not get to grade its own homework**.

## Identity

scafld is a scaffold in the literal sense: temporary structure that shapes the build while the work is in progress.

The lifecycle stays familiar:

```text
draft -> harden -> approve -> build -> review -> complete
```

What changed is the core model underneath it.

## Primitives

Two nouns carry the system:

- `spec`: what must be true. The reviewed contract and lifecycle source of truth.
- `session`: what happened. The durable run ledger under `.ai/runs/{task-id}/session.json`.

`handoff` is transport, not a primitive. It is the structured brief for the next voice.

Every generated handoff is:

- immutable
- sibling `*.md + *.json`
- tagged by `role × gate`

Current runtime pairings:

- `executor × phase`
- `executor × recovery`
- `challenger × review`

## Review Gate

Challenge fires at one gate only in v1: `review`.

That keeps the system sharp without turning every phase into ceremony.

- `review` emits a challenger handoff
- the challenger writes a verdict into `.ai/reviews/{task-id}.md`
- `complete` closes only if the review gate passes, or a human applies the audited override path

The override path is explicit:

```bash
scafld complete <task-id> --human-reviewed --reason "manual audit"
```

## Runtime Layout

```text
.ai/
  specs/{drafts,approved,active,archive}/
  runs/
    {task-id}/
      handoffs/
        executor-phase-phase1.md
        executor-phase-phase1.json
        executor-recovery-ac1_1-1.md
        executor-recovery-ac1_1-1.json
        challenger-review.md
        challenger-review.json
      diagnostics/
        ac1_1-attempt1.txt
      session.json
    archive/{YYYY-MM}/{task-id}/
```

Hard rules:

- `spec` never carries runtime state
- `handoff` is never read back to compute state
- `session` is the only durable run-state source
- recovery is a handoff gate plus counters in `session`, not a subsystem
- telemetry is a view of `session`, not a separate artifact
- v1 makes zero spec schema changes

## Agent Surface

Default help teaches the slim workflow surface, including repo seeding:

```bash
scafld init
scafld plan <task-id>
scafld approve <task-id>
scafld build <task-id>
scafld review <task-id>
scafld complete <task-id>
scafld status <task-id>
scafld list
scafld report
scafld handoff <task-id>
scafld update
```

Use `scafld --help --advanced` to show the remaining operator tools such as
`harden`, `validate`, `branch`, `sync`, `audit`, `diff`, `summary`, `checks`,
and `pr-body`.

Wrapper intent:

- `plan`: create a draft spec or reopen harden on an existing draft
- `build`: start approved work and immediately drive validation to the next handoff or block
- `review`: run the adversarial review gate and emit the challenger handoff

## Success Metrics

scafld claims quality lift only where it can measure it.

The canonical metrics are:

- `first_attempt_pass_rate`
- `recovery_convergence_rate`
- `challenge_override_rate`

`report` surfaces all three per task and in aggregate.

There is also one honest boundary: scafld can emit a better handoff, but an
external harness may still ignore it. That is why the metrics are framed as
session outcomes, not proof of prompt consumption.

## Install

```bash
pip install scafld
npm install -g scafld
git clone https://github.com/nilstate/scafld.git ~/.scafld && ~/.scafld/install.sh
curl -fsSL https://raw.githubusercontent.com/nilstate/scafld/main/install.sh | sh
```

`pip install scafld` installs the console entry point plus the managed runtime bundle used by `scafld init` and `scafld update`.

`npm install -g scafld` installs the same CLI package for environments that ship tooling through npm. The CLI still requires `python3` at runtime. Commands that edit YAML specs, such as `scafld harden`, also require `PyYAML` in that Python runtime:

```bash
python3 -m pip install PyYAML
```

## Setup

```bash
cd your-project
scafld init
```

This creates the managed runtime bundle, prompts, schemas, run directories, and
project-owned overlays:

```text
your-project/
  .ai/
    scafld/                # Managed reset copy refreshed by `scafld update`
    config.yaml            # Project config overlay
    config.local.yaml      # Local machine overrides
    prompts/               # Active project-owned template sources
    runs/                  # Generated handoffs, diagnostics, session state
    reviews/               # Adversarial review artifacts
    specs/                 # Specs by lifecycle state
  AGENTS.md
  CLAUDE.md
  CONVENTIONS.md
```

Start by customizing:

1. `AGENTS.md`
2. `CLAUDE.md`
3. `CONVENTIONS.md`
4. `.ai/config.local.yaml`

Repo-aware planning also works for mixed Python+Node repos. When both stacks are
present, scafld merges the signals and prefers concrete detected commands over
placeholder defaults.

Prompt ownership is deliberate:

- `.ai/prompts/*` is the active template layer the runtime reads first
- `.ai/scafld/prompts/*` is the managed reset copy refreshed by `scafld update`

## Minimal Runtime Config

```yaml
llm:
  model_profile: "default"
  context:
    budget_tokens: 12000
  recovery:
    max_attempts: 1
```

Anything beyond that waits until it earns its place through measured wins.

## Next Docs

- `docs/execution.md`
- `docs/review.md`
- `docs/run-artifacts.md`
- `docs/cli-reference.md`
- `AGENTS.md`
