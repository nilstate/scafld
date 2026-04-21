import datetime
import hashlib
import json
import os
import re
import sys
import sysconfig
from pathlib import Path

from scafld._version import __version__ as VERSION
from scafld.config import load_config as load_workspace_config, parse_yaml_value
from scafld.git_state import source_git_metadata
from scafld.reviewing import build_review_topology, render_review_pass_results, review_passes_by_kind
from scafld.spec_store import yaml_read_field, yaml_read_nested

AI_DIR = ".ai"
FRAMEWORK_DIR = f"{AI_DIR}/scafld"
SPECS_DIR = f"{AI_DIR}/specs"
DRAFTS_DIR = f"{SPECS_DIR}/drafts"
APPROVED_DIR = f"{SPECS_DIR}/approved"
ACTIVE_DIR = f"{SPECS_DIR}/active"
ARCHIVE_DIR = f"{SPECS_DIR}/archive"
LOGS_DIR = f"{AI_DIR}/logs"
REVIEWS_DIR = f"{AI_DIR}/reviews"
SCHEMA_PATH = f"{AI_DIR}/schemas/spec.json"
CONFIG_PATH = f"{AI_DIR}/config.yaml"
CONFIG_LOCAL_PATH = f"{AI_DIR}/config.local.yaml"
FRAMEWORK_CONFIG_PATH = f"{FRAMEWORK_DIR}/config.yaml"
FRAMEWORK_SCHEMA_PATH = f"{FRAMEWORK_DIR}/schemas/spec.json"
FRAMEWORK_MANIFEST_PATH = f"{FRAMEWORK_DIR}/manifest.json"
FRAMEWORK_MANIFEST_SCHEMA_VERSION = 1
FRAMEWORK_BUNDLE_FILES = [
    (f"{AI_DIR}/config.yaml", FRAMEWORK_CONFIG_PATH),
    (f"{AI_DIR}/schemas/spec.json", FRAMEWORK_SCHEMA_PATH),
    (f"{AI_DIR}/prompts/plan.md", f"{FRAMEWORK_DIR}/prompts/plan.md"),
    (f"{AI_DIR}/prompts/harden.md", f"{FRAMEWORK_DIR}/prompts/harden.md"),
    (f"{AI_DIR}/prompts/exec.md", f"{FRAMEWORK_DIR}/prompts/exec.md"),
    (f"{AI_DIR}/prompts/review.md", f"{FRAMEWORK_DIR}/prompts/review.md"),
    (f"{AI_DIR}/README.md", f"{FRAMEWORK_DIR}/README.md"),
    (f"{AI_DIR}/OPERATORS.md", f"{FRAMEWORK_DIR}/OPERATORS.md"),
    (f"{SPECS_DIR}/README.md", f"{FRAMEWORK_DIR}/specs/README.md"),
    (f"{SPECS_DIR}/examples/add-error-codes.yaml", f"{FRAMEWORK_DIR}/specs/examples/add-error-codes.yaml"),
]
AUDIT_IGNORED_PREFIXES = (
    f"{SPECS_DIR}/",
    f"{REVIEWS_DIR}/",
    f"{LOGS_DIR}/",
)
AUDIT_IGNORED_FILES = {CONFIG_LOCAL_PATH}
CHANGE_OWNERSHIP_VALUES = {"exclusive", "shared"}

C_RESET = "\033[0m"
C_BOLD = "\033[1m"
C_DIM = "\033[2m"
C_RED = "\033[31m"
C_GREEN = "\033[32m"
C_YELLOW = "\033[33m"
C_BLUE = "\033[34m"
C_MAGENTA = "\033[35m"
C_CYAN = "\033[36m"

STATUS_COLORS = {
    "draft": C_DIM,
    "under_review": C_YELLOW,
    "approved": C_BLUE,
    "in_progress": C_CYAN,
    "completed": C_GREEN,
    "failed": C_RED,
    "cancelled": C_DIM,
}

DEFAULT_ACCEPTANCE_TIMEOUT_SECONDS = 600
GENERIC_PASS_EXPECTATIONS = {"all pass", "all tests pass", "all specs pass"}


def git_sync_excluded_paths():
    """Ignore scafld control-plane artifacts when computing git sync drift."""
    return sorted((*AUDIT_IGNORED_PREFIXES, *AUDIT_IGNORED_FILES))


