---
title: Scope Discipline
description: Keeping implementation inside the approved contract
---

# Scope Discipline

Scope discipline is still a core scafld idea: the spec says what work is in
bounds, and review should challenge changes that drift outside that contract.

The current Go binary does not expose a standalone scope-audit command. Scope
checks are handled through the spec contract, acceptance evidence, and
adversarial review.

## What to Put in the Spec

Every phase should name the files or areas it expects to touch:

```markdown
Changes:
- `internal/app/build/build.go` -- update phase execution behavior.
- `internal/app/build/build_test.go` -- prove the lifecycle boundary.
```

That gives the executor and challenger a concrete ownership boundary.

## What Review Should Attack

The challenger should compare the approved scope against the actual workspace:

- files changed but not declared in the spec
- declared files that were never touched
- cross-cutting changes hidden inside a narrow phase
- generated or package files changed without a release reason
- prompt/core-bundle edits that alter agent behavior

A legitimate scope expansion should be worked back into the spec and explained
as a deviation before the work is completed.

## Future Standalone Audit

A standalone audit command is a good fit once the Go runtime has a stable
file-ownership model over Markdown phase changes. Until then, avoid advertising
a command that does not exist; use adversarial review as the scope gate.
