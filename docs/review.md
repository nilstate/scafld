---
title: Review
description: Challenger review gate behavior
---

# Review

`review` is the load-bearing gate in scafld.

Execution tries to finish the job. Review tries to break confidence in the job.

That split is explicit in v1:

- one challenger handoff per task
- one gate that determines whether `complete` can close
- one honest attribution metric: `challenge_override_rate`

## Run Review

```bash
scafld review <task-id>
```

The command:

1. runs automated passes
2. appends a new round to `.ai/reviews/{task-id}.md`
3. resolves the configured review runner
4. either executes a fresh external challenger or emits the `challenger × review` handoff for an explicit degraded path

The handoff lives at:

- `.ai/runs/{task-id}/handoffs/challenger-review.md`
- `.ai/runs/{task-id}/handoffs/challenger-review.json`

If the latest review round is still `in_progress`, rerunning `scafld review
<task-id>` refreshes that same round in place. It does not append a second
round until the challenger has actually finished the current one.

The default runner is `external`:

- resolve `codex` first
- fall back to `claude` when codex is unavailable, with a visible weaker
  isolation warning before the Claude subprocess starts
- fail cleanly if neither exists
- stop after `review.external.timeout_seconds` instead of hanging indefinitely

Set `review.external.fallback_policy: "disable"` to require Codex for
`provider: auto`. `warn` is the default and prints a warning on Claude fallback;
`allow` records the downgrade in provenance without escalating the message.

Explicit degraded modes stay available:

```bash
scafld review <task-id> --runner local
scafld review <task-id> --runner manual
```

- `local`: prints the challenger prompt for the current shared runtime and leaves
  the round `in_progress`
- `manual`: handoff-only mode, also leaving the round `in_progress`

`--json` remains the control-plane snapshot. It does not spawn an external
reviewer. It reports runner/provider/model overrides as snapshot metadata only.

When the workspace includes them, the thin review wrappers remain optional
integration surfaces:

- `scripts/scafld-codex-review.sh <task-id>`
- `scripts/scafld-claude-review.sh <task-id>`

They are not the primary review product surface anymore. They are thin provider
adapters over the same handoff contract.

## Challenger Stance

The challenger is not another executor pass.

Its job is to:

- attack the diff
- attack the contract
- cite exact file and line
- separate blocking vs non-blocking findings
- use explicit severity: `critical`, `high`, `medium`, or `low`

The challenger does not edit code.

When scafld runs an external challenger itself, it still owns the canonical
review artifact. The subprocess returns prose markdown sections; scafld writes a
candidate latest review round, parses it with the same gate as a manual review,
and only persists it if the artifact is valid. Invalid external output fails
`scafld review` and does not print a completion command.

Failed external attempts, timeouts, and malformed external output write raw
provider diagnostics under `.ai/runs/<task-id>/diagnostics/`. The command error
prints the diagnostic path so the paid model output remains inspectable even
when it cannot be accepted as a review round.
Diagnostics include the prompt sha256 and a bounded prompt preview so the
operator can connect a rejected provider response to the exact challenger prompt
that produced it.

Finding format:

- `- **high** \`path/file.py:88\` — the exact failure mode and why it matters`

`Blocking` and `Non-blocking` accept only finding bullets or `None.`.
`Verdict` accepts only `pass`, `fail`, or `pass_with_issues`.
Any finding recorded in an adversarial section must be collected into
`Blocking` or `Non-blocking`; a clean verdict cannot coexist with section
findings.

If an adversarial section is clean, it must still say what was checked:

- `No issues found — checked callers of path/file.py`
- `No additional issues found — checked callers of path/file.py`
- `No issues found — checked AGENTS.md and CONVENTIONS.md`

Generic clean notes such as `checked everything` or `checked the diff` are not
evidence.

Provider isolation is recorded in review provenance. Codex runs with the
read-only ephemeral subprocess path. Claude uses restricted tools and a fresh
session, but its CLI does not expose an equivalent sandbox here, so fallback from
Codex to Claude is marked as weaker isolation.

Provider invocation session entries also carry status, timing, timeout, exit,
diagnostic, and attribution confidence fields. Observed provider facts can
coexist with unknown model facts; reports keep requested-only and unknown model
attribution separate from proven model separation.
When a provider exposes its billed model in a parseable envelope or stable output
hint, scafld records `model_observed` and marks confidence as `observed`. If the
provider only exposes the requested model, the entry stays `requested_only`; if
neither is available, it stays `unknown`.

Reports distinguish auto fallback downgrades from weaker challenger review
isolation. `isolation downgrades` counts `provider: auto` runs that fell through
to Claude. `weaker review isolation` also counts explicit Claude challenger
review runs because they do not use the Codex read-only ephemeral subprocess
path.

The report's clean-review count is a format-compliance signal. It means the
review used accepted no-issues phrasing with concrete checked targets; it does
not prove the reviewer actually inspected those targets.

The external runner prompt keeps trusted challenger instructions outside the
untrusted handoff boundary. The generated handoff, spec text, automated results,
and session notes are fenced as data for the reviewer to inspect, not
instructions the provider may obey.

## Complete

```bash
scafld complete <task-id>
scafld complete <task-id> --human-reviewed --reason "manual audit"
```

`complete` checks:

- review exists
- latest round is structurally valid
- configured adversarial sections are filled
- adversarial findings use grounded severity plus `file:line`
- verdict is not blocking
- reviewed git state still matches the workspace

That reviewed git state is bound to the engineering workspace, not scafld's own
review control plane. The gate excludes:

- the review artifact itself: `.ai/reviews/{task-id}.md`
- the task-scoped run tree: `.ai/runs/{task-id}/`

That means rerendering the challenger handoff or updating task-run metadata does
not create fake review drift. Real product-file changes still do.

If the challenger blocks completion, a human may apply the audited override
path after a completed challenger review round exists. That override is
recorded in both the review artifact and the session ledger.

## Session Entries

The review gate writes typed session entries such as:

- `challenge_verdict`
- `human_override`

`report` derives `challenge_override_rate` from those entries.

`report` also summarizes challenger review signal across the workspace:

- completed challenger rounds
- grounded findings
- clean reviews that still record what was checked
