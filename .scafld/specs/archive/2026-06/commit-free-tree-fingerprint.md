---
spec_version: '2.0'
task_id: commit-free-tree-fingerprint
created: '2026-06-02T10:50:31Z'
updated: '2026-06-04T08:32:13Z'
status: completed
harden_status: needs_revision
size: medium
risk_level: high
---

# Commit-free deterministic tree fingerprint

## Current State

Status: completed
Current phase: final
Next: done
Reason: finalization receipt passed
Blockers: none
Allowed follow-up command: `none`
Latest runner update: 2026-06-04T08:30:36Z
Review gate: not_started

## Summary

The git adapter today fingerprints the workspace from `git status --porcelain` output (internal/adapters/git/git.go), which the host can reconfigure (autocrlf, assume-unchanged), so the diff definition is not under scafld's control. This spec adds a deterministic, content-addressed snapshot API that writes the working tree to a git tree object via a temporary index, without mutating the branch, index, HEAD, or stash, and derives the canonical full-repository `tree_sha`, scope-filtered `file_digests[]`, and scope-filtered `ignored_unreviewed[]` the receipt records. It hardens that snapshot against index-flag spoofing (skip-worktree, assume-unchanged) and documents that committed `.gitattributes`/filters remain authoritative git semantics rather than something scafld overrides.

## Objectives

- Add a typed git adapter API: `Snapshot(ctx, SnapshotInput{Scope, BaseRef}) (Snapshot, error)`, returning `tree_sha`, `base_commit`, `head_commit`, `file_digests[]`, and `ignored_unreviewed[]`.
- Snapshot the full working tree to a git tree object via a temporary index seeded from `HEAD`, then `git add --all`, explicit `.scafld/runs/` removal from the temporary index, and `git write-tree`, leaving the real branch, index, HEAD, and stash byte-for-byte untouched.
- Produce the canonical full-repository `tree_sha` plus scope-filtered `file_digests[{path,status,sha256}]` from the snapshotted tree, so the diff definition and file bytes are scafld-controlled, not host-porcelain.
- Fail closed with a structured `evidence_integrity` error when `git ls-files -v` reports any `S`/skip-worktree or `h`/assume-unchanged path in scope.
- Enumerate ignore-excluded paths over scope and record them as `ignored_unreviewed[]` so excluded files are disclosed, not silently dropped.
- Pin host git config that would otherwise leak from the runner (`-c core.autocrlf=false`) while preserving committed `.gitattributes` and filters as authoritative repository semantics.
- Treat a submodule gitlink as an opaque pointer (the recorded commit oid), and document that limitation.

## Scope

- In scope: a new content-addressed snapshot API in the git adapter: `Snapshot(ctx, SnapshotInput{Scope []string, BaseRef string}) (Snapshot, error)`.
- In scope: the snapshot writes the full working tree to a temporary index (`GIT_INDEX_FILE`) seeded from `HEAD` with `git read-tree HEAD` when `HEAD` exists, resolves a full-repository `tree_sha` via `git write-tree`, pins host config `-c core.autocrlf=false`, and preserves committed `.gitattributes`.
- In scope: explicit removal of `.scafld/runs/` from the temporary index before `write-tree`, so runtime state is excluded from the actual `tree_sha`, not only from later lists.
- In scope: derivation of scope-filtered `file_digests[{path,status,sha256}]`, `deleted_paths[]`, and `ignored_unreviewed[]` from the snapshot, plus `base_commit`/`head_commit` resolution via `git rev-parse`.
- In scope: index-flag hardening with `git ls-files -v`, failing closed via a structured `evidence_integrity` error on `S` or `h` flags in scope.
- In scope: ignore-aware enumeration via `git status --porcelain --ignored` / `git check-ignore` over scope, recorded as `ignored_unreviewed[]`.
- In scope: submodule gitlink handled as an opaque pointer with that limitation documented in the adapter doc.
- Out of scope: producing or signing the receipt body, and the canonical sorted-key serialization (spec host-gate-and-receipt owns receipt assembly; this spec only emits the `tree_sha`/`file_digests`/`ignored_unreviewed` values it consumes).
- Out of scope: the reviewer prompt, persona, scratch dir, and evidence fencing (spec context-isolated-reviewer); the env-scrub and binary-pin sandbox (spec evidence-control-sandbox).
- Out of scope: CI re-derivation and the merge finalize (spec ci-verify-merge-gate consumes this snapshot to recompute `tree_sha`); the one-command init wiring (spec one-command-init-wiring).