def require_pyyaml():
    """Import PyYAML with an actionable error for source/npm installs."""
    try:
        import yaml
    except ModuleNotFoundError as exc:
        print(f"{c(C_RED, 'error')}: scafld harden requires PyYAML")
        print("  install it into the Python runtime that executes scafld:")
        print(f"  {c(C_BOLD, 'python3 -m pip install PyYAML')}")
        raise SystemExit(1) from exc
    return yaml


def supports_color():
    """Check if terminal supports color."""
    if os.environ.get("NO_COLOR"):
        return False
    if not hasattr(sys.stdout, "isatty") or not sys.stdout.isatty():
        return False
    return True


USE_COLOR = supports_color()


def c(code, text):
    """Colorize text if terminal supports it."""
    if not USE_COLOR:
        return text
    return f"{code}{text}{C_RESET}"


def now_iso():
    return datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).strftime("%Y-%m-%dT%H:%M:%SZ")


def scafld_source_root():
    """Directory that contains the canonical scafld source tree."""
    override = os.environ.get("SCAFLD_SOURCE_ROOT")
    if override:
        return Path(override).expanduser().resolve()

    candidates = [
        Path(__file__).resolve().parents[2],
        Path(sysconfig.get_path("data")) / "share" / "scafld",
    ]
    for candidate in candidates:
        if (candidate / "cli" / "scafld").exists():
            return candidate
    return candidates[0]


def sha256_bytes(payload):
    return hashlib.sha256(payload).hexdigest()


def write_bytes_if_changed(path, payload):
    """Write bytes only when the content changes. Returns created/updated/unchanged."""
    if path.exists():
        if path.read_bytes() == payload:
            return "unchanged"
        path.write_bytes(payload)
        return "updated"

    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(payload)
    return "created"


def resolve_framework_asset(root, legacy_rel, managed_rel):
    """Prefer the managed framework bundle when it exists, else fall back to legacy paths."""
    managed = root / FRAMEWORK_DIR / managed_rel
    if managed.exists():
        return managed
    return root / legacy_rel


def resolve_schema_path(root):
    return resolve_framework_asset(root, SCHEMA_PATH, "schemas/spec.json")


def resolve_prompt_path(root, name):
    return resolve_framework_asset(root, f"{AI_DIR}/prompts/{name}", f"prompts/{name}")


def build_framework_manifest(workspace_root, source_root, managed_assets):
    """Build deterministic metadata that proves which scafld bundle a workspace uses."""
    git_metadata = source_git_metadata(source_root)
    return {
        "schema_version": FRAMEWORK_MANIFEST_SCHEMA_VERSION,
        "scafld_version": VERSION,
        "source_commit": git_metadata["commit"],
        "source_dirty": git_metadata["dirty"],
        "workspace_config_mode": "legacy_overlay" if (workspace_root / CONFIG_PATH).exists() else "managed_only",
        "managed_assets": managed_assets,
    }


def sync_framework_bundle(workspace_root, source_root=None):
    """Install/update the framework-managed runtime bundle inside a workspace."""
    source_root = source_root or scafld_source_root()
    created = []
    updated = []
    unchanged = []
    managed_assets = {}

    for src_rel, dest_rel in FRAMEWORK_BUNDLE_FILES:
        src = source_root / src_rel
        if not src.exists():
            continue

        payload = src.read_bytes()
        managed_assets[dest_rel] = {
            "source": src_rel,
            "sha256": sha256_bytes(payload),
        }

        dest = workspace_root / dest_rel
        status = write_bytes_if_changed(dest, payload)
        if status == "created":
            created.append(dest_rel)
        elif status == "updated":
            updated.append(dest_rel)
        else:
            unchanged.append(dest_rel)

    manifest_payload = build_framework_manifest(workspace_root, source_root, managed_assets)
    manifest_text = (json.dumps(manifest_payload, indent=2, sort_keys=True) + "\n").encode()
    manifest_status = write_bytes_if_changed(workspace_root / FRAMEWORK_MANIFEST_PATH, manifest_text)

    return {
        "workspace": workspace_root,
        "created": created,
        "updated": updated,
        "unchanged": unchanged,
        "manifest_status": manifest_status,
        "manifest_path": workspace_root / FRAMEWORK_MANIFEST_PATH,
        "manifest": manifest_payload,
    }


def load_config(root):
    """Load config with local overlay support."""
    return load_workspace_config(root, FRAMEWORK_CONFIG_PATH, CONFIG_PATH, CONFIG_LOCAL_PATH)


