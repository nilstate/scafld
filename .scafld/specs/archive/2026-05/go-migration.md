---
spec_version: '2.0'
task_id: go-migration
created: '2026-05-01T00:00:00Z'
updated: '2026-05-03T23:51:41Z'
status: completed
harden_status: passed
size: large
risk_level: high
---

# Migrate scafld to a Go implementation

## Current State

Status: completed
Current phase: phase9
Next: done
Reason: all Go migration phases and validation gates executed successfully
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-05-03T23:51:41Z
Review gate: passed

## Summary

Build a new Go implementation of scafld in a sibling folder, with the draft spec remaining in this scafld workspace. The goal is not a literal Python port. The goal is a smaller, faster, sharper hexagonal implementation that preserves scafld's product contract: living Markdown specs, evidence-backed execution, session-led reconciliation, adversarial review, deterministic handoffs, and a command surface that agents can use without ceremony.

The new codebase should feel like a product-grade Go tool from the first commit: pure domain core, use-case packages that read like product workflows, narrow ports owned by the use case, adapters isolated at the edges, deterministic tests, atomic file writes, stable JSON contracts, and zero compatibility shims for retired task-spec formats. It should make the important behavior easy to reason about and the unsafe behavior hard to write.

Implementation target: `../scafld-go/`.

Module path decision: the sibling codebase uses `github.com/nilstate/scafld-go`. It must not reuse `github.com/nilstate/scafld` unless a later cutover task physically moves the Go module into the existing repository and retires or renames the Python distribution.

The current repository remains the planning and validation source while the Go implementation is built. The final migration may later replace the Python runtime, but this task starts the Go codebase in a clean sibling folder so design quality is not constrained by the old package layout.

## Context

CWD: `.`

Packages:
- `scafld`
- `scafld/commands`
- `tests`
- `.scafld/core`
- `../scafld-go`

Files impacted:
- `../scafld-go/go.mod` (all, exclusive) - New Go module root using the current stable Go toolchain target
- `../scafld-go/cmd/scafld/main.go` (all, exclusive) - Thin executable entrypoint
- `../scafld-go/internal/core/spec` (all, exclusive) - Pure spec domain model and validation rules
- `../scafld-go/internal/core/session` (all, exclusive) - Pure session model, replay, and phase state rules
- `../scafld-go/internal/core/acceptance` (all, exclusive) - Pure criterion semantics and expected-kind evaluation
- `../scafld-go/internal/core/lifecycle` (all, exclusive) - Pure lifecycle state machine and transition policy
- `../scafld-go/internal/core/review` (all, exclusive) - Pure review packet, finding, verdict, and severity model
- `../scafld-go/internal/core/reconcile` (all, exclusive) - Pure projection planning from session truth into spec state
- `../scafld-go/internal/app` (all, exclusive) - Use-case packages for plan, approve, build, exec, review, complete, status, handoff, update, reconcile, report, fail, cancel; each package owns its required port interfaces
- `../scafld-go/internal/adapters/cli` (all, exclusive) - Command table, help text, JSON output, terminal output, and exit mapping
- `../scafld-go/internal/adapters/markdown` (all, exclusive) - Markdown spec parser, renderer, section updater, validation adapter, and golden fixtures
- `../scafld-go/internal/adapters/jsonstore` (all, exclusive) - JSON session persistence, atomic writes, and run artifact stores
- `../scafld-go/internal/adapters/filesystem` (all, exclusive) - Real filesystem implementation of workspace and file ports
- `../scafld-go/internal/adapters/process` (all, exclusive) - Process execution, PTY support, timeout handling, diagnostics
- `../scafld-go/internal/adapters/git` (all, exclusive) - Git baseline, dirty-tree checks, origin metadata, and changed-path classification
- `../scafld-go/internal/adapters/providers` (all, exclusive) - Codex, Claude, and local provider adapter implementations
- `../scafld-go/internal/adapters/terminal` (all, exclusive) - Interactive terminal capabilities and non-JSON rendering support
- `../scafld-go/internal/platform` (all, exclusive) - Cross-cutting infrastructure primitives with no product policy: atomic files, path normalization, locks, logging
- `../scafld-go/internal/platform/signal` (all, exclusive) - Signal handling, cancellation propagation, subprocess signal masking, and escalation policy
- `../scafld-go/internal/arch` (all, exclusive) - Import-boundary tests that enforce the hexagonal dependency rule
- `../scafld-go/internal/testkit` (all, exclusive) - Fixture workspaces, fake ports, clocks, process fakes, golden helpers
- `../scafld-go/internal/testkit/providerfake` (all, exclusive) - Provider fakes for streamed output, idle timeout, infinite stream, mutation, invalid packet, and crash scenarios
- `../scafld-go/internal/testkit/contracts` (all, exclusive) - Shared contract suites that run against fake and real adapters
- `../scafld-go/test/e2e` (all, exclusive) - Cross-platform binary-level workflow tests written in Go
- `../scafld-go/docs` (all, shared) - Go implementation notes and architecture decisions
- `../scafld-go/.github/workflows` (all, shared) - Go CI and release candidate checks
- `.scafld/specs/drafts/go-migration.md` (all, exclusive) - This living migration spec

