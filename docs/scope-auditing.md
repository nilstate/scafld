---
title: Scope Auditing
description: Detecting scope drift with scafld audit
---

# Scope Auditing

`scafld audit` compares the files declared in the spec against the actual git diff. It catches scope creep -- changes the agent made that weren't in the plan.

## Usage

```bash
scafld audit add-auth -b main
```

The `-b` flag sets the git reference to diff against. Defaults to `HEAD~1`, falls back to `main` or `master` if that fails.

## Output categories

| Category | Color | Meaning |
|----------|-------|---------|
| Declared and changed | Green | Files in the spec that were actually modified. Expected. |
| Changed but not in spec | Red | Files modified in git but not declared in any phase. Scope creep. |
| In spec but not changed | Yellow | Files declared in the spec but not modified. Possibly incomplete work. |

## Exit codes

- `0` -- clean. All changes match the spec.
- `1` -- scope creep detected. Undeclared files were changed.

## How it works

The audit collects every `file` path from all `phases[*].changes[*].file` entries in the spec, then diffs against the base ref. Any file in the git diff that doesn't appear in the spec's declared changes is flagged.

## Integration with review

The `scope_drift` automated review pass runs `scafld audit` internally. If scope creep is detected, the review fails and blocks completion.

## When scope creep is legitimate

Sometimes the agent discovers a necessary change that wasn't anticipated in the spec. The right response is to update the spec with the additional file, record a deviation, and re-run the audit. Ignoring scope creep defeats the purpose of the spec as a contract.
