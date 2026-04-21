---
title: Review
description: Automated and adversarial review
---

# Review

After execution, `scafld review` runs automated checks and scaffolds an adversarial review artifact. The review gate is what prevents `scafld complete` from archiving work that hasn't been properly examined.

## Running a review

```bash
scafld review add-auth
scafld review add-auth --json
```

This does two things:

### 1. Automated passes

Deterministic checks that run immediately:

- **spec_compliance** -- re-runs all acceptance criteria to confirm the implementation still satisfies the spec
- **scope_drift** -- runs `scafld audit` to compare declared files against the current workspace change set

If either automated pass fails, the review stops with an error.

In JSON mode, the command still creates or appends the review artifact, but the
CLI returns a machine-readable envelope containing:

- automated pass results
- the opened review round number
- the review file path
- the required adversarial section titles
- the reviewer handoff prompt text

### 2. Adversarial scaffold

Creates `.ai/reviews/{task-id}.md` with a Review Artifact v3 structure. This scaffolds sections for three adversarial passes:

- **regression_hunt** -- for each modified file, find every caller and importer. What assumptions do they make that this change violates?
- **convention_check** -- read CONVENTIONS.md and AGENTS.md. Does the new code violate any documented rule?
- **dark_patterns** -- hunt for hardcoded values, off-by-one errors, missing null checks, race conditions, copy-paste errors, security issues

A reviewer (ideally a fresh agent session) fills in these sections with findings, each citing a specific file and line.

The scaffolded metadata also binds the review to the git state that was actually reviewed: current `HEAD`, whether the workspace was dirty, and a fingerprint of staged, unstaged, and untracked changes. The review file itself is excluded from that fingerprint so writing findings does not invalidate the review.

## Review artifact format

The review file at `.ai/reviews/{task-id}.md` contains metadata and findings:

```json
{
  "schema_version": 3,
  "round_status": "completed",
  "reviewer_mode": "fresh_agent",
  "reviewer_session": "",
  "reviewed_at": "2026-04-16T12:00:00Z",
  "reviewed_head": "0123456789abcdef0123456789abcdef01234567",
  "reviewed_dirty": true,
  "reviewed_diff": "8e2e0f2d5fe2a5e6db5a5f9e4d63ed1f4996f2d2cdece9d89a43be9b6f21a1a3",
  "pass_results": {
    "spec_compliance": "pass",
    "scope_drift": "pass",
    "regression_hunt": "pass",
    "convention_check": "pass_with_issues",
    "dark_patterns": "pass"
  }
}
```

## Verdict

The reviewer sets a verdict based on findings:

| Verdict | Meaning |
|---------|---------|
| `pass` | Zero findings. All passes clean. |
| `pass_with_issues` | Non-blocking findings only. Noted but doesn't block completion. |
| `fail` | One or more blocking findings. Must be fixed before completion. |

## Completing work

```bash
scafld complete add-auth
scafld complete add-auth --json
```

This reads the latest review round and gates on:

- Review file exists
- Latest round has `round_status: completed`
- Review metadata includes the reviewed git state
- Current `HEAD` and workspace fingerprint still match what was reviewed
- All adversarial sections have content
- Verdict is `pass` or `pass_with_issues`

If the verdict is `fail`, completion is blocked until the issues are fixed and a new review round passes.

In JSON mode, blocked completion returns a structured `review_gate_blocked`
error with the gate reason, review file, pass results, and any blocking
findings or malformed review details. Successful completion returns the archive
path, verdict, pass results, review round, and whether a human override was
applied.

## Human override

When the review gate is blocked and you need to proceed anyway:

```bash
scafld complete add-auth --human-reviewed --reason "manual audit completed"
```

This requires:
- Interactive terminal (TTY check)
- Explicit confirmation (you type the task-id to confirm)
- Re-runs automated passes if not already passed
- Creates an override review round with `reviewer_mode: human_override`
- Records the current reviewed git state in the override round as audit evidence

The override is audited -- the reason and timestamp are recorded in the review file.

## Why adversarial review works

Ask an AI "how did you do?" and it says great. Ask it "what's wrong with this?" and it finds real things -- a missing null check on line 47, a caller that assumes a parameter that just changed shape. The same model that rubber-stamps its own work will tear it apart when the task is framed as critique.

scafld structures this separation. Execution optimises for completion. Review optimises for finding flaws. The honesty is structural, not a prompt trick.