Invariants:
- Dependency direction is absolute: `cmd/scafld -> internal/adapters/cli -> internal/app -> internal/core`; adapters implement ports and must not be imported by core or app.
- `internal/core/**` imports only standard library and other `internal/core/**` packages.
- `internal/core/**` has zero non-stdlib transitive dependencies.
- `internal/app/**` imports only standard library and `internal/core/**`; use-case-owned ports live inside the app package that needs them.
- Non-CLI adapters import standard library, `internal/core/**`, and `internal/platform/**` only; they satisfy app ports through Go's implicit interfaces without importing app packages.
- The CLI adapter is the composition root and is the only adapter allowed to import `internal/app/**`.
- Adapters must not call other adapters except through explicit CLI composition.
- `internal/platform/**` imports only standard library and contains no scafld product policy.
- Every IO-facing port method accepts `context.Context`; no use case starts work without a context.
- Errors are wrapped with `fmt.Errorf("%w", ...)`; callers match with `errors.Is` and `errors.As`; sentinel errors live in the owning package and app-level error codes are mapped once at the CLI adapter boundary.
- Signal handling is installed in one place. SIGINT and SIGTERM cancel the root context, subprocess start avoids the inherited-signal race window, and repeated interrupts escalate to process termination with recorded diagnostics.
- CLI exit codes are a published contract: `0` success, `1` generic/runtime failure, `2` invalid input or invalid spec, `3` validation or acceptance failure, `4` review gate failure, `5` cancelled or interrupted, `6` workspace/configuration failure.
- Workspace discovery is explicit: `--root` wins, then `SCAFLD_ROOT`, then walk up from cwd until `.scafld/` is found; commands that create a workspace require an explicit root or cwd target and never walk into a parent by surprise.
- Concurrency scope is explicit: command output pumps, subprocess watchdogs, provider invocations, signal cancellation, and multi-spec operations are race-tested; normal session writes are serialized per task run.
- Test architecture stays pragmatic: pure core and app tests carry most behavior, adapter integration tests prove real IO, and binary e2e tests cover command contracts plus known expensive failure classes.
- Fakes and real adapters share contract tests for provider, process, session store, and spec store behavior.
- Golden files are updated only through an explicit update flag or command, and generated diffs are reviewed as behavior changes.
- Markdown bytes are parsed and rendered only in `internal/adapters/markdown`; core and app operate on typed models.
- Session JSON files are read and written only in `internal/adapters/jsonstore`; core and app operate on typed session models and ports.
- Process execution, Git, provider calls, and terminal behavior live only in their adapter packages.
- The Go implementation must understand only living Markdown task specs under `.scafld/specs/**/*.md`.
- Session data is authoritative for execution evidence; specs are human-readable projections plus task intent.
- Every runtime write that records execution evidence appends or updates session before projecting the spec.
- Spec rendering preserves human-owned prose and fails closed on phase shape mismatch.
- CLI JSON contracts remain stable and are covered by golden tests.
- The final Go runtime contains no Python subprocess dependency for normal scafld commands.
- The new implementation lives in `../scafld-go/` until an explicit cutover task moves or replaces distribution paths.

Related docs:
- `README.md`
- `docs/spec-schema.md`
- `docs/run-artifacts.md`
- `docs/lifecycle.md`
- `docs/review.md`
- `docs/release.md`
- `https://go.dev/dl/`
- `https://go.dev/doc/modules/gomod-ref`

## Objectives

- Create a clean sibling Go codebase in `../scafld-go/` rather than embedding the rewrite inside the Python package tree.
- Resolve the current stable Go toolchain at phase start, then pin the selected minor and patch in `go.mod`, CI, and release docs.
- Preserve the scafld product model: living spec, session truth, deterministic reconciliation, execution evidence, adversarial review, and recovery-first ergonomics.
- Use hexagonal architecture as a hard implementation constraint, not a diagram: pure core, app use cases, use-case-owned ports, edge adapters, and import-boundary tests.
- Design package boundaries that map to product concepts, not old Python files.
- Use standard library first, with external dependencies admitted only when they buy clear correctness at IO, YAML, schema, PTY, or terminal boundaries.
- Build a test suite that proves behavior through golden fixtures, property/fuzz tests, race tests, and smoke tests instead of retrofitting every old Python test one-for-one.
- Keep the CLI pleasant for humans and precise for agents: predictable help, stable JSON, exact error codes, and actionable next commands.
- Produce a migration path where the Go runtime can replace the Python runtime without dual-mode task-spec behavior.

## Scope

In scope:

- A new Go module rooted at `../scafld-go/`.
- A production CLI binary at `cmd/scafld`.
- A hexagonal package layout with pure domain packages under `internal/core`, use cases and caller-owned ports under `internal/app`, concrete IO under `internal/adapters`, and infrastructure primitives under `internal/platform`.
- Go-native packages for spec modeling, session replay, lifecycle execution, review workflows, handoffs, workspace management, and JSON output.
- Adapters for Markdown specs, JSON session stores, filesystem workspaces, process execution, Git, external review providers, and terminal output.
- Import-boundary tests that fail the build when core or app code reaches outward, including transitive dependency checks for core.
- Golden fixtures copied from the released Markdown spec examples, then maintained as Go test fixtures.
- CI for formatting, vetting, tests, race tests, golden stability, and binary smoke checks.
- Release candidate packaging that can later feed GitHub releases, npm wrappers, and PyPI wrappers.
- Documentation that explains the Go architecture, package ownership, and migration cutover.

Out of scope for this task:

- Rewriting the current Python implementation in place.
- Supporting YAML task specs or `.ai/` workspaces. YAML config remains supported.
- Keeping Go and Python implementations feature-divergent after the final cutover.
- Preserving old tests when they encode old implementation details rather than product behavior.
- Shipping a public release from the Go codebase before parity and review gates pass.

## Dependencies

- A currently stable Go toolchain available locally or through Go's toolchain selection; this spec was drafted against go1.26.2, but the implementation must choose the stable version at phase start.
- The existing scafld Markdown spec examples remain available as golden fixtures.
- GitHub Actions can run Go test, vet, and race jobs.
- External Go dependencies are reviewed before adoption and pinned through `go.mod` and `go.sum`.
- Release packaging decisions for npm and PyPI can be deferred until the Go binary is parity-ready.

## Assumptions

- `../scafld-go/` may become a standalone repository or may be folded back into `scafld`; the module layout should not make either path painful.
- The initial module path is `github.com/nilstate/scafld-go` because the implementation starts in a sibling folder and must not collide with the released Python repository path.
- The current Python implementation is a behavioral oracle during migration, but the Go implementation is not required to preserve Python package names, internal file names, or old helper abstractions.
- The best Go implementation will be smaller than the Python implementation because it can start from the narrowed Markdown-only contract.
- Some existing smoke tests should be replaced with clearer Go-native acceptance tests rather than translated line by line.
- Interfaces should be introduced only at boundaries where a use case needs external capabilities; no package should define a broad service interface just to make mocking easy.
- The CLI adapter composes real adapters and passes ports into app use cases; business logic must not live in CLI handlers.

