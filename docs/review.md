---
title: Review
description: Adversarial review finalize behavior
---

# Review

`finalize` is the default completion finalize for agent-driven work. `review` is
the direct lifecycle review command for spec-backed operator work.

Execution tries to finish the job. Review tries to break confidence in the job.
The implementation agent should not grade its own work.

`cross_vendor` receipts mean multi-model review: the reviewer and host agent are
from different model vendors, which reduces correlated blind spots. They remain
single-party local tooling unless a separate operator or CI trust domain verifies
the signed receipt. `isolation_only` receipts mean the reviewer was isolated, but
cross-vendor separation was not proven.

## Run Review

```bash
scafld review <task-id>
```

For the default host-agent path, call `finalize` instead. A passing gate
returns a signed receipt; CI verifies that receipt with `scafld verify`.

By default, review uses the provider configured in `.scafld/config.yaml` at
`review.external.provider`. Fresh workspaces use `auto`: when scafld can infer
the current host agent, it prefers the other installed challenger (`claude` for
Codex-driven work, `codex` for Claude-driven work), can use Gemini as another
external challenger, and fails closed when only the host provider is available.
Without a detected host, it uses the default order `codex`, then `claude`, then
`gemini`. If no independent external provider is available, review fails closed.
That is intentional; a missing challenger should not silently become a clean
review. Set `SCAFLD_HOST_AGENT=codex` or `SCAFLD_HOST_AGENT=claude` when a
wrapper does not expose a recognizable host-agent environment marker.

Provider-specific model defaults also come from config:

```yaml
review:
  external:
    provider: "auto"
    fallback_policy: "disable"
    codex:
      model: "gpt-5.5"
    claude:
      model: "claude-opus-4-7"
    gemini:
      # model: "" # leave empty to use Gemini CLI's configured default
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
scafld review <task-id> --provider gemini
scafld review <task-id> --provider command --provider-command "./reviewer"
scafld review <task-id> --provider local
scafld review <task-id> --provider codex --model gpt-5
scafld review <task-id> --human-reviewed --reason "operator reviewed PR 123"
```

For small diffs, keep the same finalize but tighten the review budget:

```bash
scafld review <task-id> --review-depth light --max-findings 4 --min-attack-angles 3
```

`review_depth` is a contract, not just a label:

- `light`: completion blockers and regression risk; avoid advisory churn.
- `standard`: balanced blocker hunt, regression tracing, and concise useful
  non-blocking findings.
- `deep`: broader adversarial pass across callers, invariants, edge cases, and
  operational risks within the requested budget.

Provider meanings:

- `codex`: read-only ephemeral Codex review using a structured output schema;
  scafld disables Codex user config and execpolicy rules for the review
  subprocess.
- `claude`: Claude review with session persistence disabled, slash commands and
  browser integration disabled, built-in tools restricted to `Read`, `Grep`,
  and `Glob`, and final dossier submission forced through scafld's
  `submit_review` MCP tool.
- `gemini`: Gemini CLI review in plan/read-only mode. scafld writes a temporary
  Gemini settings file that exposes only its `submit_review` MCP tool for the
  final dossier; final text is ignored. Gemini CLI must already be
  authenticated locally, through its settings or supported Google credential
  environment variables.
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
task contract, review request, declared task scope, workspace classification,
approval baseline, task changes since approval, ambient drift, acceptance
evidence, configured review agenda, selected project docs, root agent guidance,
`.claude/rules` when present, and schema context. Each project-context section
includes source path, hash, and byte count.

Every packet begins with a `Context Budget Manifest`. It records:

- max section-body bytes
- rendered section-body bytes
- omitted section-body bytes
- included sections
- truncated sections
- omitted sections with source paths and reason

`review.context.max_bytes` is an aggregate section-body budget for the rendered
packet, not a per-file allowance. Omission is never silent: the reviewer can see
what did not fit and which source paths to open if a specific attack requires
that material.

Review modes change the attack shape, not the completion standard:

- `discover`: broad search for new completion blockers across the approved
  task scope.
- `verify`: check known findings, repair regressions, and release blockers
  introduced by the fix.

Both modes use the same ReviewDossier schema and the same pass/fail finalize.

The prompt tells the challenger not to mutate the workspace, not to emit
placeholder output while investigating, and to submit one final ReviewDossier.

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

Provider adapters must deliver this one dossier shape before core validation.
Codex writes a schema-constrained output file. Claude and Gemini must call
scafld's `submit_review` MCP tool exactly once; scafld reads the tool payload
from that submission channel and ignores final prose or fenced JSON. Accepted
reviews record `output_format` so operators can see whether scafld consumed
`claude.mcp_submit_review`, `gemini.mcp_submit_review`, `codex.output_file`, or
another explicit provider path. Provider text that does not arrive through the
configured submit channel does not satisfy the finalize.

Findings require:

- `id`
- `severity`: `critical`, `high`, `medium`, or `low`
- `blocks_completion`: boolean
- `location`, `evidence`, `impact`, and `validation` when `blocks_completion`
  is true
- `summary` for readable repair output

Any open finding with `blocks_completion: true` forces verdict `fail`. Severity
and the completion finalize are deliberately separate: a high-severity accepted risk
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
- the latest review provider is `codex`, `claude`, `gemini`, `command`, or an
  audited `human` review override

If review fails, repair the work, rerun acceptance as needed, rerun review, then
complete only after the challenger clears the finalize.

Use `--human-reviewed` only when the provider finalize is blocked for an external
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
as a valid review dossier. For Claude and Gemini, final prose and fenced JSON
are ignored: the provider must call the `submit_review` tool exactly once.

During a running external review, the terminal shows summary progress only:
start, periodic running heartbeat, structured provider events when available,
and the final result. Raw provider stdout and stderr stay in diagnostics so the
outer agent gets liveness without having to parse exploratory model logs or
placeholder output.
