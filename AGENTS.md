# AI Agent Policies & Invariants

**Purpose:** High-level policies and architectural invariants for AI coding agents.

**Scope:** Applies to all AI agent interactions with this codebase.

**See also:**
- [CONVENTIONS.md](CONVENTIONS.md) — Detailed coding standards and patterns
- [CLAUDE.md](CLAUDE.md) — Claude-specific integration guide
- [.ai/README.md](.ai/README.md) — Task planning and execution system

---

## The Trellis System

Trellis uses a **spec-driven development** approach for AI agents. The `.ai/` directory contains:

- **Planning prompts** (`.ai/prompts/plan.md`) — How to create task specifications
- **Execution prompts** (`.ai/prompts/exec.md`) — How to execute approved specifications
- **Configuration** (`.ai/config.yaml`) — Validation rules, rubrics, and settings
- **Schemas** (`.ai/schemas/spec.json`) — Machine-readable spec format
- **Specifications** (`.ai/specs/`) — Task artifacts organized by lifecycle status

### How It Works

1. **Planning Mode:** AI analyzes a task, explores the codebase, and generates a YAML specification
2. **Review:** Human reviews and approves the spec
3. **Execution Mode:** AI executes the approved spec phase-by-phase with validation
4. **Archive:** Completed specs are archived for audit trail

**Key principle:** The spec is the contract. AI operates autonomously within the bounds of an approved spec.

---

## Architectural Invariants

These rules MUST NOT be violated. They define the boundaries of acceptable changes.

### 1. Layer Separation

**Invariant:** Code must respect architectural layer boundaries.

- Domain logic stays in domain modules
- HTTP/transport concerns stay in handlers/controllers
- External integrations go through ports/adapters
- No circular dependencies between layers

**Enforcement:** Boundary checks in acceptance criteria.

### 2. Stable Public APIs

**Invariant:** Public API changes require explicit approval.

- HTTP endpoints, event schemas, and public interfaces are stable contracts
- Changes to public surfaces must be documented in specs
- Breaking changes require migration plans

**Enforcement:** `safety.require_approval_for` in config.yaml.

### 3. No Legacy Fallbacks

**Invariant:** Do not add dual-reads, dual-writes, or runtime fallbacks.

- When changing schemas or identifiers, adopt the new scheme immediately
- Use one-off migration scripts, not runtime code
- Keep application code free of migration branches

**Enforcement:** Code review and spec approval.

### 4. No Hardcoded Secrets

**Invariant:** Configuration must come from environment, never hardcoded.

- Database URLs, API keys, credentials from env vars or secrets management
- No secrets in code, logs, or diffs
- Security scan in pre-commit validation

**Enforcement:** Automated security scan.

### 5. Test-Logic Separation

**Invariant:** Test-only code stays in test files.

- No test fixtures or mocks in production code
- No conditional logic that only runs in tests
- Test utilities in dedicated test helper modules

**Enforcement:** Boundary checks.

---

## Agent Operating Modes

### Planning Mode

**Enter when:** Starting a new task, exploring requirements, or creating a spec.

**Behaviors:**
- Search and read codebase (no modifications)
- Analyze architecture and patterns
- Generate spec file in `.ai/specs/drafts/`
- Ask clarifying questions when needed

**Output:** YAML spec file with status `draft` or `under_review`.

**Constraint:** NO code changes outside `.ai/specs/`.

### Execution Mode

**Enter when:** Spec has been approved (status: `approved`).

**Behaviors:**
- Follow spec exactly (deviations require approval)
- Apply changes phase-by-phase
- Run acceptance criteria after each phase
- Rollback on failure
- Log all actions to `.ai/logs/{task-id}.log`

**Output:** Code changes, validation results, updated spec file.

**Constraint:** Stay within spec boundaries; pause for approval if deviation needed.

---

## Validation Requirements

### Per-Phase Validation

After each phase, run:
1. **Compile check** — Code builds without errors
2. **Targeted tests** — Tests related to the change pass
3. **Acceptance criteria** — All phase criteria pass

### Pre-Commit Validation

Before marking task complete:
1. **Full test suite** — All tests pass
2. **Linters** — Code style checks pass
3. **Type check** — Static type analysis passes
4. **Security scan** — No hardcoded secrets
5. **Self-evaluation** — Score ≥7/10 on rubric

### Validation Profiles

- **light** — For micro/small, low-risk tasks
- **standard** — For typical feature work
- **strict** — For high-impact changes

---

## Safety Controls

### Require Approval For

The following actions require explicit human approval:
- Schema migrations
- Public API changes
- Data deletion operations
- Production deployments

### Prevent

The system automatically checks for and prevents:
- Hardcoded secrets
- Unbounded database queries
- SQL injection patterns
- XSS vulnerabilities

---

## Self-Evaluation Rubric

AI agents score their work on a 0-10 scale:

| Dimension | Weight | Description |
|-----------|--------|-------------|
| Completeness | 0-3 | Meets requirements, handles edge cases, follows conventions |
| Architecture Fidelity | 0-3 | Respects boundaries, uses patterns, improves separation |
| Spec Alignment | 0-2 | Matches approved plan, proposes improvements |
| Validation Depth | 0-2 | Has targeted tests, runs broader checks |

**Threshold:** ≥7/10 required for task completion.

If below threshold, AI must perform a second pass to address deficiencies.

---

## Communication Guidelines

### Progress Updates

**Do:**
- Report phase completion status
- Show acceptance criteria pass/fail counts
- Indicate next action

**Don't:**
- Verbose preambles ("Now I will...", "Let me...")
- Repetitive explanations
- Expose internal reasoning (that goes in logs)

### When Blocked

Report with structured format:
1. What phase/criterion is blocked
2. Brief error description
3. One concrete recommendation
4. Options for resolution

### Final Summary

Include:
- Phases completed
- Acceptance criteria results
- Self-evaluation score
- Deviations (if any)
- Files changed

---

## Best Practices

### For AI Agents

1. **Always start in PLAN mode** for non-trivial tasks
2. **Read before editing** — understand existing code
3. **Follow the spec** — deviations require approval
4. **Validate obsessively** — run criteria after every phase
5. **Log transparently** — ReAct traces in logs
6. **Self-evaluate honestly** — don't inflate scores

### For Developers

1. **Review specs before approval** — check phases and criteria
2. **Monitor logs during execution** — `tail -f .ai/logs/{task-id}.log`
3. **Validate spec files** — use JSON schema validator
4. **Version control specs** — commit for audit trail
5. **Iterate on config** — adjust rubrics and checks as team learns

---

## Quick Reference

### Spec Status Lifecycle

```
draft → under_review → approved → in_progress → completed
                                      ↓           ↓
                                   (blocked)   failed
                                                  ↓
                                              cancelled
```

### Key Files

- `.ai/config.yaml` — Configuration and validation rules
- `.ai/prompts/plan.md` — Planning mode instructions
- `.ai/prompts/exec.md` — Execution mode instructions
- `.ai/schemas/spec.json` — Spec validation schema
- `.ai/specs/` — Task specifications by status
- `.ai/logs/` — Execution logs

### Commands

```bash
# View spec
cat .ai/specs/drafts/{task-id}.yaml

# Monitor execution
tail -f .ai/logs/{task-id}.log

# Validate spec
ajv validate -s .ai/schemas/spec.json -d .ai/specs/{task-id}.yaml
```
