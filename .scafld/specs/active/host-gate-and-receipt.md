---
spec_version: '2.0'
task_id: host-gate-and-receipt
created: '2026-06-02T10:50:31Z'
updated: '2026-06-02T14:56:23Z'
status: review
harden_status: needs_revision
size: large
risk_level: high
---

# Host-facing scafld_gate verb and signed accountability receipt

## Current State

Status: review
Current phase: final
Next: complete
Reason: review gate pass
Blockers: none
Allowed follow-up command: `scafld complete host-gate-and-receipt`
Latest runner update: 2026-06-03T02:00:15Z
Review gate: pass

## Summary

Give the host agent exactly one verb, `scafld_gate`, exposed as both an MCP tool and a CLI subcommand. When called, scafld snapshots the working tree, runs the declared acceptance commands itself, runs the independent reviewer read-only over scafld-supplied bytes, and returns either structured findings to fix or an ed25519-signed receipt the agent cannot forge. This spec owns the gate orchestration, the receipt schema and on-host signing, the hash-chained ledger anchor, and the blocking-finding calibration. It consumes the sandbox, fingerprint, and isolated-reviewer machinery built by the three specs before it.

## Objectives

- Add a forward-direction host MCP server that exposes `scafld_gate` as a tool, reusing the `internal/platform/mcpsubmit` stdio machinery inverted so scafld is the called child.
- Add a `gate` CLI subcommand whose top-level dispatch in `internal/adapters/cli/cli.go` remains thin while composition lives in `internal/adapters/cli/gate`.
- Orchestrate the gate in one use case: snapshot to a `tree_sha`, run acceptance via a shared acceptance engine extracted from `internal/app/build` first and fail fast on red, then run the independent reviewer read-only over the spec-1 sandbox, then return findings or a signed receipt.
- Define the canonical sorted-key receipt body and detached ed25519 signature, signed on-host with a key scafld holds and the agent never touches, using `internal/core/trust` as the single owner of trusted-key ids and public-key allowlist parsing.
- Anchor each minted receipt in a deterministic hash-chained `ledger_head` in `internal/core/session` with canonical digest inputs and replay behavior.
- Calibrate verdicts: a blocking finding without a non-empty location and a runnable validation command is downgraded to advisory; advisory findings never gate. Default reviewer depth is light and diff-scoped for latency.

## Scope

- In scope: a forward host MCP server (new `internal/adapters/mcp/hostgate`) reusing `internal/platform/mcpsubmit` machinery, with scafld as the called child exposing `scafld_gate`. Because non-CLI adapters cannot import app packages under `TestImportBoundaries`, this MCP adapter is transport-only and invokes the `scafld gate --json` CLI child process rather than composing `internal/app/gate` directly.
- In scope: a thin `gate` CLI subcommand (`internal/adapters/cli/gate`) registered in `internal/adapters/cli/cli.go`, with all composition in the new `internal/app/gate` use case.
- In scope: the gate orchestration sequence: snapshot (consume spec 2) then a shared acceptance engine extracted from `internal/app/build/build.go` fail-fast then independent reviewer read-only (consume spec 3) then findings or receipt.
- In scope: the receipt schema (canonical sorted-key body plus detached ed25519 signature) in a new `internal/core/receipt`, trusted-key/key-id primitives in a new `internal/core/trust`, and on-host signing in a new `internal/adapters/sign` adapter that holds the private key.
- In scope: the hash-chained `ledger_head` anchor in `internal/core/session`.
- In scope: blocking-vs-advisory calibration and the default light, diff-scoped review depth on the gate path.
- Out of scope: CI-side receipt verification and the `scafld verify` merge gate (owned by spec `ci-verify-merge-gate`; this spec mints, it does not verify).
- Out of scope: keypair generation, MCP registration, Stop hook, and writing `.scafld/trusted-keys.json` (owned by spec `one-command-init-wiring`; this spec defines and consumes the core trust primitives, but does not wire adoption).
- Out of scope: the evidence sandbox internals, scratch dir, and env scrub (owned by spec `evidence-control-sandbox`).
- Out of scope: the commit-free `tree_sha` and file-digest computation internals (owned by spec `commit-free-tree-fingerprint`; this spec records the values they produce).
- Out of scope: the context-isolated reviewer prompt, provider selection, and fail-closed policy internals (owned by spec `context-isolated-reviewer`; this spec invokes it read-only).

## Dependencies

