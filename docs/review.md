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

## Challenger Stance

The challenger is not another executor pass.

Its job is to:

- attack the diff
- attack the contract
- cite exact file and line
- separate blocking vs non-blocking findings

The challenger does not edit code.

## Complete

```bash
scafld complete <task-id>
scafld complete <task-id> --human-reviewed --reason "manual audit"
```

`complete` checks:

- review exists
- latest round is structurally valid
- configured adversarial sections are filled
- verdict is not blocking
- reviewed git state still matches the workspace

If the challenger blocks completion, a human may apply the audited override
path. That override is recorded in both the review artifact and the session
ledger.

## Session Entries

The review gate writes typed session entries such as:

- `challenge_verdict`
- `human_override`

`report` derives `challenge_override_rate` from those entries.
