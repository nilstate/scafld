import sys

from scafld.command_runtime import require_root
from scafld.error_codes import ErrorCode as EC
from scafld.handoff_renderer import current_phase_id, phase_definitions, render_handoff
from scafld.output import emit_command_json, error_payload
from scafld.session_store import attempts_for_criterion, failed_attempts_for_criterion, latest_failed_attempt, load_session
from scafld.spec_model import get_status
from scafld.spec_store import load_spec_document, require_spec


def selected_handoff_identity(args, status):
    flags = [bool(args.phase), bool(args.recovery), bool(args.review)]
    if sum(flags) > 1:
        raise ValueError("choose only one of --phase, --recovery, or --review")
    if args.review:
        return "challenger", "review"
    if args.recovery:
        return "executor", "recovery"
    if status == "completed":
        return "challenger", "review"
    return "executor", "phase"


def first_phase_id(spec_data):
    phases = phase_definitions(spec_data)
    return phases[0].get("id") if phases else None


def cmd_handoff(args):
    """Render a model-facing handoff without moving the lifecycle."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    json_mode = bool(getattr(args, "json", False))
    session = load_session(root, args.task_id, spec_path=spec)
    spec_data = load_spec_document(spec)
    status = get_status(spec_data)

    try:
        role, gate = selected_handoff_identity(args, status)
    except ValueError as exc:
        if json_mode:
            emit_command_json(
                "handoff",
                ok=False,
                task_id=args.task_id,
                state={"status": status},
                error=error_payload(str(exc), code=EC.INVALID_ARGUMENTS, exit_code=1),
            )
        else:
            print(f"error: {exc}")
        sys.exit(1)

    context = {}
    selector = None

    if gate == "phase":
        selector = args.phase or current_phase_id(spec_data) or first_phase_id(spec_data)
        if not selector:
            selector = "phase1"
    elif gate == "recovery":
        if session is None:
            if json_mode:
                emit_command_json(
                    "handoff",
                    ok=False,
                    task_id=args.task_id,
                    state={"status": status},
                    error=error_payload(
                        "recovery handoff requires an existing session",
                        code=EC.INVALID_ARGUMENTS,
                        exit_code=1,
                    ),
                )
            else:
                print("error: recovery handoff requires an existing session")
            sys.exit(1)
        selector = args.recovery
        failed_attempt = latest_failed_attempt(session, selector)
        if failed_attempt is None:
            if json_mode:
                emit_command_json(
                    "handoff",
                    ok=False,
                    task_id=args.task_id,
                    state={"status": status},
                    error=error_payload(
                        f"no failed attempt recorded for {selector}",
                        code=EC.INVALID_ARGUMENTS,
                        exit_code=1,
                    ),
                )
            else:
                print(f"error: no failed attempt recorded for {selector}")
            sys.exit(1)
        context = {
            "failed_attempt": failed_attempt,
            "diagnostic_rel": failed_attempt.get("diagnostic_path"),
            "criterion_attempts": attempts_for_criterion(session, selector),
            "recovery_attempt": max(len(failed_attempts_for_criterion(session, selector)), 1),
        }
    else:
        selector = "review"

    rendered = render_handoff(
        root,
        args.task_id,
        spec,
        role=role,
        gate=gate,
        selector=selector,
        session=session,
        context=context,
    )

    if json_mode:
        emit_command_json(
            "handoff",
            task_id=args.task_id,
            state={
                "status": status,
                "role": rendered["role"],
                "gate": rendered["gate"],
                "selector": rendered["selector"],
            },
            result={
                "handoff_file": rendered["path_rel"],
                "handoff_json_file": rendered["json_path_rel"],
                "role": rendered["role"],
                "gate": rendered["gate"],
                "template": rendered["template"],
                "generated_at": rendered["generated_at"],
                "session_file": rendered["session_ref"],
                "content": rendered["content"],
                "payload": rendered["payload"],
            },
        )
        return

    print(rendered["content"], end="")
