import json
import sys

from scafld.errors import ScafldError


def emit_json(payload, stream=None):
    """Emit machine-readable command output for automation callers."""
    stream = stream or sys.stdout
    json.dump(payload, stream, indent=2)
    stream.write("\n")


def error_payload(error, *, code=None, message=None, details=None, next_action=None, exit_code=None):
    """Normalize structured error data for JSON command envelopes."""
    if isinstance(error, ScafldError):
        return {
            "code": code or error.code,
            "message": message or error.message,
            "details": details if details is not None else list(error.details),
            "next_action": next_action if next_action is not None else error.next_action,
            "exit_code": exit_code if exit_code is not None else error.exit_code,
        }

    return {
        "code": code or "command_failed",
        "message": message or str(error),
        "details": details if details is not None else [],
        "next_action": next_action,
        "exit_code": exit_code if exit_code is not None else 1,
    }


def emit_command_json(command, *, ok=True, task_id=None, state=None, result=None, warnings=None, error=None, stream=None):
    """Emit one stable machine-facing command envelope."""
    payload = {
        "ok": bool(ok),
        "command": command,
        "warnings": list(warnings or []),
        "state": state or {},
        "result": result or {},
        "error": error,
    }
    if task_id is not None:
        payload["task_id"] = task_id
    emit_json(payload, stream=stream)


def emit_cli_error(error, colorize=None, red_code="", stream=None):
    """Render a structured command error to the terminal."""
    stream = stream or sys.stderr
    colorize = colorize or (lambda _code, text: text)
    stream.write(f"{colorize(red_code, 'error')}: {error.message}\n")
    for detail in error.details:
        stream.write(f"  {detail}\n")
