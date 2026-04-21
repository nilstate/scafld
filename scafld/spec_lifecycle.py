import json
import re

from scafld.audit_scope import CHANGE_OWNERSHIP_VALUES, git_sync_excluded_paths
from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.git_state import bind_task_branch, build_origin_sync_payload, refresh_origin_sync
from scafld.output import error_payload
from scafld.projections import origin_payload, phase_counts
from scafld.review_artifacts import load_review_topology
from scafld.reviewing import load_review_state
from scafld.runtime_bundle import REVIEWS_DIR, resolve_schema_path
from scafld.spec_parsing import count_phases, now_iso, parse_phase_status_entries
from scafld.spec_store import load_spec_document, write_spec_document, yaml_read_field, yaml_read_nested


def move_result_payload(root, move_result):
    """Return a structured representation of a spec move/transition."""
    return {
        "from": str(move_result.source.relative_to(root)),
        "to": str(move_result.dest.relative_to(root)),
        "status": move_result.new_status,
    }


def status_snapshot(root, spec, task_id):
    text = spec.read_text()
    data = load_spec_document(spec)
    rel = spec.relative_to(root)

    status = yaml_read_field(text, "status") or "unknown"
    state = {
        "file": str(rel),
        "status": status,
        "size": yaml_read_nested(text, "task", "size") or "",
        "risk": yaml_read_nested(text, "task", "risk_level") or "",
        "updated_at": yaml_read_field(text, "updated") or "",
    }
    result = {
        "title": yaml_read_nested(text, "task", "title") or "",
        "phase_statuses": parse_phase_status_entries(text),
        "phase_counts": phase_counts(*count_phases(text)),
    }

    origin = origin_payload(data)
    sync = build_origin_sync_payload(root, origin, excluded_rels=git_sync_excluded_paths())
    result["origin"] = origin
    result["sync"] = sync

    try:
        topology = load_review_topology(root)
    except Exception as exc:
        review_state = {"exists": False, "errors": [str(exc)]}
    else:
        review_state = load_review_state(root / REVIEWS_DIR / f"{task_id}.md", topology)
    result["review_state"] = review_state

    return {
        "text": text,
        "data": data,
        "state": state,
        "result": result,
    }


def filter_specs(specs, filter_text):
    if not filter_text:
        return specs

    flt = filter_text.lower()
    if flt in ("draft", "drafts"):
        return [(spec_path, label) for spec_path, label in specs if label == "drafts"]
    if flt in ("approved",):
        return [(spec_path, label) for spec_path, label in specs if label == "approved"]
    if flt in ("active", "in_progress"):
        return [(spec_path, label) for spec_path, label in specs if label == "active"]
    if flt in ("archive", "archived", "completed", "done"):
        return [(spec_path, label) for spec_path, label in specs if label.startswith("archive")]
    return [(spec_path, label) for spec_path, label in specs if flt in spec_path.stem.lower()]


def listing_groups(specs):
    groups = {}
    for spec_path, label in specs:
        groups.setdefault(label, []).append(spec_path)
    return groups


def branch_binding_snapshot(root, spec, task_id, *, name=None, base=None, bind_current=False):
    rel = spec.relative_to(root)
    data = load_spec_document(spec)
    status = data.get("status")

    if status in ("completed", "failed", "cancelled"):
        raise ScafldError(
            f"cannot bind a branch for a {status} spec",
            [f"spec is archived at {rel}"],
            code=EC.INVALID_SPEC_STATUS,
        )

    timestamp = now_iso()
    binding = bind_task_branch(
        root,
        task_id,
        origin_payload(data),
        name=name,
        base=base,
        bind_current=bind_current,
        bound_at=timestamp,
        excluded_rels=git_sync_excluded_paths(),
    )
    data["origin"] = binding["origin"]
    data["updated"] = timestamp
    write_spec_document(spec, data)

    return {
        "state": {
            "file": str(rel),
            "status": status,
            "branch": binding["branch"],
        },
        "result": {
            "action": binding["action"],
            "origin": origin_payload(data),
            "sync": binding["sync"],
        },
        "binding": binding,
    }