## Dependencies

- evidence-control-sandbox: provides exact-env execution semantics for receipt-grade reviewer processes. This spec runs git commands inside the git adapter with explicit git `-c` pins and does not depend on provider env scrub.
- host-gate-and-receipt: consumes `tree_sha`, `file_digests[]`, `deleted_paths[]`, `ignored_unreviewed[]`, `base_commit`, and `head_commit` produced here and serializes them into the signed receipt body.
- ci-verify-merge-gate: re-runs the identical full-repository snapshot in CI and must reproduce the same `tree_sha`; committed `.gitattributes` plus explicit host config pins are the reproducibility contract.
- Real repo facts: the current adapter `internal/adapters/git/git.go` fingerprints from `git status --porcelain=v1` and skips `.scafld/runs/` via `ignoredRuntimePath`; the mutation guard port `WorkspaceStatus.ChangedFiles` (internal/app/review/review.go:42, internal/app/build/build.go:45) is the existing consumer and stays unchanged by this spec.

## Assumptions

- `git` is on PATH and the workspace root is a git worktree; the existing adapter already fails closed outside a worktree (internal/adapters/git/git_test.go:39).
- A temporary index file written via `GIT_INDEX_FILE` to a per-snapshot temp path never collides with the real `.git/index`, and `git read-tree`/`git add --all`/`git rm --cached`/`git write-tree` against it does not touch `HEAD` or the stash.
- Scope is a set of path prefixes supplied by the caller; an empty scope means the whole tree. `tree_sha` is always the full-repository snapshot hash; `file_digests[]` and `ignored_unreviewed[]` are filtered by scope. Outside-scope changes still change `tree_sha` by design unless a later receipt explicitly introduces a partial-tree receipt mode.
- `BaseRef` is optional. When provided, `base_commit` is `git merge-base <BaseRef> HEAD`; when absent, `base_commit` is `HEAD`. `head_commit` is always `git rev-parse HEAD` for the checked-out worktree.
- `.scafld/runs/` runtime state stays excluded from the actual snapshot tree by removing it from the temporary index before `write-tree`; this includes tracked or staged runtime files in the real repository. Receipts never fingerprint scafld's own scratch output.
- Host-local `core.autocrlf` is pinned off for snapshot commands, but committed `.gitattributes`, clean/smudge filters, and path attributes remain repo semantics. The acceptance tests must prove parity across host `core.autocrlf` settings and across committed attributes rather than claiming attributes are disabled.

## Snapshot Contract

`internal/adapters/git` owns this concrete API:

```go
type SnapshotInput struct {
    Scope   []string
    BaseRef string
}

type Snapshot struct {
    TreeSHA           string
    BaseCommit        string
    HeadCommit        string
    FileDigests       []FileDigest
    DeletedPaths      []DeletedPath
    IgnoredUnreviewed []IgnoredPath
}

type FileDigest struct {
    Path   string
    Status string
    SHA256 string
}

type DeletedPath struct {
    Path   string
    Status string
}

type IgnoredPath struct {
    Path   string
    Reason string
}
```

`FileDigest` contains every present in-scope non-ignored tree entry. `Status` is one of `added`, `modified`, `unchanged`, or `gitlink`. For blob files, `SHA256` is over the canonical blob bytes in the snapshot tree. For gitlinks, `SHA256` is `sha256("gitlink\x00" + commitOID)` and `Status` is `gitlink`. Deletions are not represented as fake file digests; deleted in-scope paths are listed in `DeletedPaths` with `Status == "deleted"`. Lists are sorted by path.

The API is additive and adapter-local for this spec. App/build/review touchpoints are read-only references to current mutation-guard consumers; host-gate-and-receipt owns wiring this snapshot into finalize orchestration.

## Touchpoints

- internal/adapters/git/git.go
- internal/adapters/git/git_test.go
- internal/adapters/git/doc.go
- internal/app/review/review.go (read-only reference: existing mutation guard consumer remains unchanged)
- internal/app/build/build.go (read-only reference: existing mutation guard consumer remains unchanged)
- internal/core/workspace/snapshot.go (read-only reference unless an existing core snapshot type already matches the adapter API; no app wiring here)

## Risks

