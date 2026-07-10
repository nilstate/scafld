---
title: CLI Reference
description: Current scafld command surface
---

# CLI Reference

scafld is intentionally small. The binary teaches the same command surface to
humans, agents, wrappers, and package launchers:

```bash
scafld init
scafld config
scafld plan <task-id>
scafld harden <task-id>
scafld validate <task-id>
scafld approve <task-id>
scafld build <task-id>
scafld review <task-id>
scafld finalize [task-id]
scafld complete <task-id>
scafld fail <task-id>
scafld cancel <task-id>
scafld status <task-id>
scafld list
scafld report
scafld handoff <task-id>
scafld adapter codex|claude|gemini build|review <task-id>
scafld verify <receipt> --target <commit-ish>
scafld update
```

Global flags:

- `--root PATH`: operate on a specific workspace root.
- `--json`: emit a stable JSON envelope when the command supports it.
- `--version`: print the binary version.

## JSON Mode

Automation-relevant commands emit one envelope:

```json
{
  "ok": true,
  "command": "build",
  "result": {
    "task_id": "add-auth",
    "status": "active",
    "phase": "phase1",
    "passed": 0,
    "failed": 0,
    "next": "scafld handoff add-auth"
  }
}
```

Failures use the same shape with `ok: false` and an `error` object carrying
`code`, `message`, and `exit_code`.

The envelope and every command `result` use `snake_case` JSON keys. The
Markdown spec schema, session ledger, and CLI automation output therefore share
one public casing convention.

Exit codes:

- `0`: success
- `1`: generic runtime failure
- `2`: invalid command or flag
- `3`: validation or acceptance failure
- `4`: review gate failure
- `5`: cancelled context
- `6`: workspace discovery or bootstrap failure

## init

```bash
scafld init [--root PATH] [--json]
```

Bootstraps `.scafld/` in the workspace. It installs project-owned config,
creates spec/run directories, and installs managed core assets under
`.scafld/core/`. Project prompt overrides are optional.

`init` is deterministic. It does not ask an agent to infer project policy.

## config

```bash
scafld config [--root PATH] [--json]
```

Scans the workspace in read-only mode and writes
`.scafld/config.proposed.yaml`. The proposal contains cited evidence,
agent instructions, suggested invariant IDs, discovered validation commands,
and open questions.

`config` does not mutate `.scafld/config.yaml`. The operator or agent must
open the cited sources and copy only verified runtime policy into the real
config. Agent guidance belongs in `AGENTS.md`, `CLAUDE.md`, `.claude/rules`, or
project prompts rather than unsupported config fields.

## update

```bash
scafld update [--root PATH] [--json]
```

Refreshes managed `.scafld/core/` files from the bundled runtime. It also
refreshes existing `.scafld/prompts/*` copies only when the prompt manifest
proves they are unmodified defaults. Customized project prompts are skipped. It
refreshes root agent docs and renders generated `.scafld/config.yaml` into the
current strict runtime shape. Specs, runs, reviews, and local config are
preserved.

## plan

```bash
scafld plan <task-id> [--title TITLE] [--summary TEXT] [--size SIZE] [--risk RISK] [--command CMD] [--json]
```

Creates `.scafld/specs/drafts/<task-id>.md`. `--command` seeds the first
executable acceptance criterion. Existing drafts are not overwritten.

## harden

```bash
scafld harden <task-id> [--provider auto|codex|claude|gemini|command|local] [--json]
scafld harden <task-id> --mark-passed [--json]
```

Hardening is the pre-build adversarial pass. It attacks the draft before
approval: product goal, authority, ownership boundaries, halfway failure
repair, hidden cutovers, testable invariants, golden examples, and recovery
commands.

Without flags, `harden` appends a round, sets `harden_status: in_progress`, and
prints the active prompt from `.scafld/prompts/harden.md`, falling back to
`.scafld/core/prompts/harden.md` and then the built-in prompt. The current
state records a blocker until each required observation has a `Result` and
`Anchor`.

With `--mark-passed`, it verifies the latest round's harden observations and
`Anchor` citations, closes the round, and sets `harden_status: passed`.
Missing dimensions, invalid results, open blocking observations, and unresolved
citations keep the round open. Advisory observations stay recorded but do not
block approval.

