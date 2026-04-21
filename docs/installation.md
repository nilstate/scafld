---
title: Installation
description: Install and configure scafld
---

# Installation

## Requirements

- Python 3 (ships as a single-file CLI)
- Git (for scope auditing and diff tracking)

## Install

Install from PyPI:

```bash
pip install scafld
```

The PyPI package installs `PyYAML` automatically as a runtime dependency.

Install from npm:

```bash
npm install -g scafld
```

The npm package ships the same CLI/runtime bundle, but the executable still requires `python3` at runtime. Commands that edit YAML specs, such as `scafld harden`, also need `PyYAML` installed in that Python runtime:

```bash
python3 -m pip install PyYAML
```

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
scafld update --self
```

To refresh the current workspace's managed bundle:

```bash
scafld update
```

To refresh every scafld workspace under a root directory:

```bash
scafld update --scan-root ~/dev
```

## Initialize a project

Navigate to your project root and run:

```bash
scafld init
```

This scaffolds the full `.ai/` structure:

```
.ai/
  scafld/               # Managed runtime bundle refreshed by `scafld update`
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

When `scafld init` sees common repo markers such as `package.json`, lockfiles, `pyproject.toml`, or `requirements.txt`, it now seeds `.ai/config.local.yaml` with suggested build, test, lint, and typecheck commands. Unknown repo shapes keep the existing safe placeholder commands.

## Configuration

scafld uses two config files in `.ai/`:

- `scafld/config.yaml` -- framework-managed defaults refreshed by `scafld update`
- `config.yaml` -- project-level configuration and overrides committed with the repo
- `config.local.yaml` -- project-specific overrides (build/test/lint commands)

The local config merges on top of the project config, which itself layers on top of the managed bundle. See [Configuration](configuration) for the full reference.

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