def parse_numeric_scalar(value):
    """Parse a YAML scalar that should contain a numeric value."""
    if value in (None, "", "null", "{}"):
        return None
    try:
        return float(str(value).strip().strip('"').strip("'"))
    except (TypeError, ValueError):
        return None


def extract_self_eval_score(text):
    """Read the recorded self-eval score from supported spec shapes."""
    for parent in ("self_eval", "perf_eval"):
        for field in ("total", "score"):
            score = parse_numeric_scalar(yaml_read_nested(text, parent, field))
            if score is not None:
                return score
        score = parse_numeric_scalar(yaml_read_field(text, parent))
        if score is not None:
            return score

    return parse_numeric_scalar(yaml_read_field(text, "score"))


def parse_phase_status_entries(text):
    """Parse phase ids/statuses from the phases block without counting unrelated statuses."""
    lines = text.splitlines()
    entries = []
    in_phases = False
    i = 0

    while i < len(lines):
        line = lines[i]
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())

        if not in_phases:
            if re.match(r'^phases:\s*$', line):
                in_phases = True
            i += 1
            continue

        if stripped and indent == 0:
            break

        match = re.match(r'^\s+-\s+id:\s*"?(phase\d+)"?', line)
        if not match:
            i += 1
            continue

        phase_id = match.group(1)
        item_indent = len(match.group(0)) - len(match.group(0).lstrip())
        status = "pending"
        i += 1

        while i < len(lines):
            field_line = lines[i]
            field_stripped = field_line.strip()
            field_indent = len(field_line) - len(field_line.lstrip())

            if field_stripped and field_indent <= item_indent:
                break

            status_match = re.match(
                r'^\s+status:\s*"?(pending|in_progress|completed|failed|skipped)"?',
                field_line,
            )
            if status_match:
                status = status_match.group(1)

            i += 1

        entries.append({"id": phase_id, "status": status})

    return entries


def parse_phase_statuses(text):
    """Parse phase statuses from the phases block without counting unrelated statuses."""
    return [entry["status"] for entry in parse_phase_status_entries(text)]


def count_phases(text):
    """Count phases and their statuses."""
    phase_statuses = parse_phase_statuses(text)
    total = len(phase_statuses)
    completed = sum(1 for status in phase_statuses if status == "completed")
    failed = sum(1 for status in phase_statuses if status == "failed")
    in_progress = sum(1 for status in phase_statuses if status == "in_progress")
    return total, completed, failed, in_progress


def print_move_result(root, move_result):
    """Render a spec move/transition result for humans."""
    rel_from = move_result.source.relative_to(root)
    rel_to = move_result.dest.relative_to(root)
    print(f"{c(C_GREEN, '  moved')}: {rel_from} -> {rel_to}")
    print(f" {c(C_DIM, 'status')}: {c(STATUS_COLORS.get(move_result.new_status, ''), move_result.new_status)}")


def move_result_payload(root, move_result):
    """Return a structured representation of a spec move/transition."""
    return {
        "from": str(move_result.source.relative_to(root)),
        "to": str(move_result.dest.relative_to(root)),
        "status": move_result.new_status,
    }


def collect_changed_files_regex(text):
    """Collect declared files with a lightweight regex fallback."""
    return sorted(set(re.findall(r'^\s+-\s+file:\s*"?([^"\n]+)"?', text, re.MULTILINE)))


def normalize_change_ownership(value):
    """Normalize one change ownership value, defaulting to exclusive."""
    if isinstance(value, str) and value in CHANGE_OWNERSHIP_VALUES:
        return value
    return "exclusive"


def collect_declared_change_map(text):
    """Collect declared files and ownership from spec phases."""
    try:
        yaml = require_pyyaml()
        data = yaml.safe_load(text) or {}
    except Exception:
        return {path: "exclusive" for path in collect_changed_files_regex(text)}

    if not isinstance(data, dict):
        return {}

    phases = data.get("phases")
    if not isinstance(phases, list):
        return {}

    by_file = {}
    for phase in phases:
        if not isinstance(phase, dict):
            continue
        changes = phase.get("changes")
        if not isinstance(changes, list):
            continue
        for change in changes:
            if not isinstance(change, dict):
                continue
            path = change.get("file")
            if not isinstance(path, str):
                continue
            path = path.strip()
            if not path:
                continue
            ownership = normalize_change_ownership(change.get("ownership"))
            current = by_file.get(path)
            if current == "exclusive" or ownership == "exclusive":
                by_file[path] = "exclusive"
            else:
                by_file[path] = "shared"
    return dict(sorted(by_file.items()))