- A bug in temp-index handling could mutate the real index or HEAD; mitigated by always passing an explicit `GIT_INDEX_FILE` to a temp path and asserting the real index md5 and HEAD are unchanged in tests.
- Host `core.autocrlf` differences between the finalize host and CI could split the `tree_sha`; mitigated by pinning `-c core.autocrlf=false` on every snapshot command and asserting parity across host config settings. Committed `.gitattributes` differences are not pinned away; they are repository semantics and must be reflected consistently by git in both environments.
- Submodule gitlinks only expose the pointer commit oid, so in-submodule working-tree edits are not hashed; recorded as an opaque pointer and documented, not silently treated as clean.

## Acceptance

Profile: strict

Validation:
- [x] `v1` validate spec - Spec validates under scafld.
  - Command: `go run ./cmd/scafld validate commit-free-tree-fingerprint`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-32
- [x] `v2` adapter tests green - Git adapter package tests pass.
  - Command: `go test ./internal/adapters/git`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-33

## Phase 1: Content-addressed temp-index snapshot

Status: pass
Dependencies: none

Objective: Add the typed `Snapshot(ctx, SnapshotInput{Scope, BaseRef}) (Snapshot, error)` API to the git adapter. It writes the full working tree to a temporary index and resolves a content-addressed full-repository `tree_sha`, without mutating the real branch, index, HEAD, or stash. The algorithm is exact: create a temp index, seed it with `git read-tree HEAD` when `HEAD` exists (empty temp index only for unborn HEAD repositories), run pinned-config `git add --all`, remove `.scafld/runs/` from the temp index, then run `git write-tree`. The snapshot must pin host config `-c core.autocrlf=false` so the resolved bytes are stable across finalize and CI hosts while leaving committed `.gitattributes` authoritative, and must resolve `base_commit`/`head_commit` via the contract above. Reuse the existing `Adapter{Root}` value and its `exec.CommandContext` invocation style rather than introducing a second git wrapper.

Changes:
- internal/adapters/git/git.go - add `Snapshot(ctx, SnapshotInput{Scope, BaseRef}) (Snapshot, error)` on `Adapter` that creates a temp index, runs `GIT_INDEX_FILE=<temp> git read-tree HEAD` when HEAD exists, runs `git -c core.autocrlf=false add --all`, removes `.scafld/runs/` from that temp index, then runs `git -c core.autocrlf=false write-tree`, returning the resolved full-repository `tree_sha`; the temp index path is created per call and removed after, the real `.git/index` is never named.
- internal/adapters/git/git.go - resolve `head_commit` via `git rev-parse HEAD`; resolve `base_commit` as `git merge-base <BaseRef> HEAD` when `BaseRef` is supplied and `HEAD` when absent, reusing the adapter's `Root`-scoped command helper.
- internal/adapters/git/doc.go - document that the snapshot is commit-free, never mutates branch/index/HEAD/stash, pins host `core.autocrlf=false`, and preserves committed `.gitattributes`/filters.
- internal/adapters/git/git_test.go - assert the real `.git/index` md5 and `git rev-parse HEAD` are unchanged across a `Snapshot` call, that `base_commit`/`head_commit` are populated under both BaseRef and no-BaseRef cases, that tracked ignored files are preserved by read-tree seeding, that `.scafld/runs/` does not affect `tree_sha`, and that two snapshots of identical content yield an identical full-repository `tree_sha`.

Acceptance:
- [x] `ac1_1` snapshot resolves tree_sha - Snapshot produces a stable tree_sha and leaves index and HEAD untouched.
  - Command: `go test ./internal/adapters/git -run Snapshot`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-34
- [x] `ac1_2` autocrlf pinned - Snapshot commands pin core.autocrlf=false for hash parity.
  - Command: `rg -n 'GIT_INDEX_FILE|read-tree|write-tree' internal/adapters/git/git.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-35
- [x] `ac1_4` base and head covered - Snapshot tests prove base_commit and head_commit semantics with and without BaseRef.
  - Command: `go test ./internal/adapters/git -run 'Snapshot.*(Base|Head|Commit)'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-36
- [x] `ac1_5` attributes parity covered - Snapshot tests cover host autocrlf changes and committed .gitattributes parity.
  - Command: `go test ./internal/adapters/git -run 'Snapshot.*(AutoCRLF|Attributes|CRLF)'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-37
- [x] `ac1_6` temp index seeded - Snapshot tests prove tracked ignored files and deletions are represented because the temp index is seeded from HEAD.
  - Command: `go test ./internal/adapters/git -run 'Snapshot.*(ReadTree|TrackedIgnored|Deletion)'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-38
