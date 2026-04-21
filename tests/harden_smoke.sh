#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI="$REPO_ROOT/cli/scafld"
TMP_DIRS=()
source "$SCRIPT_DIR/smoke_lib.sh"

run_scafld() {
  (cd "$WS" && python3 "$CLI" "$@")
}

field() {
  python3 -c "import yaml; d=yaml.safe_load(open('$1')); v=d; [v:=v[k] for k in '$2'.split('.') if v is not None and k in v]; print(v if not isinstance(v,(dict,list)) else '<complex>')" 2>/dev/null || true
}

WS=$(mktemp -d)
TMP_DIRS+=("$WS")

echo "[1/13] scafld init"
run_scafld init >/dev/null || fail "scafld init failed"
[ -d "$WS/.ai/specs/drafts" ] || fail "init did not create drafts/"

echo "[2/13] scafld new emits harden_status: not_run"
run_scafld new t1 -t 'test one' -s small -r low >/dev/null || fail "scafld new t1 failed"
grep -q 'harden_status: "not_run"' "$WS/.ai/specs/drafts/t1.yaml" \
  || fail "spec from template missing harden_status: not_run"

echo "[3/13] scafld harden prints HARDEN MODE and advances to in_progress"
output=$(run_scafld harden t1)
[[ "$output" == *"HARDEN MODE"* ]] || fail "harden prompt missing HARDEN MODE header"
python3 - <<PY || fail "harden did not set in_progress / append round"
import yaml
d = yaml.safe_load(open("$WS/.ai/specs/drafts/t1.yaml"))
assert d.get("harden_status") == "in_progress", d.get("harden_status")
assert len(d.get("harden_rounds") or []) == 1, d.get("harden_rounds")
PY

echo "[4/13] scafld harden --mark-passed closes the round"
run_scafld harden t1 --mark-passed >/dev/null || fail "mark-passed failed"
python3 - <<PY || fail "mark-passed did not set passed / close round"
import yaml
d = yaml.safe_load(open("$WS/.ai/specs/drafts/t1.yaml"))
assert d.get("harden_status") == "passed", d.get("harden_status")
r = d["harden_rounds"][-1]
assert r.get("outcome") == "passed", r.get("outcome")
assert r.get("ended_at"), "round missing ended_at"
PY

echo "[5/13] re-running harden appends a round and resets to in_progress"
run_scafld harden t1 >/dev/null || fail "re-run harden failed"
python3 - <<PY || fail "re-run did not append round or reset status"
import yaml
d = yaml.safe_load(open("$WS/.ai/specs/drafts/t1.yaml"))
assert d.get("harden_status") == "in_progress", d.get("harden_status")
assert len(d["harden_rounds"]) == 2, d["harden_rounds"]
PY

echo "[6/13] scafld harden --json opens a structured round"
output=$(run_scafld harden t1 --json)
assert_json "$output" "data['command'] == 'harden' and data['result']['action'] == 'round_opened'" "harden --json should emit a round_opened action"
assert_json "$output" "data['state']['harden_status'] == 'in_progress' and data['state']['round'] == 3" "harden --json should emit round and status"

echo "[7/13] scafld harden --mark-passed --json closes the round structurally"
output=$(run_scafld harden t1 --mark-passed --json)
assert_json "$output" "data['command'] == 'harden' and data['result']['action'] == 'round_passed'" "mark-passed --json should emit a round_passed action"
assert_json "$output" "data['state']['harden_status'] == 'passed' and data['state']['round'] == 3" "mark-passed --json should emit passed status"

echo "[8/13] scafld approve works regardless of harden_status"
# t1 currently is in_progress; substitute a minimal valid spec for approve test
cat > "$WS/.ai/specs/drafts/t2.yaml" <<'YAML'
spec_version: "1.1"
task_id: "t2"
created: "2026-04-20T02:00:00Z"
updated: "2026-04-20T02:00:00Z"
status: "draft"
harden_status: "not_run"

