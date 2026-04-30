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
  repo="$(mktemp -d /tmp/scafld-review-packet.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    scafld_cmd init >/dev/null
    printf 'base\n' > app.txt
    git add .
    git commit -m "init" >/dev/null 2>&1
  )
  printf '%s\n' "$repo"
}

repo="$(new_repo)"

echo "[1/5] valid packets normalize and project to gate-readable markdown"
REVIEW_REPO="$repo" REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
from pathlib import Path
from types import SimpleNamespace
import os
import sys

repo = Path(os.environ["REVIEW_REPO"])
sys.path.insert(0, os.environ["REPO_ROOT"])

from scafld.review_packet import (
    normalize_review_packet,
    review_packet_projection,
    write_executor_repair_handoff,
    write_review_packet_artifact,
)
from scafld.review_workflow import load_review_topology
from scafld.reviewing import build_review_metadata, parse_review_file
from scafld.review_workflow import render_review_round_text

topology = load_review_topology(repo)
pass_ids = ["regression_hunt", "convention_check", "dark_patterns"]
packet = {
    "schema_version": "review_packet.v1",
    "review_summary": "The packet smoke checked app.txt callers and found no issues.",
    "verdict": "pass",
    "pass_results": {pass_id: "pass" for pass_id in pass_ids},
    "checked_surfaces": [
        {
            "pass_id": pass_id,
            "targets": ["app.txt:1"],
            "summary": f"callers and rules touching app.txt for {pass_id}",
            "limitations": [],
        }
        for pass_id in pass_ids
    ],
    "findings": [],
}
normalized = normalize_review_packet(packet, topology, root=repo)
projection = review_packet_projection(normalized, topology)
assert projection["verdict"] == "pass", projection
assert projection["blocking"] == [], projection
assert "No issues found" in projection["sections"]["regression_hunt"], projection

packet_rel = write_review_packet_artifact(repo, "packet-task", 1, normalized)
repair_rel, repair_json_rel = write_executor_repair_handoff(repo, "packet-task", 1, normalized, packet_rel)
assert (repo / packet_rel).exists(), packet_rel
assert (repo / repair_rel).exists(), repair_rel
assert (repo / repair_json_rel).exists(), repair_json_rel
repair_text = (repo / repair_rel).read_text()
assert "Executor Review Repair" in repair_text, repair_text
assert "Do not apply spec suggestions blindly" in repair_text, repair_text

review_path = repo / ".scafld" / "reviews" / "packet-task.md"
review_path.parent.mkdir(parents=True, exist_ok=True)
metadata = build_review_metadata(
    topology,
    reviewer_mode="fresh_agent",
    round_status="completed",
    pass_results={
        "spec_compliance": "pass",
        "scope_drift": "pass",
        **projection["pass_results"],
    },
    reviewed_at="2026-04-26T00:00:00Z",
    reviewer_session="sess-1",
    reviewer_isolation="packet_smoke",
    review_provenance={"review_packet": packet_rel, "repair_handoff": repair_rel},
)
review_path.write_text(
    render_review_round_text(
        topology,
        metadata,
        1,
        verdict=projection["verdict"],
        blocking=projection["blocking"],
        non_blocking=projection["non_blocking"],
        section_bodies=projection["sections"],
    )
)
parsed = parse_review_file(review_path, topology)
assert not parsed["errors"], parsed
assert parsed["verdict"] == "pass", parsed
PY

echo "[2/5] rich finding packets carry executor repair context"
REVIEW_REPO="$repo" REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
from pathlib import Path
import os
import sys

repo = Path(os.environ["REVIEW_REPO"])
sys.path.insert(0, os.environ["REPO_ROOT"])

from scafld.review_packet import normalize_review_packet, review_packet_projection, write_executor_repair_handoff, write_review_packet_artifact
from scafld.review_workflow import load_review_topology