With `--provider`, scafld delegates the harden round to a separate read-only
provider. The provider must submit one strict `HardenDossier` through the
structured submit channel. scafld derives `pass` or `needs_revision` from
dimension coverage and unresolved `blocks` observations. Non-blocking advisory
observations remain in the harden round as evidence, not as forced rework.
Provider transport, invalid dossier, or unverified anchor problems are recorded
as `harden_status: error`.

Accepted citation shapes are `Anchor: spec_gap:<field>`,
`Anchor: code:<path>:<line>`, and `Anchor: archive:<task-id>`.
Code citations must use an existing workspace-relative path and a real line
number. Line ranges are rejected; cite the single line that anchors the
evidence.

Required dimensions are `design`, `scope`, `path`, `command`, `timing`, and
`rollback`. Each observation must record `Result: clean`, `advisory`, `blocks`,
or `n/a` plus an `Anchor`. The design dimension is not a style preference: it
must challenge why the plan exists, which shared core/app contract owns the
behavior, and whether API/MCP/CLI/provider/docs surfaces stay light adapters
instead of separate implementations.

## validate

```bash
scafld validate <task-id> [--json]
```

Parses the Markdown spec into the normalized model and rejects malformed
lifecycle state, phase identity, harden state, duplicate criteria, or
non-executable acceptance criteria.

## approve

```bash
scafld approve <task-id> [--json]
```

Records approval in the session ledger, then moves the draft spec to
`.scafld/specs/approved/`. Approval is explicit operator action; it is not
implied by hardening.

## build

```bash
scafld build <task-id> [--json]
```

Runs the governed implementation loop. From `approved`, it activates the task,
captures the workspace baseline, opens the first phase, and points the agent at
`scafld handoff <task-id>`. It does not run future acceptance before the phase
has been implemented.

From `active` or `blocked`, `build` records evidence for the current phase. If
the phase passes, it opens the next phase. If the final phase and global
acceptance pass, it moves the task to `review`. Drafts, terminal specs, and
already-ready review specs are rejected.

Acceptance commands inherit the process environment plus `execution` overrides
from `.scafld/config.yaml` and `.scafld/config.local.yaml`. Use that config for
repo-wide toolchain setup such as rbenv shims instead of relying on interactive
shell startup. Acceptance commands default to a 300-second absolute timeout;
raise `execution.absolute_timeout_seconds` for legitimate slow project tools.
Set `execution.idle_timeout_seconds` only when an idle-output watchdog is useful
for the project.

Phase acceptance runs in order. If a phase blocks, later phase commands are not
run and the next command becomes `scafld handoff <task-id>` so the repair agent
gets the failed criterion, command, and evidence instead of a vague blocked
status.

## review

```bash
scafld review <task-id> [--provider auto|codex|claude|gemini|command|local] [--provider-command CMD] [--provider-binary PATH] [--model MODEL] [--review-scope PATH[,PATH...]] [--force] [--print-context] [--human-reviewed --reason TEXT] [--json]
```

`review` is the adversarial completion gate. Defaults come from
`.scafld/config.yaml` under `review.external`. Fresh workspaces use
`provider: auto`, which prefers the other installed agent when the current host
is detected, can use Gemini as another external challenger, and fails closed
when only the host provider is available. Without a detected host, the default
order is `codex`, then `claude`, then `gemini`. If no external provider is
available, the
command fails and tells the operator to install a provider, use
`--provider command`, or explicitly choose `--provider local` for development
smoke tests. Local verdicts cannot satisfy `complete`.

Provider modes:

- `auto`: choose an installed external challenger.
- `codex`: run Codex in read-only ephemeral mode with user config and
  execpolicy rules disabled for the review subprocess.
- `claude`: run Claude with session persistence, slash commands, and browser
  integration disabled; built-in tools are restricted to `Read`, `Grep`, and
  `Glob`, and the verdict must be submitted through the `submit_review` MCP
  tool.
- `gemini`: run Gemini CLI in plan/read-only mode with a temporary scafld MCP
  settings file; the verdict must be submitted through the `submit_review` MCP
  tool and final text is ignored.
- `command`: run a custom reviewer command; requires `--provider-command`.
  The command receives the review prompt on stdin and must emit one complete
  ReviewDossier JSON object on stdout. Progress belongs on stderr.