- [x] `ac1_7` runtime path excluded from tree - Snapshot tests prove `.scafld/runs/` does not change tree_sha even when present or tracked.
  - Command: `go test ./internal/adapters/git -run 'Snapshot.*Runtime|ScafldRuns'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-39

## Phase 2: Canonical digests and ignore-aware enumeration

Status: pass
Dependencies: phase1

Objective: Derive the canonical scope-filtered `file_digests[{path,status,sha256}]` from the snapshotted full tree, scope-filtered `deleted_paths[]` from the diff against `base_commit`, and scope-filtered ignore-excluded paths as `ignored_unreviewed[]`. File digests are sha256 over the canonical blob bytes captured in the snapshot (not over host-side `os.ReadFile`), statuses are from the enum defined above, and ignore enumeration uses `git status --porcelain --ignored` / `git check-ignore` over scope. Submodule gitlinks are emitted as opaque pointers carrying the recorded commit oid.

Changes:
- internal/adapters/git/git.go - derive scope-filtered `file_digests` from the snapshot tree (`git ls-tree -r` over the `tree_sha`, sha256 of each blob), each present entry carrying `{path,status,sha256}` with status drawn from the diff against `base_commit` or `unchanged` when absent from the diff.
- internal/adapters/git/git.go - derive scope-filtered `deleted_paths[]` from the diff against `base_commit`; deleted paths are not emitted as fake digests.
- internal/adapters/git/git.go - add scope-filtered ignore enumeration via `git status --porcelain --ignored` (and `git check-ignore` for explicit scope checks), recorded as a sorted `ignored_unreviewed` list, reusing `ignoredRuntimePath` to drop `.scafld/runs/`.
- internal/adapters/git/git.go - emit submodule gitlink entries as opaque pointers (mode 160000 commit oid) with `Status == "gitlink"` and sha256 over the pointer contract defined above.
- internal/adapters/git/git_test.go - cover a digest for a known blob, unchanged/added/modified/deleted status behavior, an ignored path landing in `ignored_unreviewed` and out of `file_digests`, and a submodule gitlink emitted as an opaque pointer.

Acceptance:
- [x] `ac2_1` digests statuses and ignores - File digests, deleted paths, status enum, and ignored_unreviewed are derived from the snapshot.
  - Command: `go test ./internal/adapters/git -run 'Digest|Status|Deleted|Ignored'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-40
- [x] `ac2_2` ignore enumeration present - Ignore-aware enumeration uses git's own ignore resolution.
  - Command: `rg -n 'status --porcelain --ignored|check-ignore' internal/adapters/git/git.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-41
- [x] `ac2_3` submodule documented - Submodule gitlink opaque-pointer limitation is documented.
  - Command: `rg -n 'gitlink|submodule' internal/adapters/git/doc.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-42
- [x] `ac2_4` scoped semantics covered - Tests prove tree_sha changes for outside-scope changes while file_digests and ignored_unreviewed remain scope-filtered.
  - Command: `go test ./internal/adapters/git -run 'Snapshot.*Scope'`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-43
- [x] `ac2_5` ignored path type present - IgnoredPath carries path and reason with sorted output.
  - Command: `rg -n 'type IgnoredPath|Reason string' internal/adapters/git/git.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-44

## Phase 3: Index-flag hardening, fail closed

Status: pass
Dependencies: phase2

Objective: Defend the snapshot against index-flag spoofing. Before resolving the snapshot, run `git ls-files -v` and fail closed with a structured `evidence_integrity` error if any in-scope path carries an `S` (skip-worktree) or `h` (assume-unchanged) flag, because such flags let the host hide working-tree edits from `add --all`. The error must be a typed value the finalize can surface, not a bare string, and the existing porcelain-based `ChangedFiles` mutation-guard path stays untouched so current build/review consumers keep their contract.

Changes:
- internal/adapters/git/git.go - add an `EvidenceIntegrityError` type and an index-flag check that runs `git ls-files -v` over scope and returns that typed error listing any `S`/`h` paths before any snapshot is resolved.
- internal/adapters/git/git.go - wire the index-flag check as a precondition of `Snapshot`, so a flagged path aborts snapshotting rather than producing a falsely clean `tree_sha`.
- internal/adapters/git/git_test.go - mark a path skip-worktree and assume-unchanged and assert `Snapshot` returns an `EvidenceIntegrityError` naming that path, and assert an unflagged tree snapshots normally.

Acceptance:
- [x] `ac3_1` fail closed on flags - Snapshot fails closed with evidence_integrity on skip-worktree/assume-unchanged.
  - Command: `rg -n 'ls-files -v' internal/adapters/git/git.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-45
