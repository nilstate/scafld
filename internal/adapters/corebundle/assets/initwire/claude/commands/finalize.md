---
description: Seal accepted scafld review evidence and get a signed receipt.
---

After `scafld review` passes, run `finalize` for the current work. Report the accepted review result, any deterministic acceptance blockers, and the signed receipt details when finalization passes. Finalize does not invoke a model.

Use the governed task id as `task_id`. Include `base_ref` when a branch or pull request base is known, and include `scope_hint` for no-spec work that needs explicit changed paths.