- `local`: deterministic pass-through provider for development and tests only;
  its verdict cannot satisfy `complete`.
- `--human-reviewed`: record an audited operator review instead of invoking a
  provider. `--reason` is required and is stored in the session ledger.

Provider-specific model defaults come from
`review.external.codex.model`, `review.external.claude.model`, and
`review.external.gemini.model`. `--provider`, `--provider-command`,
`--provider-binary`, and `--model` override config for one invocation.

`--review-depth`, `--max-findings`, and `--min-attack-angles` override the
review dossier budget for one run. Configured review passes are rendered into
the same provider brief; scafld still makes one provider invocation and records
one ReviewDossier. The accepted dossier must satisfy the budget: too few
`attack_log` entries or too many findings fails the attempt before any review is
recorded. For small diffs, keep the same gate but request a cheaper
blocker-focused review:

```bash
scafld review <task-id> --review-depth light --max-findings 4 --min-attack-angles 3
```

`--print-context` prints the exact deterministic review-context packet without
invoking a provider. Use it when an agent needs to see why a reviewer is
under-informed or why a gate is likely to block before spending a model run.

Each provider run starts with a leased `review_attempt`. An unexpired running
attempt blocks another review. If the process died and the lease has expired,
the next `scafld review` records the old attempt as `abandoned` and starts a new
one automatically. Provider failures are retryable. Review verdict failures are
not: repair from `scafld handoff`, run `scafld build` to record fresh evidence,
then run `scafld review` again. A current passing review is a no-op unless
`--force` is set.

scafld derives review scope from spec packages, impacted files, and phase
changes. Use `--review-scope` only when a dirty monorepo or workspace needs an
explicit path boundary:

```bash
scafld review email-contracts --review-scope api
scafld review email-contracts --review-scope api,cli/packages/mcp
```

The approval baseline is captured before task execution. Review compares the
current workspace to that baseline, reports task-scoped changes to the provider,
and includes changes outside declared scope as ambient workspace drift.
Unchanged baseline dirt and ambient drift are context, not findings by
themselves. Task-relevant files changed during review still fail closed;
unrelated workspace churn does not discard a valid review.

After a passing review, scafld seals the reviewed task material when it has a
scope. `status` and `complete` compare that scoped content digest, not the whole
worktree, so committing the reviewed files or having another agent touch
unrelated paths does not require a second review. If the reviewed material
changes, the next action becomes `scafld review <task-id>`.

The provider returns a ReviewDossier. scafld validates it, rejects workspace
mutation in the review-relevant surface, writes the review event to session, and
projects the verdict back into the spec. A human-reviewed override writes a
`review_override` event before the passing review event. `complete` will not
archive the task unless the review verdict is `pass`.

On review failure, the text output prints the findings and next repair command.
The same findings appear in `scafld status`, `scafld handoff`, the session
review entry, and the spec `## Review` section.

## finalize

```bash
scafld finalize [task-id] [--base-ref REF] [--scope-hint PATH] [--json] [--stdin]
```

`finalize` is the single-call completion authority. One invocation snapshots
the workspace into an immutable git tree, runs the spec's acceptance criteria
against that snapshot, runs the independent adversarial review, and mints an
ed25519-signed receipt anchored in the task's session ledger. The JSON result
carries the receipt itself plus `receipt_path`, `task_receipt_path`, and
`ledger_head`.

Receipts land in `.scafld/receipts/<task-id>.json`. The
`.scafld/receipts/latest.json` pointer is written only after the receipt is
anchored in the ledger, so hosts reading it never see an unanchored receipt.

When acceptance or review blocks, finalize returns the verdict, findings, and
per-criterion acceptance results instead of a receipt, and exits through the
normal gate codes.

Flags:

- `--base-ref REF`: override the snapshot comparison base.
- `--scope-hint PATH`: add a path boundary hint; repeatable.
- `--stdin`: read the finalize request from stdin. The finalize MCP tool uses
  this mode; operators normally do not.

## complete

```bash
scafld complete <task-id> [--json]
```

Archives completed work only after the latest session review event has a
`pass` verdict from `codex`, `claude`, `gemini`, `command`, or an audited human
review.

