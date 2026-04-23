import io
import json
from argparse import Namespace
from contextlib import redirect_stdout

from scafld.command_runtime import require_root
from scafld.commands.execution import cmd_exec, cmd_harden
from scafld.commands.lifecycle import cmd_new, cmd_start
from scafld.error_codes import ErrorCode as EC
from scafld.output import emit_command_json, error_payload
from scafld.runtime_bundle import DRAFTS_DIR
from scafld.spec_store import find_spec, require_spec, yaml_read_field


def invoke_json(handler, namespace):
    buffer = io.StringIO()
    exit_code = 0
    with redirect_stdout(buffer):
        try:
            handler(namespace)
        except SystemExit as exc:
            exit_code = exc.code if isinstance(exc.code, int) else 1
    payload_text = buffer.getvalue().strip()
    payload = json.loads(payload_text) if payload_text else None
    return exit_code, payload


def cmd_plan(args):
    """Create a draft spec or reopen harden on an existing draft."""
    root = require_root()
    json_mode = bool(getattr(args, "json", False))
    existing_spec = find_spec(root, args.task_id)

    if existing_spec is not None:
        rel = existing_spec.relative_to(root)
        status = yaml_read_field(existing_spec.read_text(), "status")
        if str(rel).startswith(DRAFTS_DIR):
            if not json_mode:
                cmd_harden(Namespace(task_id=args.task_id, mark_passed=False, json=False))
                return

            harden_args = Namespace(task_id=args.task_id, mark_passed=False, json=True)
            harden_code, harden_payload = invoke_json(cmd_harden, harden_args)
            if harden_code or not harden_payload or not harden_payload.get("ok", False):
                emit_command_json(
                    "plan",
                    ok=False,
                    task_id=args.task_id,
                    state={"status": status, "file": str(rel)},
                    error=(harden_payload or {}).get("error") if isinstance(harden_payload, dict) else error_payload(
                        "failed to reopen harden round",
                        code=EC.COMMAND_FAILED,
                        exit_code=harden_code or 1,
                    ),
                )
                raise SystemExit(harden_code or 1)

            emit_command_json(
                "plan",
                task_id=args.task_id,
                state={
                    "status": status,
                    "file": str(rel),
                    "harden_status": harden_payload["state"].get("harden_status"),
                    "round": harden_payload["state"].get("round"),
                },
                result={
                    "reused_existing_draft": True,
                    "prompt": harden_payload["result"].get("prompt"),
                    "mark_passed_command": harden_payload["result"].get("mark_passed_command"),
                    "approve_command": f"scafld approve {args.task_id}",
                },
            )
            return

        message = f"plan can only create or reopen a draft (current: {status})"
        if json_mode:
            emit_command_json(
                "plan",
                ok=False,
                task_id=args.task_id,
                state={"status": status, "file": str(rel)},
                error=error_payload(
                    message,
                    code=EC.SPEC_EXISTS,
                    next_action=f"scafld status {args.task_id}",
                    exit_code=1,
                ),
            )
        else:
            print(f"error: {message}")
            print(f"  next: scafld status {args.task_id}")
        raise SystemExit(1)

    if not json_mode:
        cmd_new(args)
        print()
        cmd_harden(Namespace(task_id=args.task_id, mark_passed=False, json=False))
        return

    new_args = Namespace(
        task_id=args.task_id,
        title=getattr(args, "title", None),
        size=getattr(args, "size", None),
        risk=getattr(args, "risk", None),
        json=True,
    )
    new_code, new_payload = invoke_json(cmd_new, new_args)
    if new_code or not new_payload or not new_payload.get("ok", False):
        emit_command_json(
            "plan",
            ok=False,
            task_id=args.task_id,
            error=(new_payload or {}).get("error") if isinstance(new_payload, dict) else error_payload(
                "failed to create spec",
                code=EC.COMMAND_FAILED,
                exit_code=new_code or 1,
            ),
        )
        raise SystemExit(new_code or 1)

    harden_args = Namespace(task_id=args.task_id, mark_passed=False, json=True)
    harden_code, harden_payload = invoke_json(cmd_harden, harden_args)
    if harden_code or not harden_payload or not harden_payload.get("ok", False):
        emit_command_json(
            "plan",
            ok=False,
            task_id=args.task_id,
            state={"status": "draft"},
            error=(harden_payload or {}).get("error") if isinstance(harden_payload, dict) else error_payload(
                "failed to open harden round",
                code=EC.COMMAND_FAILED,
                exit_code=harden_code or 1,
            ),
        )
        raise SystemExit(harden_code or 1)

    emit_command_json(
        "plan",
        task_id=args.task_id,
        state={
            "status": new_payload["state"].get("status"),
            "file": new_payload["state"].get("file"),
            "harden_status": harden_payload["state"].get("harden_status"),
            "round": harden_payload["state"].get("round"),
        },
        result={
            "title": new_payload["result"].get("title"),
            "size": new_payload["result"].get("size"),
            "risk": new_payload["result"].get("risk"),
            "repo_context": new_payload["result"].get("repo_context"),
            "prompt": harden_payload["result"].get("prompt"),
            "mark_passed_command": harden_payload["result"].get("mark_passed_command"),
            "approve_command": f"scafld approve {args.task_id}",
        },
    )