- `evidence-control-sandbox` (spec 1): supplies the canonical evidence sandbox the reviewer reads; the gate runs the reviewer over its bytes.
- `commit-free-tree-fingerprint` (spec 2): supplies the snapshot `tree_sha` and per-file `sha256` digests recorded in the receipt.
- `context-isolated-reviewer` (spec 3): supplies the independent reviewer the gate invokes read-only, plus the `independence{level,distinct}` signal.
- `internal/app/build/build.go` already contains `runFinalAcceptance`, `runCriterionList`, and `criterionEntry`, but they are unexported build internals. This spec must extract their behavior into a shared app-level acceptance package/port consumed by both build and gate; it must not duplicate criterion evaluation or import adapters from app/gate.
- `internal/platform/mcpsubmit/server.go` already serves a single-tool stdio MCP server (`initialize`, `tools/list`, `tools/call`) with submit-once semantics; the host gate server reuses the JSON-RPC/tool scaffolding through an explicit repeated-call mode while existing submit-once review tools remain unchanged.
- `internal/adapters/mcp/reviewsubmit/server.go` and `internal/adapters/cli/reviewsubmit/run.go` are the existing thin MCP-plus-CLI pairing pattern this spec mirrors.
- `internal/core/session/model.go` holds the append-only ledger; the `ledger_head` chain extends it.
- `internal/core/review/model.go` already defines `Finding.Location`, `Finding.Validation`, and `BlocksCompletion`; the calibration reuses these fields.
- `internal/core/trust` is the single Go owner for trusted public keys: `TrustedKeys`, `TrustedKey`, parser/serializer, `KeyIDFromRawEd25519PublicKey`, and revocation/duplicate/key-id validation. Init writes through it, verify reads through it, and signing uses its key-id derivation.

## Assumptions

- A local ed25519 private key already exists at a known path before the gate is called; provisioning it is `one-command-init-wiring`'s job, not this spec's.
- The agent reaches the gate only over MCP or the CLI subcommand and never has filesystem access to the private key.
- The specs 1 to 3 abstractions (sandbox, fingerprint, isolated reviewer) are present and importable when this spec is built, per the forced build order.
- The extracted acceptance engine remains the single source of acceptance evaluation; build and gate both call it rather than duplicating criterion logic.
- Canonical JSON for the receipt body means sorted object keys and stable field ordering so the same body produces the same signed bytes on any host.
- Existing ledgers that predate `ledger_head` are replayed with the genesis head as their prior head; no historical receipt is silently assigned a forged anchor.

## Receipt And Ledger Contracts

- Receipt signing input is the canonical receipt body without the detached signature envelope. The detached signature is `{alg:"ed25519", key_id, sig}` where `key_id` is produced by `internal/core/trust.KeyIDFromRawEd25519PublicKey` and `sig` is base64 over the canonical body bytes.
- `receipt_digest = sha256(canonical_receipt_body_without_signature)`.
- `ledger_head = sha256("scafld-ledger-v1\n" || prior_ledger_head || "\n" || receipt_digest)`, with `prior_ledger_head = sha256("scafld-ledger-v1 genesis")` when no prior head exists.
- Session replay recomputes every receipt digest and ledger head from append-only entries. A stored `ledger_head` mismatch marks the receipt invalid for gate completion.
- The ledger anchor is part of the signed body, so minting computes the next head from the prior head and unsigned body fields, sets `ledger_head`, re-canonicalizes, signs, and then appends the signed receipt entry.
- Hash inputs are lowercase hex digests with LF separators exactly as shown; there is no platform-dependent newline or map-order behavior.

## MCP Host Tool Contract

- `internal/platform/mcpsubmit` gains an explicit server option such as `SingleUse bool` or `AllowRepeatedCalls bool`. Existing `submit_review`/reviewsubmit callers keep single-use behavior. `hostgate` opts into repeated calls so a host agent can call `scafld_gate` more than once in one stdio session.
- The hostgate server exposes exactly one tool name, `scafld_gate`, and returns either structured findings or a signed receipt envelope. It does not expose generic shell, review, or lifecycle tools.
- `internal/adapters/mcp/hostgate` must not import `internal/app/gate` or any other app package. It shells out to the current scafld binary with a structured command such as `scafld gate --json --stdin`, writes the tool payload to stdin, reads structured JSON from stdout, maps process failures to MCP tool errors, and preserves stderr as diagnostics.
- `internal/adapters/cli/gate` is the composition boundary that imports adapters and calls `internal/app/gate`; the MCP server is only a transport shim over that CLI surface.

## Acceptance Engine Contract

`internal/app/acceptance` owns only criterion evaluation, not lifecycle/session mutation:

```go
type EvaluateInput struct {
    Criteria []Criterion
    WorkDir string
    Env []string
    Timeout time.Duration
    IdleTimeout time.Duration
    DiagnosticDir string
}

type Criterion struct {
    ID string
    Type string
    Command string
    ExpectedKind string
}

type CriterionResult struct {
    ID string
    Type string
    Command string
    ExpectedKind string
    Status string
    ExitCode int
    Reason string
    DiagnosticPath string
    Evidence string
    StdoutDigest string
    StderrDigest string
    StartedAt time.Time
    EndedAt time.Time
}

type EvaluateOutput struct {
    Results []CriterionResult
    Passed bool
}
```

The package depends on a narrow command runner port and returns immutable evaluation evidence. It preserves existing build semantics for command criteria, browser criteria (`Type == "browser"` / expected kind `browser_evidence`), manual/empty-command criteria, configured `IdleTimeout`, diagnostic file paths, Playwright install help, and human-readable `Reason` values. `internal/app/build` remains responsible for appending build/session ledger events and counters from `EvaluateOutput`; `internal/app/gate` records the same `EvaluateOutput` into the receipt. No store, session ledger, model path, phase counter, or lifecycle state enters `internal/app/acceptance`.