For current review entries with `reviewed_scope` and
`reviewed_material_digest`, completion ignores commit-only and unrelated
workspace drift but refuses if scoped reviewed bytes changed.

## fail

```bash
scafld fail <task-id> [--reason TEXT] [--json]
```

Records the failure in session, then archives the spec.

## cancel

```bash
scafld cancel <task-id> [--reason TEXT] [--json]
```

Records the cancellation in session, then archives the spec.

## status

```bash
scafld status <task-id> [--json]
```

Shows lifecycle status, the next allowed follow-up command, latest review
findings when present, and `task_material` in JSON. `task_material` is the
derived task-owned scope: baseline paths, task changes since baseline, ambient
drift outside scope, and reviewed/current material digest status when available.

## list

```bash
scafld list [--json]
```

Lists all known specs by task id, status, and title.

## report

```bash
scafld report [--json]
```

Aggregates workspace spec counts and session-derived product metrics:

- `first_attempt_pass_rate`: tasks whose first completed build moved straight
  to review.
- `recovery_convergence_rate`: blocked first attempts that later recovered to
  review.
- `challenge_override_rate`: challenged tasks completed without a later
  passing review from `codex`, `claude`, `gemini`, or `command`. This should
  normally stay at `0`.
- `review_pass_rate`: accepted review verdicts over all review verdicts.
- `review_dossier_coverage`: review events that stored a valid ReviewDossier.
- `review_findings_total`: findings accepted across all valid dossiers.
- `review_open_blockers_total`: findings that still blocked completion when
  recorded.
- `review_attack_angles_total`: attack-log entries accepted across dossiers.
- `workspace_baseline_coverage`: sessions with an approval/build baseline.

Human output keeps the same numbers compact:

```text
total specs: 12
by status:
- review: 1
- completed: 9
metrics:
- first_attempt_pass_rate: 66.7% (8/12)
- recovery_convergence_rate: 75.0% (3/4)
- review_pass_rate: 80.0% (8/10)
- review_dossier_coverage: 100.0% (10/10)
- review_findings_total: 14
- review_open_blockers_total: 3
- review_attack_angles_total: 42
- review_mode_distribution:
  - discover: 7
  - verify: 3
- challenge_override_rate: 0.0% (0/2)
- workspace_baseline_coverage: 100.0% (12/12)
```

## handoff

```bash
scafld handoff <task-id>
```

Renders model-facing context from the current spec and session state. Handoffs
include failed or pending acceptance criteria while a task is blocked, and
latest review findings when present. The top section includes task material
scope, task changes, ambient drift, and search discipline so agents can avoid
staging or reviewing unrelated work. They are transport, not source of truth.

## adapter

```bash
scafld adapter codex build <task-id>
scafld adapter claude review <task-id>
```

Renders a provider-facing trigger packet for thin wrapper scripts. The packet
includes current status, deterministic `next_action` fields, and the current
handoff text. It does not execute an agent runtime and does not advance state.

## verify

```bash
scafld verify <receipt-path> --target <commit-ish> [--trusted-keys PATH] [--ci] [--self-check] [--root PATH] [--json]
```

`verify` is the independent merge wall. It checks a signed receipt without
trusting the host that minted it:

- verifies the ed25519 signature against an active key in the trusted-keys
  file, recomputing the canonical receipt digest.
- snapshots the target workspace and compares file digests against the
  digests the receipt fingerprints.
- checks that `--target` is an ancestor-consistent commit for the receipt.
- re-runs the acceptance criteria recorded in the receipt.
- enforces `verify.min_independence` from `.scafld/config.yaml`.

The receipt path falls back to `SCAFLD_RECEIPT_PATH` and then
`verify.receipt_path` from config. The trusted-keys path falls back to
`SCAFLD_TRUSTED_KEYS` and then `verify.trusted_keys_path`. Verify reads base
config only; `config.local.yaml` cannot repoint the trust anchors.

CI mode applies when `--ci` is set or the `CI` environment variable is truthy.
It fails closed when `--target` or the trusted-keys path is missing.

`--self-check` prints a wiring report for the workspace (key material, trusted
keys, config paths) without verifying a receipt.

Exit codes: `0` verified, `3` verification failed, `2` usage or runtime error.