def cmd_build(args):
    """Start approved work and drive execution to the next handoff or block."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    status = yaml_read_field(spec.read_text(), "status")
    json_mode = bool(getattr(args, "json", False))

    if status == "approved":
        if not json_mode:
            cmd_start(Namespace(task_id=args.task_id, json=False))
            print()
            cmd_exec(Namespace(task_id=args.task_id, phase=None, resume=True, json=False))
            return

        start_code, start_payload = invoke_json(cmd_start, Namespace(task_id=args.task_id, json=True))
        if start_code or not start_payload or not start_payload.get("ok", False):
            emit_command_json(
                "build",
                ok=False,
                task_id=args.task_id,
                state={"status": "approved", "action": "start_exec"},
                error=(start_payload or {}).get("error") if isinstance(start_payload, dict) else error_payload(
                    "failed to start approved work",
                    code=EC.COMMAND_FAILED,
                    exit_code=start_code or 1,
                ),
            )
            raise SystemExit(start_code or 1)

        exec_code, exec_payload = invoke_json(cmd_exec, Namespace(task_id=args.task_id, phase=None, resume=True, json=True))
        exec_result = (exec_payload or {}).get("result") or {}
        start_result = (start_payload or {}).get("result") or {}
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
        emit_command_json(
            "build",
            ok=bool(exec_payload and exec_payload.get("ok", False) and exec_code == 0),
            task_id=args.task_id,
            state={
                "status": (exec_payload or {}).get("state", {}).get("status") or "in_progress",
                "action": "start_exec",
            },
            result=merged_result,
            error=(exec_payload or {}).get("error"),
            warnings=(exec_payload or {}).get("warnings") or [],
        )
        if exec_code:
            raise SystemExit(exec_code)
        return

    if status == "in_progress":
        if not json_mode:
            cmd_exec(Namespace(task_id=args.task_id, phase=None, resume=True, json=False))
            return
        exit_code, payload = invoke_json(cmd_exec, Namespace(task_id=args.task_id, phase=None, resume=True, json=True))
        emit_command_json(
            "build",
            ok=bool(payload and payload.get("ok", False) and exit_code == 0),
            task_id=args.task_id,
            state={"status": status, "action": "exec"},
            result=(payload or {}).get("result") or {},
            error=(payload or {}).get("error"),
            warnings=(payload or {}).get("warnings") or [],
        )
        if exit_code:
            raise SystemExit(exit_code)
        return

    message = f"build requires an approved or in_progress spec (current: {status})"
    if json_mode:
        emit_command_json(
            "build",
            ok=False,
            task_id=args.task_id,
            state={"status": status},
            error=error_payload(
                message,
                code=EC.INVALID_SPEC_STATUS,
                next_action=f"scafld approve {args.task_id}" if status == "draft" else None,
                exit_code=1,
            ),
        )
    else:
        print(f"error: {message}")
    raise SystemExit(1)
