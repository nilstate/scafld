import re

from scafld.runtime_bundle import ACTIVE_DIR, CONFIG_LOCAL_PATH, REVIEWS_DIR, RUNS_DIR, SPECS_DIR
from scafld.spec_parsing import require_pyyaml


AUDIT_IGNORED_PREFIXES = (
    f"{SPECS_DIR}/",
    f"{REVIEWS_DIR}/",
    f"{RUNS_DIR}/",
)
AUDIT_IGNORED_FILES = {CONFIG_LOCAL_PATH}
CHANGE_OWNERSHIP_VALUES = {"exclusive", "shared"}


def git_sync_excluded_paths():
    """Ignore scafld control-plane artifacts when computing git sync drift."""
    return sorted((*AUDIT_IGNORED_PREFIXES, *AUDIT_IGNORED_FILES))


def collect_changed_files_regex(text):
    """Collect declared files with a lightweight regex fallback."""
    return sorted(set(re.findall(r'^\s*-\s+file:\s*"?([^"\n]+)"?', text, re.MULTILINE)))


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