def collect_changed_files(text):
    """Collect unique changed files declared in spec phases."""
    return sorted(collect_declared_change_map(text))


def filter_audit_paths(paths):
    """Drop scafld execution artifacts from scope auditing."""
    return {
        path
        for path in paths
        if path not in AUDIT_IGNORED_FILES
        and not any(path == prefix[:-1] or path.startswith(prefix) for prefix in AUDIT_IGNORED_PREFIXES)
    }


def active_declared_changes(root, exclude_task_id=None):
    """Collect declared file ownership by task for other active specs."""
    active_dir = root / ACTIVE_DIR
    declared = {}
    if not active_dir.is_dir():
        return declared

    for spec_path in sorted(active_dir.glob("*.yaml")):
        task_id = spec_path.stem
        if task_id == exclude_task_id:
            continue
        text = spec_path.read_text()
        files = {
            path: ownership
            for path, ownership in collect_declared_change_map(text).items()
            if path != "TODO"
        }
        if files:
            declared[task_id] = dict(sorted(files.items()))
    return declared


def active_declared_files(root, exclude_task_id=None):
    """Collect declared file sets for other active specs."""
    return {
        task_id: sorted(files)
        for task_id, files in active_declared_changes(root, exclude_task_id=exclude_task_id).items()
    }


def active_changes_by_file(active_changes):
    """Invert active change ownership by task into per-file declarations."""
    by_file = {}
    for task_id, files in active_changes.items():
        for path, ownership in files.items():
            by_file.setdefault(path, []).append({
                "task_id": task_id,
                "ownership": ownership,
            })
    for path, entries in by_file.items():
        by_file[path] = sorted(entries, key=lambda entry: entry["task_id"])
    return by_file


def classify_active_overlap(declared_changes, other_active_changes):
    """Split shared overlap from conflicting overlap across active specs."""
    other_by_file = active_changes_by_file(other_active_changes)
    shared_with_other_active = set()
    active_overlap = set()
    shared_details = {}
    conflict_details = {}

    for path, ownership in declared_changes.items():
        overlapping = other_by_file.get(path, [])
        if not overlapping:
            continue
        task_ids = [entry["task_id"] for entry in overlapping]
        if ownership == "shared" and all(entry["ownership"] == "shared" for entry in overlapping):
            shared_with_other_active.add(path)
            shared_details[path] = task_ids
        else:
            active_overlap.add(path)
            conflict_details[path] = task_ids

    return {
        "shared_with_other_active": shared_with_other_active,
        "active_overlap": active_overlap,
        "shared_details": shared_details,
        "conflict_details": conflict_details,
        "other_by_file": other_by_file,
    }


def describe_other_active_specs(path, detail_map):
    """Format the other active specs that also declare one path."""
    task_ids = detail_map.get(path, [])
    return ", ".join(task_ids)


def build_audit_file_payloads(
    declared_changes,
    actual,
    covered_by_other_active,
    shared_with_other_active,
    active_overlap,
    overlap_details,
    other_by_file,
):
    """Build per-file structured audit payloads."""
    files = []
    all_paths = sorted(set(declared_changes) | set(actual))
    for path in all_paths:
        status = "matched"
        if path in covered_by_other_active:
            status = "covered_by_other_active"
        elif path in actual and path not in declared_changes:
            status = "undeclared"
        elif path in declared_changes and path not in actual:
            status = "missing"

        overlap = "none"
        if path in shared_with_other_active:
            overlap = "shared"
        elif path in active_overlap:
            overlap = "conflict"

        payload = {
            "path": path,
            "status": status,
            "declared": path in declared_changes,
            "changed": path in actual,
            "overlap": overlap,
        }
        ownership = declared_changes.get(path)
        if ownership:
            payload["ownership"] = ownership
        task_ids = overlap_details.get(path) or [entry["task_id"] for entry in other_by_file.get(path, [])]
        if task_ids:
            payload["other_active_specs"] = task_ids
        files.append(payload)
    return files


def load_review_topology(root):
    """Load the configured review topology for built-in review passes."""
    config = load_config(root)
    review_config = config.get("review")
    if not isinstance(review_config, dict):
        raise ValueError("config.review must be a mapping")
    return build_review_topology(review_config)


