---
title: Installation
description: Install and configure scafld
---

# Installation

## Requirements

- Git
- A supported OS/architecture for the native binary:
  - macOS amd64/arm64
  - Linux amd64/arm64
  - Windows amd64/arm64

## Install

Install the latest GitHub release binary:

```bash
curl -fsSL https://raw.githubusercontent.com/nilstate/scafld/main/install.sh | sh
```

Install with Go:

```bash
go install github.com/nilstate/scafld/v2/cmd/scafld@latest
```

Install with Homebrew:

```bash
brew install nilstate/tap/scafld
```

Install from npm:

```bash
npm install -g scafld
```

Install from PyPI:

```bash
pipx install scafld
```

Install from Scoop:

```powershell
scoop bucket add nilstate https://github.com/nilstate/scoop-bucket
scoop install scafld
```

WinGet is submitted upstream as `0state.scafld`; it becomes installable with
`winget install 0state.scafld` after Microsoft package review.

npm and PyPI packages are thin launchers. Homebrew, Scoop, and WinGet point at
the same native Go release assets and checksums.

## Verify

```bash
scafld --version
```

## Development Checkout

When working inside the scafld source repository, use the checked-in dev wrapper
instead of a copied compiled binary:

```bash
./bin/scafld status my-task
```

The wrapper runs the current Go source with `go run ./cmd/scafld` and preserves
the caller's workspace root when `--root` is not supplied. That keeps dogfood
runs from reading stale lifecycle code.

## Initialize a Project

Navigate to your project root and run:

```bash
scafld init
```

This creates the `.scafld/` workspace structure:

```text
.scafld/
  config.yaml
  config.local.yaml
  core/                 # generated reset copy: schemas, prompts, scripts
  prompts/             # optional project-owned prompt overrides
  runs/                # local runtime evidence and diagnostics
  specs/               # committed living specs
AGENTS.md
CLAUDE.md
```

Root `AGENTS.md` and `CLAUDE.md` are real merged files, not symlinks. They are
installed at the repository root because that is the discovery surface most
agents read before doing work. scafld owns only the top-level scafld section and
preserves project-owned headings below it.

Use `scafld init --no-agent-docs` only when you will wire the agent contract
yourself.

Commit `.scafld/config.yaml`, `.scafld/specs/`, custom `.scafld/prompts/`,
`AGENTS.md`, and `CLAUDE.md` when they describe shared project behavior. Keep
`.scafld/core/`, `.scafld/config.local.yaml`, and `.scafld/runs/` local.
The committed `.scafld/config.yaml` is sparse by design. Runtime defaults live
in the binary, and the full example shape lives in `.scafld/core/config.yaml`.
Use `.scafld/config.local.yaml` only for personal machine overrides such as
shim paths, provider binaries, or temporary model choices.
Project convention docs are optional context. scafld does not require a
specific conventions filename; convention enforcement comes from config
invariants, spec scope, acceptance criteria, and review agenda.

To tighten config for a real project, run:

```bash
scafld config
```

This writes `.scafld/config.proposed.yaml` with evidence-backed suggestions.
Review the cited sources before copying anything into `.scafld/config.yaml`.
