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

## Agent-Facing Deterministic Gates

Every gate has a repair contract: trusted state, failure reason, evidence path,
expected shape, and allowed next command. That contract is visible in human
output, projected into the spec, and available to automation through
`status --json` and `handoff`.

scafld is strict in what it trusts and generous in what it explains. A gate can
block hard without making the next agent guess where to look.

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

From a source checkout, use the dev wrapper so local dogfood runs execute the
current Go tree instead of a stale copied binary:

```bash
./bin/scafld --version
```

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

For a new repository, or after project policy changes, run `scafld config`
once to write an evidence-backed configuration brief for the agent. The agent
opens the cited sources, writes only verified runtime policy into
`.scafld/config.yaml`, and puts non-runtime guidance in `AGENTS.md`,
`CLAUDE.md`, `.claude/rules`, or project prompts.

Acceptance runs do not depend on interactive shell startup. scafld detects
checked-in toolchain files such as `.tool-versions`, `mise.toml`,
`.ruby-version`, `.python-version`, `.node-version`, `.go-version`, and
`.java-version`, prepends the matching version-manager shims, then applies
explicit `execution` config.

## Concrete Artifacts

scafld is not a wrapper around a prompt. It writes artifacts the next agent can
read and the runtime can project deterministically.

A draft is ordinary Markdown:

```markdown
---
spec_version: "2.0"
task_id: add-cache
status: draft
harden_status: in_progress
---

# Add Cache

## Acceptance

- [ ] `ac1` test - cache package tests pass.
  - Command: `go test ./internal/cache`
  - Expected kind: `exit_code_zero`

## Harden Rounds

### round-1

Status: in_progress
Started: 2026-05-07T09:15:00Z
Ended: none

Questions:
- Which invariant makes cache invalidation safe across tenants?
  - Grounded in: spec_gap:Context
  - Recommended answer: Add `tenant_isolation` to the task invariants and test it with a cross-tenant fixture.
  - Answered with: Accepted; phase 2 now owns the fixture and assertion.
```

Status exposes the same state without scraping Markdown:

```json
{
  "task_id": "add-cache",
  "status": "review",
  "title": "Add Cache",
  "next": "scafld review add-cache",
  "session_ok": true,
  "review": {
    "running": true,
    "attempt_status": "running",
    "reason": "review provider running"
  }
}
```

Review failure is structured, not a vibe:

```json
{
  "verdict": "fail",
  "mode": "discover",
  "summary": "Review found one open completion blocker.",
  "findings": [
    {
      "id": "cache-tenant-leak",
      "severity": "high",
      "blocks_completion": true,
      "location": {"path": "internal/cache/store.go", "line": 88},
      "evidence": "invalidation keys omit tenant id",
      "impact": "cross-tenant cache state can leak",
      "validation": "go test ./internal/cache",
      "summary": "internal/cache/store.go:88 invalidation keys omit tenant id."
    }
  ],
  "attack_log": [
    {"target": "cache invalidation", "attack": "trace tenant key construction", "result": "finding"}
  ],
  "budget": {"actual_findings": 1, "actual_attack_angles": 1}
}
```

```text
review verdict: fail
review mode: discover
summary: Review found one open completion blocker.
findings:
- [high/blocks completion] cache-tenant-leak: internal/cache/store.go:88 invalidation keys omit tenant id.
  location: internal/cache/store.go:88
  validation: go test ./internal/cache
next: scafld handoff add-cache
```

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
- `config`: propose evidence-backed project config without applying it
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

`scafld update` refreshes default project prompt copies when they are still
known defaults. Customized project prompts are skipped. It also refreshes root
agent docs and renders generated `.scafld/config.yaml` into the current strict
runtime shape.

## Adversarial Review

`scafld review` defaults to the provider configured in `.scafld/config.yaml`.
Fresh workspaces use `provider: auto`, selecting an installed external
challenger. It also supports explicit command, Claude, Codex, and local paths.
External providers receive a review brief, inspect the work, and return a
structured verdict. Provider adapters run read-only by default. scafld checks
local scope drift before invoking the provider, then accepts the verdict only if
the task-relevant review surface stayed stable while the provider ran.

Approval captures the workspace baseline before task execution starts. Review
uses the spec's packages, impacted files, and phase changes to derive task
scope, so pre-existing unrelated dirt is context, not a blocker. Use
`--review-scope` only when a dirty monorepo needs an explicit path boundary.
Unrelated workspace churn from another task should not make you pay for another
review run.

```bash
scafld review add-cache --provider claude
scafld review add-cache --provider codex
scafld review add-cache --provider command --provider-command "./reviewer"
scafld review add-cache --review-scope api,cli/packages/mcp
scafld review add-cache --print-context
scafld review add-cache --human-reviewed --reason "operator reviewed PR 123"
```

`--print-context` renders the exact deterministic review brief without invoking
a provider, so agents can debug what the challenger will see before spending a
review run.

`--human-reviewed` is the audited escape hatch. It belongs to `review`, not
`complete`: it records a `review_override` event and a passing human review
event in the session ledger. Use it only when a human has actually reviewed the
diff, spec, acceptance evidence, and scope.

Model defaults are configurable per provider:

```yaml
review:
  external:
    provider: "auto"
    codex:
      model: "gpt-5.5"
    claude:
      model: "claude-opus-4-7"
  context:
    # Aggregate rendered section-body budget for the provider brief.
    max_bytes: 16384
    files:
      - AGENTS.md
      - CLAUDE.md
      - .claude/rules
      - README.md
      - docs/review.md
      - .scafld/core/schemas/review_dossier.json
  dossier:
    max_findings: 12
    min_attack_angles: 6
    review_depth: "standard"
    rerun_policy: "verify_open_blockers"
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
- `review_pass_rate`
- `review_dossier_coverage`
- `review_findings_total`
- `review_open_blockers_total`
- `review_attack_angles_total`
- `challenge_override_rate`

`scafld report --json` derives those metrics from session ledgers:

```json
{
  "total": 12,
  "by_status": {
    "draft": 2,
    "review": 1,
    "completed": 9
  },
  "metrics": {
    "first_attempt_pass_rate": 0.67,
    "first_attempt_passes": 8,
    "first_attempt_total": 12,
    "recovery_convergence_rate": 0.75,
    "recovered_tasks": 3,
    "recovery_total": 4,
    "challenge_override_rate": 0,
    "challenge_overrides": 0,
    "review_challenge_total": 2,
    "review_dossier_coverage": 1,
    "review_dossier_total": 10,
    "review_findings_total": 14,
    "review_open_blockers_total": 3,
    "review_attack_angles_total": 42,
    "review_mode_distribution": {
      "discover": 7,
      "verify": 3
    }
  }
}
```

The honest boundary is that scafld can produce a better contract, handoff, and
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
