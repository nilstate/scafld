# scafld

**A deterministic protocol for multi-phase agent work.**
The agent passes through. The protocol stays.

Plans outlive agents. Sessions hold the receipts. Reviews take nothing on faith.

Given the same spec and session ledger, scafld derives the same state, next
command, and review gate.

scafld is spec-driven orchestration for AI coding agents. The work starts from
an explicit spec, gets hardened before real effort, executes phase-bounded, and
ships only through adversarial review. The differentiator is simple: **the agent
does not get to grade its own homework**.

## Install

```bash
pipx install scafld
scafld --version
```

This PyPI package installs a `scafld` console script that downloads and runs the
native Go binary from the matching GitHub release.

## Quick Start

```bash
scafld init
scafld plan add-cache --command "go test ./..."
scafld harden add-cache
scafld harden add-cache --mark-passed
scafld approve add-cache
scafld build add-cache
scafld review add-cache
scafld complete add-cache
```

The lifecycle is deliberately small:

```text
draft -> harden -> approved -> active -> review -> completed
```

`complete` only closes reviewed work. If adversarial review finds a blocking
issue, scafld sends the task to repair instead of letting the implementation
agent wave itself through.

## Distribution Shim

The Go binary is the product. This PyPI package is a distribution shim that
fetches the matching native binary from GitHub releases.

Environment overrides:

- `SCAFLD_BINARY=/path/to/scafld` runs a local binary instead of downloading.
- `SCAFLD_INSTALL_DIR=/custom/cache` controls where downloaded binaries are cached.
- `SCAFLD_INSTALL_BASE_URL=https://host/assets` downloads release assets from a mirror.

## Links

- Docs: <https://0state.com/scafld/docs>
- Source: <https://github.com/nilstate/scafld>
- Releases: <https://github.com/nilstate/scafld/releases>
