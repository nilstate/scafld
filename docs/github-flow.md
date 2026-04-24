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
If a wrapper keeps its own receipts, journals, or pushed outputs, those records
live outside scafld and consume the projections scafld emits.

## What belongs in scafld

scafld owns the local engineering state:

- what the task is
- what branch it is bound to
- whether the workspace drifted
- whether review passed
- how much acceptance work is done

GitHub-facing tools should project that state outward. They should not rebuild
it from markdown scraping, branch-name conventions, or extra wrapper-only task
objects.

## Issue -> Branch -> Review -> PR

The happy path looks like this:

```bash
scafld plan add-auth -t "Add auth middleware"
scafld approve add-auth
scafld build add-auth
scafld branch add-auth
scafld status add-auth
scafld review add-auth
scafld summary add-auth
scafld checks add-auth --json
scafld pr-body add-auth
```

That sequence is enough to drive local work, issue updates, CI checks, and PR
body generation from one spec.

## 1. Record the issue source

scafld itself does not fetch GitHub issues. A wrapper such as runx, or a human,
can stamp provider metadata into `origin.source` when a task starts.

Example:

```yaml
origin:
  source:
    system: "github"
    kind: "issue"
    id: "123"
    title: "Add auth middleware"
    url: "https://github.com/org/repo/issues/123"
```

That tells the rest of the workflow what this task came from without making
scafld depend on the GitHub API.

## 2. Bind the task to a branch

Use `scafld branch` to create or bind the working branch and record the git
binding in the spec:

```bash
scafld branch add-auth
```

After that, `scafld status` should tell a human operator what this task is tied
to without needing `--json`:

```text
Add Auth Middleware
     id: add-auth
   file: .ai/specs/active/add-auth.yaml
 status: in_progress
 phases: 1 active / 2 pending  (3 total)
 source: github issue #123 - Add auth middleware
    url: https://github.com/org/repo/issues/123
 branch: add-auth  base: origin/main
upstream: origin/add-auth
binding: created branch
 remote: origin
   sync: in_sync
updated: 2026-04-21T10:00:00Z
```

The important point is not the exact spacing. The important point is that the
human surface now answers:

- what upstream issue this task came from
- what branch is bound
- whether the branch was created, rebound, or checked out
- whether the workspace is still in sync

## 3. Execute and review locally

The branch is not the workflow object. The spec is. Keep driving the task
through the spec lifecycle:

```bash
scafld build add-auth
scafld review add-auth
```

At that point scafld owns the information GitHub projections need:

- acceptance progress
- review verdict and findings
- branch/base/upstream binding
- sync drift reasons, if any

## 4. Publish the issue update

Use `summary` for issue comments, chat updates, or job summaries:

```bash
scafld summary add-auth
```

The markdown is concise on purpose. It tells outside systems what matters
without inventing another summary format.

Wrappers should publish that markdown directly or consume `summary --json` if
they need both the rendered markdown and the underlying projection model. JSON
mode includes `result.projection.surface = engineering_summary` so wrappers can
route it without guessing.

## 5. Publish the CI check

Use `checks --json` inside GitHub Actions or any other CI runner:

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
`checks --json` also includes `result.projection.surface = ci_check` for
callers that map scafld projections into their own output records.

## 6. Publish the PR body

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
`pr-body --json` includes `result.projection.surface = pull_request_body` for
the same reason.

## 7. Keep wrappers thin

Wrappers such as runx should stay thin here:

- record issue metadata in `origin.source`
- call native scafld commands
- consume native JSON envelopes
- map `summary`, `checks`, and `pr-body` into wrapper-owned output objects or
  upstream push calls when needed
- keep receipts, journals, and pushed-output identifiers outside the scafld
  spec
- publish the rendered markdown or structured check payloads
- avoid rebuilding task, review, or git-binding logic from file paths or
  markdown parsing

For governed wrappers that need explicit lifecycle boundaries, scafld also
exposes the split native surface behind `--advanced`:

- `new` for draft creation
- `start` for the approved -> in-progress transition
- `exec` for acceptance execution

Those commands are the same underlying lifecycle, not a second workflow model.
Human operators can keep using `plan` and `build`; orchestration layers such as
runx can consume the thinner split surface without reimplementing scafld state.

That is how scafld stops feeling like an extra system and starts feeling like
the engineering system.