## Touchpoints

- CLI surface: `init`, `plan`, `approve`, `build`, `exec`, `review`, `complete`, `status`, `list`, `report`, `handoff`, `update`.
- Architecture surface: `cmd`, `core`, app-owned ports, `adapters`, `platform`, `testkit`, and `arch` tests.
- Workspace model: `.scafld/config.yaml`, `.scafld/config.local.yaml`, `.scafld/core/`, `.scafld/specs/`, `.scafld/runs/`, `.scafld/reviews/`.
- Spec model: Markdown front matter, H1 title, exact `##` section grammar, numbered phase headings, checklists, current-state projection.
- Evidence model: `session.json`, criterion states, phase blocks, provider invocations, diagnostics, review packets.
- Agent experience: handoff Markdown, JSON sidecars, next-action guidance, recovery handoffs.
- Distribution: native binary first, then npm and PyPI wrappers around that binary.

## Test Architecture

The test suite should stay small enough to run often and strong enough to catch the failures scafld is prone to:

- Core and app tests are pure Go and fast.
- Adapter tests exercise real filesystem, process, Git, Markdown, JSON, and provider boundaries.
- Binary e2e tests are reserved for CLI contracts and known IO failure classes.
- Provider fakes must simulate streamed JSON, silence, endless streams, workspace mutation, invalid packets, and mid-stream crashes.
- Contract tests run the same behavior suite against fakes and real adapters where both exist.
- Race tests include concrete concurrent scenarios: session write contention, reconcile contention, watchdog while output is pumping, and signal cancellation during subprocess start.
- CI runs at minimum on Linux and macOS.

## Risks

- A mechanical port could preserve old complexity and miss the chance to simplify around the Markdown-only contract.
- Starting in a sibling folder can hide integration issues until late if parity tests do not exercise real workspaces.
- Go's static types can make the model clearer, but over-abstracting early could create ceremony instead of leverage.
- Hexagonal architecture can become performative if ports are too broad, adapters leak policy, or core packages accept IO-shaped data instead of domain values.
- Reusing the existing `github.com/nilstate/scafld` module path in a sibling repository would create import, release, and package identity confusion.
- Pinning the exact Go patch release in the spec would make the plan stale; the implementation must resolve the current stable toolchain at phase start and pin that in `go.mod` plus CI.
- Signal handling and subprocess cancellation have caused regressions before; the Go runtime must treat signal lifecycle as a top-level design surface, not a runner detail.
- CLI parity is broader than it appears because humans and agents rely on exact JSON envelopes, exit codes, and next-action text.
- Review and provider execution paths can mutate workspaces; isolation and dirty-tree checks need to be redesigned deliberately.
- Packaging native binaries through npm and PyPI adds release complexity that should not be mixed into the first executable slice.
- If session replay is under-tested, the living spec can drift from evidence under interrupted writes or failed projections.

## Acceptance

Profile: strict

Strict means this spec requires architecture and operational contract gates in addition to behavior gates. Import-boundary tests, context propagation, signal handling, error semantics, workspace discovery, mutation guard, and exit-code golden tests are release-blocking, not optional hardening.

Definition of done:
- [x] `dod1` The Go codebase exists in `../scafld-go/` with a clean module, command entrypoint, hexagonal internal package layout, and documented architecture.
- [x] `dod2` The Go CLI can initialize a workspace, create a Markdown spec, approve it, execute criteria, update session, reconcile spec state, and report status against a fixture task.
- [x] `dod3` Import-boundary tests prove core is pure, app depends only inward, ports are use-case-owned, and adapters stay at the edge.
- [x] `dod4` The Markdown adapter passes golden round-trip tests for the released skeleton and add-error-codes examples.
- [x] `dod5` Session writes are atomic, replay is deterministic, and spec projection is derived from session truth.
- [x] `dod6` The primary CLI JSON contracts match the released product behavior for success, validation failure, blocked execution, and review failure paths.
- [x] `dod7` Context propagation, wrapped error semantics, signal cancellation, and the CLI exit code table are documented and tested.
- [x] `dod8` Go CI passes format, vet, import-boundary tests, unit tests, race tests, golden stability, and smoke checks.
- [x] `dod9` Provider fakes, adapter contract tests, concrete race scenarios, and golden update discipline are implemented before parity work begins.
- [x] `dod10` The migration plan documents what Python runtime code can be deleted at final cutover and what packaging wrappers must remain or be rewritten.

