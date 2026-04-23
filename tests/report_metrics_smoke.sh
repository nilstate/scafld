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
  repo="$(mktemp -d /tmp/scafld-report-metrics.XXXXXX)"
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

write_approved_spec() {
  local repo="$1"
  cat > "$repo/.ai/specs/approved/report-task.yaml" <<'EOF'
spec_version: "1.1"
task_id: "report-task"
created: "2026-04-23T00:00:00Z"
updated: "2026-04-23T00:00:00Z"
status: "approved"
harden_status: "not_run"

task:
  title: "Report metrics smoke"
  summary: "Exercise session-derived report metrics"
  size: "small"
  risk_level: "low"

planning_log:
  - timestamp: "2026-04-23T00:00:00Z"
    actor: "user"
    summary: "Bootstrap report metrics smoke fixture"

phases:
  - id: "phase1"
    name: "Write the report marker"
    objective: "metric.txt should end up green"
    changes:
      - file: "metric.txt"
        action: "update"
        lines: "1"
        content_spec: "replace red with green"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "metric.txt contains green"
        command: "grep -q '^green$' metric.txt"
        expected: "exit code 0"
    status: "pending"
EOF
}

repo="$(new_repo)"
write_approved_spec "$repo"

echo "[1/3] create one failure followed by one recovery"
(
  cd "$repo"
  printf 'red\n' > metric.txt
)
if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build report-task --json"; then
  fail "first build should fail"
fi
(
  cd "$repo"
  printf 'green\n' > metric.txt
)
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build report-task --json"
assert_json "$output" "data['ok'] is True" "second build should pass"

echo "[2/3] human report shows the LLM execution signals section"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld report"
assert_contains "$output" "LLM execution signals:" "report should show the session-derived section"
assert_contains "$output" "recovery convergence: 1/1" "report should show recovered criteria"
assert_contains "$output" "challenge override: none recorded" "report should show challenge override metrics"
assert_contains "$output" "attempts per phase: phase1=2" "report should show attempts per phase"
assert_contains "$output" "Per-task metrics" "report should show per-task runtime metrics"

echo "[3/3] json report exposes the same metrics"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld report --json"
assert_json "$output" "data['result']['llm_runtime']['first_attempt_pass_rate']['total'] == 1" "json report should count first attempts"
assert_json "$output" "data['result']['llm_runtime']['recovery_convergence_rate']['recovered'] == 1" "json report should count recovered criteria"
assert_json "$output" "data['result']['llm_runtime']['challenge_override_rate']['total'] == 0" "json report should expose challenge override totals"
assert_json "$output" "data['result']['llm_runtime']['attempts_per_phase']['phase1'] == 2" "json report should expose attempts per phase"
assert_json "$output" "data['result']['llm_runtime']['per_task']['report-task']['recovery_convergence_rate']['recovered'] == 1" "json report should expose per-task runtime metrics"

echo "PASS: report metrics smoke"
