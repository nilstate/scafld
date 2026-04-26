#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TMP_DIRS=()
source "$SCRIPT_DIR/smoke_lib.sh"

ROOT="$(mktemp -d /tmp/scafld-run-contracts.XXXXXX)"
TMP_DIRS+=("$ROOT")

mkdir -p "$ROOT/.ai"
cat > "$ROOT/.ai/config.yaml" <<'EOF'
llm:
  model_profile: "smoke-profile"
  context:
    budget_tokens: 4096
  recovery:
    max_attempts: 2
EOF

ROOT="$ROOT" REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
import os
import pathlib
import sys

root = pathlib.Path(os.environ["ROOT"])
sys.path.insert(0, os.environ["REPO_ROOT"])

from scafld.runtime_contracts import ensure_run_dirs, handoff_json_path, handoff_path, load_llm_settings, review_packets_dir, session_path
from scafld.session_store import ensure_session
from scafld.spec_parsing import parse_acceptance_criteria

paths = ensure_run_dirs(root, "demo-task")
assert session_path(root, "demo-task") == paths["session_path"]
assert handoff_path(root, "demo-task", role="executor", gate="phase", selector="phase1").relative_to(root).as_posix() == ".ai/runs/demo-task/handoffs/executor-phase-phase1.md"
assert handoff_path(root, "demo-task", role="executor", gate="recovery", selector="ac1_1", attempt=2).relative_to(root).as_posix() == ".ai/runs/demo-task/handoffs/executor-recovery-ac1_1-2.md"
assert handoff_json_path(root, "demo-task", role="executor", gate="phase", selector="phase1").relative_to(root).as_posix() == ".ai/runs/demo-task/handoffs/executor-phase-phase1.json"
assert handoff_path(root, "demo-task", role="challenger", gate="review").relative_to(root).as_posix() == ".ai/runs/demo-task/handoffs/challenger-review.md"
assert handoff_path(root, "demo-task", role="executor", gate="review_repair").relative_to(root).as_posix() == ".ai/runs/demo-task/handoffs/executor-review-repair.md"
assert review_packets_dir(root, "demo-task").relative_to(root).as_posix() == ".ai/runs/demo-task/review-packets"
assert paths["review_packets_dir"].relative_to(root).as_posix() == ".ai/runs/demo-task/review-packets"

agent_guide = (pathlib.Path(os.environ["REPO_ROOT"]) / "AGENTS.md").read_text()
assert "executor × review_repair" in agent_guide, agent_guide
assert "executor-review-repair.md" in agent_guide, agent_guide

settings = load_llm_settings(root)
assert settings["model_profile"] == "smoke-profile", settings
assert settings["context_budget_tokens"] == 4096, settings
assert settings["recovery_max_attempts"] == 2, settings

session = ensure_session(root, "demo-task")
assert session["schema_version"] == 3, session
assert session["model_profile"] == "smoke-profile", session
assert session["entries"] == [], session
assert session["workspace_baseline"] is None, session

spec_text = """
phases:
  - id: "phase1"
    acceptance_criteria:
      - id: "ac1_1"
        type: "documentation"
        description: "folded command support"
        command: >
          bash -lc 'printf hello'
        expected: "exit code 0"
"""
criteria = parse_acceptance_criteria(spec_text)
assert criteria[0]["command"].startswith("bash -lc"), criteria
PY

echo "PASS: run contracts smoke"
