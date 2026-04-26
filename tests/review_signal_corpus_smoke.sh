#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI_ROOT="${CLI_ROOT:-$REPO_ROOT/cli}"
TMP_DIRS=()
source "$SCRIPT_DIR/smoke_lib.sh"

scafld_cmd() {
  PATH="$CLI_ROOT:$PATH" scafld "$@"
}

new_repo() {
  local repo
  repo="$(mktemp -d /tmp/scafld-review-corpus.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    scafld_cmd init >/dev/null
  )
  printf '%s\n' "$repo"
}

repo="$(new_repo)"

echo "[1/2] grounded clean reviews pass the stronger parser"
REVIEW_REPO="$repo" REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
from pathlib import Path
import os
import sys

repo = Path(os.environ["REVIEW_REPO"])
sys.path.insert(0, os.environ["REPO_ROOT"])
from scafld.review_signal import review_signal_payload
from scafld.review_workflow import load_review_topology
from scafld.reviewing import clean_no_issues_has_evidence, parse_review_file

review_path = repo / ".ai" / "reviews" / "clean.md"
review_path.parent.mkdir(parents=True, exist_ok=True)
review_path.write_text(
    """# Review: clean

## Review 1 — 2026-04-24T00:00:00Z

### Metadata
```json
{
  "schema_version": 3,
  "round_status": "completed",
  "reviewer_mode": "challenger",
  "reviewer_session": "sess-1",
  "reviewed_at": "2026-04-24T00:00:00Z",
  "override_reason": null,
  "reviewed_head": null,
  "reviewed_dirty": null,
  "reviewed_diff": null,
  "pass_results": {
    "spec_compliance": "pass",
    "scope_drift": "pass",
    "regression_hunt": "pass",
    "convention_check": "pass",
    "dark_patterns": "pass"
  }
}
```

### Pass Results
- spec_compliance: PASS
- scope_drift: PASS
- regression_hunt: PASS
- convention_check: PASS
- dark_patterns: PASS

### Regression Hunt
No issues found — checked callers of api.py.

### Convention Check
No issues found — checked AGENTS.md and CONVENTIONS.md.

### Dark Patterns
No issues found — checked obvious null and retry paths.

### Blocking
None.

### Non-blocking
None.

### Verdict
pass
""",
    encoding="utf-8",
)
topology = load_review_topology(repo)
parsed = parse_review_file(review_path, topology)
assert not parsed["errors"], parsed
assert parsed["verdict"] == "pass", parsed
signal = review_signal_payload(parsed)
assert signal["clean_review_with_evidence"] is True, signal
assert signal["grounded_findings"] == 0, signal
PY

echo "[2/2] low-signal uncited reviews fail the stronger parser"
REVIEW_REPO="$repo" REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
from pathlib import Path
import os
import sys

repo = Path(os.environ["REVIEW_REPO"])
sys.path.insert(0, os.environ["REPO_ROOT"])
from scafld.review_signal import review_signal_payload
from scafld.review_workflow import load_review_topology
from scafld.reviewing import clean_no_issues_has_evidence, parse_review_file

review_path = repo / ".ai" / "reviews" / "flawed.md"
review_path.write_text(
    """# Review: flawed

## Review 1 — 2026-04-24T00:00:00Z

### Metadata
```json
{
  "schema_version": 3,
  "round_status": "completed",
  "reviewer_mode": "challenger",
  "reviewer_session": "sess-1",
  "reviewed_at": "2026-04-24T00:00:00Z",
  "override_reason": null,
  "reviewed_head": null,
  "reviewed_dirty": null,
  "reviewed_diff": null,
  "pass_results": {
    "spec_compliance": "pass",
    "scope_drift": "pass",
    "regression_hunt": "fail",
    "convention_check": "pass",
    "dark_patterns": "pass"
  }
}
```

### Pass Results
- spec_compliance: PASS
- scope_drift: PASS
- regression_hunt: FAIL
- convention_check: PASS
- dark_patterns: PASS

### Regression Hunt
- this feels risky and should get another pass

### Convention Check
No issues found.

### Dark Patterns
No issues found — checked nothing in particular.

### Blocking
- risky edge case

### Non-blocking
None.

### Verdict
fail
""",
    encoding="utf-8",
)
topology = load_review_topology(repo)
parsed = parse_review_file(review_path, topology)
assert parsed["errors"], parsed
assert any("must use '- **severity** `file:line` — explanation'" in error for error in parsed["errors"]), parsed
assert any("Convention Check" in error for error in parsed["errors"]), parsed
signal = review_signal_payload(parsed)
assert signal["clean_review_with_evidence"] is False, signal
assert clean_no_issues_has_evidence("No issues found — checked many things.") is False
assert clean_no_issues_has_evidence("No issues found — checked the relevant code.") is False
assert clean_no_issues_has_evidence("No issues found — checked callers.") is False
assert clean_no_issues_has_evidence("No issues found — checked null.") is False
assert clean_no_issues_has_evidence("No issues found — checked rule.") is False
assert clean_no_issues_has_evidence("No issues found — checked callers of api.py.") is True
PY

echo "PASS: review signal corpus smoke"