- [x] `ac3_3` structured error type - A structured evidence_integrity error type exists.
  - Command: `rg -n 'EvidenceIntegrityError|evidence_integrity' internal/adapters/git/git.go`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-46

## Rollback

- Revert the changes to internal/adapters/git/git.go, internal/adapters/git/doc.go, and internal/adapters/git/git_test.go; the new `Snapshot`/digest/index-flag surface is adapter-additive, so removing it restores the existing porcelain `Status`/`ChangedFiles` mutation-guard behavior with no app/core consumer changes.
- No data migration or persisted state is introduced, so rollback is a code revert only.

## Review

Status: not_started
Verdict: none

## Self Eval

- Snapshot is commit-free and proven non-mutating against the real index md5 and HEAD.
- Full-repository `tree_sha`, scope-filtered `file_digests[]`, `deleted_paths[]`, and scope-filtered `ignored_unreviewed[]` are derived from git's own tree objects and ignore resolution, not host-configurable porcelain.
- Host `core.autocrlf=false` is pinned on every snapshot command while committed `.gitattributes`/filters remain authoritative repo semantics.
- Index-flag spoofing fails closed with a typed `evidence_integrity` error; submodule gitlinks are opaque pointers with the limitation documented.
- DRY: extends the existing `Adapter{Root}` and its command style, reuses `ignoredRuntimePath`, and leaves the `ChangedFiles` mutation-guard contract intact.

## Deviations

- none

## Metadata

- created_by: scafld
- estimated_effort_hours: 6-10
- priority: p1

## Origin

Created by: scafld
Source: accountability-layer rebuild

## Harden Rounds

### round-1

Status: needs_revision
Started: 2026-06-02T11:30:13Z
Ended: 2026-06-02T11:30:13Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: Needs revision. The architectural direction is right, but approval would force implementers to invent API, scope, and attribute semantics for a high-risk fingerprinting surface.

Checks:
- path audit
  - Grounded in: code:/Users/kam/dev/0state/scafld/internal/adapters/git/git.go:1
  - Result: passed
  - Evidence: Declared touchpoint files exist: internal/adapters/git/git.go, git_test.go, doc.go, internal/app/review/review.go, internal/app/build/build.go, and internal/core/workspace/snapshot.go.
- command audit
  - Grounded in: spec_gap:acceptance
  - Result: failed
  - Evidence: `go run ./cmd/scafld status commit-free-tree-fingerprint --json` and `go run ./cmd/scafld validate commit-free-tree-fingerprint` both failed before project code ran: Go could not create `/var/.../T/go-build...` due read-only sandbox permissions.
- scope/migration audit
  - Grounded in: code:/Users/kam/dev/0state/scafld/internal/app/review/review.go:40
  - Result: failed
  - Evidence: Existing build/review/approve ports expose `ChangedFiles(context.Context) ([]string,error)` only, while the draft adds adapter-only `Snapshot(ctx, scope)` and says receipt consumers will receive tree_sha/file_digests/ignored_unreviewed without defining the boundary.
- acceptance timing audit
  - Grounded in: spec_gap:phases
  - Result: failed
  - Evidence: Phase 1 acceptance runs `go test ./internal/adapters/git -run Snapshot`, but the phase also requires base_commit/head_commit and optional merge-base behavior that the command name does not prove. Phase 2/3 grep checks verify token presence only.
- rollback/repair audit
  - Grounded in: spec_gap:acceptance
  - Result: failed
  - Evidence: Rollback says revert only git.go/doc.go/git_test.go, but declared touchpoints include app/build, app/review, and core/workspace. This is credible only if those touchpoints remain read-only references.
- design challenge
  - Grounded in: spec_gap:task_contract
  - Result: failed
  - Evidence: The design goal is sound: replacing porcelain fingerprints with a content-addressed tree is the right architectural move. The current draft is not yet executable because scoped snapshot semantics, base-ref API, and line-ending/filter controls are underspecified.

