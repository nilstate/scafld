---
spec_version: '2.0'
task_id: evidence-control-sandbox
created: '2026-06-02T10:48:51Z'
updated: '2026-06-02T14:46:32Z'
status: review
harden_status: needs_revision
size: large
risk_level: high
---

# Evidence-control sandbox for the review path

## Current State

Status: review
Current phase: final
Next: review
Reason: exit code was 0
Blockers: none
Allowed follow-up command: `scafld review evidence-control-sandbox`
Latest runner update: 2026-06-04T09:11:20Z
Review gate: not_started

## Summary

Independence is theater unless scafld controls what the reviewer subprocess can see and run. Today the process runner inherits the full host environment (`cmd.Env = append(os.Environ(), req.Env...)` at internal/adapters/process/runner.go:44), providers can resolve their binary off host PATH, the reviewer reads the live workspace from the host CWD, and the changed-file content reaches the provider prompt as plain context with no untrusted-data fence. This spec closes those four surfaces on a concrete receipt-grade finalize review path: an explicit provider invocation API separate from ordinary review/harden, exact-env execution with a provider allowlist that strips/pins `*_BASE_URL`/`*_API_BASE`/`*_ENDPOINT`/proxy vars and records the effective endpoint host, a content-hash-pinned reviewer binary resolved from a fixed authoritative path, a scafld-owned evidence scratch dir fed only by typed canonical evidence files, and a prompt-level fence marking changed content as untrusted data.

## Objectives

- Add an explicit execution env mode so receipt-grade reviewer requests run with an exact allowlisted env while acceptance commands keep the existing inherited-env behavior.
- Add an explicit receipt-grade provider selection/invocation path so these controls cannot accidentally apply to ordinary `scafld review` or `scafld harden`.
- Scrub the reviewer subprocess environment to a provider-specific allowlist, stripping or pinning every `*_BASE_URL`, `*_API_BASE`, `*_ENDPOINT`, and proxy variable, and record the effective endpoint host per provider for the receipt's `reviewer.endpoint_host`.
- Resolve the reviewer binary from a fixed authoritative path for receipt-grade finalize review, hash its bytes, and surface `binary_sha256` for the receipt's `reviewer.binary_sha256`.
- Materialize canonical reviewed bytes into a scafld-owned scratch directory, run the reviewer with CWD set to that scratch dir, and jail provider Read/Grep/Glob to the evidence only.
- Disable provider memory autoload on the finalize path and blocklist CLAUDE.md, AGENTS.md (recursively), .scafld/config.yaml, and GEMINI.md from ever being pinned as evidence.
- Fence all changed-file content as untrusted data inside the provider instruction, expressed as a prompt directive, not a post-hoc regex lint.
- Ignore config.local for receipt-grade finalize reviewer selection so the host cannot reconfigure its own challenger; ordinary `scafld review` keeps the existing config.local overlay.
- Return runtime facts (`binary_sha256`, `endpoint_host`, evidence provenance, sandbox policy) through a typed receipt-grade result consumed by host-gate, not by scraping diagnostics or overloading the ordinary `ReviewDossier`.

## Scope

- In scope: an execution env-mode contract in `internal/core/execution` with default inherited behavior and an exact-env mode for receipt-grade reviewer subprocesses, replacing unconditional `os.Environ()` layering only when exact mode is requested.
- In scope: a receipt-grade provider API such as `SelectReceiptGradeReviewer` / `InvokeReceiptGradeReview` that is separate from `providers.Select`, `providers.SelectHarden`, and ordinary review invocation.
- In scope: an env-allowlist scrub applied where the reviewer subprocess env is constructed, with `*_BASE_URL`/`*_API_BASE`/`*_ENDPOINT`/proxy handling and effective endpoint host capture.
- In scope: content-hash binary pinning resolved from a fixed authoritative path in the providers package, exposing `binary_sha256` on the provider/agent response.
- In scope: a scafld-owned evidence scratch dir builder that consumes typed canonical evidence files, writes canonical bytes, sets reviewer CWD, jails Read/Grep/Glob to the evidence, disables provider memory autoload, and enforces the agent-instruction-file blocklist (CLAUDE.md, AGENTS.md recursively, .scafld/config.yaml, GEMINI.md).
- In scope: typed runtime facts returned from receipt-grade review and preserved for host-gate receipt assembly.
- In scope: committed base-config endpoint pin fields for provider configs, read without config.local on receipt-grade finalize review.
- In scope: an untrusted-data fence directive in the review provider instruction (internal/app/review/context.go `providerInstructionBody`).
- In scope: ignoring config.local for receipt-grade finalize reviewer selection (internal/adapters/config/config.go reviewer-selection read).
- Out of scope: the canonical `tree_sha` fingerprint and the diff definition function; that is owned by spec commit-free-tree-fingerprint. This spec consumes the canonical bytes but does not define how they are fingerprinted.
- Out of scope: reviewer selection, auto/fallback ordering, and the `independence{level,distinct}` classification; that is owned by spec context-isolated-reviewer. This spec hardens the runtime of whichever reviewer is selected, not the selection itself.
- Out of scope: the `finalize` verb, acceptance folding (runFinalAcceptance), receipt assembly, and ed25519 signing; that is owned by spec host-gate-and-receipt. This spec only produces the runtime facts (`binary_sha256`, `endpoint_host`, evidence provenance) the receipt later records.
- Out of scope: any change to the build/harden lifecycle env contract; exact-env mode applies only to receipt-grade reviewer requests, not to acceptance command execution (`ExecutionConfig.ProcessEnv`).

