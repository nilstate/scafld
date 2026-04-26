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

echo "[1/5] create one failure followed by one recovery"
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

echo "[2/5] record provider telemetry for attribution reporting"
REPO_UNDER_TEST="$repo" PYTHONPATH="$REPO_ROOT" python3 - <<'PY'
import os
from pathlib import Path
from scafld.session_store import record_provider_invocation, session_summary_payload

root = Path(os.environ["REPO_UNDER_TEST"])
record_provider_invocation(
    root,
    "report-task",
    role="executor",
    gate="build",
    provider="codex",
    provider_bin="codex",
    model_requested="gpt-smoke",
    model_observed="",
    model_source="requested",
    isolation_level="provider_adapter",
)
record_provider_invocation(
    root,
    "report-task",
    role="challenger",
    gate="review",
    provider="claude",
    provider_bin="claude",
    provider_requested="auto",
    model_requested="",
    model_observed="",
    model_source="unknown",
    isolation_level="claude_restricted_tools_fresh_session",
    isolation_downgraded=True,
    fallback_policy="warn",
)
record_provider_invocation(
    root,
    "report-task",
    role="challenger",
    gate="review",
    provider="codex",
    provider_bin="codex",
    provider_requested="codex",
    model_requested="",
    model_observed="gpt-5.3-codex",
    model_source="inferred",
    isolation_level="codex_read_only_ephemeral",
)
try:
    record_provider_invocation(
        root,
        "report-task",
        role="challenger",
        gate="review",
        provider="codex",
        status="sort_of_okay",
    )
except ValueError as exc:
    assert "provider invocation status" in str(exc)
else:
    raise AssertionError("invalid provider invocation status should fail")

try:
    record_provider_invocation(
        root,
        "report-task",
        role="challenger",
        gate="review",
        provider="codex",
        confidence="maybe",
    )
except ValueError as exc:
    assert "provider invocation confidence" in str(exc)
else:
    raise AssertionError("invalid provider invocation confidence should fail")

def separation_state(entries):
    return session_summary_payload({"entries": entries, "attempts": []})["provider_model_separation"]["state"]

assert separation_state([]) == "none"
assert separation_state([
    {"type": "provider_invocation", "role": "executor", "model_observed": "gpt-a"},
]) == "unknown_challenger"
assert separation_state([
    {"type": "provider_invocation", "role": "challenger", "model_observed": "gpt-b"},
]) == "unknown_executor"
assert separation_state([
    {"type": "provider_invocation", "role": "executor", "model_observed": ""},
    {"type": "provider_invocation", "role": "challenger", "model_observed": ""},
]) == "unknown_both"
assert separation_state([
    {"type": "provider_invocation", "role": "executor", "model_observed": "gpt-a"},
    {"type": "provider_invocation", "role": "challenger", "model_observed": "gpt-a"},
]) == "same_model"
assert separation_state([
    {"type": "provider_invocation", "role": "executor", "model_observed": "gpt-a"},
    {"type": "provider_invocation", "role": "challenger", "model_observed": "gpt-b"},
]) == "separated"
assert separation_state([
    {"type": "provider_invocation", "role": "executor", "model_observed": "gpt-a", "confidence": "observed"},
    {"type": "provider_invocation", "role": "challenger", "model_observed": "gpt-b", "confidence": "inferred"},
]) == "unknown_challenger"
legacy_summary = session_summary_payload({
    "entries": [
        {
            "type": "provider_invocation",
            "role": "challenger",
            "gate": "review",
            "status": "legacy_status",
            "confidence": "legacy_confidence",
            "isolation_level": "claude_restricted_tools_fresh_session",
        }
    ],
    "attempts": [],
})
assert legacy_summary["provider_statuses"]["legacy_status"] == 1
assert legacy_summary["provider_confidence"]["legacy_confidence"] == 1
assert legacy_summary["provider_weaker_review_isolation"] == 1
PY