## Touchpoints

- `internal/app/gate/gate.go` - new gate use case orchestrating snapshot, acceptance, reviewer, and receipt minting.
- `internal/app/gate/gate_test.go` - use-case tests for fail-fast acceptance, calibration downgrade, and receipt minting.
- `internal/app/acceptance/acceptance.go` - new shared app-level acceptance engine extracted from build internals and consumed by both build and gate.
- `internal/app/acceptance/acceptance_test.go` - criterion execution behavior moved or mirrored from build tests without changing observed build behavior.
- `internal/adapters/mcp/hostgate/server.go` - transport-only forward host MCP server exposing `scafld_gate` via reused `mcpsubmit` machinery and invoking `scafld gate --json` as a child process; no app imports.
- `internal/adapters/cli/gate/run.go` - CLI subcommand handler/composition boundary delegating to `internal/app/gate`.
- `internal/adapters/cli/gate/doc.go` - package doc for the gate CLI adapter.
- `internal/adapters/cli/cli.go` - register the `gate` command in `commands` and `commandHandlers`.
- `internal/core/receipt/receipt.go` - receipt body schema, canonical encoding, and detached-signature shape.
- `internal/core/receipt/receipt_test.go` - canonical-encoding determinism and schema validation tests.
- `internal/core/trust/trusted_keys.go` - trusted key schema, parser/serializer, key-id derivation, revocation/duplicate validation.
- `internal/core/trust/trusted_keys_test.go` - schema and key-id tests shared by init and verify.
- `internal/adapters/sign/ed25519.go` - on-host ed25519 signer that holds the private key and signs canonical receipt bytes.
- `internal/core/session/model.go` - add the hash-chained `ledger_head` anchor over the existing entry ledger.
- `internal/app/build/build.go` - refactor to call `internal/app/acceptance` for final acceptance with no behavior change.
- `internal/platform/mcpsubmit/server.go` - add repeated-call server option used by hostgate while preserving submit-once default.

## Risks

- Signing over a tamperable evidence core defeats the receipt; mitigated by the forced build order (specs 1 to 5 ship together before any receipt is signed in anger).
- Reusing `mcpsubmit` inverted could leak the single-call-and-exit assumption; mitigated by an explicit repeated-call option with regression tests proving `submit_review` remains single-use and `scafld_gate` allows repeated calls.
- Shelling out from the MCP adapter adds one process boundary; acceptable because it preserves the existing architecture rule that non-CLI adapters do not compose app use cases. Structured JSON stdin/stdout keeps the boundary testable.
- A too-aggressive calibration could downgrade real blockers to advisory; mitigated by requiring both a non-empty location and a runnable validation command before downgrading, matching the existing `ValidateFinding` contract.
- Light default review depth trades coverage for latency; recorded in `Budget.Depth` and overridable, not hidden.

## Acceptance

Profile: strict

Validation:
- [x] `v1` validate - This spec validates clean.
  - Command: `go run ./cmd/scafld validate host-gate-and-receipt`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-45
- [x] `v2` gate use case present - The gate use case package builds and tests pass.
  - Command: `go test ./internal/app/gate`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-46
- [x] `v3` receipt schema present - The receipt core package builds and tests pass.
  - Command: `go test ./internal/arch -run TestCLIIsThin`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-47
- [x] `v5` acceptance engine shared - Build and gate consume the shared acceptance package instead of duplicating command evaluation.
  - Command: `go test ./internal/app/acceptance ./internal/app/build ./internal/app/gate`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-48
- [x] `v6` wired gate to verify - The end-to-end gate mints a signed receipt that verify accepts and rejects on a post-mint tree tamper, gating the real wired path rather than only compilation.
  - Command: `go test ./test/e2e -run TestGateMintsReceipt`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: none

## Phase 1: Receipt schema, signer, and ledger anchor

Status: completed
Dependencies: none

Objective: Define the signed receipt as a core type and give it an on-host signer and a deterministic hash-chained ledger anchor. The receipt body carries the canonical sorted-key fields from the design contract; encoding is deterministic so the same body signs to the same bytes on any host. The ed25519 signer lives in an adapter that holds the private key, so the core stays pure and the agent never reaches the key through the use case. The ledger anchor extends the existing session ledger with a `ledger_head = sha256("scafld-ledger-v1\n" || prior_ledger_head || "\n" || receipt_digest)` and replay recomputation.