Validation:
- [x] `v1` scaffold - Go module root exists in the sibling folder.
  - Command: `test -f ../scafld-go/go.mod && test -f ../scafld-go/cmd/scafld/main.go`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `v2` module - Go module uses the sibling module path and pins a stable Go toolchain selected at implementation time.
  - Command: `grep -q '^module github.com/nilstate/scafld-go$' ../scafld-go/go.mod && grep -Eq '^go 1\\.[0-9]+$' ../scafld-go/go.mod && grep -Eq '^toolchain go1\\.[0-9]+\\.[0-9]+$' ../scafld-go/go.mod`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `v3` format - Go code is formatted.
  - Command: `cd ../scafld-go && test -z "$(gofmt -l .)"`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `v4` vet - Go vet passes for all packages.
  - Command: `cd ../scafld-go && go vet ./...`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `v5` tests - Go tests pass for all packages.
  - Command: `cd ../scafld-go && go test ./...`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `v6` race - Race tests pass for packages with session, runner, and lifecycle concurrency.
  - Command: `cd ../scafld-go && go test -race ./...`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `v7` architecture - Import boundaries enforce the hexagonal dependency rule.
  - Command: `cd ../scafld-go && go test ./internal/arch -run ImportBoundaries`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `v8` golden - Spec and JSON golden fixtures are stable.
  - Command: `cd ../scafld-go && go test ./internal/adapters/markdown ./internal/adapters/cli ./internal/core/reconcile -run Golden`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `v9` smoke - Built Go binary completes the local fixture lifecycle.
  - Command: `cd ../scafld-go && go build -o ./bin/scafld ./cmd/scafld && go test ./test/e2e -run Lifecycle`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `v10` no legacy spec formats - Go production code has no YAML task-spec or `.ai/` workspace compatibility path.
  - Command: `rg -n '\.ai/' ../scafld-go --glob '*.go'`
  - Expected kind: `no_matches`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `v11` spec - This migration spec validates in the current scafld workspace.
  - Command: `./cli/scafld validate go-migration --json`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `v12` operational contracts - Context, errors, signals, workspace discovery, and exit codes are covered by focused tests.
  - Command: `cd ../scafld-go && go test ./internal/arch ./internal/app/... ./internal/adapters/cli ./internal/platform/signal -run 'Context|Error|Signal|ExitCode|WorkspaceDiscovery'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `v13` test architecture - Provider fakes, fake-vs-real contracts, race scenarios, and golden update discipline have focused tests.
  - Command: `cd ../scafld-go && go test ./internal/testkit/... ./internal/adapters/... ./test/e2e -run 'ProviderFake|Contract|RaceScenario|GoldenUpdate'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 1: Create the Go module and architecture spine

Goal: Establish `../scafld-go/` as a clean Go module with hexagonal package boundaries before product behavior is implemented.

Status: completed
Dependencies: none

Changes:
- `../scafld-go/go.mod` (all, exclusive) - Create module `github.com/nilstate/scafld-go`, set the currently stable `go 1.N` directive, set the matching `toolchain go1.N.P` directive, and keep dependencies empty until a package needs them.
- `../scafld-go/cmd/scafld/main.go` (all, exclusive) - Add a tiny main that delegates only to `internal/adapters/cli.Run(context.Context, []string, io.Writer, io.Writer) int`.
- `../scafld-go/internal/core` (all, exclusive) - Create package roots for pure spec, session, acceptance, lifecycle, review, and reconcile domain code.
- `../scafld-go/internal/app` (all, exclusive) - Create package roots for command use cases. Each public command maps to a use case with typed input and output.
- `../scafld-go/internal/app/*/ports.go` (all, exclusive) - Create use-case-owned interfaces for external capabilities. Keep interfaces narrow and delete any port that has no immediate use case.
- `../scafld-go/internal/adapters` (all, exclusive) - Create adapter roots for CLI, Markdown, JSON store, filesystem, process, Git, providers, and terminal.
- `../scafld-go/internal/platform` (all, exclusive) - Create infrastructure primitives for atomic files, path normalization, locks, and logging with no scafld product decisions.
- `../scafld-go/internal/platform/signal` (all, exclusive) - Create root-context cancellation, signal handling, subprocess signal-mask, and escalation primitives.
- `../scafld-go/internal/arch` (all, exclusive) - Add import-boundary tests that inspect `go list -json ./...` and fail on outward dependencies.
- `../scafld-go/internal/testkit` (all, exclusive) - Add fake ports, temp workspace helper, golden helper, and command-capture helpers.
- `../scafld-go/internal/testkit/contracts` (all, exclusive) - Add a tiny contract-test harness that can run the same assertions against fake and real adapters.
- `../scafld-go/internal/testkit/golden` (all, exclusive) - Add explicit golden update helpers that require an update flag and make generated diffs easy to review.
- `../scafld-go/Makefile` (all, shared) - Add `fmt`, `vet`, `test`, `race`, `smoke`, and `check` targets that wrap Go commands without hiding failures.
- `../scafld-go/docs/architecture.md` (all, shared) - Document the dependency rule, allowed imports, package ownership, and what belongs in core, app, ports, adapters, and platform.

Acceptance:
- [x] `ac1_1` scaffold - Go module and command entrypoint exist.
  - Command: `test -f ../scafld-go/go.mod && test -f ../scafld-go/cmd/scafld/main.go`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_2` command - The CLI prints version and help from the Go entrypoint.
  - Command: `cd ../scafld-go && go run ./cmd/scafld --version && go run ./cmd/scafld --help`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_3` architecture - Import-boundary tests pass before feature implementation begins.
  - Command: `cd ../scafld-go && go test ./internal/arch -run ImportBoundaries`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_4` quality - The initial Go developer loop passes.
  - Command: `cd ../scafld-go && make check`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac1_5` testkit - Contract-test and golden-update helpers exist and have self-tests.
  - Command: `cd ../scafld-go && go test ./internal/testkit/contracts ./internal/testkit/golden`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 2: Define pure core models and caller-owned ports

Goal: Define scafld's domain language and external capability seams before any adapter owns IO behavior.

Status: completed
Dependencies: phase1

Changes:
- `../scafld-go/internal/core/spec` (all, exclusive) - Define the normalized spec model, phase model, acceptance model, status values, validation rules, and spec errors with no Markdown dependency.
- `../scafld-go/internal/core/session` (all, exclusive) - Define session schema, entries, criterion states, phase blocks, provider invocation state, and replay rules with no filesystem dependency.
- `../scafld-go/internal/core/acceptance` (all, exclusive) - Define expected-kind semantics, result classification, manual criterion policy, and evidence requirements with no process execution dependency.
- `../scafld-go/internal/core/lifecycle` (all, exclusive) - Define lifecycle states, transition policy, phase progression policy, and recovery state without reading or writing files.
- `../scafld-go/internal/core/review` (all, exclusive) - Define review packets, findings, severity, verdicts, overrides, and repair guidance.
- `../scafld-go/internal/core/reconcile` (all, exclusive) - Define projection plans from spec model plus session state into updated spec state.
- `../scafld-go/internal/app/*/ports.go` (all, exclusive) - Define narrow use-case-owned interfaces for clocks, spec repository, session repository, workspace repository, process runner, Git state, provider invocation, diagnostics, bundle source, and terminal output.
- `../scafld-go/internal/app/contracts` (all, exclusive) - Define app-level command inputs, outputs, domain result DTOs, error envelopes, and next actions without JSON serialization or terminal formatting.
- `../scafld-go/internal/core/errors` (all, shared) - Define shared error classification helpers only if owning-package sentinel errors are insufficient.
- `../scafld-go/internal/arch` (all, shared) - Add tests that reject `os`, `os/exec`, Git, terminal, Markdown parser, JSON persistence imports, and non-stdlib transitive dependencies from core packages.

