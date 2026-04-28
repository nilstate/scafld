from scafld.command_runtime import require_root
from scafld.output import emit_command_json
from scafld.workflow_runtime import build_snapshot, plan_snapshot


def print_build_summary(task_id, payload):
    state = payload.get("state") or {}
    result = payload.get("result") or {}
    summary = result.get("summary") or {}
    criteria = result.get("criteria") or []
    next_action = result.get("next_action") or {}
    current_handoff = result.get("current_handoff") or {}
    warnings = payload.get("warnings") or []
    error = payload.get("error") or {}

    print(f"Build: {task_id}")
    print(f" status: {state.get('status') or 'unknown'}")

    parts = []
    if summary.get("passed"):
        parts.append(f"{summary['passed']} passed")
    if summary.get("failed"):
        parts.append(f"{summary['failed']} failed")
    if summary.get("manual"):
        parts.append(f"{summary['manual']} manual")
    if summary.get("skipped_resume"):
        parts.append(f"{summary['skipped_resume']} prior")
    if parts:
        print(f"summary: {' / '.join(parts)}")
    if summary.get("completed_phases"):
        print(f" phases: {', '.join(summary['completed_phases'])}")
    if summary.get("skipped_resume"):
        print(f" resume: skipping {summary['skipped_resume']} already-passed criteria")
    for criterion in criteria[:10]:
        criterion_id = criterion.get("id") or "unknown"
        description = criterion.get("description") or criterion_id
        status = (criterion.get("status") or "").lower()
        output = criterion.get("output") or ""
        expected = criterion.get("expected") or ""
        exit_code = criterion.get("exit_code")
        if status == "pass":
            status_label = "PASS"
        elif status == "failed_exhausted":
            status_label = "FAILED_EXHAUSTED"
        elif status == "fail":
            status_label = "FAIL"
        elif status == "manual":
            status_label = "MANUAL"
        else:
            status_label = status.upper() or "UNKNOWN"
        print(f"  {criterion_id}: {description} [{status_label}]")
        if status in {"fail", "failed_exhausted"}:
            if exit_code is None and output.startswith("Command timed out"):
                print(f"    TIMEOUT ({output.split()[-1]})")
            elif exit_code is None and output:
                print(f"    ERROR: {output}")
            elif exit_code is not None:
                print(f"    FAIL (exit code {exit_code})")
            if expected:
                print(f"    expected: {expected}")
            if output:
                for line in output.splitlines()[:5]:
                    print(f"    {line}")
    if current_handoff.get("handoff_file"):
        print(f"handoff: {current_handoff['handoff_file']}")
    if next_action.get("command"):
        print(f"   next: {next_action['command']}")
    elif next_action.get("message"):
        print(f"   next: {next_action['message']}")
    if next_action.get("followup_command"):
        print(f"followup: {next_action['followup_command']}")
    if error and error.get("message"):
        print(f"  block: {error['message']}")
    for warning in warnings[:5]:
        print(f"  warn: {warning}")


def cmd_plan(args):
    """Create a draft spec or reopen harden on an existing draft."""
    root = require_root()
    payload, exit_code = plan_snapshot(
        root,
        args.task_id,
        title=getattr(args, "title", None),
        size=getattr(args, "size", None),
        risk=getattr(args, "risk", None),
        command=getattr(args, "command", None),
        files=getattr(args, "files", None),
    )

    if getattr(args, "json", False):
        emit_command_json(
            "plan",
            ok=payload.get("ok", False),
            task_id=args.task_id,
            state=payload.get("state"),
            result=payload.get("result"),
            error=payload.get("error"),
            warnings=payload.get("warnings") or [],
        )
    else:
        if not payload.get("ok"):
            error = payload.get("error") or {}
            print(f"error: {error.get('message') or 'plan failed'}")
            if error.get("next_action"):
                print(f"  next: {error['next_action']}")
        else:
            state = payload.get("state") or {}
            result = payload.get("result") or {}
            print(f"Plan: {args.task_id}")
            if result.get("reused_existing_draft"):
                print(f" status: reopened {state.get('file')}")
            else:
                print(f" status: created {state.get('file')}")
            print()
            prompt = result.get("prompt")
            if prompt:
                print(prompt, end="" if prompt.endswith("\n") else "\n")
            if result.get("mark_passed_command"):
                print()
                print(f"  next: {result['mark_passed_command']}")
            if result.get("approve_command"):
                print(f"followup: {result['approve_command']}")
    if exit_code:
        raise SystemExit(exit_code)


def cmd_build(args):
    """Start approved work and drive execution to the next handoff or block."""
    root = require_root()
    payload, exit_code = build_snapshot(root, args.task_id)
    if getattr(args, "json", False):
        emit_command_json(
            "build",
            ok=payload.get("ok", False),
            task_id=args.task_id,
            state=payload.get("state"),
            result=payload.get("result"),
            error=payload.get("error"),
            warnings=payload.get("warnings") or [],
        )
    else:
        print_build_summary(args.task_id, payload)
    if exit_code:
        raise SystemExit(exit_code)
