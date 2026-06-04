# scafld Runtime

scafld builds long-running AI coding work under adversarial review.

## Core Model

- `spec`: reviewed contract
- `session`: durable run ledger
- `handoff`: generated transport for the next voice

The taught surface is deliberately small:

```text
agent work -> finalize -> signed receipt -> scafld verify
```

The lifecycle remains available for operators, CI debugging, and direct human
control, but it is not the default agent path.

## Directory Layout

```text
.scafld/
  config.yaml
  config.local.yaml
  prompts/                 # optional project-owned overrides
  runs/
    {task-id}/
      diagnostics/
      session.json
  specs/
    drafts/
    approved/
    active/
    archive/YYYY-MM/
  core/
    prompts/
    schemas/              # spec, review dossier, and harden dossier schemas
    scripts/              # optional lifecycle helper scripts installed by update
```

Prompt ownership:

- `.scafld/prompts/*` is the active template layer
- `.scafld/core/prompts/*` is the managed reset copy

`scafld config` writes `.scafld/config.proposed.yaml` with evidence-backed
config suggestions. It does not mutate `.scafld/config.yaml`.

`scafld update` refreshes managed core assets and existing manifest-backed
prompt copies. Customized project prompts are skipped. It also refreshes root
agent docs and renders generated `.scafld/config.yaml` into the current strict
runtime shape.

## Handoffs

`scafld handoff <task-id>` renders current model-facing context to stdout from
the spec and session. It is one-way transport: scafld emits it, the next model
reads it, and scafld never reads it back for state.

## Default Integrations

Fresh `scafld init` installs the `finalize` affordance and CI-facing
`scafld verify` wiring. It does not install lifecycle wrapper scripts by
default.

Optional lifecycle helper scripts are operator utilities. They are installed by
`scafld update` or an explicit managed bundle install path, not by default init:

- `.scafld/core/scripts/scafld-codex-build.sh <task-id>`
- `.scafld/core/scripts/scafld-codex-review.sh <task-id>`
- `.scafld/core/scripts/scafld-claude-build.sh <task-id>`
- `.scafld/core/scripts/scafld-claude-review.sh <task-id>`

They call `scafld adapter codex|claude|gemini <mode> <task-id>` to render
current status, next-action fields, and handoff text as a provider-facing
packet. They do not execute an external agent runtime or advance lifecycle
state.

## Adversarial Review

Default agent work challenges completion through `finalize`. Direct lifecycle
work challenges completion through `scafld review`.

That means:

- `finalize` returns blockers or a signed receipt in one agent-facing call
- `review` remains the operator/lifecycle review command for spec-backed runs
- `verify` is the CI merge wall for signed receipts
- diagnostics are transport evidence, not the primary finding surface

## Metrics

`report` surfaces:

- `first_attempt_pass_rate`
- `recovery_convergence_rate`
- `review_pass_rate`
- `review_dossier_coverage`
- `review_findings_total`
- `review_open_blockers_total`
- `review_attack_angles_total`
- `challenge_override_rate`

Use `scafld report` to inspect workspace-wide task state.