task:
  title: "Fixture"
  summary: "A minimal test fixture for verifying approve behaviour."
  size: "small"
  risk_level: "low"
  context:
    packages:
      - "cli"
    invariants:
      - "domain_boundaries"
  objectives:
    - "Verify approve behaviour."
  touchpoints:
    - area: "cli"
      description: "Fixture."
  acceptance:
    definition_of_done:
      - id: "dod1"
        description: "Approve works."
        status: "pending"
    validation:
      - id: "v1"
        type: "test"
        description: "noop"
        command: "true"
        expected: "0 failures"

planning_log:
  - timestamp: "2026-04-20T02:00:00Z"
    actor: "user"
    summary: "Fixture."

phases:
  - id: "phase1"
    name: "Phase"
    objective: "Noop."
    changes:
      - file: "none"
        action: "update"
        content_spec: "noop"
    acceptance_criteria:
      - id: "ac1_1"
        type: "test"
        description: "noop"
        command: "true"
        expected: "0 failures"
    status: "pending"

rollback:
  strategy: "per_phase"
  commands:
    phase1: "true"
YAML
run_scafld approve t2 >/dev/null || fail "approve refused a fixture with harden_status not_run"

echo "[9/13] scafld approve works when harden_status is missing entirely"
# Remove harden_status and re-approve a fresh fixture
cp "$WS/.ai/specs/approved/t2.yaml" "$WS/.ai/specs/drafts/t3.yaml"
python3 -c "
import yaml
p = '$WS/.ai/specs/drafts/t3.yaml'
d = yaml.safe_load(open(p))
d['task_id'] = 't3'
d['status'] = 'draft'
d.pop('harden_status', None)
open(p, 'w').write(yaml.safe_dump(d, sort_keys=False, default_flow_style=False))
"
# scafld uses a regex that requires indented block style for phases - rewrite using raw YAML
cat > "$WS/.ai/specs/drafts/t3.yaml" <<'YAML'
spec_version: "1.1"
task_id: "t3"
created: "2026-04-20T02:00:00Z"
updated: "2026-04-20T02:00:00Z"
status: "draft"

task:
  title: "Fixture"
  summary: "A minimal test fixture for verifying approve behaviour without harden field."
  size: "small"
  risk_level: "low"
  context:
    packages:
      - "cli"
    invariants:
      - "domain_boundaries"
  objectives:
    - "Verify legacy approve."
  touchpoints:
    - area: "cli"
      description: "Fixture."
  acceptance:
    definition_of_done:
      - id: "dod1"
        description: "Approve works."
        status: "pending"
    validation:
      - id: "v1"
        type: "test"
        description: "noop"
        command: "true"
        expected: "0 failures"

planning_log:
  - timestamp: "2026-04-20T02:00:00Z"
    actor: "user"
    summary: "Fixture."

phases:
  - id: "phase1"
    name: "Phase"
    objective: "Noop."
    changes:
      - file: "none"
        action: "update"
        content_spec: "noop"
    acceptance_criteria:
      - id: "ac1_1"
        type: "test"
        description: "noop"
        command: "true"
        expected: "0 failures"
    status: "pending"

rollback:
  strategy: "per_phase"
  commands:
    phase1: "true"
YAML
run_scafld approve t3 >/dev/null || fail "approve refused a spec with harden_status missing"