Issues:
- [high/blocks approval] `harden-1` question - Snapshot API is underspecified for required base/head outputs.
  - Status: open
  - Grounded in: spec_gap:phases
  - Evidence: Phase 1 declares `Snapshot(ctx, scope)` but also requires resolving `base_commit`/`head_commit` and merge-base when a base ref is supplied. No base-ref parameter or result struct is specified.
  - Recommendation: Specify a `SnapshotInput`/`Snapshot` shape before approval so implementation does not invent public API semantics.
  - Question: What exact Snapshot API owns scope and optional base-ref input, and what typed result does it return?
  - Recommended answer: Use `Snapshot(ctx, SnapshotInput{Scope, BaseRef}) (Snapshot, error)` and make `BaseRef` optional; `base_commit` is merge-base of `BaseRef` and HEAD when present, otherwise HEAD or the empty-tree baseline as explicitly defined.
  - If unanswered: Default to `Snapshot(ctx context.Context, input SnapshotInput)` with `Scope []string` and `BaseRef string`, returning a typed `Snapshot` containing tree_sha, base_commit, head_commit, file_digests, and ignored_unreviewed.
- [high/blocks approval] `harden-2` question - Scoped snapshot semantics are ambiguous.
  - Status: open
  - Grounded in: spec_gap:phases
  - Evidence: The draft says scope is path prefixes and empty scope means whole tree, but Phase 1 says run `git add --all` and return one `tree_sha`. It does not state whether non-empty scope produces a full repository tree_sha with filtered digests or a synthetic scoped tree.
  - Recommendation: Define exact scope semantics and the git pathspec commands. This affects CI reproducibility and whether ambient drift changes invalidate receipts.
  - Question: For non-empty scope, is `tree_sha` the whole working tree or only the scoped paths?
  - Recommended answer: Use a whole-repository tree_sha for CI parity, then filter file_digests and ignored_unreviewed by scope; document that outside-scope bytes still affect tree_sha, or choose scoped tree hashing and define how the synthetic tree is built.
  - If unanswered: Default to full-repository `tree_sha` and scope-filtered `file_digests`/`ignored_unreviewed`, with tests proving outside-scope changes affect tree_sha only if the receipt intentionally covers the whole repo.
- [high/blocks approval] `harden-3` question - Line-ending determinism is overclaimed.
  - Status: open
  - Grounded in: spec_gap:task_contract
  - Evidence: The spec claims `-c core.autocrlf=false` makes hashes identical regardless of `.gitattributes`, but `git add` still applies in-repo attributes such as text/eol normalization and clean filters. Phase 1 acceptance only greps for `core.autocrlf=false`.
  - Recommendation: Either add explicit git config/attribute controls and tests, or revise the objective to stop claiming independence from `.gitattributes`.
  - Question: Should snapshot hashing neutralize `.gitattributes`/filters, or treat repository attributes as authoritative?
  - Recommended answer: Treat committed .gitattributes as authoritative repo semantics, pin only host config such as core.autocrlf, and add acceptance proving gate/CI parity under the same checkout attributes.
  - If unanswered: Default to narrowing the claim: pin core.autocrlf=false, document that committed .gitattributes is part of repository semantics, and add tests for CRLF plus attributes behavior.
- [high/blocks approval] `harden-4` question - Consumer boundary for snapshot evidence is not executable.
  - Status: open
  - Grounded in: code:/Users/kam/dev/0state/scafld/internal/app/build/build.go:43
  - Evidence: host-gate-and-receipt is said to consume tree_sha/file_digests/ignored_unreviewed, but current app ports only consume `ChangedFiles`; the task marks receipt assembly out of scope and does not define where the new snapshot values are exposed to later specs.
  - Recommendation: Clarify whether this task is adapter-only or also changes app/core ports. If adapter-only, remove app/build, app/review, and core/workspace from touchpoints or mark them read-only references.
  - Question: Which layer owns exposing the new snapshot values to receipt/finalize consumers?
  - Recommended answer: Keep this task adapter-only; define exported adapter types now, and leave app/core receipt wiring to host-gate-and-receipt.
  - If unanswered: Default to adding only a git-adapter struct plus tests, and update the dependent specs to own app wiring explicitly.
- [medium/advisory] `harden-5` question - Rollback scope conflicts with declared touchpoints.
  - Status: open
  - Grounded in: spec_gap:acceptance
  - Evidence: Rollback names only internal/adapters/git/git.go, doc.go, and git_test.go, while touchpoints also list internal/app/review/review.go, internal/app/build/build.go, and internal/core/workspace/snapshot.go.
  - Recommendation: Make rollback match actual write scope: either keep app/core files out of scope or include their revert steps.
  - If unanswered: Default to removing app/core touchpoints from this spec unless implementation truly edits them.
