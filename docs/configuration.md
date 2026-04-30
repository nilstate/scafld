---
title: Configuration
description: Minimal runtime config for model profile, context budget, and recovery cap
---

# Configuration

scafld merges:

- `.scafld/config.yaml`
- `.scafld/config.local.yaml`

The local file overlays the committed base file.

## Minimal LLM Surface

The runtime surface stays intentionally small:

```yaml
llm:
  model_profile: "default"
  context:
    budget_tokens: 12000
  recovery:
    max_attempts: 1
```

Meaning:

- `model_profile`: label written into handoffs and session
- `context.budget_tokens`: renderer hint for how much context to pack
- `recovery.max_attempts`: hard cap for executor recovery handoffs

If an external wrapper knows token or cost usage, it may write optional `usage`
fields into session. scafld does not require that data.

## Acceptance strictness

Acceptance criteria gate phase advance in `scafld build`. Each executable
criterion requires an explicit structured matcher:

```yaml
- id: "ac1_1"
  type: "test"
  command: "python3 -m unittest tests.test_X"
  expected_kind: "exit_code_zero"
```

Three `expected_kind` values:

- `exit_code_zero` — passes when the command exits 0.
- `exit_code_nonzero` — passes on any non-zero exit; pin a specific code with `expected_exit_code: <n>`.
- `no_matches` — passes when stdout is empty OR the command exits non-zero; rg-style "no match found" returns exit code 1 with empty stdout.

Optional `evidence_required: true` additionally requires non-empty
stdout. Stops the "compile + unittest pass with no real work"
pattern.

Hard cutover: criteria without an explicit `expected_kind` are rejected loudly
without running the command. Free-form `expected:` text is documentation only;
it is never enough to execute a criterion.

## Slim plan

`scafld plan <id> -t "<title>" --command "<cmd>" --files "a.py,b.py"`
produces a complete spec ready to approve. No `TODO` placeholders, no manual
fill round, no validation surprises.

```markdown
---
spec_version: "2.0"
task_id: fix-typo
status: draft
---

# Fix typo

## Phase 1: Fix typo

Goal:

Acceptance:
- [ ] `ac1_1` test
  - Command: `grep -q 'the' README.md`
  - Expected kind: `exit_code_zero`
```

The slim criterion always declares `expected_kind: exit_code_zero`
explicitly so the strict-unset-reject in
`evaluate_acceptance_criterion` never fires.

The verbose template path is used when
`--command` is omitted; multi-phase complex specs stay verbose by
default.

## Plan-time conflict detection

Whenever `scafld plan` writes a slim spec, it reads the declared
file paths and compares them against other active specs:

- If another active spec declares the same file as `ownership: shared`,
  the new spec auto-tags its change entry as `shared` too.
- If another active spec declares the file exclusively, `scafld plan`
  refuses with `EC.SCOPE_DRIFT` and names the conflicting task. No
  partial spec file is written; nothing to clean up.

This catches scope conflicts at plan time instead of waiting for
review-time `scope_drift` to surface them.

## Review seal

Each external-reviewer round writes `canonical_response_sha256` into
the review markdown's metadata block (computed in
[`scafld/review_runner.py`](../scafld/review_runner.py) over the
canonical `{packet, projection}` shape) and persists the canonical
packet body to `.scafld/runs/<task>/review-packets/review-<n>.json`.

`scafld complete` recomputes the hash from the parsed packet artifact
and refuses to advance on mismatch with `EC.REVIEW_GATE_BLOCKED`.
Hand-edits to the review markdown — flipping a verdict, changing a
finding bullet, swapping pass results — fail loudly instead of
slipping past the gate.

External review rounds must carry a packet artifact and nested
`review_provenance.canonical_response_sha256` metadata. If either part is
missing or mismatched, `scafld complete` fails with guidance to re-run
`scafld review`.

## Review gate severity

`review.gate_severity` in `.scafld/config.yaml` controls how non-blocking
findings affect `scafld complete`. Default is `blocking`.

```yaml
review:
  gate_severity: "blocking"   # default; only Blocking-section findings gate complete
```

Single-round-by-default semantics:

- Verdict `pass` or `pass_with_issues` (no blocking findings) ships in
  one review round. `complete` advances.
- Iteration only happens when blocking (high/critical) findings
  exist. Fix → re-review → repeat.
- Medium and low findings are advisory output only — they don't block
  `complete` and they don't drive iteration.

Operators who want strict iteration opt in:

```yaml
review:
  gate_severity: "medium"   # non-blocking medium+ findings now gate complete
```

When advisory findings exist under the threshold, `scafld complete`
prints an `advisory: N finding(s)` summary with a per-severity
breakdown so the operator/agent knows what was deferred.

Findings must declare severity (already enforced by the review parser
via `FINDING_LINE_RE` in `scafld/reviewing.py`).

## Review Topology

Review ordering stays explicit:

```yaml
review:
  automated_passes:
    spec_compliance:
      order: 10
    scope_drift:
      order: 20
  adversarial_passes:
    regression_hunt:
      order: 30
    convention_check:
      order: 40
    dark_patterns:
      order: 50
```

