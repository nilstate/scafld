---
title: Installation
description: Install and configure scafld
---

# Installation

## Requirements

- Python 3 (ships as a single-file CLI)
- Git (for scope auditing and diff tracking)

## Install

Clone the repo and run the install script:

```bash
git clone https://github.com/nilstate/scafld.git ~/.scafld && ~/.scafld/install.sh
```

Or as a one-liner:

```bash
curl -fsSL https://raw.githubusercontent.com/nilstate/scafld/main/install.sh | sh
```

This clones scafld to `~/.scafld` and symlinks the `scafld` command to `~/.local/bin/`.

## Verify

```bash
scafld --version
```

## Update

```bash
cd ~/.scafld && git pull origin main
```

## Initialize a project

Navigate to your project root and run:

```bash
scafld init
```

This scaffolds the full `.ai/` structure:

```
.ai/
  config.yaml            # Validation rules, rubric, safety controls
  config.local.yaml      # Your overrides (build/test/lint commands)
  prompts/               # Plan + exec mode instructions
    plan.md
    exec.md
    review.md
  schemas/               # Spec validation schema
    spec.json
  specs/
    drafts/              # Planning in progress
    approved/            # Ready for execution
    active/              # Currently executing
    archive/             # Completed work
    examples/            # Reference specs
  logs/                  # Execution logs (gitignored)
  reviews/               # Review findings (gitignored)
AGENTS.md                # Your project's invariants and policies
CONVENTIONS.md           # Coding standards
```

The `.ai/specs/` directory and `AGENTS.md` should be committed to version control. Specs are project artifacts, not personal notes.

## Configuration

scafld uses two config files in `.ai/`:

- `config.yaml` -- base configuration (validation rules, rubric, review passes)
- `config.local.yaml` -- project-specific overrides (build/test/lint commands)

The local config merges on top of the base. See [Configuration](configuration) for the full reference.

## Editor integration

scafld specs are YAML files validated against `.ai/schemas/spec.json`. Point your editor's YAML language server at the schema for autocompletion:

```json
// .vscode/settings.json
{
  "yaml.schemas": {
    ".ai/schemas/spec.json": ".ai/specs/**/*.yaml"
  }
}
```
