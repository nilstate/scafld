---
name: finalize
description: Run an independent scafld accountability finalize and return a signed receipt for completed work.
---

# Finalize

Use the `finalize` MCP tool when work is ready to be checked. It runs the independent reviewer path and returns either actionable findings or a signed receipt.

The host agent may work however it needs to work before the finalize. The finalize is the single accountability verb: it records the target, acceptance evidence, independent review result, and receipt status.

Pass `task_id` for the governed task. When the work is on a branch or pull request, pass `base_ref` as the base commit/ref so the receipt attests the base-to-head delta instead of only the current working tree. If no hand-authored spec exists, also pass `scope_hint` with the changed paths to review.

## Local finalize is the baseline

`finalize` needs no CI. The independent review and the signed receipt it writes under `.scafld/receipts/` are the complete accountability outcome on their own, and a plain `scafld init` sets this up without installing any workflow.

## CI verify is the opt-in upgrade

The CI `scafld verify` check is an additive merge gate, not part of the baseline. Opt in with `scafld init --ci`, which installs `.github/workflows/scafld-verify.yml` so committed receipts are re-verified on pull requests. Declare intent with the `verify.policy` config field (`local` default, `advisory`, `required`) and run `scafld verify --self-check` to see what is actually wired. Requiring the check before a merge is a GitHub branch-protection step the operator owns; scafld scaffolds and reports it, it does not enforce it.
