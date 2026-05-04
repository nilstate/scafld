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
  core/
  prompts/
  runs/
  specs/
AGENTS.md
CONVENTIONS.md
```

Specs are project artifacts. Commit `.scafld/specs/`, `AGENTS.md`, and
`CONVENTIONS.md` when they describe shared project behavior.
