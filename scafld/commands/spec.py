import sys

from scafld.command_runtime import require_root
from scafld.error_codes import ErrorCode
from scafld.errors import ScafldError
from scafld.output import emit_command_json, error_payload
from scafld.runtime_contracts import locate_session_path, relative_path
from scafld.session_store import load_session
from scafld.spec_reconcile import rebuild_spec_from_session
from scafld.spec_store import require_spec
from scafld.terminal import C_GREEN, C_RED, C_YELLOW, c


def cmd_reconcile(args):
    """Check or repair session-derived spec sections."""
    root = require_root()
    spec_path = require_spec(root, args.task_id)
    rel_spec = relative_path(root, spec_path)
    json_mode = bool(getattr(args, "json", False))
    repair = bool(getattr(args, "repair", False))

    session = load_session(root, args.task_id, spec_path=spec_path)
    session_file = locate_session_path(root, args.task_id, spec_path=spec_path)
    if session is None:
        error = ScafldError(
            "cannot reconcile without a session ledger",
            [f"missing session file: {relative_path(root, session_file)}"],
            code=ErrorCode.INVALID_SPEC_DOCUMENT,
            exit_code=1,
        )
        if json_mode:
            emit_command_json(
                "reconcile",
                ok=False,
                task_id=args.task_id,
                state={"file": rel_spec},
                error=error_payload(error),
            )
        else:
            print(f"{c(C_RED, 'error')}: {error.message}")
            for detail in error.details:
                print(f"  {detail}")
        sys.exit(error.exit_code)

    before = spec_path.read_text(encoding="utf-8")
    after = rebuild_spec_from_session(before, session)
    drift = after != before
    repaired = bool(drift and repair)
    if repaired:
        spec_path.write_text(after, encoding="utf-8")

    result = {
        "spec_file": rel_spec,
        "session_file": relative_path(root, session_file),
        "drift": drift,
        "matched": not drift,
        "repaired": repaired,
    }

    if json_mode:
        emit_command_json(
            "reconcile",
            ok=not drift or repaired,
            task_id=args.task_id,
            state={"file": rel_spec},
            result=result,
        )
        return

    if drift and not repair:
        print(f"{c(C_YELLOW, 'drift')}: {rel_spec}")
        print("  run with --repair to rebuild runner-derived sections from session")
        sys.exit(1)
    if repaired:
        print(f"{c(C_GREEN, 'repaired')}: {rel_spec}")
    else:
        print(f"{c(C_GREEN, 'clean')}: {rel_spec}")