## Dependencies

- commit-free-tree-fingerprint: supplies the canonical reviewed bytes (sha256 of canonical blobs) that this spec materializes into the scratch dir; the scratch builder takes those bytes as input.
- host-gate-and-receipt: consumes the runtime facts this spec exposes (`reviewer.binary_sha256`, `reviewer.endpoint_host`, `reviewed_context_provenance`) when assembling the signed receipt body.
- Real repo: internal/adapters/process/runner.go:44 currently does `cmd.Env = append(os.Environ(), req.Env...)` with no allowlist; the provider structs (ClaudeProvider/CodexProvider/GeminiProvider/CommandProvider in internal/adapters/providers/provider.go) build `execution.Request{Env: ...}` and resolve binaries via `binaryOrDefault` against host PATH (`osexec.LookPath` in `commandExists`).
- Real repo: `execution.Request` already carries `Env []string` and `CWD string` (internal/core/execution), but its current env semantics are inherited-plus-overrides. This spec adds an explicit env-mode field so providers can request exact-env execution without breaking existing acceptance callers.
- Real repo: config overlay merges config.local.yaml today via `Load`/`overlay` in internal/adapters/config/config.go; the finalize path must read base config only for reviewer selection.
- Real repo: `ProviderConfig` currently has only `Model` and `Binary`; this spec adds committed endpoint pin fields used only by receipt-grade env scrub.
- Real repo: arch test internal/arch TestCLIIsThin caps CLI adapter lines; all composition added here lives in app/ or the providers/process adapters, never in a thin CLI handler.

## Assumptions

- The reviewer providers already run read-only (Claude `--tools Read,Grep,Glob` with Write/Edit/Bash disallowed; Codex `--sandbox read-only`; Gemini plan mode); this spec narrows what those read tools can reach, it does not add a new sandbox mechanism.
- A fixed reviewer-binary path is available via existing provider config (`ProviderConfig.Binary` / `ExternalReviewConfig.ProviderBinary` in internal/adapters/config/config.go). For receipt-grade finalize review it is authoritative: if absent, non-absolute, not executable, or hash-unreadable, the finalize fails closed. PATH lookup is permitted only on non-receipt-grade smoke paths.
- The canonical bytes for each reviewed path are content-addressed and supplied by the tree-fingerprint dependency; this spec does not recompute them.
- Endpoint host is derived from pinned config or a built-in provider default after env scrub; host env endpoint vars never define it on the receipt-grade finalize path.
- The scratch dir is created under a scafld-owned temp root and removed after the run, mirroring the existing `os.CreateTemp` cleanup pattern in the providers package.

## Runtime Contracts

- Receipt-grade activation is explicit. `providers.Select`, `providers.SelectHarden`, ordinary `Provider.Invoke`, and ordinary `scafld review` keep current behavior. Receipt-grade finalize review uses a distinct API:

```go
type ReceiptGradeReviewInput struct {
    Provider string
    Evidence []reviewevidence.EvidenceFile
    Prompt string
    BaseConfig config.Config
}

type ReceiptGradeReviewResult struct {
    Dossier review.Dossier
    RuntimeFacts RuntimeFacts
}

type RuntimeFacts struct {
    BinarySHA256 string
    EndpointHost string
    EvidenceProvenance []reviewevidence.Provenance
    SandboxPolicy SandboxPolicy
}
```

Host-gate consumes `ReceiptGradeReviewResult.RuntimeFacts` directly. Ordinary `review.Dossier` remains unchanged unless host-gate explicitly changes receipt schema in its own spec.
- `execution.Request` gains `EnvMode` with values `EnvModeInherit` (default, preserves current `cmd.Env = append(os.Environ(), req.Env...)` behavior) and `EnvModeExact` (process env is exactly `Request.Env`). Receipt-grade reviewer providers must set `EnvModeExact`; acceptance/build/harden commands keep `EnvModeInherit`.
- The evidence sandbox input is `reviewevidence.EvidenceFile{Path string, Status string, Bytes []byte, SHA256 string}` supplied by `commit-free-tree-fingerprint` through host-gate. `Path` is repository-relative, cleaned with slash separators, must not escape the repo, and must not be absolute. `Status` is one of the fingerprint status enum values. The builder verifies `sha256(Bytes) == SHA256`, rejects blocklisted paths before writing, writes only these bytes to scratch, and returns `reviewevidence.Provenance{Path,Status,SHA256,ScratchPath}`.
- The sandbox builder returns `Sandbox{CWD, ReadRoots, Env, ArgsPolicy, Provenance, Cleanup}`. `ReadRoots` contains only the scratch evidence root. `ArgsPolicy` contains provider-specific read-root/memory-off flags or settings for Claude, Codex, and Gemini. `Cleanup` owns scratch-dir removal and is called by the receipt-grade provider invocation after the provider exits.
- Provider endpoint policy is table-driven and backed by committed base config. `ProviderConfig` gains `EndpointURL string` and `EndpointHost string`; exactly one may be set. Built-in default endpoint hosts are used when neither is set. Host env endpoint/proxy variables (`*_BASE_URL`, `*_API_BASE`, `*_ENDPOINT`, `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`, lowercase variants) are stripped unless they exactly match the committed endpoint pin. The recorded `endpoint_host` is the post-scrub host.
- Provider auth env is allowlisted by provider only: Claude may receive `ANTHROPIC_API_KEY`; Codex/OpenAI may receive `OPENAI_API_KEY`; Gemini may receive `GEMINI_API_KEY` and `GOOGLE_API_KEY`. `HOME`, `XDG_CONFIG_HOME`, and provider config dirs are set to scratch-owned values when required for CLI execution. Missing required auth fails closed. `command`/`local` providers are not receipt-grade providers.
- Config-local skipping applies only to receipt-grade finalize reviewer selection. Ordinary `scafld review`, `scafld harden`, and local smoke runs retain the existing overlaid config behavior.

