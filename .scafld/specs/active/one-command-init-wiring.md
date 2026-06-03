---
spec_version: '2.0'
task_id: one-command-init-wiring
created: '2026-06-02T10:50:31Z'
updated: '2026-06-02T15:15:25Z'
status: review
harden_status: needs_revision
size: medium
risk_level: medium
---

# One-command init wiring: keypair, MCP, Stop hook, CI, trusted-keys

## Current State

Status: review
Current phase: final
Next: review
Reason: build completed; ready for review
Blockers: none
Allowed follow-up command: `scafld review one-command-init-wiring`
Latest runner update: 2026-06-02T15:15:25Z
Review gate: not_started

## Summary

Make `scafld init` wire the whole accountability surface in one command so the host agent gains the `scafld_gate` affordance and the environment does the enforcing. Init generates the on-host ed25519 keypair (private key outside the repo at chmod 600, public key into a committed allowlist), upserts the gate MCP tool into an existing or new `.mcp.json`, installs a Claude Code skill plus `/scafld-gate` slash command, upserts a Stop hook into an existing or new `.claude/settings.json`, writes a CI job that runs `scafld verify` after spec 5 exists, and rewrites the agent contract from "route every change through the lifecycle" to "work however you want, then gate; CI verifies". This is adoption wiring only. The gate, receipt, and verify internals are owned by specs 4 and 5.

## Objectives

- Add an `initwire` composition step to `scafld init` that lays down every accountability artifact a host agent and CI need, idempotently, using merge/upsert installers for JSON config and the existing `corebundle` install machinery for standalone assets.
- Generate an on-host ed25519 keypair: private key written OUTSIDE the repo under the user config dir at mode 0600, public key appended idempotently into a committed `.scafld/trusted-keys.json` allowlist through `internal/core/trust`.
- Register `scafld_gate` in `.mcp.json` by preserving unrelated user MCP entries and upserting only the scafld gate server/tool entry.
- Install a Claude Code skill (`.claude/skills/scafld-gate/SKILL.md`) and `/scafld-gate` slash command whose descriptions headline independence and the signed receipt.
- Install a Stop hook in `.claude/settings.json` by preserving unrelated user settings/hooks and upserting only the scafld Stop hook that BLOCKS turn-end while an open gate or unsatisfied receipt exists, speaking only through its unstructured `reason` field.
- Write a CI workflow that runs `scafld verify <receipt> --target <commit-ish>` as the only hard merge wall, and make this phase depend on `ci-verify-merge-gate` being present.
- Rewrite `AGENTS.md` and `CLAUDE.md` (root assets and embedded `corebundle` assets) from lifecycle-routing to gate-and-verify.

## Scope

- In scope: a new `internal/adapters/corebundle/initwire.go` (and `initwire_test.go`) that installs the keypair, merges `.mcp.json`, installs `.claude/` skill + slash command, merges Stop hook settings, installs the CI workflow, and maintains `.scafld/trusted-keys.json`, called from `initcmd.Run`.
- In scope: keypair generation using `crypto/ed25519`, private key persisted outside the workspace at chmod 0600, public key allowlist committed in-repo using `internal/core/trust`.
- In scope: embedding default `.mcp.json`, Claude Code skill/slash/Stop-hook, and CI workflow templates as new files under `internal/adapters/corebundle/assets/`, while `.mcp.json` and `.claude/settings.json` installation uses merge-specific upsert helpers when live files already exist.
- In scope: rewriting the agent contract assets and the live root `AGENTS.md` / `CLAUDE.md`.
- In scope: `.gitignore` entries so any repo-local private-key reference/path is ignored and `.scafld/trusted-keys.json` is explicitly unignored/committed.
- Out of scope: the gate/receipt/verify internals. `scafld_gate`'s snapshot, acceptance run, independent reviewer, and signed receipt body are owned by `host-gate-and-receipt` (spec 4). `scafld verify`'s receipt validation and merge verdict are owned by `ci-verify-merge-gate` (spec 5). This spec only registers and references them.
- Out of scope: the evidence sandbox, tree fingerprint, and reviewer isolation (`evidence-control-sandbox`, `commit-free-tree-fingerprint`, `context-isolated-reviewer`); init wiring consumes those through the gate, it does not build them.
- Out of scope: signing keys with hosted infrastructure or any network key exchange; the trust model is on-host local ed25519, GPG-signed-commit mental model.

