import json
import re

from scafld.audit_scope import filter_audit_paths, git_sync_excluded_paths
from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.git_state import build_origin_sync_payload, list_working_tree_changed_files
from scafld.handoff_renderer import render_handoff
from scafld.projections import origin_payload, phase_counts
from scafld.runtime_bundle import CONFIG_LOCAL_PATH, CONFIG_PATH, DRAFTS_DIR, FRAMEWORK_CONFIG_PATH, resolve_schema_path
from scafld.runtime_guidance import derive_task_guidance, review_gate_snapshot
from scafld.session_store import ensure_session, ensure_workspace_baseline, load_session, record_approval, session_summary_payload
from scafld.spec_parsing import count_phases, now_iso, parse_phase_status_entries
from scafld.spec_store import (
    find_spec,
    load_spec_document,
    move_spec,
    require_spec,
    yaml_read_field,
    yaml_read_nested,
)
from scafld.spec_templates import build_new_spec_scaffold


def new_spec_snapshot(root, task_id, *, title=None, size=None, risk=None, auto_initialized=False):
    if not re.match(r"^[a-z0-9][a-z0-9-]*[a-z0-9]$", task_id) and not re.match(r"^[a-z0-9]$", task_id):
        raise ScafldError("task-id must be kebab-case (a-z, 0-9, hyphens)", code=EC.INVALID_TASK_ID)

    existing = find_spec(root, task_id)
    if existing:
        rel = existing.relative_to(root)
        raise ScafldError(f"spec already exists: {rel}", code=EC.SPEC_EXISTS)

    dest_dir = root / DRAFTS_DIR
    dest_dir.mkdir(parents=True, exist_ok=True)
    dest = dest_dir / f"{task_id}.yaml"

    ts = now_iso()
    scaffold = build_new_spec_scaffold(
        root,
        task_id,
        timestamp=ts,
        title=title,
        size=size,
        risk=risk,
        framework_config_path=FRAMEWORK_CONFIG_PATH,
        config_path=CONFIG_PATH,
        config_local_path=CONFIG_LOCAL_PATH,
    )
    dest.write_text(scaffold["text"])
    rel = dest.relative_to(root)
    return {
        "task_id": task_id,
        "state": {"status": "draft", "file": str(rel)},
        "result": {
            "title": scaffold["title"],
            "size": scaffold["size"],
            "risk": scaffold["risk"],
            "auto_initialized": auto_initialized,
            "repo_context": scaffold["repo_context"],
            "next_commands": [
                f"scafld harden {task_id}",
                f"scafld approve {task_id}",
            ],
        },
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

    errors = []
    try:
        data = load_spec_document(spec)
    except ScafldError as exc:
        return [exc.message, *exc.details]

    text = spec.read_text()

    required_top = schema.get("required", [])
    for field in required_top:
        if data.get(field) in (None, ""):
            errors.append(f"missing required field: {field}")

    task_id = data.get("task_id")
    if task_id and not re.match(r"^[a-z0-9-]+$", task_id):
        errors.append(f"task_id must be kebab-case: got '{task_id}'")

    status = data.get("status")
    valid_statuses = ["draft", "blocked", "under_review", "approved", "in_progress", "completed", "failed", "cancelled"]
    if status and status not in valid_statuses:
        errors.append(f"invalid status: '{status}' (expected one of: {', '.join(valid_statuses)})")

    spec_version = data.get("spec_version")
    if spec_version and not re.match(r"^\d+\.\d+$", spec_version):
        errors.append(f"spec_version must be semver: got '{spec_version}'")

    phases = data.get("phases")
    if not isinstance(phases, list):
        errors.append("missing required field: phases")
    else:
        phase_ids = [phase.get("id") for phase in phases if isinstance(phase, dict)]
        if not phase_ids:
            errors.append("phases array is empty (at least 1 phase required)")

    if not isinstance(data.get("planning_log"), list):
        errors.append("missing required field: planning_log")

    task = data.get("task")
    if not isinstance(task, dict):
        errors.append("missing required field: task")
    else:
        for field in ["title", "summary", "size", "risk_level"]:
            if task.get(field) in (None, ""):
                errors.append(f"missing required task field: task.{field}")

    todo_patterns = [
        (r'^\s+(?:command|content_spec|description|file):\s*"?TODO', "has TODO placeholder"),
        (r'^\s*-\s+"?TODO', "has TODO list item"),
    ]
    for pattern, message in todo_patterns:
        matches = re.findall(pattern, text, re.MULTILINE)
        if matches:
            errors.append(message)

    return errors


def validation_snapshot(root, spec):
    rel = spec.relative_to(root)
    text = spec.read_text()
    errors = validate_spec(root, spec)
    invalid_document = any(error.startswith("invalid spec document:") for error in errors)

    if invalid_document:
        total = completed = failed = in_progress = 0
    else:
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


def approve_spec_snapshot(root, task_id):
    spec = require_spec(root, task_id)
    errors = validate_spec(root, spec)
    if errors:
        raise ScafldError(
            "spec has validation errors, cannot approve",
            [f"- {error}" for error in errors] + [f"fix the spec, then retry: scafld approve {task_id}"],
            code=EC.VALIDATION_FAILED,
            next_action=f"scafld validate {task_id}",
        )

    move_result = move_spec(root, spec, "approved")
    session = ensure_session(root, task_id, spec_path=move_result.dest)
    session = record_approval(root, task_id, gate="approve", note="spec approved", spec_path=move_result.dest)
    return {
        "move_result": move_result,
        "state": {"status": move_result.new_status},
        "result": {
            "transition": {
                "from": str(move_result.source.relative_to(root)),
                "to": str(move_result.dest.relative_to(root)),
                "status": move_result.new_status,
            },
            "session_file": f".ai/runs/{task_id}/session.json",
            "entry_count": session_summary_payload(session).get("entry_count", 0),
        },
    }


def start_spec_snapshot(root, task_id):
    spec = require_spec(root, task_id)
    move_result = move_spec(root, spec, "in_progress")
    active_spec = move_result.dest
    session = ensure_session(root, task_id, spec_path=active_spec)
    changed_files, _error = list_working_tree_changed_files(root, excluded_rels=git_sync_excluded_paths())
    if changed_files is not None:
        session = ensure_workspace_baseline(
            root,
            task_id,
            paths=filter_audit_paths(changed_files),
            source="start",
            spec_path=active_spec,
        )
    rendered = render_handoff(
        root,
        task_id,
        active_spec,
        role="executor",
        gate="phase",
        session=session,
    )
    return {
        "move_result": move_result,
        "state": {"status": move_result.new_status},
        "result": {
            "transition": {
                "from": str(move_result.source.relative_to(root)),
                "to": str(move_result.dest.relative_to(root)),
                "status": move_result.new_status,
            },
            "session_file": f".ai/runs/{task_id}/session.json",
            "handoff_file": rendered["path_rel"],
            "handoff_json_file": rendered["json_path_rel"],
            "handoff_role": rendered["role"],
            "handoff_gate": rendered["gate"],
            "selector": rendered["selector"],
        },
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

    review_snapshot = review_gate_snapshot(root, task_id)
    review_state = review_snapshot["review_state"]
    review_gate = review_snapshot["review_gate"]
    result["review_state"] = review_state
    result["review_gate"] = review_gate
    session = load_session(root, task_id, spec_path=spec)
    result["runtime"] = session_summary_payload(session) if session is not None else {}
    guidance = derive_task_guidance(root, task_id, spec, data, status, session, review_state, review_gate)
    result["next_action"] = guidance["next_action"]
    result["current_handoff"] = guidance["current_handoff"]
    result["block_reason"] = guidance["block_reason"]

    return {
        "text": text,
        "data": data,
        "state": state,
        "result": result,
    }