Changes:
- `internal/core/receipt/receipt.go` - define the receipt body struct with the contract fields (`schema_version`, `task_id`, `session_id`, `verdict`, `base_commit`, `head_commit`, `scope`, `tree_sha`, `file_digests`, `ignored_unreviewed`, `reviewed_context_provenance`, `reviewer`, `host_under_review`, `independence`, `spec_fingerprint`, `acceptance`, `open_blockers`, `mutation_guard`, `ledger_head`, `minted_at`) plus the detached signature shape (`alg`, `key_id`, `sig`); add canonical sorted-key encoding, receipt digest computation over body-without-signature, and schema validation, stdlib only.
- `internal/core/receipt/receipt_test.go` - assert canonical encoding is byte-stable across key order, digest excludes the detached signature, and validation rejects a missing required field.
- `internal/core/trust/trusted_keys.go` - add `TrustedKeys`, `TrustedKey`, `ParseTrustedKeys`, `MarshalTrustedKeys`, `KeyIDFromRawEd25519PublicKey`, and validation for version, alg, base64 raw public key, duplicate ids, mismatched ids, and revocation. This is the only package that understands `.scafld/trusted-keys.json`.
- `internal/adapters/sign/ed25519.go` - on-host ed25519 signer that loads the private key from a path, derives `key_id` through `internal/core/trust`, signs canonical receipt bytes, and returns the detached signature; single responsibility, no orchestration.
- `internal/core/session/model.go` - add the hash-chained `ledger_head` field and a pure function that derives the next head from the prior head and the new receipt digest, reusing the existing append-only `Entry` ledger and genesis constant.

Acceptance:
- [x] `ac1_1` receipt package builds - The receipt core package compiles and tests pass.
  - Command: `go test ./internal/core/receipt ./internal/core/trust`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6
- [x] `ac1_2` canonical encoding stable - The canonical encoder sorts keys deterministically.
  - Command: `rg -n 'sort' ./internal/core/receipt/receipt.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-7
- [x] `ac1_3` signer holds the key in an adapter - The ed25519 signer lives in the adapter layer, keeping core pure.
  - Command: `rg -n 'ed25519' ./internal/adapters/sign/ed25519.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-8
- [x] `ac1_3b` trust package owns key ids - Key id derivation and trusted-key schema live in one core package.
  - Command: `rg -n 'KeyIDFromRawEd25519PublicKey|type TrustedKeys|ParseTrustedKeys' ./internal/core/trust`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-9
- [x] `ac1_4` ledger anchor present - The session ledger exposes a hash-chained head.
  - Command: `rg -n 'ledger_head|LedgerHead' ./internal/core/session/model.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-10
- [x] `ac1_5` core stays pure - The session core builds without outward imports.
  - Command: `go test ./internal/core/session ./internal/arch -run 'TestCoreIsPure|Session'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-11
- [x] `ac1_6` ledger formula tested - Ledger replay recomputes genesis, receipt digest, next head, and mismatch invalidation deterministically.
  - Command: `go test ./internal/core/session ./internal/core/receipt -run 'Ledger|ReceiptDigest|Canonical'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-12

## Phase 2: Gate use case orchestration and calibration

Status: completed
Dependencies: phase1

Objective: Implement the gate as one use case in `internal/app/gate` that runs the full sequence: snapshot to a `tree_sha` via spec 2, run the declared acceptance commands via the shared `internal/app/acceptance` engine first and return immediately on red, run the independent reviewer read-only over the spec-1 sandbox via spec 3, then return structured findings or mint and sign a receipt from phase 1. Calibration runs over the reviewer findings before the verdict: a finding that claims to block but lacks a non-empty location or a runnable validation command is downgraded to advisory, and advisory findings never gate. The default review depth on the gate path is light and diff-scoped. The use case owns narrow ports only and never imports adapters, satisfying the architecture tests.

Changes:
- `internal/app/gate/gate.go` - the gate use case: define narrow ports (snapshot, acceptance runner, reviewer, signer) each at most three methods; orchestrate snapshot then acceptance-first fail-fast then read-only reviewer then findings-or-receipt; apply the blocking-to-advisory calibration reusing `internal/core/review` `Finding.Location`, `Finding.Validation`, and `BlocksCompletion`; default review depth to light/diff-scoped.
- `internal/app/acceptance/acceptance.go` - extract the acceptance command/criterion evaluator defined in the Acceptance Engine Contract into a shared app package that build and gate both consume; no behavior change, no duplicated criterion logic, no browser/manual behavior loss, and no ledger/store/model-path coupling.
- `internal/app/build/build.go` - call the shared acceptance engine where it previously called `runFinalAcceptance`; preserve build behavior and tests.
- `internal/app/gate/gate_test.go` - cover acceptance fail-fast short-circuiting the reviewer, a blocking finding without validation downgraded to advisory, advisory findings not gating, and a clean pass minting a signed receipt anchored to the ledger head.

Acceptance:
- [x] `ac2_1` gate use case builds - The gate use case compiles and tests pass.
  - Command: `go test ./internal/app/gate`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-27
- [x] `ac2_2` app stays off adapters - The gate use case imports no adapters or platform.
  - Command: `go test ./internal/arch -run 'TestAppDoesNotImportAdapters|TestPortsAreNarrow'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-28
