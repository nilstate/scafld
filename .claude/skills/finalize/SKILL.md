---
name: finalize
description: Run an independent scafld accountability finalize and return a signed receipt for completed work.
---

# Finalize

Use the `finalize` MCP tool when work is ready to be checked. It runs the independent reviewer path and returns either actionable findings or a signed receipt.

The host agent may work however it needs to work before the finalize. The finalize is the single accountability verb: it records the target, acceptance evidence, independent review result, and receipt status.

Pass `task_id` for the governed task. When the work is on a branch or pull request, pass `base_ref` as the base commit/ref so the receipt attests the base-to-head delta instead of only the current working tree. If no hand-authored spec exists, also pass `scope_hint` with the changed paths to review.
