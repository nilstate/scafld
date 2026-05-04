---
title: GitHub Flow
description: Using scafld with issues, pull requests, and CI
---

# GitHub Flow

scafld is not a GitHub API client. It is the local workflow kernel that gives a
GitHub wrapper something trustworthy to publish.

The spec, session, and review verdict stay local and deterministic. A wrapper
can project that state into issue comments, PR bodies, or CI checks without
inventing a second task object.

## Practical Flow

```bash
scafld plan add-auth --title "Add auth middleware"
scafld harden add-auth
scafld harden add-auth --mark-passed
scafld validate add-auth
scafld approve add-auth
scafld build add-auth
scafld review add-auth --provider codex
scafld status add-auth --json
scafld complete add-auth
```

Branch creation, issue updates, PR creation, and check publication are wrapper
responsibilities. scafld's job is to make the local state hard enough that those
wrappers do not need to scrape chat or reinterpret Markdown.

## What Wrappers Should Consume

Use JSON envelopes:

```bash
scafld status add-auth --json
scafld list --json
scafld report --json
```

Use text surfaces when a human or model needs to read them:

```bash
scafld handoff add-auth
scafld report
```

Wrappers should publish scafld state outward. They should not rebuild lifecycle,
review, or acceptance state from branch names, issue labels, or private wrapper
journals.

## What Belongs Outside scafld

These are intentionally outside the current core binary:

- creating or switching git branches
- posting GitHub issue comments
- creating pull requests
- publishing CI check runs
- storing upstream push receipts

That boundary keeps scafld small and portable. The package release ships the
same local workflow kernel through Go, npm, PyPI, Homebrew, Docker, Scoop, and
Winget paths; platform-specific publishing can live in wrappers.

## Useful Wrapper Contract

A thin GitHub wrapper should:

- create or select the git branch
- call native scafld lifecycle commands
- publish `status --json` and `report --json` into GitHub surfaces
- keep GitHub receipt IDs outside the spec
- fail CI when `review` fails or `complete` refuses the gate

The important rule is ownership: GitHub is the collaboration surface; scafld is
the local evidence and review surface.