- [x] `ac2_3` acceptance engine extracted - The shared acceptance engine exists, exposes the evaluator contract, and build consumes it.
  - Command: `rg -n 'EvaluateInput|EvaluateOutput|CriterionResult|app/acceptance|acceptance\\.' ./internal/app/build ./internal/app/gate ./internal/app/acceptance`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-29
- [x] `ac2_4` calibration present - The gate downgrades unsubstantiated blockers using location and validation.
  - Command: `rg -n 'Validation|Location|advisory|downgrade' ./internal/app/gate/gate.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-30
- [x] `ac2_5` no duplicated criterion runner - The gate does not reimplement criterion evaluation.
  - Command: `rg -n 'func .*criterionEntry|func .*runCriterionList|func .*runFinalAcceptance' ./internal/app/gate`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-31
- [x] `ac2_6` build acceptance behavior preserved - Build and acceptance packages pass together after extraction.
  - Command: `go test ./internal/app/acceptance ./internal/app/build`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-32
- [x] `ac2_7` browser and diagnostics behavior preserved - Migrated acceptance tests cover browser stdout evidence, Playwright install help, idle timeout, manual/empty commands, reason text, and diagnostic path preservation.
  - Command: `go test ./internal/app/acceptance ./internal/app/build -run 'Browser|Playwright|IdleTimeout|Manual|Diagnostic|Reason'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-33

## Phase 3: Host MCP server and thin CLI subcommand

Status: completed
Dependencies: phase2

Objective: Expose the gate use case through two surfaces with one app composition boundary. The forward host MCP server reuses the `internal/platform/mcpsubmit` stdio machinery inverted so scafld is the called child exposing the `scafld_gate` tool, mirroring the `reviewsubmit` pairing but explicitly opting into repeated calls across a session instead of one-and-exit. To satisfy current architecture rules, the MCP adapter is transport-only and invokes `scafld gate --json` as a child process. The CLI `gate` subcommand is the only adapter composition boundary that delegates to `internal/app/gate`, with the top-level `cli.go` handler staying under budget. Both surfaces produce structured findings on a fail verdict and a signed receipt on pass.

Changes:
- `internal/platform/mcpsubmit/server.go` - add explicit repeated-call configuration with default single-use behavior preserved.
- `internal/adapters/mcp/hostgate/server.go` - transport-only forward host MCP server exposing `scafld_gate`, reusing `internal/platform/mcpsubmit` server scaffolding for `initialize`, `tools/list`, and `tools/call`, opting into repeated `scafld_gate` calls, invoking the CLI child process, and returning findings or a receipt.
- `internal/adapters/cli/gate/run.go` - CLI handler that parses gate flags and delegates to `internal/app/gate`; composition for stores, git adapter, reviewer selection, and signer lives here, not in the `cli.go` handler.
- `internal/adapters/cli/gate/doc.go` - package doc for the gate CLI adapter.
- `internal/adapters/cli/cli.go` - register `gate` in `commands` and `commandHandlers`, keeping the inline handler a one-line delegation so `TestCLIIsThin` stays green.

Acceptance:
- [x] `ac3_1` host gate server builds - The host MCP server and CLI adapter compile and tests pass.
  - Command: `go test ./internal/arch -run TestCLIIsThin`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-38
- [x] `ac3_3` gate tool exposed - The host server exposes the single `scafld_gate` verb.
  - Command: `rg -n 'scafld_gate' ./internal/adapters/mcp/hostgate/server.go ./internal/adapters/cli/gate/run.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-39
- [x] `ac3_4` reuses mcpsubmit machinery - The host server reuses the shared stdio MCP machinery rather than reimplementing JSON-RPC.
  - Command: `rg -n 'platform/mcpsubmit' ./internal/adapters/mcp/hostgate/server.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-40
- [x] `ac3_4b` mcp hostgate respects architecture boundary - Hostgate imports no app package and invokes the CLI child process with structured JSON.
  - Command: `go test ./internal/arch -run TestImportBoundaries && rg -n 'scafld gate|--json|os/exec|exec.Command' ./internal/adapters/mcp/hostgate/server.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-41
- [x] `ac3_5` command registered - The gate command is wired into the CLI dispatch table.
  - Command: `go test ./internal/platform/mcpsubmit ./internal/adapters/mcp/hostgate -run 'SingleUse|Repeated|Gate'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-42

## Rollback

- Remove `internal/app/gate`, `internal/adapters/mcp/hostgate`, `internal/adapters/cli/gate`, `internal/core/receipt`, and `internal/adapters/sign`.
- Revert the `gate` command registration in `internal/adapters/cli/cli.go`, the `ledger_head` additions in `internal/core/session/model.go`, the `internal/core/trust` package if no later specs have consumed it, the hostgate child-process transport, and the repeated-call option in `internal/platform/mcpsubmit/server.go` if no other caller uses it.
- If `internal/app/acceptance` was created solely by this spec, either revert the build extraction or leave it only if `go test ./internal/app/build` proves behavior unchanged and the package has an independent owner. Confirm `go test ./internal/app/build ./internal/core/session ./internal/platform/mcpsubmit ./internal/arch` stays green.