## Dependencies

- `host-gate-and-receipt` (spec 4): defines the `scafld_gate` MCP tool and CLI subcommand this wiring registers, the receipt schema CI checks, and `internal/core/trust`. The `.mcp.json` entry and slash command point at that verb; trusted keys are written through the shared trust package.
- `ci-verify-merge-gate` (spec 5): defines `scafld verify <receipt> --target <commit-ish>`, which the CI workflow written here invokes as the merge wall. This is a hard merge prerequisite for the CI workflow phase; this spec must not land a workflow that calls a missing command.
- Real repo facts: `scafld init` runs through `internal/adapters/cli/initcmd/init.go`, which already calls `corebundle.Init`, `corebundle.InitAgentDocs`, and `corebundle.InitGitignore`; the new `corebundle.InitWire` step folds in alongside these.
- The existing MCP stdio server lives at `internal/platform/mcpsubmit/server.go` and is adapted per-tool (see `internal/adapters/mcp/reviewsubmit/server.go`); the gate MCP server from spec 4 is exposed the same way, so `.mcp.json` invokes the gate stdio subcommand rather than a bespoke server.
- `corebundle` installs embedded assets with `writeManagedFile` (skip-if-exists, atomic write, mode by path) in `bundle.go`; the new standalone artifacts reuse that path. `.mcp.json` and `.claude/settings.json` require merge/upsert helpers because skip-if-exists would fail to wire real workspaces that already have config. `.scafld/trusted-keys.json` is parsed and updated through `internal/core/trust`.

## Assumptions

- The host agent reaches `scafld_gate` over MCP and never touches the private key, keeping the receipt unforgeable by the agent (per the trust model).
- The user config dir for the private key is resolvable via `os.UserConfigDir` (or `os.UserHomeDir` fallback), giving a stable on-host path outside any single repo.
- Re-running `scafld init` must not regenerate or overwrite an existing keypair; key generation is create-if-absent.
- Re-running `scafld init` must not duplicate public keys, MCP entries, Stop hooks, workflow entries, or overwrite unrelated user config.
- Claude Code reads `.mcp.json`, `.claude/skills/*/SKILL.md`, slash commands, and `.claude/settings.json` hooks from the workspace root.
- The Stop hook can only emit an unstructured `reason`; it cannot inject `additionalContext`, so the structured fix-loop stays on the `scafld_gate` MCP tool and the hook is only the wall.

## Trusted Keys Ownership

`internal/core/trust` is the single owner of `.scafld/trusted-keys.json`: `TrustedKeys`, `TrustedKey`, parser/serializer, `KeyIDFromRawEd25519PublicKey`, duplicate detection, revocation checks, and key-id/public-key consistency. Init appends the generated public key through this package only if its `key_id` is absent, preserves existing non-revoked and revoked keys, rejects malformed files instead of rewriting them, and never stores the private key or a private-key path in `trusted-keys.json`. Verify reads the same file through the same package; init must not define a parallel trust schema.

## Config Merge Contract

- `.mcp.json`: parse as JSON object, preserve unrelated servers/tools/settings, and upsert the scafld gate entry by stable key. If an existing scafld key points to incompatible command/args, write a `.bak` conflict copy and replace only the scafld-owned entry.
- `.claude/settings.json`: parse as JSON object, preserve unrelated settings and hooks, and upsert one scafld Stop hook by stable id/name/command. Do not duplicate hooks on repeated init.
- Standalone skill, slash command, CI workflow, and agent-doc assets keep create-if-absent/managed-file behavior unless explicitly overwritten by the user.

## Touchpoints