topology = load_review_topology(repo)
packet = {
    "schema_version": "review_packet.v1",
    "review_summary": "The challenger found one blocking ReviewPacket regression.",
    "verdict": "fail",
    "pass_results": {
        "regression_hunt": "fail",
        "convention_check": "pass",
        "dark_patterns": "pass",
    },
    "checked_surfaces": [
        {"pass_id": "regression_hunt", "targets": ["app.txt:1"], "summary": "callers of app.txt", "limitations": []},
        {"pass_id": "convention_check", "targets": ["AGENTS.md#review"], "summary": "review convention rules", "limitations": []},
        {"pass_id": "dark_patterns", "targets": ["app.txt:1"], "summary": "hardcoded and null paths in app.txt", "limitations": []},
    ],
    "findings": [
        {
            "id": "F1",
            "pass_id": "regression_hunt",
            "severity": "high",
            "blocking": True,
            "target": "app.txt:1",
            "summary": "app.txt does not carry packet repair context.",
            "failure_mode": "The executor would only see a verdict and not the repair evidence.",
            "why_it_matters": "The next LLM would have to rediscover the failure before updating the spec.",
            "evidence": ["app.txt:1 is the fixture target."],
            "suggested_fix": "Persist the packet and render a repair handoff.",
            "tests_to_add": ["Assert executor-review-repair.md contains this suggested fix."],
            "spec_update_suggestions": [
                {
                    "kind": "acceptance_criteria_add",
                    "phase_id": "phase1",
                    "suggested_text": "Repair handoff includes finding evidence and suggested fix context.",
                    "reason": "The executor needs review context without rediscovery.",
                    "validation_command": "bash tests/review_packet_smoke.sh",
                }
            ],
        }
    ],
}
normalized = normalize_review_packet(packet, topology, root=repo)
projection = review_packet_projection(normalized, topology)
assert projection["verdict"] == "fail", projection
assert projection["blocking"] == ["- **high** `app.txt:1` — app.txt does not carry packet repair context."], projection
packet_rel = write_review_packet_artifact(repo, "repair-task", 2, normalized)
repair_rel, _repair_json_rel = write_executor_repair_handoff(repo, "repair-task", 2, normalized, packet_rel)
repair_text = (repo / repair_rel).read_text()
assert "Failure mode: The executor would only see a verdict" in repair_text, repair_text
assert "Repair handoff includes finding evidence" in repair_text, repair_text
assert "bash tests/review_packet_smoke.sh" in repair_text, repair_text
PY

echo "[3/5] invalid packets are rejected before projection"
REVIEW_REPO="$repo" REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
from pathlib import Path
import os
import sys

repo = Path(os.environ["REVIEW_REPO"])
sys.path.insert(0, os.environ["REPO_ROOT"])

from scafld.errors import ScafldError
from scafld.review_packet import normalize_review_packet
from scafld.review_workflow import load_review_topology

topology = load_review_topology(repo)
bad = {
    "schema_version": "review_packet.v1",
    "reviewer_session": "model-controlled",
    "review_summary": "Bad packet.",
    "verdict": "pass",
    "pass_results": {
        "regression_hunt": "pass",
        "convention_check": "pass",
        "dark_patterns": "pass",
    },
    "checked_surfaces": [
        {"pass_id": "regression_hunt", "targets": ["everything"], "summary": "everything", "limitations": []},
    ],
    "findings": [],
}
try:
    normalize_review_packet(bad, topology, root=repo)
except ScafldError as exc:
    assert "scafld-owned fields" in " ".join([exc.message, *exc.details]), [exc.message, *exc.details]
else:
    raise AssertionError("invalid packet should fail")
PY

echo "[4/5] duplicate surfaces and markdown-breaking strings are rejected"
REVIEW_REPO="$repo" REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
from pathlib import Path
import copy
import os
import sys

repo = Path(os.environ["REVIEW_REPO"])
sys.path.insert(0, os.environ["REPO_ROOT"])

from scafld.errors import ScafldError
from scafld.review_packet import normalize_review_packet
from scafld.review_workflow import load_review_topology

topology = load_review_topology(repo)
base = {
    "schema_version": "review_packet.v1",
    "review_summary": "Base packet.",
    "verdict": "pass",
    "pass_results": {
        "regression_hunt": "pass",
        "convention_check": "pass",
        "dark_patterns": "pass",
    },
    "checked_surfaces": [
        {"pass_id": "regression_hunt", "targets": ["app.txt:1"], "summary": "regression callers", "limitations": []},
        {"pass_id": "convention_check", "targets": ["AGENTS.md#review"], "summary": "review rules", "limitations": []},
        {"pass_id": "dark_patterns", "targets": ["app.txt:1"], "summary": "dark-pattern paths", "limitations": []},
    ],
    "findings": [],
}

duplicate = copy.deepcopy(base)
duplicate["checked_surfaces"].append({
    "pass_id": "regression_hunt",
    "targets": ["app.txt:1"],
    "summary": "duplicate regression surface",
    "limitations": [],
})
try:
    normalize_review_packet(duplicate, topology, root=repo)
except ScafldError as exc:
    assert "duplicate pass ids" in " ".join([exc.message, *exc.details]), [exc.message, *exc.details]
else:
    raise AssertionError("duplicate checked_surfaces should fail")

multiline = copy.deepcopy(base)
multiline["checked_surfaces"][0]["summary"] = "line one\nline two"
try:
    normalize_review_packet(multiline, topology, root=repo)
