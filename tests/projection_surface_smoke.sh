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

write_spec() {
  local path="$1"
  SCAFLD_SPEC_CREATED="2026-04-21T00:00:00Z" \
  SCAFLD_SPEC_PHASE_STATUS="completed" \
  SCAFLD_SPEC_CRITERION_RESULT="pass" \
    write_markdown_spec "$path" "projection-flow" "draft" "Projection Flow" "tracked.txt" "test -f tracked.txt"
}

write_review() {
  local path="$1"
  local reviewed_head="$2"
  cat > "$path" <<EOF
# Review: projection-flow

## Review 1 — 2026-04-21T00:05:00Z

### Metadata

\`\`\`json
{
  "schema_version": 3,
  "round_status": "completed",
  "reviewer_mode": "fresh_agent",
  "reviewer_session": "",
  "reviewed_at": "2026-04-21T00:05:00Z",
  "reviewed_head": "$reviewed_head",
  "reviewed_dirty": false,
  "reviewed_diff": "projection-smoke",
  "pass_results": {
    "spec_compliance": "pass",
    "scope_drift": "pass",
    "regression_hunt": "pass",
    "convention_check": "pass_with_issues",
    "dark_patterns": "pass"
  }
}
\`\`\`

### Pass Results

- Spec Compliance: PASS
- Scope Drift: PASS
- Regression Hunt: PASS
- Convention Check: PASS WITH ISSUES
- Dark Patterns: PASS

### Regression Hunt

No regressions found.

### Convention Check

- Non-blocking: keep projection wording terse.

### Dark Patterns

No issues found.

### Blocking

None

### Non-blocking

- projection copy should stay compact.

### Verdict

pass_with_issues
EOF
}

WS="$(mktemp -d /tmp/scafld-projection-smoke.XXXXXX)"
TMP_DIRS+=("$WS")

echo "[1/6] init workspace, baseline commit, and active fixture"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld init >/dev/null"
printf 'seed\n' > "$WS/tracked.txt"
(cd "$WS" && git init -b main >/dev/null 2>&1 && git config user.email smoke@example.com && git config user.name "Smoke Test" && git add . && git commit -m "chore: seed workspace" >/dev/null 2>&1)
write_spec "$WS/.scafld/specs/drafts/projection-flow.md"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld approve projection-flow >/dev/null && PATH='$CLI_ROOT':\"\$PATH\" scafld build projection-flow >/dev/null && PATH='$CLI_ROOT':\"\$PATH\" scafld branch projection-flow >/dev/null"
write_review "$WS/.scafld/reviews/projection-flow.md" "$(git -C "$WS" rev-parse HEAD)"

echo "[2/6] summary renders aligned markdown and JSON"
capture markdown bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld summary projection-flow"
assert_contains "$markdown" "## scafld: Projection Flow" "summary should render the task title"
assert_contains "$markdown" 'Review: `pass_with_issues`' "summary should render the review verdict"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld summary projection-flow --json"
assert_json "$output" "data['command'] == 'summary' and data['result']['model']['title'] == 'Projection Flow'" "summary --json should expose the model title"
assert_json "$output" "data['result']['projection']['surface'] == 'engineering_summary' and data['result']['projection']['rendering'] == 'markdown'" "summary --json should describe the intended projection surface"
assert_json "$output" "'## scafld: Projection Flow' in data['result']['markdown']" "summary --json markdown should match the human surface"

echo "[3/6] checks --json emits CI-friendly success when review and sync are clean"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld checks projection-flow --json"
assert_json "$output" "data['command'] == 'checks' and data['result']['check']['status'] == 'success'" "checks --json should succeed for a clean reviewed spec"
assert_json "$output" "data['result']['projection']['surface'] == 'ci_check' and data['result']['projection']['rendering'] == 'json'" "checks --json should describe the intended projection surface"
assert_json "$output" "'review pass_with_issues' in data['result']['check']['summary']" "checks --json should summarize the review state"

echo "[4/6] pr-body renders deterministic workflow markdown"
capture markdown bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld pr-body projection-flow"
assert_contains "$markdown" "# Projection Flow" "pr-body should render the title heading"
assert_contains "$markdown" "## Workflow State" "pr-body should render workflow state"
assert_contains "$markdown" "## Objectives" "pr-body should include objectives"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld pr-body projection-flow --json"
assert_json "$output" "data['command'] == 'pr-body' and '## Workflow State' in data['result']['markdown']" "pr-body --json should return the markdown body"
assert_json "$output" "data['result']['projection']['surface'] == 'pull_request_body' and data['result']['projection']['rendering'] == 'markdown'" "pr-body --json should describe the intended projection surface"

echo "[5/6] checks fail structurally when engineering drift appears"
printf 'dirty\n' >> "$WS/tracked.txt"
if capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld checks projection-flow --json"; then
  fail "checks should fail when engineering drift exists"
fi
assert_json "$output" "data['error']['code'] == 'projection_check_failed' and data['state']['check_status'] == 'failure'" "checks should emit a structured projection failure"
assert_json "$output" "'workspace has uncommitted changes' in data['result']['model']['sync']['reasons']" "checks should surface sync drift reasons directly"

echo "[6/6] summary reflects drift in markdown too"
capture markdown bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld summary projection-flow"
assert_contains "$markdown" 'Sync: `drift`' "summary markdown should show sync drift"

echo "PASS: projection surface smoke"