Acceptance:
- [x] `ac2_1` core - Core package tests pass without filesystem, process, Git, provider, or terminal imports.
  - Command: `cd ../scafld-go && go test ./internal/core/...`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_2` ports - Ports are narrow and use-case-owned; no interface exceeds the documented method limit without a written exception.
  - Command: `cd ../scafld-go && go test ./internal/arch -run PortsAreNarrow`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_3` boundary - Core and app import rules are enforced by architecture tests.
  - Command: `cd ../scafld-go && go test ./internal/arch -run 'CoreIsPure|CoreTransitiveDepsAreStdlib|AppDoesNotImportAdapters|PortsAreUseCaseOwned'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac2_4` errors - Error wrapping and classification are consistent across core and app packages.
  - Command: `cd ../scafld-go && go test ./internal/core/... ./internal/app/... -run 'Errors|ErrorWrapping|ErrorClassification'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 3: Build workspace and Markdown adapters

Goal: Implement `.scafld/` workspace IO and Markdown spec persistence as adapters around the pure core model.

Status: completed
Dependencies: phase2

Changes:
- `../scafld-go/internal/adapters/markdown/parse.go` (all, exclusive) - Convert Markdown bytes into `core/spec.Model` using front matter, exact section lookup, fence-aware heading scanning, phase heading identity, checklists, and labels.
- `../scafld-go/internal/adapters/markdown/render.go` (all, exclusive) - Render stable Markdown from `core/spec.Model` while preserving human-owned prose where the model has not changed it.
- `../scafld-go/internal/adapters/markdown/update.go` (all, exclusive) - Replace runner-owned section bodies, fail closed on phase shape mismatch, and preserve unknown sections.
- `../scafld-go/internal/adapters/markdown/validate.go` (all, exclusive) - Translate malformed Markdown into typed app/core errors without leaking parser internals.
- `../scafld-go/internal/adapters/markdown/testdata` (all, shared) - Copy released skeleton and add-error-codes examples as golden fixtures.
- `../scafld-go/internal/adapters/markdown/fuzz_test.go` (all, shared) - Add fuzz coverage for headings, fences, front matter, phase identity, and checklist parsing.
- `../scafld-go/internal/adapters/filesystem` (all, exclusive) - Implement workspace discovery, root paths, spec path lookup, artifact paths, directory creation, and project-owned vs managed-owned checks.
- `../scafld-go/internal/adapters/config` (all, exclusive) - Parse YAML config into typed app/core settings and validate prompt, provider, review, and execution settings.
- `../scafld-go/internal/adapters/bundle` (all, exclusive) - Embed core defaults with `embed.FS`, generated from the current `.scafld/core/` bundle at implementation time with a manifest hash and generation script.
- `../scafld-go/scripts/generate-core-bundle.go` (all, exclusive) - Generate the embedded bundle source from `.scafld/core/` and verify the manifest hash.

Acceptance:
- [x] `ac3_1` golden - Released Markdown examples parse and render byte-stable.
  - Command: `cd ../scafld-go && go test ./internal/adapters/markdown -run 'Golden|RoundTrip'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_2` adversarial - Spec parser rejects malformed front matter, duplicate phases, unclosed fences, and phase shape mismatch.
  - Command: `cd ../scafld-go && go test ./internal/adapters/markdown -run 'Reject|Malformed|Mismatch'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_3` fuzz - Spec parser fuzz seeds run without crash or invalid memory behavior.
  - Command: `cd ../scafld-go && go test ./internal/adapters/markdown -run '^$' -fuzz Fuzz -fuzztime 20s`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_4` workspace - Filesystem adapter initializes `.scafld/` and never creates `.ai/`.
  - Command: `cd ../scafld-go && go test ./internal/adapters/filesystem ./internal/adapters/bundle ./internal/adapters/config -run 'Workspace|Bundle|Config'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_5` discovery - Workspace discovery honors `--root`, then `SCAFLD_ROOT`, then cwd walk-up, and init never mutates an unintended parent.
  - Command: `cd ../scafld-go && go test ./internal/adapters/filesystem ./internal/adapters/cli -run WorkspaceDiscovery`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac3_6` bundle source - Embedded core bundle is generated from `.scafld/core/` and manifest hash verification passes.
  - Command: `cd ../scafld-go && go run ./scripts/generate-core-bundle.go --check --source ../scafld/.scafld/core`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 4: Implement session truth and reconciliation

Goal: Make the session ledger the evidence source of truth while keeping replay and projection pure and persistence adapter-owned.

Status: completed
Dependencies: phase3

Changes:
- `../scafld-go/internal/core/session/replay.go` (all, exclusive) - Replay typed entries into criterion states and phase blocks without reading Markdown checkmarks or JSON files.
- `../scafld-go/internal/core/reconcile/reconcile.go` (all, exclusive) - Produce projection plans for current state, phase statuses, criterion status, evidence, source event, review state, and planning log.
- `../scafld-go/internal/adapters/jsonstore/session_store.go` (all, exclusive) - Implement load, normalize, append, update-in-place for known running entries, atomic write, and fsync-backed replace.
- `../scafld-go/internal/platform/atomicfile` (all, exclusive) - Implement same-directory temporary file, fsync, rename, and cleanup primitives.
- `../scafld-go/internal/app/reconcile` (all, exclusive) - Orchestrate loading session and spec, applying projection plans, and saving through ports.
- `../scafld-go/internal/core/reconcile/testdata` (all, shared) - Add golden session-to-spec projection fixtures for passing, failing, blocked, review-repair, and interrupted runs.

Acceptance:
- [x] `ac4_1` atomic - Session writes use same-directory temporary files, fsync, and atomic rename.
  - Command: `cd ../scafld-go && go test ./internal/platform/atomicfile ./internal/adapters/jsonstore -run 'Atomic|Replace|Cleanup'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac4_2` replay - Session replay is deterministic and append-order preserving.
  - Command: `cd ../scafld-go && go test ./internal/core/session ./internal/core/reconcile ./internal/app/reconcile -run 'Replay|Projection|Idempotent'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac4_3` source of truth - Reconciliation does not read Markdown checkmarks to infer execution truth.
  - Command: `cd ../scafld-go && go test ./internal/core/reconcile ./internal/app/reconcile -run SourceOfTruth`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac4_4` race scenarios - Session write contention and same-task reconcile contention are covered by concurrent tests.
  - Command: `cd ../scafld-go && go test -race ./internal/adapters/jsonstore ./internal/app/reconcile -run 'SessionWriteContention|ReconcileContention'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 5: Port execution, acceptance, and lifecycle behavior

Goal: Implement execution as app orchestration over pure acceptance policy and process/session/spec ports.

Status: completed
Dependencies: phase2, phase3, phase4

Changes:
- `../scafld-go/internal/core/acceptance` (all, exclusive) - Implement expected kinds, command criteria, manual criteria, validation profile, evidence requirements, and result normalization without running commands.
- `../scafld-go/internal/core/lifecycle` (all, exclusive) - Implement status transitions for draft, approved, active, blocked, review, completed, failed, and canceled tasks.
- `../scafld-go/internal/app/plan` (all, exclusive) - Create specs through ports and domain models without direct Markdown or filesystem imports.
- `../scafld-go/internal/app/approve` (all, exclusive) - Approve draft specs and initialize session evidence through ports.
- `../scafld-go/internal/app/build` (all, exclusive) - Orchestrate phase execution, criterion evaluation, session append, projection, and next action.
- `../scafld-go/internal/app/exec` (all, exclusive) - Execute selected criteria and validation profiles through process and session ports.
- `../scafld-go/internal/app/complete` (all, exclusive) - Complete reviewed work, archive run artifacts, and preserve evidence through ports.
- `../scafld-go/internal/app/fail` (all, exclusive) - Mark failed work with recorded reason and recovery state.
- `../scafld-go/internal/app/cancel` (all, exclusive) - Cancel work without losing session evidence or spec state.
- `../scafld-go/internal/app/status` (all, exclusive) - Read spec plus session state and return agent-friendly status without mutating files.
- `../scafld-go/internal/adapters/process` (all, exclusive) - Implement command execution with cwd, env, timeout, output capture, PTY support, signal handling, and diagnostics.
- `../scafld-go/internal/adapters/git` (all, exclusive) - Implement workspace baseline, dirty state, changed paths, origin metadata, and review mutation detection.
- `../scafld-go/internal/app/runtime` (all, exclusive) - Implement next-action guidance that agents can follow without guessing.
- `../scafld-go/test/e2e/lifecycle_test.go` (all, shared) - Add lifecycle, blocked-run, recovery, phase-boundary, fail, cancel, signal, and status binary tests.

Acceptance:
- [x] `ac5_1` criteria - Acceptance expected kinds pass, fail, and explain invalid configurations.
  - Command: `cd ../scafld-go && go test ./internal/core/acceptance -run 'ExpectedKind|Invalid|Evidence'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac5_2` runner - Command runner handles cwd, output snippets, timeout, exit code, diagnostics, and cancellation.
  - Command: `cd ../scafld-go && go test ./internal/adapters/process -run 'Command|Timeout|Diagnostic|Cancel'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac5_3` lifecycle - Go binary executes a fixture from plan through completed state.
  - Command: `cd ../scafld-go && go test ./test/e2e -run Lifecycle`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac5_4` phase gates - Phase progression only checks off phase and acceptance state from recorded evidence.
  - Command: `cd ../scafld-go && go test ./internal/core/lifecycle ./internal/app/build ./internal/core/reconcile -run 'Phase|Criterion|Evidence'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac5_5` app purity - App use cases depend on ports, not concrete adapters.
  - Command: `cd ../scafld-go && go test ./internal/arch -run AppDoesNotImportAdapters`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac5_6` fail cancel - Fail and cancel lifecycle commands update session, project spec state, and return the documented exit codes.
  - Command: `cd ../scafld-go && go test ./internal/app/fail ./internal/app/cancel ./test/e2e -run 'Fail|Cancel'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac5_7` signal - SIGINT and SIGTERM cancel execution, record diagnostics, avoid the subprocess signal race, and escalate repeated interrupts.
  - Command: `cd ../scafld-go && go test ./internal/platform/signal ./internal/adapters/process ./test/e2e -run 'Signal|Interrupt|Terminate|Escalate'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 6: Port handoffs, review, and adversarial gates

