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
3. emits a `challenger × review` handoff

The handoff lives at:

- `.ai/runs/{task-id}/handoffs/challenger-review.md`
- `.ai/runs/{task-id}/handoffs/challenger-review.json`

If the latest review round is still `in_progress`, rerunning `scafld review
<task-id>` refreshes that same round in place. It does not append a second
round until the challenger has actually finished the current one.

When the workspace includes them, the thin review runners make that handoff the
default input for the external reviewer runtime:

- `scripts/scafld-codex-review.sh <task-id>`
- `scripts/scafld-claude-review.sh <task-id>`

## Challenger Stance

The challenger is not another executor pass.

Its job is to:

- attack the diff
- attack the contract
- cite exact file and line
- separate blocking vs non-blocking findings
- use explicit severity: `critical`, `high`, `medium`, or `low`

The challenger does not edit code.

Finding format:

- `- **high** \`path/file.py:88\` — the exact failure mode and why it matters`

If an adversarial section is clean, it must still say what was checked:

- `No issues found — checked callers of path/file.py`

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