- internal/adapters/cli/initcmd/init.go
- internal/adapters/corebundle/initwire.go (new)
- internal/adapters/corebundle/initwire_test.go (new)
- internal/adapters/corebundle/bundle.go
- internal/adapters/corebundle/jsonmerge.go (new; merge/upsert helpers for `.mcp.json` and `.claude/settings.json`)
- internal/core/trust/trusted_keys.go (shared trusted-key schema/parser/key-id package consumed by init and verify)
- internal/adapters/corebundle/gitignore.go
- internal/adapters/corebundle/assets/agentdocs/AGENTS.md
- internal/adapters/corebundle/assets/agentdocs/CLAUDE.md
- internal/adapters/corebundle/assets/initwire/mcp.json (new)
- internal/adapters/corebundle/assets/initwire/claude/skills/scafld-gate/SKILL.md (new)
- internal/adapters/corebundle/assets/initwire/claude/commands/scafld-gate.md (new)
- internal/adapters/corebundle/assets/initwire/claude/settings.json (new)
- internal/adapters/corebundle/assets/initwire/ci/scafld-verify.yml (new)
- AGENTS.md
- CLAUDE.md

## Risks

- The Stop hook is Claude-Code-specific and leaky: it does not fire in subagent, piped, Codex, or Gemini contexts, so it cannot be the hard wall. Stated plainly in the contract; CI (`scafld verify`) is the only hard wall.
- A determined operator can extract their own private key and self-sign; the trust model records this in the receipt rather than cryptographically preventing it. Init must not imply otherwise.
- Overwriting an existing keypair on re-init would silently invalidate prior receipts; key generation must be create-if-absent and covered by a test.
- Committing the private key would break the entire trust model; the gitignore contract and the out-of-repo path must both guard against it.
- Skipping existing `.mcp.json` or `.claude/settings.json` would make init appear successful while leaving the gate unreachable; mitigated by merge/upsert helpers and existing-file tests.

## Acceptance

Profile: strict

Validation:
- [x] `v1` validate spec - This spec validates clean.
  - Command: `go run ./cmd/scafld validate one-command-init-wiring`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-32

## Phase 1: Init wiring composition and keypair

Status: completed
Dependencies: none

Objective: Add `corebundle.InitWire`, called from `initcmd.Run`, that installs the accountability artifacts in one pass and generates the on-host ed25519 keypair create-if-absent. The private key is written outside the repo under the user config dir at mode 0600; the public key is appended idempotently into committed `.scafld/trusted-keys.json` using `internal/core/trust`. Reuse the existing `writeManagedFile` install path and `atomicfile.Write` for standalone in-repo artifacts, and add merge/upsert helpers for JSON config files; use `crypto/ed25519` for key material. Composition lives in `corebundle.InitWire`; the root CLI dispatch remains thin.

Changes:
- internal/adapters/corebundle/initwire.go - new `InitWire(ctx, root) (Result, error)` that generates the keypair create-if-absent (private key at `os.UserConfigDir`/scafld/keys at 0600, public key into committed `.scafld/trusted-keys.json`) and installs the embedded `assets/initwire` tree; single responsibility, no file-write logic duplicated.
- internal/adapters/corebundle/jsonmerge.go - new helpers for idempotent JSON merge/upsert of `.mcp.json` and `.claude/settings.json`, preserving unrelated user config and backing up conflicts before replacing scafld-owned entries.
- internal/core/trust/trusted_keys.go - shared trusted-key schema/parser/key-id package used to append the generated public key; no trusted-key parsing is implemented in corebundle.
- internal/adapters/cli/initcmd/init.go - call `corebundle.InitWire(ctx, result.Root)` and `result.Merge(...)` alongside the existing bundle/agent-docs/gitignore steps; handler stays thin, composition lives in `corebundle`.
- internal/adapters/corebundle/gitignore.go - extend `scafldGitignoreBlock` to ignore the on-host key path reference and to keep `.scafld/trusted-keys.json` committed (un-ignored).
- internal/adapters/corebundle/initwire_test.go - assert keypair is create-if-absent (second init does not overwrite), private key mode is 0600 and outside `root`, public key lands in committed `.scafld/trusted-keys.json` through `internal/core/trust` with no duplicate, existing `.mcp.json` and `.claude/settings.json` are merged without losing unrelated entries, and standalone embedded artifacts are installed idempotently.

