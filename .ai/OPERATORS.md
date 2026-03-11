# Trellis — Operator Cheat Sheet

A short, human-friendly guide for working with Trellis task specs.
For full details, see `.ai/README.md` and `.ai/specs/README.md`.

---

## 1. Tiny Change (Micro/Small, Low Risk)

Use this for trivial, low-risk edits (comments, copy tweaks, tiny refactors).

- In the spec:
  - `task.size: "micro"` or `"small"`
  - `task.risk_level: "low"`
  - Optionally set `task.acceptance.validation_profile: "light"`
- Workflow:
  - Plan: generate/update spec under `.ai/specs/drafts/`
  - Approve: move to `.ai/specs/approved/` and set `status: "approved"`
  - Execute: move to `.ai/specs/active/` and set `status: "in_progress"`
  - Complete: move to `.ai/specs/archive/YYYY-MM/` and set `status: "completed"`

---

## 2. Normal Task (Small/Medium, Medium Risk)

Use this for typical feature work and non-trivial refactors.

- In the spec:
  - `task.size: "small"` or `"medium"`
  - `task.risk_level: "medium"`
  - Usually `task.acceptance.validation_profile: "standard"`
- Workflow:
  - Plan: ensure `task.acceptance.definition_of_done` and `phases[*].acceptance_criteria` tell the same story.
  - Approve: move to approved folder
  - Execute: run all `acceptance_criteria` plus per-phase validation
  - Complete: run full standard profile validation before archiving

---

## 3. Big Change (Medium/Large, High Risk)

Use this for high-impact work (auth, persistence, complex refactors).

- In the spec:
  - `task.size: "medium"` or `"large"`
  - `task.risk_level: "high"`
  - Usually `task.acceptance.validation_profile: "strict"`
- Workflow:
  - Plan:
    - Explicitly call out invariants and risks
    - Use multiple phases with narrow scopes and strong acceptance criteria
  - Approve: move to approved folder
  - Execute: run all per-phase checks plus full `strict` profile
  - Complete: thorough validation before archiving

---

## 4. Quick Commands Reference

```bash
trellis new my-task -t "My feature" -s small -r low   # scaffold spec
trellis list                      # show all specs
trellis list active               # filter by status
trellis status my-task            # show details + phase progress
trellis validate my-task          # check against schema
trellis approve my-task           # drafts/ -> approved/
trellis start my-task             # approved/ -> active/
trellis exec my-task              # run acceptance criteria, record results
trellis exec my-task -p phase1    # run criteria for one phase only
trellis audit my-task             # compare spec files vs git diff
trellis audit my-task -b main     # audit against specific base ref
trellis diff my-task              # show git history for spec
trellis complete my-task          # active/ -> archive/YYYY-MM/
trellis fail my-task              # active/ -> archive/ (failed)
trellis cancel my-task            # active/ -> archive/ (cancelled)
trellis report                    # aggregate stats across all specs
```

---

## 5. Validation Profiles

| Profile | When to Use | What Runs |
|---------|-------------|-----------|
| `light` | micro/small, low risk | compile, acceptance items, perf eval |
| `standard` | small/medium, medium risk | compile, tests, lint, typecheck, security, perf eval |
| `strict` | medium/large, high risk | all standard checks + broader coverage |

---

## 6. Status Lifecycle

```
draft → under_review → approved → in_progress → completed
                                      ↓           ↓
                                   (blocked)   failed
                                      ↓           ↓
                                   (resume)   cancelled
```

---

## 7. Verification Workflow

After execution, before completing:

```bash
trellis exec my-task              # runs acceptance criteria, records pass/fail
trellis audit my-task -b main     # checks for scope creep (undeclared file changes)
trellis complete my-task          # warns if no exec results or suspicious self-eval
```

---

## 8. Tips

- **Always read the spec before executing** — understand what you're building
- **Keep phases small** — easier to validate and rollback
- **Run `trellis exec` before completing** — prove the work, don't just claim it
- **Run `trellis audit` on big changes** — catch scope creep early
- **Self-eval honestly** — the 7/10 threshold keeps quality high; 10/10 requires justification
- **Archive completed specs** — they're your project history