except ScafldError as exc:
    assert "single-line string" in " ".join([exc.message, *exc.details]), [exc.message, *exc.details]
else:
    raise AssertionError("multiline packet fields should fail")

repair = copy.deepcopy(base)
repair["verdict"] = "fail"
repair["pass_results"]["regression_hunt"] = "fail"
repair["findings"] = [
    {
        "id": "F1",
        "pass_id": "regression_hunt",
        "severity": "high",
        "blocking": True,
        "target": "app.txt:1",
        "summary": "Injected repair field.",
        "failure_mode": "A repair field contains an embedded newline.",
        "why_it_matters": "The repair handoff would gain injected markdown.",
        "evidence": ["app.txt:1"],
        "suggested_fix": "Reject newline-containing repair fields.",
        "tests_to_add": ["ok\n- injected"],
        "spec_update_suggestions": [
            {
                "kind": "acceptance_criteria_add",
                "phase_id": "phase1",
                "suggested_text": "Reject newline-containing repair fields.",
                "reason": "line one\n- injected",
                "validation_command": "bash tests/review_packet_smoke.sh",
            }
        ],
    }
]
try:
    normalize_review_packet(repair, topology, root=repo)
except ScafldError as exc:
    assert "single-line string" in " ".join([exc.message, *exc.details]), [exc.message, *exc.details]
else:
    raise AssertionError("multiline repair fields should fail")

missing_target = copy.deepcopy(base)
missing_target["verdict"] = "fail"
missing_target["pass_results"]["regression_hunt"] = "fail"
missing_target["checked_surfaces"][0]["targets"] = ["does/not/exist.py:1"]
missing_target["findings"] = [
    {
        "id": "F1",
        "pass_id": "regression_hunt",
        "severity": "high",
        "blocking": True,
        "target": "does/not/exist.py:1",
        "summary": "Missing target.",
        "failure_mode": "The cited file does not exist.",
        "why_it_matters": "The repair handoff would point at fabricated evidence.",
        "evidence": ["does/not/exist.py:1"],
        "suggested_fix": "Reject missing file targets.",
        "tests_to_add": ["Assert missing file targets fail."],
        "spec_update_suggestions": [],
    }
]
try:
    normalize_review_packet(missing_target, topology, root=repo)
except ScafldError as exc:
    assert "does not exist" in " ".join([exc.message, *exc.details]), [exc.message, *exc.details]
else:
    raise AssertionError("missing file targets should fail")

out_of_range = copy.deepcopy(base)
out_of_range["verdict"] = "fail"
out_of_range["pass_results"]["regression_hunt"] = "fail"
out_of_range["findings"] = [
    {
        "id": "F1",
        "pass_id": "regression_hunt",
        "severity": "high",
        "blocking": True,
        "target": "app.txt:99",
        "summary": "Out-of-range target.",
        "failure_mode": "The cited line does not exist.",
        "why_it_matters": "The repair handoff would point beyond the evidence.",
        "evidence": ["app.txt:1 is the only fixture line."],
        "suggested_fix": "Reject out-of-range file targets.",
        "tests_to_add": ["Assert out-of-range file targets fail."],
        "spec_update_suggestions": [],
    }
]
try:
    normalize_review_packet(out_of_range, topology, root=repo)
except ScafldError as exc:
    assert "outside app.txt line count" in " ".join([exc.message, *exc.details]), [exc.message, *exc.details]
else:
    raise AssertionError("out-of-range file targets should fail")

missing_anchor = copy.deepcopy(base)
missing_anchor["verdict"] = "fail"
missing_anchor["pass_results"]["regression_hunt"] = "fail"
missing_anchor["checked_surfaces"][0]["targets"] = ["missing.md#review.external"]
missing_anchor["findings"] = [
    {
        "id": "F1",
        "pass_id": "regression_hunt",
        "severity": "high",
        "blocking": True,
        "target": "missing.md#nope",
        "summary": "Missing anchor file.",
        "failure_mode": "The cited anchor file does not exist.",
        "why_it_matters": "The repair handoff would point at fabricated documentation.",
        "evidence": ["missing.md#nope"],
        "suggested_fix": "Reject missing anchor files.",
        "tests_to_add": ["Assert missing anchor files fail."],
        "spec_update_suggestions": [],
    }
]
try:
    normalize_review_packet(missing_anchor, topology, root=repo)
except ScafldError as exc:
    assert "anchor file" in " ".join([exc.message, *exc.details]), [exc.message, *exc.details]
else:
    raise AssertionError("missing anchor files should fail")