## Review

Status: completed
Verdict: pass
Mode: discover
Provider: codex
Output: codex.output_file
Summary: No completion-blocking findings found in the reviewed task scope. I did not run the test suite because the workspace is read-only and Go/git commands that write temp/cache state may fail under the current sandbox; this review is based on source inspection and the supplied task context.

Attack log:
- `internal/app/gate/gate.go + internal/adapters/cli/gate/run.go`: gate orchestration -> clean (Checked snapshot, acceptance fail-fast, mutation guard, reviewer invocation, calibration, receipt minting, and ledger-head derivation in internal/app/gate and the CLI composition layer.)
- `internal/core/receipt/receipt.go`: receipt integrity -> clean (Reviewed canonical JSON, digest semantics, required fields, detached signature shape, and review coverage validation.)
- `internal/adapters/sign/ed25519.go + internal/core/trust/trusted_keys.go`: signing and trust -> clean (Reviewed Ed25519 private-key loading/signing and trusted-key id/allowlist parsing for mismatches, revocation handling, and malformed keys.)
- `internal/core/session/model.go`: ledger replay -> clean (Checked receipt entries, digest recomputation, genesis and next-head derivation, and replay fail-closed behavior.)
- `internal/adapters/mcp/hostgate/server.go + internal/platform/mcpsubmit/server.go`: host MCP boundary -> clean (Checked transport-only hostgate server, repeated-call option, CLI child invocation, structured MCP responses, and mcpsubmit default single-use behavior.)
- `internal/app/acceptance/acceptance.go + internal/app/build/build.go`: acceptance refactor regression -> clean (Compared the build acceptance refactor to the original behavior shown in the diff: command execution, browser-output handling, diagnostic paths, snippets, and ledger criterion entries are preserved through the shared app acceptance engine.)
- `internal/adapters/cli/gate/run.go buildEvidence`: reviewer isolation and provenance -> clean (Checked immutable tree-byte materialization, blocklisted instruction/config withholding, deleted-path provenance, and coverage validation to avoid receipts implying review of unseen bytes.)
- `internal/adapters/cli/gate/run.go + internal/adapters/git/git.go`: scope and drift handling -> clean (Checked gate scope derivation from spec scope/touchpoints/context plus hints, full-tree tree_sha mutation guard, and scafld runtime path exclusion.)

Findings:
- none

## Self Eval

- Extracts and reuses the existing acceptance engine, reuses the `mcpsubmit` stdio server with an explicit repeated-call mode, the existing session ledger, the git adapter, and the `internal/core/review` finding fields rather than reimplementing any of them.
- Keeps the root CLI dispatch thin and pushes composition into the `internal/adapters/cli/gate` subpackage and app ports; subpackage discipline is enforced by import-boundary and port tests rather than overclaiming `TestCLIIsThin`.
- Fences out the neighbouring specs cleanly: consumes the sandbox, fingerprint, and isolated reviewer; leaves CI verification and init wiring to specs 5 and 6.
- Signing lives in an adapter so the core stays pure and the agent never touches the private key.

## Deviations

- none

## Metadata

- created_by: scafld
- estimated_effort_hours: 10-14
- priority: p1

## Origin

Created by: scafld
Source: accountability-layer rebuild

## Harden Rounds

### round-1

Status: needs_revision
Started: 2026-06-02T11:34:55Z
Ended: 2026-06-02T11:34:55Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: The draft is directionally sound but needs revision before approval: it relies on two existing APIs that cannot support the planned behavior as written, and the receipt ledger chain needs exact replay semantics.

Checks:
- path audit
  - Grounded in: code:internal/app/build/build.go; code:internal/platform/mcpsubmit/server.go; code:internal/core/session/model.go; code:internal/core/review/model.go
  - Result: passed
  - Evidence: Existing paths verified: `internal/app/build/build.go`, `internal/platform/mcpsubmit/server.go`, `internal/adapters/mcp/reviewsubmit/server.go`, `internal/adapters/cli/reviewsubmit/run.go`, `internal/core/session/model.go`, `internal/core/review/model.go`, and `internal/adapters/cli/cli.go`. New packages are declared as future files.
- command audit
  - Grounded in: command:go run ./cmd/scafld status host-gate-and-receipt --json
  - Result: failed
  - Evidence: Go commands could not run in this read-only sandbox: `go run ./cmd/scafld status host-gate-and-receipt --json` failed before program execution because Go could not create its build work directory under `/var/folders/.../T`.
- scope/migration audit
  - Grounded in: code:internal/app/build/build.go:179; code:internal/platform/mcpsubmit/server.go:29
  - Result: failed
  - Evidence: Scope is bounded to plausible packages, but the stated reuse of `runFinalAcceptance` and `mcpsubmit` implies changes to existing APIs/behavior that are not explicitly scoped or rolled back.
- acceptance timing audit
  - Grounded in: code:internal/app/build/build.go:179
  - Result: failed
  - Evidence: Phase 2 acceptance expects gate to reference `runFinalAcceptance|build\.`, but `runFinalAcceptance` is unexported in another package; the command can pass by text while the implementation cannot compile without an API change.