def ensure_review_file_header(review_file, task_id, spec_text):
    """Create the shared review file header if it does not exist yet."""
    if review_file.exists():
        return

    task_title = yaml_read_nested(spec_text, "task", "title") or task_id
    task_summary = yaml_read_nested(spec_text, "task", "summary") or ""
    changed_files = collect_changed_files(spec_text)
    files_section = "\n".join(f"- {path}" for path in changed_files) if changed_files else "- (see git diff)"

    review_file.parent.mkdir(parents=True, exist_ok=True)
    review_file.write_text(f"""# Review: {task_id}

## Spec
{task_title}
{task_summary}

## Files Changed
{files_section}
""")


def append_review_round(review_file, task_id, spec_text, topology, metadata, verdict="", blocking=None, non_blocking=None, section_bodies=None):
    """Append a review round using Review Artifact v3."""
    ensure_review_file_header(review_file, task_id, spec_text)

    existing_text = review_file.read_text()
    review_count = len(re.findall(r'^## Review \d+\s+—', existing_text, re.MULTILINE)) + 1
    metadata_json = json.dumps(metadata, indent=2)
    blocking_body = "\n".join(blocking or [])
    non_blocking_body = "\n".join(non_blocking or [])
    verdict_body = verdict or ""
    section_bodies = section_bodies or {}
    adversarial_sections = "\n\n".join(
        f"### {definition['title']}\n{section_bodies.get(definition['id'], '')}"
        for definition in review_passes_by_kind(topology, "adversarial")
    )

    round_text = f"""## Review {review_count} — {now_iso()}

### Metadata
```json
{metadata_json}
```

### Pass Results
{render_review_pass_results(topology, metadata.get("pass_results"))}

{adversarial_sections}

### Blocking
{blocking_body}

### Non-blocking
{non_blocking_body}

### Verdict
{verdict_body}
"""

    if existing_text.strip():
        review_file.write_text(existing_text.rstrip() + "\n\n---\n\n" + round_text)
    else:
        review_file.write_text(round_text)
    return review_count


def upsert_review_block(text, review_block):
    """Replace the top-level review block or insert it before trailing metadata."""
    lines = text.splitlines(True)
    result = []
    i = 0
    replaced = False

    while i < len(lines):
        if re.match(r'^review:\s*$', lines[i]):
            replaced = True
            i += 1
            while i < len(lines):
                line = lines[i]
                if line.strip() and not line[0].isspace():
                    break
                i += 1
            continue
        result.append(lines[i])
        i += 1

    block_text = review_block.strip() + "\n"
    insert_idx = None
    for idx, line in enumerate(result):
        if re.match(r'^(self_eval|deviations|metadata):', line):
            insert_idx = idx
            break

    if insert_idx is None:
        if result and not result[-1].endswith("\n"):
            result[-1] += "\n"
        if result and result[-1].strip():
            result.append("\n")
        result.append(block_text)
    else:
        if insert_idx > 0 and result[insert_idx - 1].strip():
            result.insert(insert_idx, "\n")
            insert_idx += 1
        result.insert(insert_idx, block_text)

    return "".join(result)


def parse_acceptance_criteria(text):
    """Extract acceptance criteria from all phases using indent-aware parsing."""
    criteria = []
    lines = text.splitlines()
    current_phase = None
    i = 0

    while i < len(lines):
        line = lines[i]

        m = re.match(r'^(\s+)-\s+id:\s*(.+)$', line)
        if m:
            val = parse_yaml_value(m.group(2))
            if re.match(r'^phase\d+$', val):
                current_phase = val
                i += 1
                continue

        if current_phase and re.match(r'^(\s+)(acceptance_criteria|validation)\s*:', line):
            block_indent = len(line) - len(line.lstrip())
            i += 1
            while i < len(lines):
                item_line = lines[i]
                if not item_line.strip():
                    i += 1
                    continue
                item_indent = len(item_line) - len(item_line.lstrip())
                if item_indent <= block_indent and item_line.strip():
                    break
                m = re.match(r'^(\s+)-\s+(id|dod_id):\s*(.+)$', item_line)
                if m:
                    item_base_indent = len(m.group(1))
                    ac = {"id": parse_yaml_value(m.group(3)), "phase": current_phase}
                    result_block_indent = None
                    i += 1
                    while i < len(lines):
                        field_line = lines[i]
                        if not field_line.strip():
                            i += 1
                            continue
                        fi = len(field_line) - len(field_line.lstrip())
                        if fi <= item_base_indent and field_line.strip():
                            break
                        if field_line.strip().startswith('- ') and fi == item_base_indent:
                            break
                        if result_block_indent is not None and fi <= result_block_indent:
                            result_block_indent = None
                        fm = re.match(r'^\s+([\w_]+):\s*(.*)$', field_line)
                        if fm:
                            key = fm.group(1)
                            value = parse_yaml_value(fm.group(2))
                            if key in ("type", "description", "command", "expected", "cwd", "timeout_seconds"):
                                ac[key] = value
                            elif key == "result":
                                if value:
                                    ac[key] = value
                                else:
                                    result_block_indent = fi
                            elif key == "status" and result_block_indent is not None and fi > result_block_indent:
                                ac["result"] = value
                        i += 1
                    criteria.append(ac)
                else:
                    i += 1
            continue

        i += 1

    return criteria


