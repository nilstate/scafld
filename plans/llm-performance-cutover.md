# scafld Adversarial Review Cutover

Created: 2026-04-24
Status: planned

## Identity

scafld builds long-running AI coding work under adversarial review, so your
agent stays coherent across the whole job.

It is a scaffold: a temporary structure that shapes what gets built. The
product differentiator is adversarial review, concentrated at the gate that
matters. Agents cannot advance unchallenged there.

## Core Model

There are only two primitives:

- `spec`: what must be true
- `session`: what happened

`spec` remains the reviewed contract and lifecycle source of truth.
`session` becomes the durable run ledger with typed entries for attempts,
phase summaries, challenger verdicts, human approvals, and overrides.

## Transport

`handoff` is transport, not a primitive.

Every handoff is:

- immutable
- generated
- sibling `*.md` plus `*.json`
- tagged by `role × gate`

```yaml
role: executor | challenger | reviewer | human
gate: harden | phase | recovery | review
```

One renderer should emit all handoffs. The handoff is never read back to
compute state.

## Challenge

Challenge fires at one gate only in v1:

- `gate: review`
- `role: challenger`

The review handoff becomes a challenger brief, not a soft summary. It should be
genuinely adversarial and evidence-seeking.

The challenger verdict determines whether `complete` can close the task
normally. Human override stays available and audited.

No phase-boundary challenges. No pre-approval challenger gate. No second
post-review challenge layer. Concentrate the weight where the evidence is
complete and the stakes are real.

## Lifecycle

The lifecycle stays unchanged:

```text
new -> harden -> approve -> start -> exec -> review -> complete
```

Run artifacts archive on `complete`, `fail`, or `cancel`:

```text
.ai/runs/archive/{YYYY-MM}/{task-id}/
```

## Agent Surface

The default agent-facing surface should collapse to roughly ten verbs:

```text
plan
approve
build
review
complete
status
list
report
handoff
update
```

Meaning:

- `plan`: create + harden
- `approve`: human ratify
- `build`: run phases to done or block
- `review`: adversarial challenge, the hero gate
- `complete`: archive
- `status`: inspect, with flags for audit/diff/sync/projections

Legacy verbs stay for scripts and power users. They move behind advanced help
and stop being the default story in `AGENTS.md`, `CLAUDE.md`, `.ai/README.md`,
`.ai/OPERATORS.md`, and `.ai/prompts/*.md`.

## Code Shape

The code shape should stay small and explicit:

- `scafld/runtime_contracts.py`
  Own schema versions, path helpers, `role × gate` enums, compatibility fields,
  and run-archive layout.
- `scafld/handoff_renderer.py`
  One renderer for all handoffs. Maps `role × gate` to templates and emits the
  sibling `.md/.json` pair.
- `scafld/session_store.py`
  Owns typed session entries and aggregation helpers. No mirrored runtime state
  in specs or handoffs.
- `scafld/review_workflow.py`
  Owns challenger handoff creation, challenger verdict ingestion, and review
  gate evaluation.
- `scafld/reviewing.py`
  Keeps Review Artifact v3 coherent while referencing challenge history and
  override state.
- `scafld/commands/workflow.py`
  Thin orchestrators for agent-facing wrapper verbs such as `plan` and `build`.
- `scafld/commands/surface.py`
  Owns the public CLI surface, default help, advanced help, and legacy command
  registration.
- `scafld/commands/reporting.py`
  Headlines the three metrics and nothing more aspirational than session-backed
  evidence.

Avoid:

- `challenge.py` as a parallel subsystem
- `telemetry.py` or `telemetry.jsonl`
- separate artifact families beyond `spec`, `session`, and transport handoffs
- challenge firing at multiple gates in v1

## Artifact Layout

```text
.ai/
  specs/
    drafts/
    approved/
    active/
    archive/
  runs/
    {task-id}/
      handoffs/
        *.md
        *.json
      diagnostics/
      session.json
    archive/{YYYY-MM}/{task-id}/
```

## Hard Rules

These rules are load-bearing:

1. `spec` is contract only. No runtime state.
2. `handoff` is output for the next voice. Never read it back.
3. `session` is the only durable runtime source of truth.
4. `recovery` is `gate: recovery` transport plus counters in session, not a
   subsystem.
5. Telemetry is a view of session, not an artifact.
6. v1 makes zero spec schema changes.
7. Challenge fires at `review` only in v1.
8. The harness may ignore handoffs. `report` is the only honest attribution
   surface.

## Minimum Config

Stay with the existing minimal block:

```yaml
llm:
  model_profile: "default"
  context:
    budget_tokens: 12000
  recovery:
    max_attempts: 1
```

Anything beyond this waits until it earns its keep with measured wins.

## Metrics

The headline metrics become:

- `first_attempt_pass_rate`
- `recovery_convergence_rate`
- `challenge_override_rate`

`challenge_override_rate` is the honest signal for whether the adversarial gate
is surfacing real disagreement that humans still choose to waive.

## V1 Load-Bearing Work

To reach this cutover, v1 needs:

- `role × gate` handoff metadata and transport rendering
- typed session entries
- challenger-role review handoff
- review gate wired to challenger verdicts
- compact default help and agent docs around the ten-verb surface
- a genuinely adversarial review template
- report support for the three metrics

## Deferred Beyond V1

Do not build these in this cut:

- harden as a challenger handoff
- multi-gate challenge firing
- archive precedent retrieval as a handoff section
- model-tuned renderer hints
- full CLI primitive retirement in v2

This cutover should be small enough to teach in one paragraph, strong enough to
be a product claim, and honest about what it can and cannot prove.