echo "[3/5] human report shows the LLM execution signals section"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld report"
assert_contains "$output" "LLM execution signals:" "report should show the session-derived section"
assert_contains "$output" "recovery convergence: 1/1" "report should show recovered criteria"
assert_contains "$output" "challenge override: none recorded" "report should show challenge override metrics"
assert_contains "$output" "attempts per phase: phase1=2" "report should show attempts per phase"
assert_contains "$output" "provider invocations: 3" "report should show provider invocation totals"
assert_contains "$output" "models observed 0, inferred 1, unknown 2" "report should distinguish observed, inferred, and unknown model counts"
assert_contains "$output" "provider confidence: inferred=1, requested_only=1, unknown=1" "report should show provider confidence counts"
assert_contains "$output" "provider statuses: completed=3" "report should show provider invocation statuses"
assert_contains "$output" "isolation downgrades: 1/3" "report should show provider isolation downgrade denominator"
assert_contains "$output" "weaker review isolation: 1/3" "report should show weaker review isolation denominator"
assert_contains "$output" "model separation: unknown_both=1" "report should show unknown model separation"
assert_contains "$output" "Per-task metrics" "report should show per-task runtime metrics"

echo "[4/5] json report exposes the same metrics"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld report --json"
assert_json "$output" "data['result']['llm_runtime']['first_attempt_pass_rate']['total'] == 1" "json report should count first attempts"
assert_json "$output" "data['result']['llm_runtime']['recovery_convergence_rate']['recovered'] == 1" "json report should count recovered criteria"
assert_json "$output" "data['result']['llm_runtime']['challenge_override_rate']['total'] == 0" "json report should expose challenge override totals"
assert_json "$output" "data['result']['llm_runtime']['attempts_per_phase']['phase1'] == 2" "json report should expose attempts per phase"
assert_json "$output" "data['result']['llm_runtime']['provider_telemetry']['invocations'] == 3" "json report should expose provider invocation totals"
assert_json "$output" "data['result']['llm_runtime']['provider_telemetry']['confidence']['inferred'] == 1" "json report should expose inferred confidence counts"
assert_json "$output" "data['result']['llm_runtime']['provider_telemetry']['confidence']['requested_only'] == 1" "json report should expose requested-only confidence counts"
assert_json "$output" "data['result']['llm_runtime']['provider_telemetry']['confidence']['unknown'] == 1" "json report should expose unknown confidence counts"
assert_json "$output" "data['result']['llm_runtime']['provider_telemetry']['statuses']['completed'] == 3" "json report should expose provider status counts"
assert_json "$output" "data['result']['llm_runtime']['provider_telemetry']['models_observed'] == 0" "json report should expose observed provider model counts"
assert_json "$output" "data['result']['llm_runtime']['provider_telemetry']['models_inferred'] == 1" "json report should expose inferred provider model counts"
assert_json "$output" "data['result']['llm_runtime']['provider_telemetry']['models_unknown'] == 2" "json report should expose unknown provider model counts"
assert_json "$output" "data['result']['llm_runtime']['provider_telemetry']['isolation_downgrades'] == 1" "json report should expose isolation downgrade counts"
assert_json "$output" "data['result']['llm_runtime']['provider_telemetry']['weaker_review_isolation'] == 1" "json report should expose weaker review isolation counts"
assert_json "$output" "data['result']['llm_runtime']['provider_telemetry']['model_separation']['unknown_both'] == 1" "json report should expose unknown model separation"
assert_json "$output" "data['result']['llm_runtime']['per_task']['report-task']['recovery_convergence_rate']['recovered'] == 1" "json report should expose per-task runtime metrics"
assert_json "$output" "data['result']['llm_runtime']['per_task']['report-task']['provider_telemetry']['isolation_downgrades'] == 1" "json report should expose per-task provider telemetry"
assert_json "$output" "data['result']['llm_runtime']['per_task']['report-task']['provider_telemetry']['weaker_review_isolation'] == 1" "json report should expose per-task weaker isolation telemetry"
assert_json "$output" "'review_signal' in data['result'] and 'format_compliant_clean_reviews' in data['result']['review_signal'] and 'clean_reviews_with_evidence' in data['result']['review_signal']" "json report should expose renamed review signal metrics and legacy alias"

echo "[5/5] runtime-only report filters to the session cohort"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld report --json --runtime-only"
assert_json "$output" "data['result']['runtime_only'] is True" "runtime-only report should flag the filtered cohort"
assert_json "$output" "data['result']['total_specs'] == 1" "runtime-only report should keep only specs with runtime sessions"
assert_json "$output" "data['result']['by_status']['in_progress'] == 1" "runtime-only report should preserve the runtime cohort status counts"

echo "PASS: report metrics smoke"