Configured titles and order flow into both:

- the review scaffold under `.scafld/reviews/`
- the generated challenger handoff

## Review Runner

The review runner contract stays narrow:

```yaml
review:
  runner: "external"     # external | local | manual
  external:
    provider: "auto"     # auto | codex | claude
    idle_timeout_seconds: 180
    absolute_max_seconds: 1800
    fallback_policy: "warn" # warn | allow | disable
    codex:
      model: "gpt-5.5"
    claude:
      model: "claude-opus-4-7"
```

Meaning:

- `runner`: default review execution mode
- `external.provider`: provider selection for external review; `auto` prefers
  another installed provider when the current agent is detected as Codex,
  otherwise `codex` first, then `claude`
- `external.idle_timeout_seconds`: maximum time without stdout/stderr bytes
  before `scafld review` treats the provider as hung
- `external.absolute_max_seconds`: maximum wall-clock runtime before
  `scafld review` fails with fallback guidance
- `external.fallback_policy`: behavior when `provider: auto` cannot find
  the preferred provider but can find the alternate provider; `warn` allows
  fallback or Codex self-review avoidance with a warning, `allow` records the
  weaker isolation without warning, and `disable` requires Codex

### Structured output enforcement

The reviewer's response shape is enforced at generation time by the provider
CLI. scafld passes the ReviewPacket schema to claude via `--json-schema`
(inline JSON) and to codex via `--output-schema <path>` (temporary file).
The model is constrained to produce JSON matching the schema; no prose
preface, no markdown fences. Python-side `normalize_review_packet` remains
authoritative at runtime.

The schema lives at `.scafld/core/schemas/review_packet.json` and is bundled by
`scafld init` and `scafld update`. Operator overrides in the workspace are
preserved (matches the existing `spec.json` behavior). At prompt-build time
scafld narrows the schema's `pass_results` properties to the exact
adversarial pass_ids declared by the spec's review topology.

Known constraints:

- codex `--output-schema` is gated to the `gpt-5.x` model family
  (codex#4181). The default pin is `gpt-5.5`; non-gpt-5 overrides silently
  bypass the constraint.
- The combination of codex `--json` and `--output-schema` with active MCP
  servers can drop the strict `response_format` (codex#15451). scafld
  passes `--ignore-user-config` and does not load MCP for codex review,
  so this should not trip; the Python validator catches it if it does.

### Per-provider model

- `external.<provider>.model`: optional provider-specific model pin. Accepts
  a single string or a list of strings. Codex review defaults to `gpt-5.5`
  for the strongest available Codex review path; Claude review defaults to
  `claude-opus-4-7`. Override either if that model is not enabled for the
  operator account.

  When the model is configured as a list, scafld retries the same provider
  with the next entry whenever a non-zero exit reports a recognized
  rejection signature (`unknown model`, `model not found`, `model not
  available`, `does not have access`, `not authorized to use this model`).
  Other failures still fail fast as before. Each attempted model is
  recorded as its own `provider_invocation` entry in
  `.scafld/runs/<task>/session.json`, so `scafld status` and `scafld report`
  show the full sequence:

  ```yaml
  review:
    external:
      codex:
        model:
          - "gpt-5.5"
          - "gpt-5"
      claude:
        model:
          - "claude-opus-4-7"
          - "claude-sonnet-4-6"
  ```

CLI overrides are explicit:

```bash
scafld review <task-id> --runner local
scafld review <task-id> --runner manual
scafld review <task-id> --provider codex --model gpt-5
```

There is no silent fallback from external review into local review. If no
external provider exists, scafld fails cleanly and tells you to opt into
`local` or `manual`.

Provider fallback is not treated as equivalent isolation. Codex provenance is
recorded as read-only and ephemeral. Claude provenance is recorded as restricted
tools plus fresh session, which is weaker than the Codex sandbox unless the
installed Claude CLI grows an equivalent control. Set
`review.external.fallback_policy: "disable"` to prevent automatic Claude
fallback.

Current-agent detection is automatic for Codex environment variables. Tests,
CI, and wrapper scripts can override it with
`SCAFLD_CURRENT_AGENT_PROVIDER=codex|claude|unknown`.

Review provenance separates requested and observed model fields. An empty
observed model means scafld could not verify what the provider actually billed.
Provider invocation session entries include `confidence`: `observed`,
`inferred`, `requested_only`, or `unknown`.
Reports use conservative separation states: `separated`, `same_model`,
`unknown_executor`, `unknown_challenger`, and `unknown_both`.

## Why It Stays Small

The cutover goal is clean execution, not config sprawl.

Strategy lives in config. State lives in session. The spec schema stays
unchanged until more surface earns its place through measured wins.

## Repo-Aware Scaffolding

`scafld init`, `new`, and `plan` also derive suggested validation commands from
the current workspace. Mixed Python+Node repos are handled explicitly: scafld
merges both detectors, prefers concrete commands over placeholder defaults, and
combines commands with `&&` when both stacks provide real signals.
