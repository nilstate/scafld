---
title: GitHub Flow
description: Project scafld state onto issues, pull requests, and CI
---

# GitHub Flow

scafld is not a GitHub API client. It is the workflow kernel that produces the
state GitHub-facing tools should publish.

That distinction matters. The spec, review artifact, origin binding, and sync
state remain the source of truth. PR bodies, issue comments, and CI checks are
deterministic projections of that source state, not a second workflow object.

## Core flow

```bash
scafld branch add-auth
scafld summary add-auth
scafld checks add-auth --json
scafld pr-body add-auth
```

- `branch` binds the task to the working branch and records repo/base/upstream
  facts in `origin`
- `summary` renders concise markdown for issue comments, chat updates, or job
  summaries
- `checks --json` emits CI-friendly status/detail fields without terminal
  scraping
- `pr-body` renders a deterministic pull-request body from the same spec state

## Example CI usage

Inside GitHub Actions or another local CI runner:

```bash
scafld checks add-auth --json > /tmp/scafld-check.json
python3 - <<'PY'
import json
payload = json.load(open("/tmp/scafld-check.json"))
check = payload["result"]["check"]
print(check["status"])
print(check["summary"])
for line in check["details"]:
    print(line)
PY
```

That is the whole point of the projection layer: CI consumes structured fields
that already exist natively in scafld.

## PR body generation

Use the rendered body directly:

```bash
scafld pr-body add-auth > /tmp/pr-body.md
```

The output carries:

- workflow state
- bound branch and sync state
- review verdict and finding counts
- acceptance progress
- objectives
- risk notes

Because it is generated from the live spec state, it stays aligned with the
actual engineering work instead of depending on hand-maintained PR prose.

## Wrapper boundary

Wrappers such as runx should stay thin here:

- call native scafld commands
- consume native JSON envelopes
- publish the rendered markdown or structured check payloads
- avoid rebuilding task/review/origin logic from file paths or markdown parsing

That is how scafld stops feeling like an extra system and starts feeling like
the engineering system.