Acceptance:
- [x] `ac1_1` initwire package compiles and passes - corebundle tests are green with the new wiring.
  - Command: `go test ./internal/adapters/corebundle/`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-6
- [x] `ac1_2` init handler invokes InitWire - the thin init handler composes the new step.
  - Command: `rg -n 'corebundle.InitWire' internal/adapters/cli/initcmd/init.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-7
- [x] `ac1_3` root cli dispatch stays thin - the root CLI dispatch table remains thin after adding init wiring; corebundle tests cover init subpackage behavior.
  - Command: `go test ./internal/core/trust ./internal/adapters/corebundle/ -run 'TrustedKeys|KeyID|InitWire'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-8

## Phase 2: Host affordances and Stop hook

Status: completed
Dependencies: phase1

Objective: Author and install the host affordance artifacts under `internal/adapters/corebundle/assets/initwire`: `.mcp.json` registering `scafld_gate` (invoking the gate stdio subcommand from spec 4, reusing the existing MCP stdio transport, not a new server), a Claude Code skill and `/scafld-gate` slash command whose descriptions headline independence and the signed receipt, and a `.claude/settings.json` Stop hook that blocks turn-end while an open gate exists and speaks only through its `reason` field. The `.mcp.json` and settings files are live-merged/upserted when they already exist; the slash command references the gate verb by name only; the gate behavior is owned by spec 4.

Changes:
- internal/adapters/corebundle/assets/initwire/mcp.json - new embedded template registering the `scafld_gate` MCP tool by invoking the gate stdio subcommand; reuses the existing `mcpsubmit` transport contract, defines no new server.
- internal/adapters/corebundle/assets/initwire/claude/skills/scafld-gate/SKILL.md - new skill whose description headlines independent reviewer plus signed receipt and points at the single `scafld_gate` verb.
- internal/adapters/corebundle/assets/initwire/claude/commands/scafld-gate.md - new `/scafld-gate` slash command mirroring the skill, headlining independence and receipt.
- internal/adapters/corebundle/assets/initwire/claude/settings.json - new Stop hook template that blocks turn-end while an open gate or unsatisfied receipt exists, with an honest `reason` noting CI is the hard wall; documents that it cannot inject additionalContext.

Acceptance:
- [x] `ac2_1` gate registered in mcp.json asset - the host tool router gains `scafld_gate`.
  - Command: `rg -n 'scafld_gate' internal/adapters/corebundle/assets/initwire/mcp.json`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-13
- [x] `ac2_2` skill and slash command headline independence and receipt - the affordance advertises the value, not the lifecycle.
  - Command: `rg -n -i 'independ|receipt' internal/adapters/corebundle/assets/initwire/claude/skills/scafld-gate/SKILL.md internal/adapters/corebundle/assets/initwire/claude/commands/scafld-gate.md`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-14
- [x] `ac2_3` Stop hook present in settings asset - the wall is installed.
  - Command: `rg -n 'Stop' internal/adapters/corebundle/assets/initwire/claude/settings.json`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-15
- [x] `ac2_4` existing JSON config is merged - existing `.mcp.json` and Claude settings are updated without losing unrelated user entries.
  - Command: `go test ./internal/adapters/corebundle/ -run 'MergeMCP|MergeClaudeSettings|InitWireExisting'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-16

## Phase 3: CI workflow after verify exists

Status: completed
Dependencies: phase2, ci-verify-merge-gate

Objective: Install the CI workflow only after `ci-verify-merge-gate` has landed the real `scafld verify <receipt> --target <commit-ish>` command. The workflow runs verify as the merge wall and passes an explicit target commit/ref supplied by CI. This phase must prove the command is registered before the workflow asset is considered complete; no placeholder verify shim is allowed.

Changes:
- internal/adapters/corebundle/assets/initwire/ci/scafld-verify.yml - new CI workflow running `scafld verify <receipt> --target <commit-ish>` as the merge wall.
- internal/adapters/corebundle/initwire.go - install the CI workflow through the existing managed-file path after the command availability check passes.
- internal/adapters/corebundle/initwire_test.go - assert the workflow references `--target` and init reports a clear prerequisite failure if `scafld verify` is unavailable in the built command registry.

Acceptance:
- [x] `ac3_1` verify command registered before CI asset - scafld verify exists before init installs a workflow that calls it.
  - Command: `rg -n '"verify"' internal/adapters/cli/cli.go && rg -n 'verify.*Run|verify\\.Run|cli/verify' internal/adapters/cli/cli.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-21