owned_fields = copy.deepcopy(base)
owned_fields["isolation"] = "fake"
owned_fields["timing"] = "fake"
try:
    normalize_review_packet(owned_fields, topology, root=repo)
except ScafldError as exc:
    assert "scafld-owned fields" in " ".join([exc.message, *exc.details]), [exc.message, *exc.details]
else:
    raise AssertionError("scafld-owned top-level fields should fail")

too_many = copy.deepcopy(base)
too_many["verdict"] = "pass_with_issues"
too_many["pass_results"]["regression_hunt"] = "pass_with_issues"
too_many["findings"] = []
for index in range(11):
    too_many["findings"].append({
        "id": f"F{index}",
        "pass_id": "regression_hunt",
        "severity": "low",
        "blocking": False,
        "target": "app.txt:1",
        "summary": f"Extra finding {index}.",
        "failure_mode": "The packet exceeds the cap.",
        "why_it_matters": "Oversized repair handoffs are harder to consume.",
        "evidence": ["app.txt:1"],
        "suggested_fix": "Reject packets with too many findings.",
        "tests_to_add": ["Assert the finding cap is enforced."],
        "spec_update_suggestions": [],
    })
try:
    normalize_review_packet(too_many, topology, root=repo)
except ScafldError as exc:
    assert "at most 10" in " ".join([exc.message, *exc.details]), [exc.message, *exc.details]
else:
    raise AssertionError("packets with more than 10 findings should fail")
PY

echo "[5/5] rejected review projections do not persist packet artifacts"
REVIEW_REPO="$repo" REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
from pathlib import Path
from types import SimpleNamespace
import os
import sys

repo = Path(os.environ["REVIEW_REPO"])
sys.path.insert(0, os.environ["REPO_ROOT"])

from scafld.errors import ScafldError
from scafld.review_packet import normalize_review_packet
from scafld.review_workflow import complete_review_round_from_result, load_review_topology
from scafld.spec_markdown import render_spec_markdown
from tests.spec_fixture import basic_spec

topology = load_review_topology(repo)
packet = normalize_review_packet({
    "schema_version": "review_packet.v1",
    "review_summary": "Valid packet used to prove rejected projections do not persist artifacts.",
    "verdict": "pass",
    "pass_results": {
        "regression_hunt": "pass",
        "convention_check": "pass",
        "dark_patterns": "pass",
    },
    "checked_surfaces": [
        {"pass_id": "regression_hunt", "targets": ["app.txt:1"], "summary": "regression callers", "limitations": []},
        {"pass_id": "convention_check", "targets": ["AGENTS.md#review"], "summary": "review rules", "limitations": []},
        {"pass_id": "dark_patterns", "targets": ["app.txt:1"], "summary": "dark-pattern paths", "limitations": []},
    ],
    "findings": [],
}, topology, root=repo)
task_id = "reject-artifact-task"
review_file = repo / ".scafld" / "reviews" / f"{task_id}.md"
review_data = {
    "metadata": {
        "pass_results": {"spec_compliance": "pass", "scope_drift": "pass"},
        "review_handoff": ".scafld/runs/reject-artifact-task/handoffs/challenger-review.md",
    },
    "review_count": 1,
}
malformed_finding = "- **high** `app.txt:1` — first line\ncontinued line"
runner_result = SimpleNamespace(
    reviewer_mode="fresh_agent",
    reviewer_session="",
    reviewer_isolation="packet_smoke",
    pass_results={"regression_hunt": "fail", "convention_check": "pass", "dark_patterns": "pass"},
    sections={
        "regression_hunt": malformed_finding,
        "convention_check": "No issues found — checked AGENTS.md#review.",
        "dark_patterns": "No issues found — checked app.txt:1.",
    },
    blocking=[malformed_finding],
    non_blocking=[],
    verdict="fail",
    provenance={"runner": "external", "prompt_sha256": "packet-smoke"},
    raw_output="{}",
    packet=packet,
)
try:
    spec_text = render_spec_markdown(
        basic_spec(
            task_id,
            status="in_progress",
            title="Reject Artifact Task",
            file_path="app.txt",
            command="true",
        )
    )
    complete_review_round_from_result(repo, review_file, task_id, spec_text, topology, review_data, runner_result)
except ScafldError as exc:
    assert "invalid review artifact" in exc.message, [exc.message, *exc.details]
else:
    raise AssertionError("malformed projection should fail")

assert not (repo / ".scafld" / "runs" / task_id / "review-packets" / "review-1.json").exists()
assert not (repo / ".scafld" / "runs" / task_id / "handoffs" / "executor-review-repair.md").exists()
assert "review_packet" not in runner_result.provenance, runner_result.provenance
PY

echo "PASS: review packet smoke"
