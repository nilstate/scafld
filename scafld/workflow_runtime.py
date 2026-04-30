from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.execution_runtime import exec_snapshot, harden_open_snapshot
from scafld.handoff_renderer import current_phase_id
from scafld.lifecycle_runtime import new_spec_snapshot, start_spec_snapshot, status_snapshot
from scafld.output import error_payload
from scafld.runtime_bundle import DRAFTS_DIR
from scafld.runtime_guidance import derive_task_guidance
from scafld.session_store import load_session
from scafld.spec_model import get_status
from scafld.spec_store import find_spec, load_spec_document, require_spec


def scafld_error_to_payload(exc):
    return error_payload(
        exc.message,
        code=exc.code,
        details=exc.details,
        next_action=exc.next_action,
        exit_code=exc.exit_code,
    )


def plan_snapshot(root, task_id, *, title=None, size=None, risk=None, command=None, files=None):
    existing_spec = find_spec(root, task_id)

    if existing_spec is not None:
        rel = existing_spec.relative_to(root)
        status = get_status(load_spec_document(existing_spec))
        if str(rel).startswith(DRAFTS_DIR):
            try:
                harden_snapshot = harden_open_snapshot(root, task_id)
            except ScafldError as exc:
                return ({
                    "ok": False,
                    "command": "plan",
                    "task_id": task_id,
                    "state": {"status": status, "file": str(rel)},
                    "result": None,
                    "warnings": [],
                    "error": scafld_error_to_payload(exc),
                }, exc.exit_code)

            return ({
                "ok": True,
                "command": "plan",
                "task_id": task_id,
                "state": {
                    "status": status,
                    "file": str(rel),
                    "harden_status": harden_snapshot["state"].get("harden_status"),
                    "round": harden_snapshot["state"].get("round"),
                },
                "result": {
                    "reused_existing_draft": True,
                    "prompt": harden_snapshot["result"].get("prompt"),
                    "mark_passed_command": harden_snapshot["result"].get("mark_passed_command"),
                    "approve_command": f"scafld approve {task_id}",
                },
                "warnings": harden_snapshot.get("warnings") or [],
                "error": None,
            }, 0)

        message = f"plan can only create or reopen a draft (current: {status})"
        return ({
            "ok": False,
            "command": "plan",
            "task_id": task_id,
            "state": {"status": status, "file": str(rel)},
            "result": None,
            "warnings": [],
            "error": error_payload(
                message,
                code=EC.SPEC_EXISTS,
                next_action=f"scafld status {task_id}",
                exit_code=1,
            ),
        }, 1)

    try:
        new_snapshot = new_spec_snapshot(
            root,
            task_id,
            title=title,
            size=size,
            risk=risk,
            command=command,
            files=files,
            auto_initialized=False,
        )
    except ScafldError as exc:
        return ({
            "ok": False,
            "command": "plan",
            "task_id": task_id,
            "state": {},
            "result": None,
            "warnings": [],
            "error": scafld_error_to_payload(exc),
        }, exc.exit_code)

    try:
        harden_snapshot = harden_open_snapshot(root, task_id)
    except ScafldError as exc:
        return ({
            "ok": False,
            "command": "plan",
            "task_id": task_id,
            "state": {"status": "draft"},
            "result": None,
            "warnings": [],
            "error": scafld_error_to_payload(exc),
        }, exc.exit_code)

    return ({
        "ok": True,
        "command": "plan",
        "task_id": task_id,
        "state": {
            "status": new_snapshot["state"].get("status"),
            "file": new_snapshot["state"].get("file"),
            "harden_status": harden_snapshot["state"].get("harden_status"),
            "round": harden_snapshot["state"].get("round"),
        },
        "result": {
            "title": new_snapshot["result"].get("title"),
            "size": new_snapshot["result"].get("size"),
            "risk": new_snapshot["result"].get("risk"),
            "repo_context": new_snapshot["result"].get("repo_context"),
            "prompt": harden_snapshot["result"].get("prompt"),
            "mark_passed_command": harden_snapshot["result"].get("mark_passed_command"),
            "approve_command": f"scafld approve {task_id}",
        },
        "warnings": harden_snapshot.get("warnings") or [],
        "error": None,
    }, 0)


