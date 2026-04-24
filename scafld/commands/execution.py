import sys

from scafld.command_runtime import require_root
from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.execution_runtime import (
    empty_exec_summary,
    exec_snapshot,
    harden_open_snapshot,
    harden_pass_snapshot,
)
from scafld.output import emit_command_json, error_payload
from scafld.runtime_bundle import DRAFTS_DIR
from scafld.spec_parsing import require_pyyaml
from scafld.spec_store import require_spec
from scafld.terminal import C_BOLD, C_CYAN, C_DIM, C_GREEN, C_RED, C_YELLOW, c


def cmd_harden(args):
    """Scaffold HARDEN MODE prompt or mark a hardening round passed."""
    try:
        yaml = require_pyyaml()
    except RuntimeError:
        print(f"{c(C_RED, 'error')}: scafld harden requires PyYAML")
        print("  install it into the Python runtime that executes scafld:")
        print(f"  {c(C_BOLD, 'python3 -m pip install PyYAML')}")
        sys.exit(1)
    root = require_root()
    spec = require_spec(root, args.task_id)
    json_mode = bool(getattr(args, "json", False))

    rel = spec.relative_to(root)
    if not str(rel).startswith(DRAFTS_DIR):
        if json_mode:
            emit_command_json(
                "harden",
                ok=False,
                task_id=args.task_id,
                state={"file": str(rel)},
                error=error_payload(
                    f"harden only operates on drafts (current: {rel.parent})",
                    code=EC.INVALID_SPEC_LOCATION,
                    details=[f"spec must live in {DRAFTS_DIR}/"],
                    exit_code=1,
                ),
            )
        else:
            print(f"{c(C_RED, 'error')}: harden only operates on drafts (current: {rel.parent})")
            print(f"  spec must live in {DRAFTS_DIR}/")
        sys.exit(1)

    if args.mark_passed:
        try:
            snapshot = harden_pass_snapshot(root, args.task_id)
        except ScafldError as exc:
            if json_mode:
                emit_command_json(
                    "harden",
                    ok=False,
                    task_id=args.task_id,
                    state={"file": str(rel), "harden_status": (yaml.safe_load(spec.read_text()) or {}).get("harden_status")},
                    error=error_payload(exc.message, code=exc.code, details=exc.details, next_action=exc.next_action, exit_code=exc.exit_code),
                )
            else:
                print(f"{c(C_RED, 'error')}: {exc.message}")
                if exc.next_action:
                    print(f"  run {c(C_BOLD, exc.next_action)} first, then re-run with --mark-passed")
            sys.exit(exc.exit_code)
        if json_mode:
            emit_command_json(
                "harden",
                task_id=args.task_id,
                state=snapshot["state"],
                result=snapshot["result"],
                warnings=snapshot["warnings"],
            )
            return
        citation_warnings = snapshot["warnings"]
        if citation_warnings:
            print(f"{c(C_YELLOW, 'warn')}: {len(citation_warnings)} harden citation(s) could not be resolved")
            for warning in citation_warnings:
                print(f"  - {warning}")
        print(f"{c(C_GREEN, 'hardened')}: {rel}")
        print(f"  harden_status: passed  round: {snapshot['state']['round']}")
        return

    try:
        snapshot = harden_open_snapshot(root, args.task_id)
    except ScafldError as exc:
        if json_mode:
            emit_command_json(
                "harden",
                ok=False,
                task_id=args.task_id,
                state={"file": str(rel)},
                error=error_payload(exc.message, code=exc.code, details=exc.details, next_action=exc.next_action, exit_code=exc.exit_code),
            )
        else:
            print(f"{c(C_RED, 'error')}: {exc.message}")
        sys.exit(exc.exit_code)
    if json_mode:
        emit_command_json(
            "harden",
            task_id=args.task_id,
            state=snapshot["state"],
            result=snapshot["result"],
        )
        return

    print(snapshot["result"]["prompt"])
    print()
    print("---")
    print(f"spec: {rel}")
    print(f"round: {snapshot['state']['round']}")
    print(f"when done, mark the round passed: {c(C_BOLD, snapshot['result']['mark_passed_command'])}")


