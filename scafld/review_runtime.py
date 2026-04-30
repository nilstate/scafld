import shlex

from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.output import error_payload
from scafld.review_runner import resolve_review_runner
from scafld.review_workflow import (
    load_configured_review_topology,
    open_review_round,
    run_automated_review_suite,
)
from scafld.spec_model import get_status
from scafld.spec_store import load_spec_document, require_spec


def _review_execute_command(task_id, resolved_runner):
    parts = ["scafld", "review", task_id]
    if resolved_runner.runner:
        parts.extend(["--runner", resolved_runner.runner])
    if resolved_runner.runner == "external" and resolved_runner.provider:
        parts.extend(["--provider", resolved_runner.provider])
    if resolved_runner.runner == "external" and resolved_runner.model:
        parts.extend(["--model", resolved_runner.model])
    return " ".join(shlex.quote(str(part)) for part in parts)


def review_snapshot(
    root,
    task_id,
    *,
    use_color=False,
    resolved_runner=None,
    runner_override=None,
    provider_override=None,
    model_override=None,
):
    topology = load_configured_review_topology(root)
    try:
        resolved_runner = resolved_runner or resolve_review_runner(
            root,
            runner_override=runner_override,
            provider_override=provider_override,
            model_override=model_override,
        )
    except ValueError as exc:
        return ({
            "ok": False,
            "command": "review",
            "task_id": task_id,
            "state": {"status": get_status(load_spec_document(require_spec(root, task_id)))},
            "result": None,
            "warnings": [],
            "error": error_payload(
                str(exc),
                exit_code=1,
            ),
        }, 1)
    spec = require_spec(root, task_id)
    text = spec.read_text()
    status = get_status(load_spec_document(spec))

    if status != "in_progress":
        return ({
            "ok": False,
            "command": "review",
            "task_id": task_id,
            "state": {"status": status},
            "result": None,
            "warnings": [],
            "error": error_payload(
                f"spec must be in_progress to review (current: {status})",
                code=EC.INVALID_SPEC_STATUS,
                exit_code=1,
            ),
        }, 1)

    suite = run_automated_review_suite(root, task_id, text, topology)
    automated_results = suite["automated_results"]
    normalized_passes = suite["normalized_passes"]
    failed = suite["failed"]

    if failed:
        return ({
            "ok": False,
            "command": "review",
            "task_id": task_id,
            "state": {"status": status},
            "result": {
                "automated_passes": automated_results,
                "failed_count": failed,
            },
            "warnings": [],
            "error": error_payload(
                f"{failed} automated pass(es) failed",
                code=EC.AUTOMATED_REVIEW_FAILED,
                next_action=f"scafld review {task_id}",
                exit_code=1,
            ),
        }, 1)

    try:
        review_round = open_review_round(
            root,
            task_id,
            spec,
            text,
            topology,
            normalized_passes,
            automated_results,
            use_color=use_color,
        )
    except ScafldError as exc:
        return ({
            "ok": False,
            "command": "review",
            "task_id": task_id,
            "state": {"status": status},
            "result": None,
            "warnings": [],
            "error": error_payload(
                f"{exc.message}: {exc.details[0]}" if exc.details else exc.message,
                code=EC.REVIEW_GIT_STATE_UNAVAILABLE,
                exit_code=1,
            ),
        }, 1)

    current_handoff = {
        "role": review_round["handoff_role"],
        "gate": review_round["handoff_gate"],
        "selector": "review",
        "command": None,
        "handoff_file": review_round["review_handoff_rel"],
        "handoff_json_file": review_round["review_handoff_json_rel"],
    }
    execute_command = _review_execute_command(task_id, resolved_runner)
    snapshot_message = (
        "JSON mode is snapshot-only and did not spawn the external reviewer; rerun the command without --json to execute it."
        if resolved_runner.runner == "external"
        else "JSON mode emitted the handoff snapshot; give the handoff to the reviewer and wait for a verdict."
    )
    return ({
        "ok": True,
        "command": "review",
        "task_id": task_id,
        "state": {
            "status": status,
            "review_round": review_round["review_count"],
            "review_action": review_round["review_action"],
            "review_runner": resolved_runner.runner,
        },
        "result": {
            "review_file": review_round["review_path_rel"],
            "handoff_file": review_round["review_handoff_rel"],
            "handoff_json_file": review_round["review_handoff_json_rel"],
            "handoff_role": review_round["handoff_role"],
            "handoff_gate": review_round["handoff_gate"],
            "review_handoff": review_round["review_handoff_rel"],
            "review_handoff_json": review_round["review_handoff_json_rel"],
            "review_action": review_round["review_action"],
            "review_runner": {
                "runner": resolved_runner.runner,
                "provider": resolved_runner.provider,
                "model": resolved_runner.model,
                "timeout_seconds": resolved_runner.timeout_seconds,
                "fallback_policy": resolved_runner.fallback_policy,
                "snapshot_only": True,
                "execution_mode": "snapshot_only",
                "provider_invoked": False,
                "execute_command": execute_command if resolved_runner.runner == "external" else None,
            },
            "snapshot_only": True,
            "provider_invoked": False,
            "execute_command": execute_command if resolved_runner.runner == "external" else None,
            "snapshot_note": snapshot_message,
            "review_prompt": review_round["review_prompt"],
            "automated_passes": automated_results,
            "required_sections": review_round["required_sections"],
            "complete_command": f"scafld complete {task_id}",
            "current_handoff": current_handoff,
            "next_action": {
                "type": "challenge_handoff",
                "command": execute_command if resolved_runner.runner == "external" else None,
                "message": snapshot_message,
                "followup_command": f"scafld complete {task_id}",
                "blocked": False,
            },
        },
        "warnings": [],
        "error": None,
    }, 0)