Goal: Preserve scafld's review-driven rigor with pure review policy, app orchestration, and provider adapters isolated at the edge.

Status: completed
Dependencies: phase3, phase4, phase5

Changes:
- `../scafld-go/internal/core/review/packet.go` (all, exclusive) - Normalize review packet schema, findings, evidence, suggested fixes, tests, and spec-update suggestions.
- `../scafld-go/internal/core/review/signal.go` (all, exclusive) - Port review signal scoring and severity classification as pure logic.
- `../scafld-go/internal/app/handoff` (all, exclusive) - Build executor, challenger, recovery, and repair handoff models from spec/session state.
- `../scafld-go/internal/adapters/markdown/handoff.go` (all, exclusive) - Render handoff Markdown and JSON sidecars from typed handoff models.
- `../scafld-go/internal/app/review` (all, exclusive) - Orchestrate provider invocation, workspace mutation checks, timeouts, invalid-output handling, and repair handoff generation through ports.
- `../scafld-go/internal/adapters/providers` (all, exclusive) - Isolate Codex, Claude, and local shell provider details behind provider ports.
- `../scafld-go/internal/app/audit` (all, exclusive) - Port scope auditing and review surface projection use cases.
- `../scafld-go/internal/adapters/git` (all, shared) - Provide review mutation and baseline data through the Git port.
- `../scafld-go/internal/testkit/providerfake` (all, shared) - Provide canned stream, idle, endless stream, workspace mutation, invalid packet, and crash-mid-stream provider modes.

