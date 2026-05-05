# scafld

[![CI](https://github.com/nilstate/scafld/actions/workflows/ci.yml/badge.svg)](https://github.com/nilstate/scafld/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nilstate/scafld/v2)](https://goreportcard.com/report/github.com/nilstate/scafld/v2)
[![Go Reference](https://pkg.go.dev/badge/github.com/nilstate/scafld/v2.svg)](https://pkg.go.dev/github.com/nilstate/scafld/v2)
[![License](https://img.shields.io/github/license/nilstate/scafld)](LICENSE)

**A deterministic protocol for multi-phase agent work.**
The agent passes through. The protocol stays.

Plans outlive agents. Sessions hold the receipts. Reviews take nothing on faith.

Given the same spec and session ledger, scafld derives the same state, next
command, and review gate.

The work starts from an explicit spec, gets hardened before real effort,
executes phase-bounded, and ships only through adversarial review. The
differentiator is simple: **the agent does not get to grade its own homework**.

## Identity

scafld is a scaffold in the literal sense: temporary structure that shapes the
build while the work is in progress.

It turns a readable Markdown spec into a hardened contract, then into a
phase-bounded build loop. It records every important event in a durable session
ledger and blocks completion until the work has evidence and adversarial review
behind it. The spec stays readable. The runner stays strict. The agent always
has a contract, a next step, and a way to prove what happened.

## Why

Long-running AI coding work fails when the task only lives in chat. Context
drifts, acceptance criteria soften, reviews become vibes, and nobody can tell
which command proved which claim.

scafld gives the work a hard shape:

- `spec`: what must be true, hardened before execution
- `session`: what happened
- `handoff`: transport for the next model voice, never the source of truth

The spec is the living task contract. The session is the durable evidence
ledger. Adversarial review is the completion gate.

## Install

Installer script:

```bash
curl -fsSL https://raw.githubusercontent.com/nilstate/scafld/main/install.sh | sh
```

Go:

```bash
go install github.com/nilstate/scafld/v2/cmd/scafld@latest
```

Homebrew:

```bash
brew install nilstate/tap/scafld
```

npm:

```bash
npm install -g scafld
```

PyPI:

```bash
pipx install scafld
```

Scoop:

```powershell
scoop bucket add nilstate https://github.com/nilstate/scoop-bucket
scoop install scafld
```

WinGet is submitted upstream as `0state.scafld`; it becomes installable with
`winget install 0state.scafld` after Microsoft package review.

npm and PyPI packages are thin launchers. Homebrew, Scoop, and WinGet point at
the same native Go release assets and checksums.

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

Hardening lives between draft and approval. `scafld harden <task-id>` opens a
hardening round and `scafld harden <task-id> --mark-passed` records that the
draft has survived the interrogation. It is the discipline of attacking the spec
before build: product goal, authority, ownership boundaries, hidden cutovers,
halfway failures, recovery commands, testable invariants, and golden examples.
A weak spec should not become approved work.

`complete` only closes reviewed work. If adversarial review finds a blocking
issue, scafld sends the task to repair instead of letting the implementation
agent wave itself through.

## Command Surface

The daily surface is small:

```bash
scafld init
scafld plan <task-id>
scafld harden <task-id>
scafld validate <task-id>
scafld approve <task-id>
scafld build <task-id>
scafld review <task-id>
scafld complete <task-id>
scafld status <task-id>
scafld list
scafld report
scafld handoff <task-id>
scafld update
```

Wrapper intent:

- `plan`: create a draft Markdown spec
- `harden`: stress-test the draft before approval
- `validate`: reject malformed or non-executable spec structure
- `approve`: accept a spec only after it is clear enough to execute
- `build`: run phase acceptance criteria and write evidence
- `review`: run the adversarial review gate
- `status`: expose the current state and allowed follow-up command
- `handoff`: render model-facing repair or execution material

## Mental Model

```text
draft spec -> hardening -> approval -> phase execution -> session evidence -> spec projection -> adversarial review
```

Two artifacts matter:

- `spec`: the living task contract under `.scafld/specs/**/*.md`
- `session`: the evidence ledger under `.scafld/runs/{task-id}/session.json`

The spec is what humans and agents read. The session is what scafld trusts. When
a command passes, a phase completes, or review returns a verdict, scafld records
the evidence in the session first, then projects the current state back into the
Markdown spec.

That split is the core discipline: readable work surface, durable proof surface.

Hardening and adversarial review are the two pressure points:

- hardening challenges the contract before work starts
- adversarial review challenges the result before work completes

Hard rules:

- the session is the only durable run-state source
- handoffs are not read back to compute state
- telemetry, status, and reports are views, not separate sources of truth
- review providers must not mutate the workspace
- completion is a lifecycle transition, not a sentiment

## Hardening

Hardening is pre-build adversarial thinking. It asks whether the agent is about
to execute the right contract, not whether it can satisfy a vague request.

Run it while the spec is still a draft:

```bash
scafld harden add-cache
scafld harden add-cache --mark-passed
```

The first command enters HARDEN MODE, prints the active harden prompt, and
records a round in the spec. Questions in that round carry `Grounded in`
citations such as `spec_gap:scope`, `code:internal/app/build/build.go:42`, or
`archive:previous-cutover`. The second command verifies those citations and
refuses to close the round when they do not resolve. Approval is still an
explicit operator decision, but a complete plan spec should be
hardened when the task is ambiguous, high-risk, cross-cutting, or likely to
outlive one agent turn.

A hardened spec should answer:

- What is the real product goal, not just the requested implementation?
- What is authoritative when two artifacts contain the same fact?
- What are the ownership boundaries?
- What fails halfway, and how is it repaired?
- What invariants must be testable?
- What hidden cutovers are bundled?
- What examples or golden fixtures prove the shape?
- What operational command lets a human recover?

This is why scafld treats approval as meaningful. Approval is not "the draft
exists"; approval means the contract is sharp enough for an agent to execute
without improvising the definition of done.

## Workspace Shape

```text
.scafld/
  config.yaml              project config
  core/                    managed framework assets
  prompts/                 project-owned prompt overrides
  specs/                   living Markdown specs
    drafts/
    approved/
    active/
    archive/
  runs/                    session ledgers, diagnostics, handoffs
```

`.scafld/core/` is managed by scafld. Specs, config, prompts, and run evidence
belong to the project.

Prompt ownership is deliberate:

- `.scafld/prompts/*` is the active project-owned layer
- `.scafld/core/prompts/*` is the managed reset copy refreshed by `scafld update`

## Adversarial Review

`scafld review` defaults to the provider configured in `.scafld/config.yaml`.
Fresh workspaces use `provider: auto`, selecting an installed external
challenger. It also supports explicit command, Claude, Codex, and local paths.
External providers receive a review brief, inspect the work, and return a
structured verdict. Provider adapters run read-only by default, and the review
app checks for workspace mutation before accepting a verdict.

```bash
scafld review add-cache --provider claude
scafld review add-cache --provider codex
scafld review add-cache --provider command --provider-command "./reviewer"
```

Model defaults are configurable per provider:

```yaml
review:
  external:
    provider: "auto"
    codex:
      model: "gpt-5.5"
    claude:
      model: "claude-opus-4-7"
```

The review agenda is configurable too. `review.automated_passes` and
`review.adversarial_passes` are included in the challenger prompt in explicit
order, so the project can state what "adversarial" means without changing code.

The local provider is useful for development and smoke tests only; local
verdicts cannot satisfy `scafld complete`. The product value comes from an
independent adversarial pass that can say no.

## Success Metrics

scafld claims quality lift only where it can measure it.

The canonical metrics are session-derived:

- `first_attempt_pass_rate`
- `recovery_convergence_rate`
- `challenge_override_rate`

`scafld report` surfaces aggregate task state today. The metrics above are the
standard scafld is designed to protect as session-derived reporting deepens. The
honest boundary is that scafld can produce a better contract, handoff, and
adversarial review gate; an external harness can still ignore the brief. That is
why the claims are framed as recorded outcomes, not prompt mysticism.

## Go Runtime

The Go binary is the authoritative implementation. The codebase uses a
hexagonal layout with import-boundary tests:

```text
cmd/scafld -> internal/adapters/cli -> internal/app -> internal/core
```

- `internal/core` is pure domain code.
- `internal/app` owns use cases and narrow ports.
- `internal/adapters` contains filesystem, Markdown, Git, process, provider,
  JSON, and terminal implementations.
- `internal/platform` contains small primitives such as atomic file writes and
  signal handling.

Run the full local gate:

```bash
make check
```

Build release artifacts locally:

```bash
make release-snapshot
```

## Distribution

Package-manager integrations are adapters over GitHub release assets:

- GitHub Releases: native binaries, `checksums.txt`, `manifest.json`
- Go modules: source/install channel
- npm and PyPI: verified native-binary launchers
- Homebrew and Scoop: published registry adapters
- WinGet: upstream manifest submission
- OCI: template under `package/`

See [docs/distribution.md](docs/distribution.md),
[docs/architecture.md](docs/architecture.md), and
[docs/release.md](docs/release.md).