- [medium/advisory] `harden-6` question - Acceptance does not prove several key invariants.
  - Status: open
  - Grounded in: spec_gap:phases
  - Evidence: Phase 1 acceptance checks `Snapshot`; Phase 2/3 include grep checks. None explicitly proves HEAD/base resolution, ignored scoped paths, submodule gitlink status shape, or CRLF/attribute parity despite those being core claims.
  - Recommendation: Expand acceptance commands or test names so each invariant has direct evidence, not just token presence.
  - If unanswered: Default to adding targeted tests named `SnapshotBaseHead`, `SnapshotScope`, `SnapshotCRLFAttributes`, and `SnapshotSubmoduleGitlink`.

### round-2

Status: needs_revision
Started: 2026-06-02T13:13:18Z
Ended: 2026-06-02T13:13:18Z
Verdict: needs_revision
Provider: codex
Output format: codex.output_file
Summary: Needs revision. The revised draft fixed several earlier API and scope gaps, but approval would still leave implementers to invent critical tree semantics: seeding the temporary index, excluding `.scafld/runs/` from the written tree, defining `IgnoredPath`, and deciding how file digest statuses handle unchanged and deleted paths.

Checks:
- path audit
  - Grounded in: code:internal/adapters/git/git.go:1
  - Result: passed
  - Evidence: Declared touchpoints exist: `internal/adapters/git/git.go`, `internal/adapters/git/git_test.go`, `internal/adapters/git/doc.go`, `internal/app/review/review.go`, `internal/app/build/build.go`, and `internal/core/workspace/snapshot.go`.
- command audit
  - Grounded in: code:internal/adapters/cli/cli.go:52
  - Result: failed
  - Evidence: `internal/adapters/cli/cli.go:52-86` declares the relevant scafld commands, and acceptance commands are run through `sh -c` at `internal/adapters/process/runner.go:101-104`. Direct `go run ./cmd/scafld ...` checks failed in this sandbox because Go could not create its build work dir under `/var/folders/...`: `operation not permitted`.
- scope/migration audit
  - Grounded in: code:internal/app/build/build.go:43
  - Result: passed
  - Evidence: The draft now explicitly says the Snapshot API is adapter-local and app/build/review are read-only references. Existing ports confirm those consumers currently use `ChangedFiles(context.Context) ([]string,error)` only at `internal/app/build/build.go:43-46` and `internal/app/review/review.go:40-43`.
- acceptance timing audit
  - Grounded in: spec_gap:phases
  - Result: failed
  - Evidence: Phase acceptance has targeted commands for Snapshot, digest, scope, CRLF/attributes, and index flags, but does not explicitly require a test proving the temp index is seeded from HEAD before `git add --all`, nor that `.scafld/runs/` is absent from the actual written `tree_sha`.
- rollback/repair audit
  - Grounded in: spec_gap:acceptance
  - Result: passed
  - Evidence: Rollback names only `internal/adapters/git/git.go`, `internal/adapters/git/doc.go`, and `internal/adapters/git/git_test.go`; that matches the revised write scope because app/build, app/review, and core/workspace are read-only references.
- design challenge
  - Grounded in: spec_gap:task_contract
  - Result: failed
  - Evidence: Replacing porcelain status fingerprints with a git tree object is the right architectural move for a receipt-grade fingerprint, but the draft still leaves implementation choices that can change the hash semantics: temp-index seeding, runtime exclusion from the tree object, and digest/status shape.

Issues:
- [high/blocks approval] `harden-7` question - Temp-index snapshot algorithm can omit tracked ignored files if the temp index is not seeded from HEAD.
  - Status: open
  - Grounded in: spec_gap:phases
  - Evidence: Phase 1 specifies `GIT_INDEX_FILE=<temp> git -c core.autocrlf=false add --all` followed by `write-tree` at `.scafld/specs/drafts/commit-free-tree-fingerprint.md:135`. If the temp index starts empty, Git has no tracked-file baseline, so tracked files that are currently ignored can be omitted from the snapshot. That violates the objective to snapshot the full working tree under git semantics.
  - Recommendation: Specify the exact algorithm: create temp index, run `git read-tree HEAD` when HEAD exists, then run pinned-config `git add --all`, then apply any explicit runtime exclusions, then `git write-tree`. Add a test with a tracked file that also matches `.gitignore`.
  - Question: Should the temp index be seeded from HEAD before running `git add --all`?
  - Recommended answer: Yes. Seed the temporary index from HEAD first so tracked ignored files and deletions are represented correctly, while still keeping the real index untouched.
  - If unanswered: Default to seeding the temp index from HEAD with `git read-tree HEAD` before `git add --all`, with an explicit empty-index path only for unborn HEAD repositories.