def record_exec_result(text, ac_id, passed, output_snippet=""):
    """Record execution result for an acceptance criterion in the spec."""
    lines = text.splitlines(True)
    result = []
    i = 0
    while i < len(lines):
        line = lines[i]
        if re.search(rf'(?:id|dod_id):\s*"?{re.escape(ac_id)}"?\s*$', line):
            result.append(line)
            item_match = re.match(r'^(\s+)-\s+', line)
            if item_match:
                field_indent = ' ' * (len(item_match.group(1)) + 2)
            else:
                field_indent = ' ' * (len(line) - len(line.lstrip()))
            nested_result = False
            i += 1
            while i < len(lines):
                fl = lines[i]
                if not fl.strip():
                    break
                fi = len(fl) - len(fl.lstrip())
                expected_fi = len(field_indent)
                if fi < expected_fi:
                    break
                if fi == expected_fi - 2 and fl.strip().startswith('- '):
                    break
                if fi == expected_fi and re.match(r'^\s+result:\s*$', fl):
                    nested_result = True
                    i += 1
                    while i < len(lines):
                        nested = lines[i]
                        if not nested.strip():
                            i += 1
                            continue
                        nested_fi = len(nested) - len(nested.lstrip())
                        if nested_fi <= expected_fi:
                            break
                        i += 1
                    continue
                if fi == expected_fi and re.match(r'^\s+(result|result_output|executed_at):', fl):
                    i += 1
                    continue
                result.append(fl)
                i += 1
            status = "pass" if passed else "fail"
            executed_at = now_iso()
            snippet = output_snippet[:200].replace('"', '\\"').replace('\n', ' ')
            if nested_result:
                result.append(f"{field_indent}result:\n")
                result.append(f'{field_indent}  status: "{status}"\n')
                result.append(f'{field_indent}  timestamp: "{executed_at}"\n')
                if output_snippet:
                    result.append(f'{field_indent}  output: "{snippet}"\n')
            else:
                result.append(f'{field_indent}result: "{status}"\n')
                result.append(f'{field_indent}executed_at: "{executed_at}"\n')
                if output_snippet:
                    result.append(f'{field_indent}result_output: "{snippet}"\n')
            continue
        result.append(line)
        i += 1

    return ''.join(result)


def criterion_timeout_seconds(ac):
    raw = ac.get("timeout_seconds", "")
    if raw in ("", None):
        return DEFAULT_ACCEPTANCE_TIMEOUT_SECONDS

    try:
        seconds = int(str(raw).strip())
    except (TypeError, ValueError) as exc:
        raise ValueError(f"invalid timeout_seconds '{raw}'") from exc

    if seconds <= 0:
        raise ValueError(f"timeout_seconds must be > 0 (got {raw})")

    return seconds


def check_expected(returncode, output, expected):
    """Check command result against expected outcome."""
    if not expected:
        return returncode == 0

    exp = expected.strip()
    exp_lower = exp.lower()

    if exp_lower == "no matches":
        return returncode != 0 or not output

    match = re.match(r'^exit\s+code\s+(\d+)$', exp_lower)
    if match:
        return returncode == int(match.group(1))

    if exp_lower == "0 failures":
        if returncode != 0:
            return False
        fail_match = re.search(r'(\d+)\s+failures?', output, re.IGNORECASE)
        if fail_match and int(fail_match.group(1)) > 0:
            return False
        return True

    if exp_lower in GENERIC_PASS_EXPECTATIONS:
        if returncode != 0:
            return False
        fail_match = re.search(r'(\d+)\s+failures?', output, re.IGNORECASE)
        if fail_match and int(fail_match.group(1)) > 0:
            return False
        return True

    if returncode != 0:
        return False
    return exp_lower in output.lower()


def parse_iso8601_timestamp(value):
    if not value:
        return None
    try:
        return datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
