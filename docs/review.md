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
  dossier:
    max_findings: 12
    min_attack_angles: 6
    review_depth: "standard"
    rerun_policy: "verify_open_blockers"
```

Use `.scafld/config.local.yaml` for local-only provider or model overrides.

Explicit providers:

```bash
scafld review <task-id> --provider codex
scafld review <task-id> --provider claude
scafld review <task-id> --provider command --provider-command "./reviewer"
scafld review <task-id> --provider local
scafld review <task-id> --provider codex --model gpt-5
scafld review <task-id> --human-reviewed --reason "operator reviewed PR 123"
```

Provider meanings:

- `codex`: read-only ephemeral Codex review using a structured output schema.
- `claude`: Claude review with restricted read-only tools and stream-json
  output.
- `command`: custom reviewer command. It receives the review prompt on stdin and
  must emit a ReviewDossier-compatible response.
- `local`: deterministic local pass-through provider for development and smoke
  tests. It is not an adversarial review and cannot satisfy `complete`.
- `--human-reviewed`: audited operator override. It does not invoke a model
  provider. A reason is required, and scafld records both a `review_override`
  event and a passing `review` event with provider `human` in the session
  ledger.

## Review Scope

Dirty monorepos and multi-repo workspaces often contain changes that predate the
task: generated files, submodule pointers, archived specs, or other developers'
work. Those paths should not become findings just because they exist.

scafld derives task scope from the spec's packages, impacted files, and phase
changes. Use `--review-scope` only when the repo layout needs an explicit
boundary:

```bash
scafld review email-contracts --review-scope api
scafld review email-contracts --review-scope api,cli/packages/mcp
```

At approval, scafld records the dirty workspace baseline. At review, it compares
the current workspace to that baseline, sends task-scoped changes to the
reviewer, and includes new changes outside declared scope as ambient workspace
drift. Unchanged baseline dirt and ambient drift are context, not findings by
themselves. This keeps dirty monorepos cheap: unrelated active work from another
task should not force a stash, commit, human override, or extra provider run.

The read-only mutation guard is task-relevant rather than global. Changes inside
review scope still fail closed because the provider judged moving code.
Unrelated `.scafld/specs/drafts/**` churn from another task does not discard a
valid review. The current task spec remains guarded: if it changes during
review, the contract changed while it was being judged.

## What scafld Sends

The reviewer receives a typed review-context packet rendered as Markdown:
task contract, declared task scope, approval baseline, task changes since
approval, acceptance evidence, configured review agenda, selected project docs,
root agent guidance, `.claude/rules` when present, and schema context. Each
project-context section includes source path, hash, and byte count.
`review.context.max_bytes` is an aggregate section-body budget for the rendered
packet, not a per-file allowance.

The prompt tells the challenger not to mutate the workspace, not to emit
placeholder output while investigating, and to return one final ReviewDossier.

Print the exact packet without invoking a provider:

```bash
scafld review <task-id> --print-context
```

The dossier is the provider content contract:

```json
{
  "verdict": "pass",
  "mode": "discover",
  "summary": "No open completion blockers found.",
  "findings": [],
  "attack_log": [
    {"target": "task diff", "attack": "regression scan", "result": "clean"}
  ],
  "budget": {"actual_attack_angles": 1}
}
```

Provider adapters normalize provider-native transport into this one dossier
shape before core validation. Codex normally returns an output file; Claude
normally returns `structured_output` inside its stream. If a provider wraps the
same dossier in a single Markdown JSON fence, the adapter may extract that at
the transport boundary, but the core parser still accepts only the canonical
dossier. Accepted reviews record `output_format` so operators can see whether
scafld consumed `claude.structured_output`, `claude.result_text.fenced_json`,
`codex.output_file`, or another explicit provider path.

Findings require:

- `id`
- `severity`: `critical`, `high`, `medium`, or `low`
- `blocks_completion`: boolean
- `location`, `evidence`, `impact`, and `validation` when `blocks_completion`
  is true
- `summary` for readable repair output

Any open finding with `blocks_completion: true` forces verdict `fail`. Severity
and the completion gate are deliberately separate: a high-severity accepted risk
can be non-blocking, while a medium defect can still block if it violates the
approved contract.

## What scafld Trusts

scafld validates the dossier, checks whether Git-visible workspace state changed
during review, records the review event in session, then projects the verdict
back into the spec.

The authority order stays the same:

- session stores evidence
- spec shows the readable current projection
- provider output is accepted only after validation

Invalid dossier output fails review. Task-relevant workspace changes during
review become a blocking finding, even if the provider returned `pass`. If the
provider also returned findings, scafld keeps them and appends the
workspace-change finding so the original review signal is not hidden.

## Failed Review Output

Review findings are normal workflow data, not hidden diagnostics.

When review fails:

- `scafld review` prints the findings and the next repair command.
- `scafld status` repeats the latest review verdict and findings.
- `scafld handoff` includes the latest review findings for the next model voice.
- the session review entry stores the accepted dossier.
- the spec projects the latest verdict and findings under `## Review`.

Diagnostics remain for provider transport failures, invalid dossiers, timeouts,
and other cases where scafld could not accept normal review output.

## Complete Gate

```bash
scafld complete <task-id>
```

`complete` refuses unless:

- the latest session review event exists
- the latest review verdict is `pass`
- the latest review provider is `codex`, `claude`, `command`, or an audited
  `human` review override

If review fails, repair the work, rerun acceptance as needed, rerun review, then
complete only after the challenger clears the gate.

Use `--human-reviewed` only when the provider gate is blocked for an external
reason and a human has actually reviewed the diff, spec, acceptance evidence,
and scope. It is an audited escape hatch, not a softer review mode.

## Challenger Stance

A useful adversarial review:

- attacks the diff, not just the prose
- attacks the spec contract and acceptance evidence
- cites concrete files, commands, or spec sections
- separates severity from completion-blocking findings
- says `pass` only when the evidence holds

Generic clean notes are not useful. A clean review should still explain what was
checked and why that was enough.

## Diagnostics

External providers run through the process runner with timeout and idle-timeout
protection. Provider failures and timeouts write diagnostics under:

```text
.scafld/runs/<task-id>/diagnostics/
```

`status --json` and `handoff` show the accepted blocker summary first. Use
diagnostics as supporting evidence when paid model output could not be accepted
as a valid review dossier.

During a running external review, the terminal shows summary progress only:
start, periodic running heartbeat, structured provider events when available,
and the final result. Raw provider stdout and stderr stay in diagnostics so the
outer agent gets liveness without having to parse exploratory model logs or
placeholder output.