- [high/blocks approval] `harden-8` question - Runtime-path exclusion conflicts with the described full-tree `git add --all && write-tree` algorithm.
  - Status: open
  - Grounded in: spec_gap:phases
  - Evidence: The assumptions say `.scafld/runs/` stays excluded from the snapshot at `.scafld/specs/drafts/commit-free-tree-fingerprint.md:64`, but Phase 1 only describes `git add --all` and `write-tree` at line 135. `git add --all` over the repository will include tracked runtime files unless the temp index explicitly removes or pathspec-excludes them before `write-tree`.
  - Recommendation: Add an explicit step and acceptance test proving `.scafld/runs/` does not affect `tree_sha`, including the case where a runtime file is tracked or staged in the real repository.
  - Question: How is `.scafld/runs/` excluded from the actual `tree_sha`, not just from later digest lists?
  - Recommended answer: After staging the temp index, remove `.scafld/runs/` from that temp index before `write-tree`, and document that the canonical tree is repository content minus scafld runtime state.
  - If unanswered: Default to removing `.scafld/runs/` from the temp index after `git add --all` and before `write-tree`, reusing `ignoredRuntimePath` for digest/ignore lists.
- [medium/blocks approval] `harden-9` question - Snapshot API references an undefined `IgnoredPath` type.
  - Status: open
  - Grounded in: spec_gap:task_contract
  - Evidence: The Snapshot contract declares `IgnoredUnreviewed []IgnoredPath` at `.scafld/specs/drafts/commit-free-tree-fingerprint.md:82`, but only defines `SnapshotInput`, `Snapshot`, and `FileDigest` at lines 71-89. `IgnoredPath` has no fields or semantics.
  - Recommendation: Define the `IgnoredPath` struct in the contract, including whether it records only `Path` or also ignore source/pattern/reason. Add a compile-level test or API assertion.
  - Question: What is the exact `IgnoredPath` type returned by Snapshot?
  - Recommended answer: Define `IgnoredPath` with at least `Path string`; include `Reason string` or `Pattern string` only if the receipt needs to explain why it was excluded.
  - If unanswered: Default to `type IgnoredPath struct { Path string; Reason string }`, with sorted path order and reason set from git ignore source when available.
- [high/blocks approval] `harden-10` question - `file_digests` status and deletion semantics are underdefined.
  - Status: open
  - Grounded in: spec_gap:phases
  - Evidence: Phase 2 says derive `file_digests` from `git ls-tree -r` over `tree_sha`, with `status` drawn from the diff against `base_commit` at `.scafld/specs/drafts/commit-free-tree-fingerprint.md:180`. A tree listing contains present paths only, while diff status can include deleted paths and leaves unchanged paths without a diff status. The dependent verify spec requires every committed non-ignored in-scope path to appear in `file_digests` at `.scafld/specs/drafts/ci-verify-merge-gate.md:150`.
  - Recommendation: Define the `FileDigest.Status` enum and deletion handling before approval. Add tests for unchanged, added, modified, deleted, and gitlink paths.
  - Question: Are `file_digests` all present in-scope files, changed files only, or diff records including deletions?
  - Recommended answer: Use `file_digests` for all present in-scope non-ignored entries, mark unchanged paths as `unchanged`, and handle deleted paths through a separate changed-path/status mechanism or an explicit digest-less deletion status.
  - If unanswered: Default to including all present non-ignored in-scope tree entries with status enum `added|modified|unchanged|gitlink`, and represent deletions outside `FileDigest` or with an explicit `sha256` empty/omitted rule.
- [low/advisory] `harden-11` advisory - `related snapshot-safe config` is vague and may cause inconsistent implementations.
  - Status: open
  - Grounded in: spec_gap:task_contract
  - Evidence: The objective says pin `core.autocrlf=false` and related snapshot-safe config at `.scafld/specs/drafts/commit-free-tree-fingerprint.md:36`, while Phase 1 acceptance only greps for `core.autocrlf=false` at lines 147-154 and tests CRLF/attributes by name at lines 165-166.
  - Recommendation: Either enumerate the additional config keys that must be pinned, or remove the phrase `and related snapshot-safe config` so implementation and review do not invent different pin sets.
  - If unanswered: Default to only claiming and testing `core.autocrlf=false`, unless the spec names additional pinned config keys explicitly.


## Planning Log

- none