Acceptance:
- [x] `ac6_1` handoffs - Handoff Markdown and JSON match golden fixtures for executor, challenger, recovery, and repair flows.
  - Command: `cd ../scafld-go && go test ./internal/app/handoff ./internal/adapters/markdown -run Golden`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac6_2` review packet - Review packets validate, summarize findings, and drive repair handoffs.
  - Command: `cd ../scafld-go && go test ./internal/core/review ./internal/app/review -run 'Packet|Repair|Finding|Signal'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac6_3` isolation - Review provider execution records running, completed, timeout, failure, invalid-output, and mutation states.
  - Command: `cd ../scafld-go && go test ./internal/app/review ./internal/adapters/providers -run 'Provider|Timeout|Mutation|InvalidOutput'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac6_4` provider boundary - Core and app packages do not import provider implementation packages.
  - Command: `cd ../scafld-go && go test ./internal/arch -run ProviderBoundary`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac6_5` mutation guard - A real provider subprocess that writes to the workspace mid-review is detected, recorded, and blocked from producing a clean review verdict.
  - Command: `cd ../scafld-go && go test ./test/e2e -run ReviewProviderMutationGuard`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac6_6` provider fake - Provider fake covers stream replay, idle timeout, endless stream, workspace mutation, invalid packet, and crash-mid-stream cases.
  - Command: `cd ../scafld-go && go test ./internal/testkit/providerfake ./internal/app/review -run 'ProviderFake|IdleTimeout|EndlessStream|InvalidPacket|CrashMidStream|Mutation'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac6_7` provider contracts - Provider fake and real provider adapter share a contract test suite for streamed output and terminal states.
  - Command: `cd ../scafld-go && go test ./internal/testkit/contracts ./internal/adapters/providers -run ProviderContract`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 7: Complete the command surface and JSON contracts

Goal: Make the CLI adapter a thin composition layer over app use cases while preserving the user's and agent's product contracts.

Status: completed
Dependencies: phase2, phase3, phase4, phase5, phase6

Changes:
- `../scafld-go/internal/adapters/cli` (all, shared) - Wire all public commands, advanced help, shared flags, JSON mode, terminal mode, and exact exit behavior.
- `../scafld-go/internal/adapters/cli/contracts` (all, exclusive) - Add golden JSON contracts for success, failure, next-action, validation, review, and report outputs.
- `../scafld-go/internal/app/report` (all, exclusive) - Aggregate run statistics, review metrics, status counts, and historical summaries from ports.
- `../scafld-go/internal/app/projection` (all, exclusive) - Build user-facing status, report, and review projection models without mutating evidence.
- `../scafld-go/internal/adapters/terminal` (all, exclusive) - Render human terminal output from typed projection models.
- `../scafld-go/internal/app/bootstrap` (all, shared) - Initialize workspaces through workspace and bundle ports.
- `../scafld-go/internal/app/update` (all, shared) - Refresh managed core files through bundle and workspace ports.
- `../scafld-go/test/e2e/json_contracts_test.go` (all, shared) - Exercise JSON output for core commands through the built binary.
- `../scafld-go/test/e2e/agent_surface_test.go` (all, shared) - Exercise command output expected by agents through the built binary.

Acceptance:
- [x] `ac7_1` commands - Public command surface is wired and help is stable.
  - Command: `cd ../scafld-go && go run ./cmd/scafld --help && for c in init plan approve build exec review complete status list report handoff update; do go run ./cmd/scafld "$c" --help >/dev/null; done`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac7_2` json - JSON envelopes are golden-stable and include error codes and next actions.
  - Command: `cd ../scafld-go && go test ./internal/adapters/cli ./internal/app/contracts -run 'JSON|Envelope|NextAction|Golden'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac7_3` smoke - Agent-facing smoke tests pass against the built binary.
  - Command: `cd ../scafld-go && go test ./test/e2e -run 'JSONContracts|AgentSurface'`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac7_4` cli thinness - CLI command handlers compose dependencies and call app use cases; they do not contain product policy.
  - Command: `cd ../scafld-go && go test ./internal/arch -run CLIIsThin`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac7_5` exit codes - The published CLI exit code table is golden-tested for success, generic failure, invalid input, acceptance failure, review failure, cancellation, and workspace/config failures.
  - Command: `cd ../scafld-go && go test ./internal/adapters/cli ./test/e2e -run ExitCodeTable`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 8: Prepare packaging, CI, and release cutover

Goal: Make the Go implementation shippable without weakening the hard-cutover discipline.

Status: completed
Dependencies: phase7

Changes:
- `../scafld-go/.github/workflows/ci.yml` (all, shared) - Add format, vet, import-boundary, test, race, golden, smoke, and artifact build jobs.
- `../scafld-go/.github/workflows/release.yml` (all, shared) - Build signed native binaries and attach release artifacts from a tag.
- `../scafld-go/package/npm` (all, shared) - Draft npm wrapper package that installs or invokes the native binary.
- `../scafld-go/package/pypi` (all, shared) - Draft PyPI wrapper package that installs or invokes the native binary.
- `../scafld-go/docs/release.md` (all, shared) - Document release flow, binary matrix, wrapper packages, version synchronization, and rollback.
- `../scafld-go/docs/cutover.md` (all, shared) - Document the final replacement plan for the Python runtime and which old files disappear.
- `../scafld-go/docs/operational-contracts.md` (all, shared) - Document context propagation, signal handling, error wrapping, workspace discovery, concurrency scope, and the CLI exit code table.