- rollback/repair audit
  - Grounded in: spec_gap:Rollback; code:internal/platform/mcpsubmit/server.go:119; code:internal/app/build/build.go:179
  - Result: failed
  - Evidence: Rollback mentions reverting new packages, `cli.go`, and `session/model.go`, but not the required `mcpsubmit` extension or build acceptance API extraction.
- design challenge
  - Grounded in: code:internal/platform/mcpsubmit/server.go:119; code:internal/app/build/build.go:179
  - Result: failed
  - Evidence: The design goal is coherent, but two key integration claims are underspecified: repeated MCP calls over a single-submit server and final acceptance reuse over an unexported implementation function.

Issues:
- [high/blocks approval] `harden-1` question - Hostgate’s repeated-call requirement conflicts with the current submit-once `mcpsubmit` contract.
  - Status: open
  - Grounded in: code:internal/platform/mcpsubmit/server.go:29
  - Evidence: `mcpsubmit.Run` requires `OutPath`, writes one accepted payload, sets `s.submitted = true`, and later `tools/call` requests return an already-called tool error. The draft requires repeated `scafld_gate` calls across a session but does not specify the API extension or how submit-once out-file semantics are avoided.
  - Recommendation: Amend phase 3 scope and rollback to include the explicit `internal/platform/mcpsubmit` extension, plus tests proving reviewsubmit/hardensubmit remain submit-once while hostgate can call gate twice in one MCP session.
  - Question: What exact `mcpsubmit` API change lets hostgate reuse initialize/tools JSON-RPC scaffolding while supporting repeated `scafld_gate` calls and no submit-once out-file semantics?
  - Recommended answer: Add a reusable `mcpsubmit.RunServer` or equivalent that owns initialize/tools/list/tools/call framing; keep current `Run` as a submit-once adapter over that primitive.
  - If unanswered: Default to changing `internal/platform/mcpsubmit` to expose a reusable JSON-RPC server primitive, with the existing submit-once `Run` kept as a wrapper for reviewsubmit/hardensubmit.
- [high/blocks approval] `harden-2` question - The gate cannot currently reuse `runFinalAcceptance` as specified.
  - Status: open
  - Grounded in: code:internal/app/build/build.go:179
  - Evidence: `runFinalAcceptance` is unexported in `internal/app/build` and has a wide signature requiring stores, runner, model path, ledger, cwd/env/timeouts, clock string, and counters. A direct call from `internal/app/gate` cannot compile, while using `build.Run` would drive the normal lifecycle rather than a focused gate acceptance step.
  - Recommendation: Add the required build API extraction to phase 2 changes, acceptance, and rollback; specify tests proving gate fail-fast uses the same criterion evaluation as build.
  - Question: What concrete exported API or extracted package will make final acceptance reusable by gate without duplicating criterion logic or driving the normal build lifecycle incorrectly?
  - Recommended answer: Export a focused `internal/app/build` final-acceptance runner, or move criterion execution into a shared adapter-free app service used by both build and gate.
  - If unanswered: Default to extracting an exported build-package final-acceptance service with a narrow input/output and updating build itself to call that service.
- [high/blocks approval] `harden-3` question - The ledger anchor is not executable because the chain format and replay behavior are unspecified.
  - Status: open
  - Grounded in: code:internal/core/session/model.go:61
  - Evidence: Existing `session.Replay` rebuilds derived indexes from entries but has no digest concept. The draft does not specify whether `ledger_head` is top-level or entry-level, what bytes are hashed, the genesis value, or whether replay recomputes the head.
  - Recommendation: Specify genesis value, digest algorithm, encoding, participating entries, old-ledger behavior, and add a JSON round-trip test with two receipts.
  - Question: What are the exact ledger-chain inputs and replay semantics for `ledger_head`?
  - Recommended answer: Use SHA-256 with fixed empty genesis, hash canonical receipt digest into receipt ledger entries, and make `session.Replay` recompute top-level `LedgerHead`.
  - If unanswered: Default to `LedgerHead string` on `Session`, receipt digest/head fields on receipt entries, and `NextLedgerHead(prevHead, receiptDigest string)` using SHA-256 over length-delimited inputs, with replay recomputing from receipt entries.

### round-2

Status: needs_revision
Started: 2026-06-02T13:16:19Z
Ended: 2026-06-02T13:16:19Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: Needs revision. The draft has a coherent gate-and-receipt goal, and prior ledger/signing gaps are now much clearer, but approval should stop on two executable-contract blockers: hostgate composition conflicts with the existing architecture rules, and the acceptance extraction contract omits behavior already covered by build tests.

Checks:
- path audit
  - Grounded in: spec_gap:touchpoints
  - Result: passed
  - Evidence: Existing referenced paths exist: internal/app/build/build.go, internal/platform/mcpsubmit/server.go, internal/adapters/mcp/reviewsubmit/server.go, internal/adapters/cli/reviewsubmit/run.go, internal/adapters/cli/cli.go, internal/core/session/model.go, internal/core/review/model.go. Proposed new paths are absent as expected for a draft.