- [x] `ac3_2` CI workflow passes target - the hard wall invokes verify with an explicit target.
  - Command: `rg -n 'scafld verify|--target' internal/adapters/corebundle/assets/initwire/ci/scafld-verify.yml`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-22
- [x] `ac3_3` no placeholder verify - init does not install a fake verify shim.
  - Command: `rg -n 'placeholder|TODO.*verify|fake verify' internal/adapters/corebundle internal/adapters/cli`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-23

## Phase 4: Contract rewrite to gate-and-verify

Status: completed
Dependencies: phase3

Objective: Rewrite the agent contract from lifecycle-routing to gate-and-verify in both the embedded `corebundle` assets and the live root files. The new contract says: work however you want using the host agent's own engine, then call `scafld_gate` when you believe you are done; an independent reviewer grades the work and CI verifies the signed receipt. State the honest limits: the Stop hook is Claude-Code-specific and leaky in subagent, piped, Codex, and Gemini contexts, and CI is the only hard wall. Demote the plan/harden/approve/build/complete lifecycle to a human/CI section rather than the agent's default flow. Remove the "route every change through the lifecycle" framing.

Changes:
- internal/adapters/corebundle/assets/agentdocs/AGENTS.md - replace the lifecycle-first contract with the gate-and-verify contract: single `scafld_gate` verb for the agent, `scafld verify` for CI, lifecycle demoted to a human/CI section, honest Stop-hook and CI-wall limits stated.
- internal/adapters/corebundle/assets/agentdocs/CLAUDE.md - rewrite to point at `scafld_gate` and the `/scafld-gate` slash command as the default; drop the lifecycle "Default Flow".
- AGENTS.md - apply the same gate-and-verify rewrite to the live root contract.
- CLAUDE.md - apply the same gate-and-verify rewrite to the live root contract.

Acceptance:
- [x] `ac4_1` contract leads with scafld_gate - the agent surface is the single gate verb.
  - Command: `rg -n 'route every change through the lifecycle' AGENTS.md CLAUDE.md internal/adapters/corebundle/assets/agentdocs/`
  - Expected kind: `no_matches`
  - Status: pass
  - Evidence: output was empty
  - Source event: entry-28
- [x] `ac4_3` agent docs install path still passes - the contract rewrite does not break the embedded-doc merge.
  - Command: `go test ./internal/adapters/corebundle/ -run AgentDocs`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-29

## Rollback

- Revert `internal/adapters/cli/initcmd/init.go` to drop the `corebundle.InitWire` call; the init flow returns to bundle + agent-docs + gitignore only.
- Delete `internal/adapters/corebundle/initwire.go`, `initwire_test.go`, `jsonmerge.go`, and the `assets/initwire` tree. Leave `internal/core/trust` only if spec 4 or 5 has landed and consumes it; otherwise revert it too.
- Restore the prior `scafldGitignoreBlock` in `gitignore.go` and the prior `AGENTS.md` / `CLAUDE.md` (root and embedded) from git history.
- No persisted keypair migration is required; rollback leaves any already-generated on-host key untouched and unreferenced.

## Review

Status: not_started
Verdict: none

## Self Eval

- Reuses `corebundle` install machinery (`installTree`, `writeManagedFile`, `atomicfile.Write`) for standalone assets, adds targeted JSON merge/upsert where skip-if-exists would be wrong, and reuses the existing `mcpsubmit` transport rather than reimplementing a new MCP server.
- Keeps root CLI dispatch thin while composition lives in `corebundle.InitWire`; init subpackage behavior is verified by corebundle/initwire tests, not overclaimed through `TestCLIIsThin`.
- Holds the DRY fence against specs 4 and 5: this spec registers and references `scafld_gate`, `scafld verify <receipt> --target <commit-ish>`, and `internal/core/trust`, but does not implement their internals.
- States the honest limits (leaky Stop hook, extractable key, CI-only hard wall) in Risks and the contract rather than overclaiming.