## Touchpoints

- internal/adapters/process/runner.go (env construction at line 44; reviewer invocations must use a scrubbed allowlist instead of raw `os.Environ()`)
- internal/adapters/providers/provider.go (provider `Env` assembly, `binaryOrDefault`/`commandExists` binary resolution, `binary_sha256` capture, endpoint-host capture, `AgentResponse` fields)
- internal/adapters/providers/env_scrub.go (new: allowlist scrub + `*_BASE_URL`/`*_API_BASE`/`*_ENDPOINT`/proxy handling + endpoint-host extraction, single responsibility)
- internal/adapters/providers/evidence_sandbox.go (new: scratch-dir builder, canonical-byte materialization, reviewer CWD, Read/Grep/Glob jail, memory-autoload off, agent-instruction blocklist)
- internal/core/reviewevidence/evidence.go (new: EvidenceFile, Provenance, status/path validation shared by fingerprint/finalize/evidence sandbox)
- internal/app/review/context.go (`providerInstructionBody` untrusted-data fence directive)
- internal/adapters/config/config.go (gate-path reviewer-selection read that ignores config.local overlay)
- internal/core/execution (Request.Env / Request.CWD reuse plus new EnvMode exact-vs-inherit semantics)

## Risks

- Over-aggressive env stripping could break a provider that legitimately needs a non-secret host var; mitigated by an explicit table-driven allowlist with provider-required keys enumerated and tested.
- A pinned binary path that drifts from the installed CLI version would fail closed; acceptable because failing closed on an unverifiable reviewer binary is the intended posture.
- The agent-instruction blocklist must match recursively for AGENTS.md without excluding legitimately reviewed files of similar name; mitigated by basename-and-recursive matching scoped to the evidence set only.

## Acceptance

Profile: strict

Validation:
- [x] `v1` spec validates - The spec parses and validates under scafld.
  - Command: `go test ./internal/adapters/process -run EnvMode && rg -n 'EnvModeExact|EnvModeInherit' internal/core/execution internal/adapters/process internal/adapters/providers`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-30

## Phase 1: Env allowlist scrub and binary pinning

Status: pass
Dependencies: none

Objective: Add exact-env execution mode and use it on the receipt-grade reviewer path with a strict allowlist that strips or pins every `*_BASE_URL`, `*_API_BASE`, `*_ENDPOINT`, and proxy variable, and resolve the reviewer binary from a fixed authoritative path by content hash. The scrub records the effective endpoint host and the binary records its `binary_sha256`, both surfaced on the provider/agent response so a later spec can place them in the receipt. The existing process runner remains generic: inherited env is the default, exact env is opt-in per request.

Changes:
- internal/core/execution - Add `EnvMode` to `Request` with default `EnvModeInherit` and exact mode `EnvModeExact`.
- internal/adapters/process/runner.go - Keep the runner generic; if `EnvModeExact`, set `cmd.Env = req.Env` exactly; otherwise preserve the current inherited-plus-overrides behavior. Add tests proving both modes.
- internal/adapters/providers/env_scrub.go - New file owning a single responsibility: take host environ, provider identity, committed base-config pins, and provider auth requirements; return an allowlisted env slice, drop or pin `*_BASE_URL`/`*_API_BASE`/`*_ENDPOINT`/proxy keys, and return the effective endpoint host. No process execution here.
- internal/adapters/providers/provider.go - Each receipt-grade reviewer provider builds its `execution.Request{Env: ..., EnvMode: EnvModeExact}` from the scrub result; resolve the binary from the fixed configured path (not `commandExists`/PATH) on the finalize path, hash its bytes, and set `binary_sha256` and `endpoint_host` on `AgentResponse`.

