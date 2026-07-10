---
title: Artifacts
description: Concrete scafld artifact examples
---

# Artifacts

scafld is deterministic because the workflow is made of durable artifacts. The
agent can change; the spec, session, gate repair contract, and review dossier
remain.

## Spec Excerpt

Specs are Markdown projections of the task contract and session state:

```markdown
# Add typed error codes to document processing module

## Current State

Status: review
Current phase: final
Next: scafld review add-error-codes
Reason: build complete; review gate ready
Allowed follow-up command: scafld review add-error-codes
Review gate: not_started

## Acceptance

Validation:
- [x] v1 compile - Project compiles with no type errors
  - Command: npm run build
  - Status: pass
  - Evidence: exit=0 duration=4.2s
- [x] v2 test - Unit tests pass
  - Command: npm test -- --filter documents
  - Status: pass
  - Evidence: exit=0 duration=2.1s
```

The spec is readable state. The session ledger is the evidence source.

## Harden Round

A harden round records the pre-build challenge as one observation ledger. Each
observation covers a dimension, result, and anchor. Open `blocks` observations
block approval; advisories keep their full detail without forcing another harden
loop.

```markdown
## Harden Rounds

### round-1

Status: passed
Started: 2026-05-13T00:10:00Z
Ended: 2026-05-13T00:18:00Z
Verdict: pass
Provider: codex
Model: gpt-5.5
Output format: codex.output_file
Summary: The draft contract survived design, scope, path, command, timing, and rollback challenge observations.

Observations:
- design
  - Result: clean
  - Anchor: spec_gap:Summary
  - Note: The plan names the underlying review failure, keeps the shared context owner central, and avoids duplicating behavior across adapters.
- scope
  - Result: clean
  - Anchor: spec_gap:Risks
  - Note: No compatibility fallback, adapter-specific fork, or data migration is introduced.
- path
  - Result: clean
  - Anchor: code:internal/app/review/context.go:30
  - Note: Target package and docs paths exist; new docs file is declared.
- command
  - Result: clean
  - Anchor: code:Makefile:1
  - Note: `make check` is the repository validation command.
- timing
  - Result: clean
  - Anchor: spec_gap:Phases
  - Note: Criteria run after the parser and docs updates are in place.
- rollback
  - Result: advisory
  - Anchor: spec_gap:Rollback
  - Note: Rollback is credible but could name a recovery command.
  - Default: Use the package's existing repair command if already known.
```

Provider-backed hardening records one strict `HardenDossier` with summary and
observations. Manual hardening can still be conversational, but its durable
output should use the same observation shape.

## Status JSON

`status --json` is the stable automation surface:

```json
{
  "ok": true,
  "command": "status",
  "result": {
    "task_id": "add-error-codes",
    "status": "review",
    "next": "scafld review add-error-codes",
    "gate": "review",
    "trusted_state": "session ledger replay projected into the Markdown spec",
    "allowed_follow_up": "scafld review add-error-codes",
    "session_ok": true,
    "task_material": {
      "scope": ["src/errors/document-error.ts"],
      "baseline_paths": ["README.md"],
      "task_changes": ["changed src/errors/document-error.ts (M old -> M new)"],
      "ambient_drift": ["changed docs/index.md (M old -> M new)"],
      "material_status": "unreviewed"
    }
  }
}
```

Agents should use this instead of scraping Markdown or inventing task-owned
change manifests.

## Failed Gate

Every blocked gate should say what happened and what to do next:

```text
gate: review
status: review
reason: review provider failed
evidence:
- .scafld/runs/add-error-codes/diagnostics/command-1.txt
expected: valid ReviewDossier submitted by an external reviewer
actual: provider produced no submission; Claude must call submit_review exactly once and final text is ignored
blockers:
- review provider failed
next: scafld handoff add-error-codes
```

The same repair contract appears in JSON under `error.gate` or
`result.repair`.

## Review Context Manifest

Every provider prompt starts with a budget manifest:

```text
## Context Budget Manifest

Max section body bytes: 16384
Rendered section body bytes: 9211
Omitted section body bytes: 1840

Included sections:
- `task_contract` (Task Contract): rendered=822 body=822 omitted=0
- `task_changes` (Task Changes Since Approval Baseline): rendered=1410 body=1410 omitted=0

Truncated sections:
- `project_context:README.md` (Project Context: README.md): rendered=800 body=1900 omitted=1100 sources=`README.md`

Omitted sections:
- `project_context:docs/review.md` (Project Context: docs/review.md): rendered=0 body=740 omitted=740 sources=`docs/review.md` reason=context budget exhausted
```

Budgeting never hides omissions. The reviewer can see what fit, what was
truncated, and which source paths to open when a specific attack requires more
context.

## Review Dossier

The accepted review payload is one strict dossier shape:

```json
{
  "verdict": "fail",
  "mode": "discover",
  "summary": "One completion blocker found.",
  "findings": [
    {
      "id": "missing-error-envelope-mapping",
      "severity": "high",
      "blocks_completion": true,
      "category": "Spec Compliance",
      "confidence": "high",
      "location": {"path": "src/errors/document-error.ts", "line": 44},
      "evidence": "DocumentProcessingError exposes code but the API adapter still serializes message only.",
      "impact": "Callers cannot branch on the new typed error code.",
      "validation": "Add an API adapter test asserting the response envelope includes code.",
      "status": "open",
      "summary": "Typed error codes are not exposed at the API boundary."
    }
  ],
  "attack_log": [
    {
      "target": "error envelope callers",
      "attack": "Trace serialization from processor errors to API response",
      "result": "finding"
    }
  ],
  "budget": {
    "max_findings": 12,
    "min_attack_angles": 6,
    "actual_findings": 1,
    "actual_attack_angles": 1,
    "depth": "standard"
  },
  "provider": "codex",
  "output_format": "codex.output_file"
}
```

`complete` only trusts a passing dossier from an external provider or an
audited human review override.