echo "[10/13] valid harden citations stay silent on --mark-passed"
run_scafld new t5 -t 'valid citations' -s small -r low >/dev/null || fail "scafld new t5 failed"
run_scafld harden t5 >/dev/null || fail "scafld harden t5 failed"
mkdir -p "$WS/.ai/specs/archive/2026-04"
cat > "$WS/.ai/specs/archive/2026-04/archived-ok.yaml" <<'YAML'
spec_version: "1.1"
task_id: "archived-ok"
created: "2026-04-20T02:00:00Z"
updated: "2026-04-20T02:00:00Z"
status: "completed"
task:
  title: "Archived fixture"
  summary: "Fixture."
  size: "small"
  risk_level: "low"
  context:
    packages:
      - "cli"
    invariants:
      - "domain_boundaries"
  objectives:
    - "Fixture."
  touchpoints:
    - area: "cli"
      description: "Fixture."
  acceptance:
    definition_of_done:
      - id: "dod1"
        description: "Fixture."
        status: "completed"
    validation:
      - id: "v1"
        type: "test"
        description: "noop"
        command: "true"
        expected: "0 failures"
planning_log:
  - timestamp: "2026-04-20T02:00:00Z"
    actor: "user"
    summary: "Fixture."
phases:
  - id: "phase1"
    name: "Fixture"
    objective: "Fixture."
    changes:
      - file: "AGENTS.md"
        action: "update"
        content_spec: "Fixture."
    acceptance_criteria:
      - id: "ac1_1"
        type: "test"
        description: "noop"
        command: "true"
        expected: "0 failures"
    status: "completed"
rollback:
  strategy: "per_phase"
  commands:
    phase1: "true"
YAML
python3 - <<PY || fail "could not write valid harden citations"
import yaml
p = "$WS/.ai/specs/drafts/t5.yaml"
d = yaml.safe_load(open(p))
d["harden_rounds"][-1]["questions"] = [
    {"question": "Does AGENTS exist?", "grounded_in": "code:AGENTS.md:1"},
    {"question": "Does archived fixture exist?", "grounded_in": "archive:archived-ok"},
]
open(p, "w").write(yaml.safe_dump(d, sort_keys=False))
PY
output=$(run_scafld harden t5 --mark-passed)
[[ "$output" != *"could not be resolved"* ]] || fail "valid citations unexpectedly warned"

echo "[11/13] invalid harden citations warn but still pass"
run_scafld new t6 -t 'invalid citations' -s small -r low >/dev/null || fail "scafld new t6 failed"
run_scafld harden t6 >/dev/null || fail "scafld harden t6 failed"
python3 - <<PY || fail "could not write invalid harden citations"
import yaml
p = "$WS/.ai/specs/drafts/t6.yaml"
d = yaml.safe_load(open(p))
d["harden_rounds"][-1]["questions"] = [
    {"question": "Fake file?", "grounded_in": "code:missing.py:99"},
    {"question": "Fake archive?", "grounded_in": "archive:not-real"},
]
open(p, "w").write(yaml.safe_dump(d, sort_keys=False))
PY
output=$(run_scafld harden t6 --mark-passed) || fail "mark-passed should warn, not fail"
[[ "$output" == *"could not be resolved"* ]] || fail "invalid citations did not warn"
[[ "$output" == *"code citation not found"* ]] || fail "missing code citation warning absent"
[[ "$output" == *"archive citation not found"* ]] || fail "missing archive citation warning absent"

echo "[12/13] --mark-passed refuses when harden_rounds is empty"
run_scafld new t4 -t 'empty rounds' -s small -r low >/dev/null || fail "scafld new t4 failed"
set +e
(cd "$WS" && python3 "$CLI" harden t4 --mark-passed >/dev/null 2>&1)
rc=$?
set -e
[ "$rc" -ne 0 ] || fail "--mark-passed should exit non-zero when harden_rounds is empty"

echo "[13/13] cmd_approve body does not reference harden (anti-drift)"
python3 - <<PY || fail "cmd_approve unexpectedly references harden"
import re
src = open("$REPO_ROOT/cli/scafld").read()
m = re.search(r'def cmd_approve\(.*?(?=\ndef )', src, re.DOTALL)
assert m, "cmd_approve not found"
assert 'harden' not in m.group(0).lower(), "cmd_approve references harden - gate was added"
PY

echo "PASS: harden smoke"