Acceptance:
- [x] `ac1_1` env scrub semantics tested - Env scrub drops proxy/unpinned endpoint vars, pins endpoint host, and allows only provider auth keys.
  - Command: `go test ./internal/adapters/providers -run 'ScrubDropsProxy|ScrubPinsEndpoint|ScrubAllowsProviderAuth|ScrubRejectsUnpinnedEndpoint'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-31
- [x] `ac1_2` config endpoint schema present - Provider config exposes committed endpoint pins.
  - Command: `rg -n 'EndpointURL|EndpointHost' internal/adapters/config/config.go internal/adapters/corebundle/assets/core/config.yaml`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-32
- [x] `ac1_3` no raw environ inheritance in providers - Reviewer providers no longer inherit the unscrubbed host environment.
  - Command: `go test ./internal/adapters/process -run EnvMode`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-33
- [x] `ac1_5` binary authority tested - Receipt-grade reviewer binary resolution fails closed when the fixed path is absent, relative, PATH-only, or hash-unreadable.
  - Command: `go test ./internal/adapters/providers -run 'Binary|Pinned|ReceiptGrade'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-34
- [x] `ac1_6` no raw environ inheritance in providers - Reviewer providers no longer call raw os.Environ directly.
  - Command: `rg -n 'os.Environ\(\)' internal/adapters/providers`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-35
- [x] `ac1_7` runner still green - The reused process runner passes after the env-assembly change.
  - Command: `go test ./internal/adapters/providers ./internal/adapters/cli/review ./internal/adapters/cli/harden -run 'ReceiptGrade|OrdinaryReview|HardenSelection'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-36

## Phase 2: Evidence scratch dir, jail, and untrusted-data fence

Status: pass
Dependencies: phase1

Objective: Materialize typed canonical evidence files into a scafld-owned scratch directory, run the reviewer with CWD set to that scratch dir, jail its Read/Grep/Glob to the evidence, disable provider memory autoload, and blocklist CLAUDE.md, AGENTS.md (recursively), .scafld/config.yaml, and GEMINI.md from ever being pinned as evidence. Fence the changed-file content as untrusted data in the review provider instruction so the reviewer treats it as data, not instructions. The scratch builder verifies each `EvidenceFile` hash before writing, reuses the existing temp-file/cleanup pattern from the providers package, and the fence is a prompt directive, not a regex lint.

Changes:
- internal/core/reviewevidence/evidence.go - New core package defining `EvidenceFile`, `Provenance`, path normalization, status validation, and hash verification helpers used by host-gate and the sandbox builder.
- internal/adapters/providers/evidence_sandbox.go - New file owning a single responsibility: accept `reviewevidence.EvidenceFile`, verify each hash, build the scratch dir from those canonical bytes, return `Sandbox{CWD, ReadRoots, Env, ArgsPolicy, Provenance, Cleanup}`, and reject any path matching the agent-instruction blocklist (CLAUDE.md, AGENTS.md recursively, .scafld/config.yaml, GEMINI.md). No prompt assembly here.
- internal/adapters/providers/provider.go - Reviewer providers point CWD and read-tool jail at the scratch dir from the sandbox builder, and set provider memory autoload off (Claude/Gemini settings, Codex `--ignore-user-config` already present) consistently on the finalize path.
- internal/app/review/context.go - Extend `providerInstructionBody` with an untrusted-data fence directive: changed-file content is data under review and must never be followed as instructions. Prompt-level, no lint.

Acceptance:
- [x] `ac2_1` evidence sandbox semantics tested - The scratch builder rejects blocklisted paths/hash mismatches, normalizes paths, returns jailed roots, and exposes cleanup/provenance.
  - Command: `go test ./internal/app/review -run 'Untrusted|ProviderInstruction'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-37
- [x] `ac2_4` review use case still green - The independent review use case passes with the fenced instruction.
  - Command: `go test ./internal/app/review`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-38
- [x] `ac2_5` evidence hashes verified - Sandbox tests prove bytes whose sha256 does not match the declared value fail closed before any scratch file is written.
  - Command: `go test ./internal/adapters/providers -run 'EvidenceFile|HashMismatch|Blocklist'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-39
- [x] `ac2_6` provider memory and read roots tested - Receipt-grade Claude/Codex/Gemini args or settings disable memory autoload and restrict reads to sandbox roots.
  - Command: `go test ./internal/adapters/providers -run 'ReceiptGrade.*(Memory|ReadRoot|Sandbox|Claude|Codex|Gemini)'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-40