def sync_snapshot(root, spec, task_id):
    rel = spec.relative_to(root)
    data = load_spec_document(spec)
    status = data.get("status")
    origin = origin_payload(data)
    git_binding = origin.get("git") if isinstance(origin.get("git"), dict) else {}

    if not git_binding.get("branch"):
        raise ScafldError(
            "spec has no bound branch or origin metadata yet",
            [f"bind one first: scafld branch {task_id}"],
            code=EC.ORIGIN_UNBOUND,
            next_action=f"scafld branch {task_id}",
        )

    timestamp = now_iso()
    data["origin"], sync = refresh_origin_sync(
        root,
        data.get("origin"),
        checked_at=timestamp,
        excluded_rels=git_sync_excluded_paths(),
    )
    data["updated"] = timestamp
    write_spec_document(spec, data)

    sync_status = sync.get("status")
    ok = sync_status == "in_sync"
    error = None
    if sync_status == "drift":
        error = error_payload(
            "git drift detected",
            code=EC.GIT_DRIFT,
            details=list(sync.get("reasons") or []),
            exit_code=1,
        )
    elif sync_status == "unavailable":
        error = error_payload(
            "live git state is unavailable",
            code=EC.GIT_STATE_UNAVAILABLE,
            details=list(sync.get("reasons") or []),
            exit_code=1,
        )

    return {
        "ok": ok,
        "error": error,
        "state": {
            "file": str(rel),
            "status": status,
            "sync_status": sync_status,
        },
        "result": {
            "origin": origin_payload(data),
            "sync": sync,
        },
        "git_binding": git_binding,
    }


def validation_snapshot(root, spec):
    rel = spec.relative_to(root)
    text = spec.read_text()
    errors = validate_spec(root, spec)

    total, completed, failed, in_progress = count_phases(text)
    task_id = yaml_read_field(text, "task_id")
    status = yaml_read_field(text, "status")
    spec_version = yaml_read_field(text, "spec_version")

    return {
        "state": {
            "file": str(rel),
            "status": status,
            "schema_version": spec_version,
        },
        "result": {
            "valid": not bool(errors),
            "phase_count": total,
            "phase_counts": {
                "total": total,
                "completed": completed,
                "failed": failed,
                "in_progress": in_progress,
                "pending": total - completed - failed - in_progress,
            },
            "errors": errors,
        },
        "task_id": task_id,
        "errors": errors,
    }


def validate_spec(root, spec):
    """Validate a spec against the JSON schema. Returns list of errors (empty = valid)."""
    schema_path = resolve_schema_path(root)
    if not schema_path.exists():
        return [f"schema not found at {schema_path.relative_to(root)}"]

    try:
        schema = json.loads(schema_path.read_text())
    except json.JSONDecodeError as exc:
        return [f"invalid schema JSON: {exc}"]

    text = spec.read_text()
    errors = []

    required_top = schema.get("required", [])
    for field in required_top:
        if not yaml_read_field(text, field):
            if not re.search(rf"^{field}:", text, re.MULTILINE):
                errors.append(f"missing required field: {field}")

    task_id = yaml_read_field(text, "task_id")
    if task_id and not re.match(r"^[a-z0-9-]+$", task_id):
        errors.append(f"task_id must be kebab-case: got '{task_id}'")

    status = yaml_read_field(text, "status")
    valid_statuses = ["draft", "blocked", "under_review", "approved", "in_progress", "completed", "failed", "cancelled"]
    if status and status not in valid_statuses:
        errors.append(f"invalid status: '{status}' (expected one of: {', '.join(valid_statuses)})")

    spec_version = yaml_read_field(text, "spec_version")
    if spec_version and not re.match(r"^\d+\.\d+$", spec_version):
        errors.append(f"spec_version must be semver: got '{spec_version}'")

    if not re.search(r"^phases:", text, re.MULTILINE):
        errors.append("missing required field: phases")
    else:
        phase_ids = re.findall(r'^\s+-\s+id:\s*"?(phase\d+)"?', text, re.MULTILINE)
        if not phase_ids:
            errors.append("phases array is empty (at least 1 phase required)")

    if not re.search(r"^planning_log:", text, re.MULTILINE):
        errors.append("missing required field: planning_log")

    if re.search(r"^task:", text, re.MULTILINE):
        for field in ["title", "summary", "size", "risk_level"]:
            if not yaml_read_nested(text, "task", field):
                errors.append(f"missing required task field: task.{field}")
    else:
        errors.append("missing required field: task")

    todo_patterns = [
        (r'^\s+(?:command|content_spec|description|file):\s*"?TODO', "has TODO placeholder"),
        (r'^\s+-\s+"?TODO', "has TODO list item"),
    ]
    for pattern, message in todo_patterns:
        matches = re.findall(pattern, text, re.MULTILINE)
        if matches:
            count = len(matches)
            errors.append(f"{message} ({count} occurrence{'s' if count > 1 else ''})")
            break

    ownership_values = re.findall(r'^\s+ownership:\s*"?([^"\n]+)"?', text, re.MULTILINE)
    invalid_ownership = sorted({value for value in ownership_values if value not in CHANGE_OWNERSHIP_VALUES})
    for value in invalid_ownership:
        errors.append(
            f"invalid change ownership: '{value}' (expected one of: {', '.join(sorted(CHANGE_OWNERSHIP_VALUES))})"
        )

    return errors
