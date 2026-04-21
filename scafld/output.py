import json
import sys

from scafld.error_codes import ErrorCode
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
        "code": code or ErrorCode.COMMAND_FAILED,
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


def projection_check(model):
    """Derive a CI-friendly check payload from one projection model."""
    review = model.get("review") or {}
    sync = model.get("sync") or {}
    acceptance = model.get("acceptance") or {}
    phases = model.get("phases") or {}

    reasons = []
    if sync.get("status") == "drift":
        reasons.extend(sync.get("reasons") or ["git drift detected"])
    if review.get("verdict") == "fail" or review.get("blocking_count", 0):
        reasons.append(f"{review.get('blocking_count', 0)} blocking review finding(s)")
    if acceptance.get("failed", 0):
        reasons.append(f"{acceptance.get('failed', 0)} acceptance criterion/criteria failed")
    if phases.get("failed", 0):
        reasons.append(f"{phases.get('failed', 0)} phase(s) failed")

    if reasons:
        conclusion = "failure"
        summary = reasons[0]
    elif review.get("verdict") in ("pass", "pass_with_issues") and acceptance.get("failed", 0) == 0:
        conclusion = "success"
        summary = f"review {review.get('verdict')}"
    elif model.get("status") == "completed":
        conclusion = "success"
        summary = "spec completed and archived"
    else:
        conclusion = "pending"
        summary = f"status {model.get('status', 'unknown')}"

    details = [
        f"status: {model.get('status', 'unknown')}",
        (
            "acceptance: "
            f"{acceptance.get('passed', 0)} passed, "
            f"{acceptance.get('failed', 0)} failed, "
            f"{acceptance.get('pending', 0)} pending"
        ),
        (
            "phases: "
            f"{phases.get('completed', 0)} completed, "
            f"{phases.get('in_progress', 0)} active, "
            f"{phases.get('pending', 0)} pending"
        ),
    ]
    if sync.get("status") and sync.get("status") != "unbound":
        details.append(f"sync: {sync.get('status')}")
    if review.get("exists"):
        verdict = review.get("verdict") or review.get("round_status") or "incomplete"
        details.append(f"review: {verdict}")

    return {
        "status": conclusion,
        "summary": summary,
        "details": details,
    }


def render_projection_summary(model):
    """Render a concise markdown summary for chat comments or job summaries."""
    origin = model.get("origin") or {}
    git = origin.get("git") or {}
    sync = model.get("sync") or {}
    review = model.get("review") or {}
    acceptance = model.get("acceptance") or {}
    phases = model.get("phases") or {}

    lines = [
        f"## scafld: {model.get('title', model.get('task_id', 'task'))}",
        "",
        f"- Task: `{model.get('task_id', '')}`",
        f"- Status: `{model.get('status', 'unknown')}`",
        f"- Spec: `{model.get('file', '')}`",
    ]
    if git.get("branch"):
        branch_line = f"- Branch: `{git.get('branch')}`"
        if git.get("base_ref"):
            branch_line += f" from `{git.get('base_ref')}`"
        lines.append(branch_line)
    if sync.get("status") and sync.get("status") != "unbound":
        sync_line = f"- Sync: `{sync.get('status')}`"
        if sync.get("reasons"):
            sync_line += f" - {sync['reasons'][0]}"
        lines.append(sync_line)
    review_line = "- Review: not started"
    if review.get("exists"):
        verdict = review.get("verdict") or review.get("round_status") or "incomplete"
        review_line = f"- Review: `{verdict}`"
        if review.get("blocking_count"):
            review_line += f" with {review['blocking_count']} blocking finding(s)"
        elif review.get("non_blocking_count"):
            review_line += f" with {review['non_blocking_count']} non-blocking finding(s)"
    lines.append(review_line)
    lines.extend(
        [
            (
                "- Acceptance: "
                f"{acceptance.get('passed', 0)} passed, "
                f"{acceptance.get('failed', 0)} failed, "
                f"{acceptance.get('pending', 0)} pending"
            ),
            (
                "- Phases: "
                f"{phases.get('completed', 0)} completed, "
                f"{phases.get('in_progress', 0)} active, "
                f"{phases.get('pending', 0)} pending"
            ),
            "",
            model.get("summary", ""),
        ]
    )
    return "\n".join(line for line in lines if line is not None).strip() + "\n"


def render_projection_pr_body(model):
    """Render a fuller markdown view for PR bodies or issue updates."""
    origin = model.get("origin") or {}
    source = origin.get("source") or {}
    git = origin.get("git") or {}
    sync = model.get("sync") or {}
    review = model.get("review") or {}
    acceptance = model.get("acceptance") or {}
    phases = model.get("phases") or {}

    lines = [
        f"# {model.get('title', model.get('task_id', 'Task'))}",
        "",
        model.get("summary", ""),
        "",
        "## Workflow State",
        "",
        f"- Task: `{model.get('task_id', '')}`",
        f"- Spec: `{model.get('file', '')}`",
        f"- Status: `{model.get('status', 'unknown')}`",
    ]
    if source.get("system") or source.get("kind") or source.get("id"):
        source_bits = [bit for bit in (source.get("system"), source.get("kind"), source.get("id")) if bit]
        source_line = f"- Source: {' '.join(source_bits)}"
        if source.get("url"):
            source_line += f" ({source['url']})"
        lines.append(source_line)
    if git.get("branch"):
        branch_line = f"- Branch: `{git.get('branch')}`"
        if git.get("base_ref"):
            branch_line += f" from `{git.get('base_ref')}`"
        lines.append(branch_line)
    if sync.get("status") and sync.get("status") != "unbound":
        sync_line = f"- Sync: `{sync.get('status')}`"
        if sync.get("reasons"):
            sync_line += f" - {sync['reasons'][0]}"
        lines.append(sync_line)

    review_line = "- Review: not started"
    if review.get("exists"):
        verdict = review.get("verdict") or review.get("round_status") or "incomplete"
        review_line = f"- Review: `{verdict}`"
        if review.get("blocking_count"):
            review_line += f" with {review['blocking_count']} blocking finding(s)"
        elif review.get("non_blocking_count"):
            review_line += f" with {review['non_blocking_count']} non-blocking finding(s)"
    lines.extend(
        [
            review_line,
            (
                "- Acceptance: "
                f"{acceptance.get('passed', 0)} passed, "
                f"{acceptance.get('failed', 0)} failed, "
                f"{acceptance.get('pending', 0)} pending"
            ),
            (
                "- Phases: "
                f"{phases.get('completed', 0)} completed, "
                f"{phases.get('in_progress', 0)} active, "
                f"{phases.get('pending', 0)} pending"
            ),
        ]
    )

    objectives = model.get("objectives") or []
    if objectives:
        lines.extend(["", "## Objectives", ""])
        lines.extend(f"- {objective}" for objective in objectives)

    risks = model.get("risks") or []
    if risks:
        lines.extend(["", "## Risks", ""])
        lines.extend(f"- {risk}" for risk in risks)

    return "\n".join(lines).strip() + "\n"
