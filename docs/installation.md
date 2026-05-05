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
  core/                 # managed reset copy: schemas, prompts, scripts, agent docs
    agentdocs/
      AGENTS.md
      CLAUDE.md
  prompts/             # project-owned prompt overrides
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
yourself. The CLI still installs `.scafld/core/agentdocs/` so the canonical
contract remains inspectable.

Commit `.scafld/config.yaml`, `.scafld/core/`, `.scafld/prompts/`,
`.scafld/specs/`, `AGENTS.md`, and `CLAUDE.md` when they describe shared
project behavior. Keep `.scafld/config.local.yaml` and `.scafld/runs/` local.
`CONVENTIONS.md` is project-owned: scafld references it when present but does
not create a placeholder conventions file.