## Deviations

- none

## Metadata

- created_by: scafld
- estimated_effort_hours: 6-10
- priority: p2

## Origin

Created by: scafld
Source: accountability-layer rebuild

## Harden Rounds

### round-1

Status: needs_revision
Started: 2026-06-02T11:35:17Z
Ended: 2026-06-02T11:35:17Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: Needs revision: the draft is architecturally reasonable, but approval should wait on config merge semantics, enforced ordering for `scafld verify`, and a concrete trusted-keys schema.

Checks:
- path audit
  - Grounded in: code:internal/adapters/cli/initcmd/init.go:14
  - Result: passed
  - Evidence: Declared existing touchpoints were found; future initwire paths are intentional new files.
- command audit
  - Grounded in: code:internal/adapters/cli/cli.go:52
  - Result: failed
  - Evidence: `scafld verify` is not currently registered. The draft names `ci-verify-merge-gate` as owner, but this must be enforced as build/merge ordering before installing a CI workflow that invokes it.
- scope/migration audit
  - Grounded in: spec_gap:scope
  - Result: failed
  - Evidence: Scope correctly fences gate/receipt/verify internals to specs 4 and 5, but allowlist schema and config merge behavior cross those boundaries and need executable contracts.
- acceptance timing audit
  - Grounded in: spec_gap:phases
  - Result: failed
  - Evidence: Phase greps do not prove `.mcp.json` or Claude settings are updated when those files already exist; token presence in embedded assets can pass while init remains ineffective in real workspaces.
- rollback/repair audit
  - Grounded in: spec_gap:acceptance
  - Result: passed
  - Evidence: Rollback covers dropping InitWire and deleting new assets; leaving on-host keys untouched is credible if the allowlist schema makes orphaned keys harmless.
- design challenge
  - Grounded in: spec_gap:task_contract
  - Result: failed
  - Evidence: The architectural move is justified as adoption wiring with CI as the hard wall. The blockers are under-specified merge semantics and cross-spec contracts, not the concept.

Issues:
- [high/blocks approval] `harden-1` question - Existing config files can prevent one-command init from wiring the gate and hook.
  - Status: open
  - Grounded in: code:internal/adapters/corebundle/bundle.go:98
  - Evidence: `writeManagedFile` skips existing files when overwrite is false. Phase 1 says to install the embedded initwire tree through `installTree`/`writeManagedFile`, while Phase 2 requires wiring root `.mcp.json` and `.claude/settings.json`. Existing user config would be skipped, so `scafld_gate` or the Stop hook may not be registered.
  - Recommendation: Add explicit JSON merge/update helpers that preserve unrelated user config and upsert the scafld MCP server/hook. Test existing-file cases.
  - Question: How should `InitWire` merge `.mcp.json` and `.claude/settings.json` when the workspace already has user config?
  - Recommended answer: Use merge-specific installers for `.mcp.json` and `.claude/settings.json`; use `installTree`/`writeManagedFile` for standalone skill, slash command, CI workflow, and other create-if-absent assets.
  - If unanswered: Default to merge-specific installers for JSON config files and keep `writeManagedFile` only for standalone create-if-absent assets.
- [high/blocks approval] `harden-2` question - The CI workflow can reference a command that is not present yet.
  - Status: open
  - Grounded in: code:internal/adapters/cli/cli.go:52
  - Evidence: The CLI registry currently lacks `verify`, while this task writes a CI workflow that runs `scafld verify`. Spec 5 owns the command, but this task can still land first unless the dependency is made executable.
  - Recommendation: Make the dependency enforceable: block this task until spec 5 is merged, or move CI workflow installation to spec 5. Do not add a placeholder `verify` shim.
  - Question: Is `ci-verify-merge-gate` a hard prerequisite for merging this task, or may this task install a workflow before `scafld verify` exists?
  - Recommended answer: Treat spec 5 as a hard prerequisite for the CI workflow phase and final merge of this task.
  - If unanswered: Default to requiring `ci-verify-merge-gate` to land before Phase 2/CI workflow installation, or move the workflow to spec 5.
