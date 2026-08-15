---
name: finalize
description: Seal accepted scafld review evidence with deterministic acceptance and return a signed receipt.
---

# Finalize

Run the scafld review gate before finalization. When review passes, use the `finalize` MCP tool. Finalize consumes the accepted review evidence, runs deterministic acceptance, signs the receipt, and archives the spec. It never invokes a provider or model.

The host agent may work however it needs to work before the review. Review is the only provider/model call. Finalize is the last accountability verb: it records the target, acceptance evidence, accepted review result, and receipt status.

Pass `task_id` for the governed task. When the work is on a branch or pull request, pass `base_ref` as the base commit/ref so the receipt attests the base-to-head delta instead of only the current working tree. If no hand-authored spec exists, also pass `scope_hint` with the changed paths to review.

## Local finalize is the baseline

`review` followed by `finalize` needs no CI. The signed receipt written under `.scafld/receipts/` is the complete local accountability outcome, and a plain `scafld init` sets this up without installing any workflow. Receipts are ignored by the managed `.gitignore` by default so normal local finalize runs do not dirty the project.

## CI verify is the opt-in upgrade

The CI `scafld verify` check is an additive merge gate, not part of the baseline. Opt in with `scafld init --ci`, which installs `.github/workflows/scafld-verify.yml` so deliberately committed receipts, or an explicit `SCAFLD_RECEIPT_PATH`, are re-verified on pull requests. Declare intent with the `verify.policy` config field (`local` default, `advisory`, `required`) and run `scafld verify --self-check` to see what is actually wired. Requiring the check before a merge is a GitHub branch-protection step the operator owns; scafld scaffolds and reports it, it does not enforce it.
