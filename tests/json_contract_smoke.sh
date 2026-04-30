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
  local task_id="$2"
  local status="$3"
  local file_name="${4:-README.md}"
  local ownership="${5:-exclusive}"
  mkdir -p "$(dirname "$path")"
  if [ "$ownership" = "exclusive" ]; then
    SCAFLD_SPEC_CREATED="2026-04-21T00:00:00Z" \
      write_markdown_spec "$path" "$task_id" "$status" "Smoke $task_id" "$file_name" "test -f $file_name"
  else
    SCAFLD_SPEC_CREATED="2026-04-21T00:00:00Z" \
    SCAFLD_SPEC_OWNERSHIP="$ownership" \
      write_markdown_spec "$path" "$task_id" "$status" "Smoke $task_id" "$file_name" "test -f $file_name"
  fi
}

WS="$(mktemp -d /tmp/scafld-json-smoke.XXXXXX)"
TMP_DIRS+=("$WS")

echo "[1/12] init --json emits a stable envelope"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld init --json"
assert_json "$output" "data['ok'] is True and data['command'] == 'init'" "init --json should emit a success envelope"
assert_json "$output" "data['result']['config']['path'] == '.scafld/config.local.yaml'" "init --json should describe config.local creation"
(cd "$WS" && git init -b main >/dev/null 2>&1 && git config user.email smoke@example.com && git config user.name "Smoke Test" && git add . && git commit -m init >/dev/null 2>&1)

echo "[2/12] plan/status/validate --json emit native task state"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld plan json-template -t 'JSON Template' -s small -r low --json"
assert_json "$output" "data['command'] == 'plan' and data['task_id'] == 'json-template'" "plan --json should emit task identity"
write_spec "$WS/.scafld/specs/drafts/json-flow.md" "json-flow" "draft"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld status json-flow --json"
assert_json "$output" "data['command'] == 'status' and data['state']['status'] == 'draft'" "status --json should emit state"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld validate json-flow --json"
assert_json "$output" "data['command'] == 'validate' and data['result']['valid'] is True" "validate --json should emit validation result"

echo "[3/12] approve --json emits transitions"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld approve json-flow --json"
assert_json "$output" "data['command'] == 'approve' and data['result']['transition']['status'] == 'approved'" "approve --json should emit transition data"

printf 'json smoke\n' > "$WS/README.md"

echo "[4/12] build --json emits per-criterion results"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld build json-flow --json"
assert_json "$output" "data['command'] == 'build' and data['state']['action'] == 'start_exec'" "build --json should activate approved work"
assert_json "$output" "data['result']['summary']['failed'] == 0" "build --json should report zero failures"
assert_json "$output" "data['result']['criteria'][0]['status'] == 'pass'" "build --json should emit criterion statuses"

echo "[5/12] audit --json emits file-level structure"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld audit json-flow -b HEAD --json"
assert_json "$output" "data['command'] == 'audit' and data['result']['counts']['undeclared'] == 0" "audit --json should report clean scope"
assert_json "$output" "'README.md' in data['result']['matched']" "audit --json should surface matched files"
assert_json "$output" "'README.md' in [f['path'] for f in data['result']['files'] if f['status'] == 'matched' and f['overlap'] == 'none']" "audit --json should expose file-level audit payloads"

echo "[6/12] audit --json allows explicitly shared overlap"
mkdir -p "$WS/plans"
write_spec "$WS/.scafld/specs/active/shared-flow.md" "shared-flow" "in_progress" "plans/shared.md" "shared"
write_spec "$WS/.scafld/specs/active/shared-peer.md" "shared-peer" "in_progress" "plans/shared.md" "shared"
printf 'shared\n' > "$WS/plans/shared.md"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld audit shared-flow --json"
assert_json "$output" "data['ok'] is True and data['result']['counts']['shared_with_other_active'] == 1" "shared overlap should stay clean in JSON mode"
assert_json "$output" "'plans/shared.md' in [f['path'] for f in data['result']['files'] if f['status'] == 'matched' and f['overlap'] == 'shared' and f['ownership'] == 'shared' and f['other_active_specs'] == ['shared-peer']]" "shared overlap should be explicit in file-level audit payloads"

echo "[7/12] audit --json still fails mixed ownership overlap"
write_spec "$WS/.scafld/specs/active/conflict-flow.md" "conflict-flow" "in_progress" "plans/conflict.md" "shared"
write_spec "$WS/.scafld/specs/active/conflict-peer.md" "conflict-peer" "in_progress" "plans/conflict.md"
if capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld audit conflict-flow --json"; then
  fail "mixed ownership overlap should fail audit --json"
fi
assert_json "$output" "data['ok'] is False and data['result']['counts']['active_overlap'] == 1" "mixed ownership overlap should register as a conflict"
assert_json "$output" "'plans/conflict.md' in [f['path'] for f in data['result']['files'] if f['status'] == 'missing' and f['overlap'] == 'conflict' and f['other_active_specs'] == ['conflict-peer']]" "conflicting overlap should be explicit even without a changed file"

echo "[8/12] fail --json emits archive transition"
write_spec "$WS/.scafld/specs/active/json-fail.md" "json-fail" "in_progress"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld fail json-fail --json"
assert_json "$output" "data['command'] == 'fail' and data['state']['status'] == 'failed'" "fail --json should emit failed status"

echo "[9/12] cancel --json emits archive transition"
write_spec "$WS/.scafld/specs/active/json-cancel.md" "json-cancel" "in_progress"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld cancel json-cancel --superseded-by json-fail --reason 'replaced by json-fail' --json"
assert_json "$output" "data['command'] == 'cancel' and data['state']['status'] == 'cancelled'" "cancel --json should emit cancelled status"
assert_json "$output" "data['result']['supersession']['superseded_by'] == 'json-fail' and data['result']['reason'] == 'replaced by json-fail'" "cancel --json should emit supersession metadata"

echo "[10/12] report --json aggregates machine-readable stats"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld report --json"
assert_json "$output" "data['command'] == 'report' and data['result']['total_specs'] >= 4" "report --json should emit aggregate totals"
assert_json "$output" "'completed' in data['result']['by_status'] or 'failed' in data['result']['by_status']" "report --json should emit status buckets"
assert_json "$output" "any(item['task_id'] == 'json-cancel' and item['superseded_by'] == 'json-fail' for item in data['result']['triage']['superseded'])" "report --json should expose superseded specs"

echo "[11/12] top-level errors use structured error envelopes"
if capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld status missing-task --json"; then
  fail "missing status --json should fail"
fi
assert_json "$output" "data['ok'] is False and data['error']['code'] == 'spec_not_found'" "status --json should emit structured spec_not_found errors"

echo "[12/12] report/update path stays parseable after prior transitions"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld report --json"
assert_json "$output" "data['result']['triage']['review_drift'] == []" "report --json should keep triage collections machine-readable"

echo "PASS: json contract smoke"