def print_exec_payload(task_id, payload, *, phase=None, resume=False):
    state = payload.get("state") or {}
    result = payload.get("result") or {}
    criteria = result.get("criteria") or []
    summary = result.get("summary") or empty_exec_summary()
    warnings = payload.get("warnings") or []
    error = payload.get("error") or {}

    print(f"{c(C_BOLD, f'Executing acceptance criteria for {task_id}')}")
    if phase:
        print(f"  phase: {phase}")
    if resume and summary.get("skipped_resume"):
        print(f"  {c(C_DIM, f'resume: skipping {summary['skipped_resume']} already-passed criteria')}")
    if warnings:
        for warning in warnings:
            print(f"  {c(C_YELLOW, 'warn')}: {warning}")
    if criteria:
        print()

    for criterion in criteria:
        criterion_id = criterion.get("id") or "unknown"
        description = criterion.get("description") or criterion_id
        command = criterion.get("command") or ""
        cwd = criterion.get("cwd") or ""
        status = criterion.get("status") or "unknown"
        output = criterion.get("output") or ""
        expected = criterion.get("expected") or ""
        exit_code = criterion.get("exit_code")

        if status == "manual":
            print(f"  {c(C_DIM, criterion_id)}: {description} (manual - skipped)")
            continue

        print(f"  {c(C_CYAN, criterion_id)}: {description}")
        if command:
            cwd_suffix = f"  {c(C_DIM, '(in ' + cwd + '/)')}" if cwd else ""
            print(f"    $ {c(C_DIM, command)}{cwd_suffix}")
        if status == "pass":
            print(f"    {c(C_GREEN, 'PASS')}")
        elif status == "failed_exhausted":
            print(f"    {c(C_RED, 'FAIL')} (recovery exhausted)")
        elif exit_code is None and output.startswith("Command timed out"):
            print(f"    {c(C_RED, 'TIMEOUT')} ({output.split()[-1]})")
        elif exit_code is None:
            print(f"    {c(C_RED, 'ERROR')}: {output}")
        else:
            print(f"    {c(C_RED, 'FAIL')} (exit code {exit_code})")
        if expected and status != "pass":
            print(f"    expected: {c(C_DIM, expected)}")
        if output and status != "pass":
            for line in output.splitlines()[:5]:
                print(f"    {c(C_DIM, line)}")

    print()
    summary_parts = []
    if summary.get("passed") or summary.get("skipped_resume"):
        total_pass = (summary.get("passed") or 0) + (summary.get("skipped_resume") or 0)
        label = f"{total_pass} passed"
        if summary.get("skipped_resume"):
            label += f" ({summary['skipped_resume']} prior)"
        summary_parts.append(c(C_GREEN, label))
    if summary.get("failed"):
        summary_parts.append(c(C_RED, f"{summary['failed']} failed"))
    if summary.get("manual"):
        summary_parts.append(c(C_DIM, f"{summary['manual']} manual"))
    if summary_parts:
        print(f"  {' / '.join(summary_parts)}")
    if summary.get("completed_phases"):
        print(f"  phase summaries: {c(C_DIM, ', '.join(summary['completed_phases']))}")

    next_action = result.get("next_action") or {}
    if next_action.get("type") == "human_required":
        exhausted = ", ".join(next_action.get("criterion_ids") or [])
        print(f"  human required: recovery exhausted for {exhausted}")
    elif (result.get("recovery_handoffs") or []):
        print(f"  recovery handoff: {result['recovery_handoffs'][0]['handoff_file']}")
    elif result.get("next_handoff"):
        print(f"  next handoff: {result['next_handoff']['handoff_file']}")
    if error.get("message") and not result.get("recovery_handoffs") and next_action.get("type") != "human_required":
        print(f"  {c(C_RED, 'error')}: {error['message']}")


def cmd_exec(args):
    """Run acceptance criteria commands and record results."""
    root = require_root()
    json_mode = bool(getattr(args, "json", False))
    payload, exit_code = exec_snapshot(root, args.task_id, phase=args.phase, resume=args.resume)

    if json_mode:
        emit_command_json(
            "exec",
            ok=payload.get("ok", False),
            task_id=args.task_id,
            state=payload.get("state"),
            result=payload.get("result"),
            warnings=payload.get("warnings") or [],
            error=payload.get("error"),
        )
    else:
        print_exec_payload(args.task_id, payload, phase=args.phase, resume=args.resume)

    if exit_code:
        sys.exit(exit_code)