Acceptance:
- [x] `ac8_1` ci - Local CI script runs the same checks as GitHub Actions.
  - Command: `cd ../scafld-go && go test ./test/ci -run LocalCI`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac8_2` binaries - Native binary builds for supported release targets.
  - Command: `cd ../scafld-go && go test ./test/release -run BuildMatrix`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac8_3` cutover - Cutover documentation names every Python runtime, packaging, and test artifact to delete or replace.
  - Command: `grep -q 'Python runtime removal checklist' ../scafld-go/docs/cutover.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac8_4` architecture docs - Release documentation names the hexagonal dependency rule as a release gate.
  - Command: `grep -q 'Import-boundary tests are release-blocking' ../scafld-go/docs/release.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac8_5` operational docs - Operational contracts document context propagation, errors, signals, workspace discovery, concurrency, and exit codes.
  - Command: `grep -q 'CLI exit code table' ../scafld-go/docs/operational-contracts.md && grep -q 'Every IO port accepts context.Context' ../scafld-go/docs/operational-contracts.md && grep -q 'SIGINT' ../scafld-go/docs/operational-contracts.md`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac8_6` ci matrix - GitHub CI runs at minimum on Linux and macOS.
  - Command: `grep -q 'ubuntu-' ../scafld-go/.github/workflows/ci.yml && grep -q 'macos-' ../scafld-go/.github/workflows/ci.yml`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Phase 9: Prove parity on real workflows

Goal: Exercise the Go implementation against real scafld workflows before any release cutover.

Status: completed
Dependencies: phase8

Changes:
- `../scafld-go/testdata/parity` (all, shared) - Add representative workspaces for happy path, blocked criterion, review repair, recovery, invalid spec, prompt precedence, and report aggregation.
- `../scafld-go/test/parity` (all, shared) - Add Go parity tests that compare binary behavior to product-level expected outputs, not Python internals.
- `../scafld-go/docs/parity-report.md` (all, shared) - Track parity evidence, intentional differences, and remaining release blockers.
- `.scafld/specs/drafts/go-migration.md` (all, shared) - Keep this living spec updated with phase completion evidence.

Acceptance:
- [x] `ac9_1` parity - Product-level parity workflows pass.
  - Command: `cd ../scafld-go && go test ./test/parity`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac9_2` real spec - The Go binary can read and validate this migration spec from the scafld workspace.
  - Command: `cd ../scafld-go && ./bin/scafld --root ../scafld validate go-migration --json`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none
- [x] `ac9_3` no Python dependency - Normal Go CLI smoke tests pass with Python unavailable in PATH.
  - Command: `cd ../scafld-go && SCAFLD_E2E_BINARY=./bin/scafld SCAFLD_FORBID_PYTHON=1 go test ./test/e2e -run Lifecycle`
  - Expected kind: `exit_code_zero`
  - Timeout seconds: none
  - Result: none
  - Status: pass
  - Evidence: none
  - Source event: none
  - Last attempt: none
  - Checked at: none

## Rollback

Strategy: isolate

Commands:
- `rm -rf ../scafld-go`
- `git restore .scafld/specs/drafts/go-migration.md`

## Review

Status: not_started
Verdict: none
Timestamp: none
Review rounds: none
Reviewer mode: none
Reviewer session: none
Round status: none
Override applied: none
Override reason: none
Override confirmed at: none
Reviewed head: none
Reviewed dirty: none
Reviewed diff: none
Blocking count: none
Non-blocking count: none

Findings:
- none

Passes:
- none

## Self Eval

Status: not_started
Completeness: none
Architecture fidelity: none
Spec alignment: none
Validation depth: none
Total: none
Second pass performed: none

Notes:
The spec intentionally targets a sibling folder so the Go implementation can start clean while the current scafld workspace remains the planning authority.

Improvements:
- Harden should challenge whether `../scafld-go/` remains the right folder after repository and release strategy are decided.
- Harden should challenge whether every proposed port is caller-owned and narrow enough to justify existing.
- Harden should challenge the selected Go toolchain version at implementation time instead of preserving the draft-time patch version.
- Harden should challenge signal handling against interrupted subprocess start and repeated-interrupt escalation.
- Harden should challenge whether provider fakes model the real provider failure modes that have historically escaped unit tests.
- Harden should challenge whether CI is catching macOS process/signal behavior before release.
- Harden should pressure-test whether any external Go dependency is truly necessary.
- Harden should add more exact parity fixtures if current product behavior has unspoken CLI contracts.

## Deviations

- none

## Metadata

Estimated effort hours: 240
Actual effort hours: none
AI model: none
React cycles: none

Tags:
- go
- migration
- hard-cutover
- cli
- architecture

## Origin

Source:
- user requested a plan spec for a Go migration using modern Go best practices and the latest stable version
- official Go downloads page lists go1.26.2 as the current stable featured release on 2026-05-01
- official go.mod reference documents the `go` directive and `toolchain` directive behavior

Repo:
- scafld planning workspace: `/Users/kam/dev/0state/scafld`
- target implementation folder: `/Users/kam/dev/0state/scafld-go`

Git:
- none

Sync:
- none

Supersession:
- none

## Harden Rounds

- none

## Planning Log

- 2026-05-01: Created draft plan targeting a clean sibling Go codebase while keeping the migration spec in the scafld workspace.
- 2026-05-01: Revised the plan around explicit hexagonal architecture: pure core, app use cases, caller-owned ports, edge adapters, platform primitives, and import-boundary tests.
- 2026-05-01: Incorporated review fixes for module identity, toolchain policy, legacy-path regex, use-case-owned ports, context propagation, error semantics, signal handling, workspace discovery, exit codes, bundle source, mutation guard, cross-platform e2e tests, and realistic effort sizing.
- 2026-05-01: Added compact test-architecture gates for provider fake modes, fake-vs-real contract tests, concrete race scenarios, golden update discipline, and Linux/macOS CI.
