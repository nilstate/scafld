# scafld

[![CI](https://github.com/nilstate/scafld/actions/workflows/ci.yml/badge.svg)](https://github.com/nilstate/scafld/actions/workflows/ci.yml)

scafld is a 0state Markdown-native execution framework for long-running AI coding
work. It turns a human-readable spec into a phase-bounded build loop, records
evidence in a session ledger, and gates completion through review.

Canonical repo: `https://github.com/nilstate/scafld`. Default branch: `main`.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/nilstate/scafld/main/install.sh | sh
```

Other channels:

```bash
go install github.com/nilstate/scafld/v2/cmd/scafld@latest
npm install -g scafld
pipx install scafld
```

npm and PyPI packages are thin launchers. They download the native Go binary
from the matching GitHub release and verify it against `checksums.txt`.

## Workflow

```bash
scafld init
scafld plan <task-id> --command "go test ./..."
scafld approve <task-id>
scafld build <task-id>
scafld review <task-id>
scafld complete <task-id>
```

The lifecycle is:

```text
draft -> approved -> active -> review -> completed
```

## Runtime Model

Two artifacts matter:

- `spec`: the living Markdown task contract under `.scafld/specs/**/*.md`
- `session`: the durable evidence ledger under `.scafld/runs/{task-id}/session.json`

The spec is what the agent reads. The session is what the runner proves. scafld
projects session evidence back into the spec so the file stays readable without
becoming the source of execution truth.

## Providers

`scafld review` supports local, command, Claude, and Codex provider paths. The
provider adapters are read-only by default and guarded against workspace
mutation.

```bash
scafld review <task-id> --provider claude
scafld review <task-id> --provider codex
scafld review <task-id> --provider command --provider-command "./reviewer"
```

## Package Architecture

The Go binary is authoritative. Package-manager integrations are distribution
adapters over GitHub release assets:

- GitHub Releases: native binaries, `checksums.txt`, `manifest.json`
- Go modules: source/install channel
- npm and PyPI: verified native-binary launchers
- Homebrew, Scoop, WinGet, OCI: templates under `package/`

See [docs/distribution.md](docs/distribution.md) and
[docs/release.md](docs/release.md).