def build_snapshot(root, task_id):
    spec = require_spec(root, task_id)
    status = get_status(load_spec_document(spec))

    if status == "approved":
        try:
            start_snapshot = start_spec_snapshot(root, task_id)
        except ScafldError as exc:
            error = scafld_error_to_payload(exc)
            return ({
                "ok": False,
                "command": "build",
                "task_id": task_id,
                "state": {"status": "approved", "action": "start_exec"},
                "result": None,
                "warnings": [],
                "error": error,
            }, exc.exit_code)

        active_spec = require_spec(root, task_id)
        resolved_phase = current_phase_id(load_spec_document(active_spec))
        exec_payload, exec_code = exec_snapshot(root, task_id, phase=resolved_phase, resume=True)
        session = load_session(root, task_id, spec_path=active_spec)
        snapshot = status_snapshot(root, active_spec, task_id)
        guidance = derive_task_guidance(
            root,
            task_id,
            active_spec,
            load_spec_document(active_spec),
            get_status(load_spec_document(active_spec)),
            session,
            snapshot["result"].get("review_state"),
            snapshot["result"].get("review_gate"),
        )
        exec_result = exec_payload.get("result") or {}
        start_result = start_snapshot.get("result") or {}
        merged_result = dict(exec_result)
        merged_result["start"] = start_result
        merged_result["exec"] = exec_result
        merged_result["initial_handoff"] = {
            "handoff_file": start_result.get("handoff_file"),
            "handoff_json_file": start_result.get("handoff_json_file"),
            "role": start_result.get("handoff_role"),
            "gate": start_result.get("handoff_gate"),
            "selector": start_result.get("selector"),
        }
        merged_result["next_action"] = guidance["next_action"]
        merged_result["current_handoff"] = guidance["current_handoff"]
        merged_result["block_reason"] = guidance["block_reason"]
        return ({
            "ok": bool(exec_payload.get("ok", False) and exec_code == 0),
            "command": "build",
            "task_id": task_id,
            "state": {
                "status": snapshot["state"].get("status") or exec_payload.get("state", {}).get("status") or "in_progress",
                "action": "start_exec",
            },
            "result": merged_result,
            "error": exec_payload.get("error"),
            "warnings": exec_payload.get("warnings") or [],
        }, exec_code)

    if status == "in_progress":
        resolved_phase = current_phase_id(load_spec_document(spec))
        payload, exit_code = exec_snapshot(root, task_id, phase=resolved_phase, resume=True)
        active_spec = require_spec(root, task_id)
        session = load_session(root, task_id, spec_path=active_spec)
        snapshot = status_snapshot(root, active_spec, task_id)
        guidance = derive_task_guidance(
            root,
            task_id,
            active_spec,
            load_spec_document(active_spec),
            get_status(load_spec_document(active_spec)),
            session,
            snapshot["result"].get("review_state"),
            snapshot["result"].get("review_gate"),
        )
        result = (payload or {}).get("result") or {}
        result["next_action"] = guidance["next_action"]
        result["current_handoff"] = guidance["current_handoff"]
        result["block_reason"] = guidance["block_reason"]
        return ({
            "ok": bool(payload.get("ok", False) and exit_code == 0),
            "command": "build",
            "task_id": task_id,
            "state": {"status": snapshot["state"].get("status") or status, "action": "exec"},
            "result": result,
            "error": payload.get("error"),
            "warnings": payload.get("warnings") or [],
        }, exit_code)

    message = f"build requires an approved or in_progress spec (current: {status})"
    return ({
        "ok": False,
        "command": "build",
        "task_id": task_id,
        "state": {"status": status},
        "result": None,
        "warnings": [],
        "error": error_payload(
            message,
            code=EC.INVALID_SPEC_STATUS,
            next_action=f"scafld approve {task_id}" if status == "draft" else None,
            exit_code=1,
        ),
    }, 1)