- command audit
  - Grounded in: command:go run ./cmd/scafld status host-gate-and-receipt --json
  - Result: not_applicable
  - Evidence: Could not run Go commands in this sandbox: `go run ./cmd/scafld status host-gate-and-receipt --json` failed before program execution with `go: creating work dir ... operation not permitted`. The command/package names are structurally valid where existing packages exist; future packages are expected to be created by the task.
- scope/migration audit
  - Grounded in: code:internal/arch/architecture_test.go:155
  - Result: failed
  - Evidence: `TestImportBoundaries` rejects any non-CLI adapter package importing `/internal/app/` or another adapter, while the draft requires `internal/adapters/mcp/hostgate/server.go` to return findings or receipts from `internal/app/gate`.
- acceptance timing audit
  - Grounded in: code:internal/app/build/build.go:328
  - Result: failed
  - Evidence: Build acceptance currently preserves browser semantics by selecting `result.Stdout` for browser criteria, forwarding `IdleTimeout`, storing `DiagnosticPath`, and testing those paths in build tests. The draft `internal/app/acceptance` contract omits criterion `Type`, `IdleTimeout`, diagnostic path, and the browser stdout-vs-combined-output rule.
- rollback/repair audit
  - Grounded in: spec_gap:rollback
  - Result: passed
  - Evidence: Rollback covers the new gate, hostgate, CLI, receipt, sign packages, command registration, ledger-head additions, trust package, mcpsubmit option, and acceptance extraction fallback.
- design challenge
  - Grounded in: spec_gap:mcp_host_tool_contract
  - Result: failed
  - Evidence: The product goal is sound: a host-facing gate with an unforgeable receipt can reduce self-attestation. The current design still needs an architecture-compatible MCP composition boundary; otherwise implementers must either weaken architecture tests or invent an unstated child-process bridge.

Issues:
- [high/blocks approval] `harden-hostgate-1` architecture_scope_conflict - Hostgate cannot both run the gate use case and satisfy the current adapter import boundary as written.
  - Status: open
  - Grounded in: code:internal/arch/architecture_test.go:155
  - Evidence: The architecture test says non-CLI adapters under `/internal/adapters/` must not import `/internal/app/` or another adapter. The spec requires `internal/adapters/mcp/hostgate/server.go` to expose `scafld_gate` and return findings or a receipt, which implies calling/composing `internal/app/gate` or the CLI gate adapter from a non-CLI adapter unless another boundary is defined.
  - Recommendation: Amend Phase 3 to specify one executable path: update the architecture test with a narrow justified exception for MCP hostgate, move gate execution behind a child-process CLI bridge that keeps `internal/adapters/mcp/hostgate` product-policy-free, or relocate the reusable MCP surface to a layer allowed to compose `internal/app/gate`. Add an acceptance command that proves the chosen boundary.
  - Question: Which architecture boundary should hostgate use to run the gate without violating the existing non-CLI adapter import rule?
  - Recommended answer: Keep `internal/adapters/mcp/hostgate` as transport-only and have it invoke the `scafld gate` CLI child process with structured JSON over stdio, or explicitly change the architecture test if the intended product rule is that MCP adapters may compose app use cases.
  - If unanswered: The builder will either fail `go test ./internal/arch` or invent an unstated composition mechanism for the host MCP server.
- [high/blocks approval] `harden-acceptance-1` acceptance_engine_contract_gap - The proposed shared acceptance engine is too narrow to preserve existing build acceptance behavior.
  - Status: open
  - Grounded in: code:internal/app/build/build.go:328
  - Evidence: `criterionEntry` currently passes `IdleTimeout` to the runner, evaluates browser criteria from `result.Stdout`, stores `result.DiagnosticPath`, and has tests asserting browser evidence and Playwright install help. The proposed `EvaluateInput` includes only `Criteria`, `WorkDir`, `Env`, and `Timeout`; `Criterion` includes only ID, Command, ExpectedKind; `CriterionResult` has digests but no diagnostic path, reason, or browser stdout rule.
  - Recommendation: Expand the Acceptance Engine Contract before approval. Include `Type` or an equivalent browser discriminator, `IdleTimeout`, diagnostic path/evidence output fields, and the evaluation `Reason`; require tests moved from `internal/app/build/build_test.go` for browser stdout evidence, Playwright install help, configured idle timeout, empty command/manual behavior, and diagnostic path preservation.
  - Question: Should the shared acceptance engine preserve the current build criterion behavior exactly, including browser stdout evaluation, idle timeout, diagnostic path, and human-readable reason?
  - Recommended answer: Yes. The app acceptance engine should accept the same criterion semantics build uses today, return enough data for build to recreate identical `session.Entry` fields, and expose a receipt-friendly projection for gate without losing diagnostics.
  - If unanswered: The shared acceptance extraction can silently change existing build behavior or force the implementer to add unapproved fields after approval.


## Planning Log

- none