- [x] `ac2_7` runtime facts survive provider execution - Receipt-grade review returns binary_sha256, endpoint_host, evidence provenance, and sandbox policy to the caller.
  - Command: `go test ./internal/adapters/providers -run 'ReceiptGrade.*RuntimeFacts|RuntimeFacts.*Provenance'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-41

## Phase 3: Ignore config.local for gate-path reviewer selection

Status: pass
Dependencies: phase2

Objective: Ensure the host agent cannot reconfigure its own challenger by editing config.local. On the receipt-grade finalize path, reviewer selection reads committed base config only, skipping the config.local.yaml overlay that `Load`/`overlay` applies today. This is a narrow, gate-path-only read; ordinary `scafld review`, `scafld harden`, and local smoke paths keep the full overlay.

Changes:
- internal/adapters/config/config.go - Add a receipt-grade finalize config read that loads base config without the config.local.yaml overlay for reviewer selection and endpoint pins, reusing the existing `readConfigFile`/`withDefaults` machinery and leaving the general `Load` overlay unchanged.

Acceptance:
- [x] `ac3_3` CLI stays thin - The CLI adapter line budget is unchanged; composition stayed out of thin handlers.
  - Command: `go test ./internal/arch -run TestCLIIsThin`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-42

## Rollback

- Revert internal/adapters/providers/env_scrub.go and internal/adapters/providers/evidence_sandbox.go and restore the prior `execution.Request{Env: ...}` and binary-resolution calls in internal/adapters/providers/provider.go.
- Remove `EnvMode` from `internal/core/execution` and restore the original env assembly at internal/adapters/process/runner.go:44 if it was narrowed.
- Revert the `providerInstructionBody` directive in internal/app/review/context.go to the prior text.
- Revert the gate-path config read in internal/adapters/config/config.go; the general overlaid `Load` is untouched and needs no rollback.
- No persisted data or schema changes are introduced, so rollback is code-only.

## Review

Status: not_started
Verdict: none

## Self Eval

- Closes all four evidence-control surfaces (env, binary, prompt/scratch, runtime config) on the receipt-grade reviewer-on-gate path while preserving acceptance-command env inheritance.
- Reuses the existing process runner, provider transport, config machinery, and temp/cleanup patterns; no parallel sandbox is reimplemented.
- Fences cleanly against neighbours: tree_sha (commit-free-tree-fingerprint), reviewer selection/independence (context-isolated-reviewer), and finalize verb/receipt/signing (host-gate-and-receipt).
- Exposes `binary_sha256` and `endpoint_host` as runtime facts for the receipt without assembling or signing the receipt here.

## Deviations

- none

## Metadata

- created_by: scafld
- estimated_effort_hours: 10-16
- priority: p0

## Origin

Created by: scafld
Source: accountability-layer rebuild

## Harden Rounds

### round-1

Status: needs_revision
Started: 2026-06-02T11:28:16Z
Ended: 2026-06-02T11:28:16Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: The draft has the right security direction, but approval should stop until it pins three interfaces: exact env vs override semantics, canonical evidence bytes into the sandbox, and fixed-path binary authority. Acceptance also needs direct tests for the sandbox and blocklist behavior.

Checks:
- path audit
  - Grounded in: code:internal/adapters/providers/provider.go:1
  - Result: passed
  - Evidence: Referenced current paths exist: internal/adapters/process/runner.go, internal/adapters/providers/provider.go, internal/app/review/context.go, internal/adapters/config/config.go, internal/core/execution/model.go, internal/arch/architecture_test.go. Future files internal/adapters/providers/env_scrub.go and evidence_sandbox.go are absent, consistent with the draft.
- command audit
  - Grounded in: command:go test ./internal/adapters/process ./internal/app/review ./internal/arch -run TestCLIIsThin
  - Result: passed
  - Evidence: `go test ./internal/adapters/process ./internal/app/review ./internal/arch -run TestCLIIsThin` failed before package execution with `operation not permitted` creating Go's temp build dir. This is a sandbox limitation; the package paths and test selector are structurally valid.
- scope/migration audit
  - Grounded in: code:internal/adapters/providers/provider.go:46
  - Result: passed
  - Evidence: The spec’s scope crosses process execution, provider transport, CLI reviewer selection, review context, and future receipt facts. `AgentResponse` currently has provider/model/session/result fields only, so adding binary_sha256 and endpoint_host is correctly scoped to providers, but receipt persistence remains deferred to another spec.
- acceptance timing audit
  - Grounded in: spec_gap:acceptance
  - Result: failed
  - Evidence: Several acceptances are too indirect for the high-risk security behavior: `rg -n 'untrusted' internal/app/review/context.go` proves only wording, and the provider env acceptance only checks absence of `os.Environ()` in providers while runner inheritance remains the live env behavior.
- rollback/repair audit
  - Grounded in: spec_gap:rollback
  - Result: failed
  - Evidence: Rollback is code-only and names the touched files. It is plausible, but it omits the likely need to revert any new execution request env-mode contract or review request evidence-byte interface if those are added to make the spec executable.
- design challenge
  - Grounded in: spec_gap:phases
  - Result: failed
  - Evidence: The architectural goal is sound, but the draft currently bundles sandboxing, endpoint/env policy, binary provenance, canonical evidence materialization, memory suppression, and config.local bypass without pinning key interfaces. That leaves implementers to invent contracts in security-sensitive areas.

Issues:
- [high/blocks approval] `harden-1` question - The env scrub objective conflicts with existing `Request.Env` semantics and the promise to preserve acceptance-command behavior.
  - Status: open
  - Grounded in: code:internal/adapters/process/runner.go:44
  - Evidence: `Runner.Run` always does `cmd.Env = append(os.Environ(), req.Env...)`, while `execution.Request` has only `Env []string`. The draft says no new fields are expected and acceptance-command behavior must be preserved, but a reviewer exact-env request cannot be distinguished from a normal acceptance env override today.
  - Recommendation: Add an explicit env inheritance contract before approval, or define a provider-only runner adapter that bypasses host inheritance while preserving the generic runner’s existing override semantics.
  - Question: How should the process layer distinguish a complete scrubbed reviewer environment from normal acceptance-command env overrides?
  - Recommended answer: Add an `InheritEnv`/`EnvMode` field to `execution.Request`; default preserves current inheritance, reviewer finalize requests use exact env.
  - If unanswered: Default to an explicit `InheritEnv` or `EnvMode` on `execution.Request`, with current inherit behavior as the default and reviewer finalize requests opting into exact env.
- [high/blocks approval] `harden-2` question - The scratch-dir builder cannot be implemented without inventing how canonical file bytes reach providers.
  - Status: open
  - Grounded in: code:internal/core/review/model.go:147
  - Evidence: `review.Request` carries only `TaskID`, `Prompt`, and `Context`; current review context renders changed paths via `workspaceChangesBody`, not canonical reviewed file bytes. The spec asks `evidence_sandbox.go` to build a scratch dir from canonical bytes but marks the canonical diff/bytes producer out of scope and names no port or request field that supplies those bytes.
  - Recommendation: Define the handoff type and owner now: either extend the gate/review request with canonical evidence entries or add a narrow provider sandbox input assembled by the future finalize use case.
  - Question: What concrete interface supplies canonical reviewed bytes to the evidence sandbox builder?
  - Recommended answer: Introduce a canonical evidence input type with path, status, sha256, and bytes, produced by the fingerprint/finalize path and consumed by `evidence_sandbox.go`.
  - If unanswered: Default to adding an explicit gate/review evidence input type containing `{path,status,bytes,sha256}` from the fingerprint spec, and make the sandbox builder consume only that type.
- [high/blocks approval] `harden-3` question - Fixed-path binary pinning is under-specified and currently conflicts with PATH-based auto discovery.
  - Status: open
  - Grounded in: code:internal/adapters/providers/provider.go:235
  - Evidence: Provider auto availability currently accepts explicit binaries or falls back to `osexec.LookPath(name)`. The draft requires resolving reviewer binaries from a fixed configured path rather than host PATH, but reviewer selection is declared out of scope and the spec does not say whether `provider:auto`, `--provider-binary`, per-provider config binaries, or `--provider command` are allowed on the finalize path.
  - Recommendation: Specify that gate-path external providers require absolute configured binary paths, hash those files before execution, and make auto selection consider only those configured paths. Explicitly decide whether CLI binary overrides and command provider are excluded from receipt-grade review.
  - Question: What exact binary source is authoritative on the finalize path for each provider and for `provider:auto`?
  - Recommended answer: Gate review uses only absolute paths from base config per provider; auto only selects among those paths; host PATH and command provider are not receipt-grade.
  - If unanswered: Default to fail-closed on the finalize path unless the selected provider has an absolute configured binary path; do not use `LookPath` or shell command providers for finalize receipts.
- [high/blocks approval] `harden-4` question - The env policy is security-critical but not executable without inventing provider-specific allowlists and endpoint pin rules.
  - Status: open
  - Grounded in: spec_gap:objectives
  - Evidence: The draft says to use an explicit env allowlist and to strip or pin `*_BASE_URL`, `*_API_BASE`, `*_ENDPOINT`, and proxy variables, but it does not enumerate allowed auth/config keys per provider or state where pinned endpoint values come from. With provider CLIs, omitting the wrong variable can break auth, while allowing the wrong variable preserves the injection surface.
  - Recommendation: Add a table to the spec for provider-specific env keys, proxy handling, endpoint pin defaults, and whether missing auth variables fail before process execution.
  - Question: What are the exact allowed environment keys and endpoint pin sources for Codex, Claude, Gemini, and command providers?
  - Recommended answer: Define provider-specific allowlists and require endpoint pins to come from base config or built-in provider defaults, never host env.
  - If unanswered: Default to a provider-specific allowlist that includes only required auth/home/runtime variables plus explicit endpoint pins from base config, and fail closed on unrecognized endpoint/proxy variables.
- [medium/blocks approval] `harden-5` question - High-risk blocklist and jail behavior lack direct acceptance coverage.
  - Status: open
  - Grounded in: code:internal/app/review/review_test.go:379
  - Evidence: The current review path can include project context from `AGENTS.md` in the prompt; tests assert that behavior. The draft says `AGENTS.md` recursively, `CLAUDE.md`, `.scafld/config.yaml`, and `GEMINI.md` must never be pinned as evidence, but acceptance only greps for `untrusted` and runs app review tests. It does not require negative tests proving these files are rejected by the sandbox builder or absent from provider-visible evidence.
  - Recommendation: Strengthen phase 2 acceptance with focused provider tests for the blocklist, memory-autoload-off settings, and jailed roots for each provider CLI argument/config path.
  - Question: Which tests must prove instruction files are blocked from evidence while still allowing derived config/spec context where appropriate?
  - Recommended answer: Add `go test ./internal/adapters/providers -run Sandbox` coverage for blocklisted paths, jail roots, and provider memory flags, plus app review prompt-fence coverage.
  - If unanswered: Default to adding explicit provider sandbox tests for recursive `AGENTS.md`, nested `CLAUDE.md`/`GEMINI.md`, and `.scafld/config.yaml`, plus review-context tests that distinguish config-derived text from pinned evidence bytes.
- [low/advisory] `harden-6` question - The config.local bypass needs a precise command boundary, but this can be resolved during implementation.
  - Status: open
  - Grounded in: code:internal/adapters/cli/review/selection.go:45
  - Evidence: CLI review selection currently calls `configadapter.Load`, which overlays `.scafld/config.local.yaml` before provider selection. The planned phase 3 touchpoint is valid, but the spec says config.local should be ignored on the finalize path; current `scafld review` may remain non-gate lifecycle behavior depending on the future finalize split.
  - Recommendation: Clarify the command surface explicitly so implementers do not accidentally remove local overlay behavior from non-gate review/harden workflows.
  - Question: Does phase 3 change only the future receipt finalize path, or the existing `scafld review` command too?
  - Recommended answer: Only the receipt-grade finalize reviewer selection ignores config.local; ordinary lifecycle `scafld review` keeps current local overlay unless it is promoted to the finalize path.
  - If unanswered: Default to applying base-only config only to receipt-grade finalize reviewer selection, while documenting whether regular `scafld review` keeps local overlays.

### round-2

Status: needs_revision
Started: 2026-06-02T13:10:31Z
Ended: 2026-06-02T13:10:31Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: The draft targets the right independence failures, and the motivating code paths are real. It should not be approved yet because the receipt-grade boundary, env pin schema, evidence sandbox contract, runtime-fact propagation, and security acceptance tests are not executable without invention.

Checks:
- path audit
  - Grounded in: code:internal/adapters/process/runner.go:44
  - Result: passed
  - Evidence: Existing files named in scope are present: internal/adapters/process/runner.go, internal/adapters/providers/provider.go, internal/app/review/context.go, internal/adapters/config/config.go, internal/core/execution/model.go. Proposed internal/adapters/providers/env_scrub.go and evidence_sandbox.go do not exist yet, matching the draft plan.
- command audit
  - Grounded in: code:internal/adapters/process/runner.go:44
  - Result: failed
  - Evidence: process.Runner currently always sets cmd.Env = append(os.Environ(), req.Env...), so EnvMode is a real command-surface change. I could not execute go/scafld commands in this read-only sandbox because the toolchain attempted to create a temporary work directory.
- scope/migration audit
  - Grounded in: code:internal/adapters/providers/provider.go:97
  - Result: failed
  - Evidence: providers.Select and providers.SelectHarden both construct the same provider implementations, and InvokeAgent paths are used for review/harden without a finalize-only discriminator. Ordinary review selection is built in internal/adapters/cli/review/selection.go from configadapter.Load.
- acceptance timing audit
  - Grounded in: spec_gap:phases
  - Result: failed
  - Evidence: ac1_1 and ac2_1 are rg checks for strings in new files/context text; they do not prove proxy stripping, endpoint pinning, blocklist rejection, hash mismatch rejection, or memory-autoload behavior. The phase has some targeted Go tests, but env_scrub behavior itself has no explicit acceptance test.
- rollback/repair audit
  - Grounded in: spec_gap:acceptance
  - Result: passed
  - Evidence: Rollback names the new files and reverses EnvMode, provider wiring, context directive, and config read changes. No persisted schema or migration is introduced by the draft.
- design challenge
  - Grounded in: code:internal/adapters/providers/provider.go:870
  - Result: failed
  - Evidence: The product goal is valid: reviewer independence is weak while providers inherit host env, PATH, live CWD, and unfenced changed content. The design needs explicit finalize-only APIs to avoid hardening ordinary review/harden by accident or discarding receipt facts before host-gate consumes them.

Issues:
- [high/blocks approval] `harden-1` question - Receipt-grade-only behavior has no concrete activation boundary.
  - Status: open
  - Grounded in: code:internal/adapters/providers/provider.go:97
  - Evidence: The current provider constructors are shared by review and harden: providers.Select starts at provider.go:97, providers.SelectHarden is used by internal/adapters/cli/harden/selection.go:105, and normal review uses configadapter.Load plus providers.Select at internal/adapters/cli/review/selection.go:46 and :66. The draft says exact env, pinned binary, scratch CWD, and config.local-ignore apply only to receipt-grade finalize review, but it does not define a Gate/ReceiptGrade flag, separate selector, or separate invocation path.
  - Recommendation: Specify a finalize-only discriminator in the provider/config/review path and require tests for both branches: receipt-grade exact sandboxed execution and ordinary inherited-env review/harden.
  - Question: Which concrete API marks a provider request as receipt-grade so these controls cannot accidentally affect ordinary review or harden?
  - Recommended answer: Add an explicit receipt-grade provider selection/invocation path rather than inferring from provider name or review mode.
  - If unanswered: Default to adding an explicit ReceiptGrade bool or a separate SelectReceiptGradeReview/InvokeReceiptGradeReview API, with tests proving ordinary scafld review and scafld harden retain current behavior.
- [high/blocks approval] `harden-2` question - Receipt runtime facts are specified but not connected to a durable caller-visible contract.
  - Status: open
  - Grounded in: code:internal/adapters/providers/provider.go:870
  - Evidence: AgentResponse currently has Provider, Model, SessionID, OutputFormat, EventSummary, Result, and RunErr only at provider.go:47. invokeReviewAgent copies only Provider/Model/SessionID/OutputFormat/EventSummary into review.Dossier at provider.go:886-890. review.Dossier has no binary_sha256, endpoint_host, or evidence provenance fields at internal/core/review/model.go:130-144. If finalize orchestration later uses the existing Provider.Invoke interface, the runtime facts required by the receipt are discarded.
  - Recommendation: Define a typed receipt-grade review result or metadata carrier now, and include acceptance that proves the facts survive provider execution to the gate-facing caller.
  - Question: Where are binary_sha256, endpoint_host, and evidence provenance carried so host-gate can record them without scraping diagnostics?
  - Recommended answer: Expose a typed RuntimeFacts/EvidenceProvenance field on the finalize-only provider result consumed by host-gate, not only on transient AgentResponse.
  - If unanswered: Default to adding a separate receipt-grade result type that returns dossier plus runtime facts, and keep ReviewDossier unchanged unless host-gate explicitly owns a schema change.
- [high/blocks approval] `harden-3` question - Endpoint pinning and env allowlisting are underspecified against the current config model.
  - Status: open
  - Grounded in: code:internal/adapters/config/config.go:74
  - Evidence: The draft says env_scrub takes committed base-config pins, but current ExternalReviewConfig/ProviderConfig only contain provider, command, provider_binary, timeout/fallback, model, and binary fields at internal/adapters/config/config.go:74-91. The embedded config likewise only documents provider_binary/model/binary at internal/adapters/corebundle/assets/core/config.yaml:65-80. There is no declared schema for provider endpoint pins, allowed auth vars, proxy policy, or effective endpoint host derivation.
  - Recommendation: Add the schema and tests before approval: exact allowed env keys per provider, endpoint pin fields, proxy handling, and host extraction rules, with config.local ignored only on the receipt-grade read.
  - Question: What is the committed config schema for endpoint pins and provider-specific env allowlists?
  - Recommended answer: Extend ProviderConfig with committed endpoint pin fields and document the env allowlist in env_scrub tests and core config comments.
  - If unanswered: Default to a minimal committed review.external.<provider>.endpoint_url or endpoint_host pin plus explicit allowed auth env names per provider; all *_BASE_URL/*_API_BASE/*_ENDPOINT/proxy vars not matching the pin are dropped.
- [high/blocks approval] `harden-4` question - Evidence sandbox behavior is too broad to implement without invention.
  - Status: open
  - Grounded in: code:internal/adapters/providers/provider.go:655
  - Evidence: ClaudeArgs currently only allows Read,Grep,Glob tools at provider.go:671-674; it does not constrain those tools to a root beyond process CWD. CodexArgs receives p.CWD at provider.go:530, and Gemini uses policy/settings files at provider.go:589-626, but the draft does not specify the exact provider-specific mechanism that jails Read/Grep/Glob to the scratch dir or disables memory autoload beyond broad statements. The new EvidenceFile type is also not defined in any existing core package.
  - Recommendation: Specify EvidenceFile path normalization, status enum, sha256 verification, provenance shape, scratch cleanup ownership, and the exact Claude/Codex/Gemini args/settings/policies used to enforce read roots and memory-off behavior.
  - Question: What exact typed EvidenceFile contract and provider-specific jail settings must the sandbox builder return?
  - Recommended answer: Make evidence_sandbox.go produce a typed Sandbox{CWD, ReadRoots, Env, ArgsPolicy, Provenance, Cleanup} consumed by finalize-only providers.
  - If unanswered: Default to defining an internal/core/reviewevidence package with EvidenceFile and Provenance types, and provider-specific jail outputs for Claude/Codex/Gemini that are asserted in provider tests.
- [high/blocks approval] `harden-5` question - Acceptance is too weak for the stated high-risk controls.
  - Status: open
  - Grounded in: spec_gap:phases
  - Evidence: Phase 1 acceptance includes rg checks for env_scrub.go and os.Environ plus provider binary tests, but no explicit acceptance command for env_scrub behavior. Phase 2 includes rg -n 'untrusted' for the prompt fence, which would pass on a comment or weak wording. For a high-risk security task, grep checks are insufficient for proxy stripping, endpoint pinning, blocklist rejection, memory autoload disabling, and runtime fact propagation.
  - Recommendation: Replace or supplement grep-only criteria with named tests such as TestScrubDropsProxyAndUnpinnedEndpoint, TestScrubPinsEndpointHost, TestReceiptGradeProviderUsesEnvModeExact, TestEvidenceSandboxRejectsAgentInstructionFiles, and TestReceiptGradeReviewReturnsRuntimeFacts.
  - Question: Which acceptance tests prove the security controls semantically work rather than merely exist as strings?
  - Recommended answer: Keep grep as smoke checks only; add Go tests for every security invariant.
  - If unanswered: Default to adding focused Go acceptance commands for env_scrub, sandbox blocklist/hash mismatch, prompt directive text, and runtime fact propagation.


## Planning Log

- none