- [high/blocks approval] `harden-3` question - The trusted key allowlist is a cross-spec contract but has no executable schema.
  - Status: open
  - Grounded in: spec_gap:task_contract
  - Evidence: This task creates `.scafld/trusted-keys.json`; spec 5 reads that allowlist and requires revocation handling, and spec 4 signs receipts with `{alg,key_id,sig}`. The draft does not specify JSON shape, public-key encoding, key_id derivation, or revocation representation.
  - Recommendation: Add the schema to this spec or reference a shared receipt/trust schema from spec 4/5. Include versioning, public-key encoding, deterministic key_id derivation, revocation representation, and a golden fixture acceptance check.
  - Question: What exact `.scafld/trusted-keys.json` schema should init write and CI verify read?
  - Recommended answer: Use `{"version":1,"keys":[{"key_id":"ed25519:<sha256-raw-public-key>","alg":"ed25519","public_key":"<base64-raw-public-key>","created_at":"<rfc3339>","revoked_at":null}]}`.
  - If unanswered: Default to a versioned JSON object with `keys[]`, `key_id`, `alg`, `public_key`, `encoding`, `created_at`, and `revoked_at`; derive `key_id` from SHA-256 over raw public-key bytes.

### round-2

Status: needs_revision
Started: 2026-06-02T13:16:16Z
Ended: 2026-06-02T13:16:16Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: Needs revision. The draft is mostly executable now, but Phase 3 still has one approval blocker: the acceptance intended to prove `scafld verify` exists can be satisfied by workflow text alone, so it does not enforce the spec 5 dependency.

Checks:
- path audit
  - Grounded in: code:internal/adapters/cli/initcmd/init.go:14
  - Result: passed
  - Evidence: Existing files verified: init command, corebundle install machinery, gitignore installer, root and embedded agent docs. New paths such as `internal/core/trust` and `assets/initwire` do not exist yet, matching the draft scope.
- command audit
  - Grounded in: code:internal/adapters/cli/cli.go:52
  - Result: failed
  - Evidence: The command list and handler map currently include lifecycle commands but no `verify`; command availability derives from the handler map.
- scope/migration audit
  - Grounded in: spec_gap:scope
  - Result: passed
  - Evidence: The draft fences gate/receipt/verify internals out while assigning wiring, JSON merge helpers, and shared trusted-key schema to this task.
- acceptance timing audit
  - Grounded in: spec_gap:ac3_1
  - Result: failed
  - Evidence: Phase 3 acceptance can pass from workflow text alone because it searches both `cli.go` and the workflow asset with one regex.
- rollback/repair audit
  - Grounded in: spec_gap:rollback
  - Result: passed
  - Evidence: Rollback covers dropping InitWire, deleting new initwire/jsonmerge assets, restoring gitignore and docs, and leaving on-host keys untouched.
- design challenge
  - Grounded in: spec_gap:phase3
  - Result: failed
  - Evidence: The design honestly separates Stop-hook affordance from CI hard wall; remaining risk is proving the CI command dependency without false positives.

Issues:
- [high/blocks approval] `harden-4` question - Phase 3 acceptance can falsely pass without a registered `verify` command.
  - Status: open
  - Grounded in: code:internal/adapters/cli/cli.go:52
  - Evidence: The current CLI registry has no `verify` command in `commands` or `commandHandlers`; `knownCommand` is driven by `commandHandlers`. The draft’s `ac3_1` searches both `cli.go` and the workflow asset, so it can pass if only the workflow contains `scafld verify`.
  - Recommendation: Make the dependency executable with a command-registry-specific acceptance, then keep the workflow check separate.
  - Question: How should Phase 3 prove `scafld verify` is registered before `InitWire` installs the CI workflow?
  - Recommended answer: Split `ac3_1`: require `verify` to be present in the CLI command list and handler map, then separately require the CI workflow to invoke `scafld verify` with `--target`.
  - If unanswered: Default to splitting `ac3_1` into a CLI-registration check and a separate workflow invocation check.


## Planning Log

- none
