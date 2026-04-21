import re
import subprocess
import sys

from scafld.command_runtime import require_root
from scafld.error_codes import ErrorCode as EC
from scafld.hardening import verify_harden_round_citations
from scafld.output import emit_command_json, error_payload
from scafld.spec_store import require_spec
from scafld.spec_store import yaml_read_field

from .shared import (
    ARCHIVE_DIR,
    DRAFTS_DIR,
    c,
    check_expected,
    criterion_timeout_seconds,
    now_iso,
    parse_acceptance_criteria,
    record_exec_result,
    require_pyyaml,
    resolve_prompt_path,
    C_BOLD,
    C_CYAN,
    C_DIM,
    C_GREEN,
    C_RED,
    C_YELLOW,
)


def cmd_harden(args):
    """Scaffold HARDEN MODE prompt or mark a hardening round passed."""
    yaml = require_pyyaml()
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

    data = yaml.safe_load(spec.read_text()) or {}
    rounds = data.get("harden_rounds") or []

    if args.mark_passed:
        if not rounds:
            if json_mode:
                emit_command_json(
                    "harden",
                    ok=False,
                    task_id=args.task_id,
                    state={"file": str(rel), "harden_status": data.get("harden_status")},
                    error=error_payload(
                        "no hardening round to mark passed",
                        code=EC.MISSING_HARDEN_ROUND,
                        next_action=f"scafld harden {args.task_id}",
                        exit_code=1,
                    ),
                )
            else:
                print(f"{c(C_RED, 'error')}: no hardening round to mark passed")
                print(f"  run {c(C_BOLD, f'scafld harden {args.task_id}')} first, then re-run with --mark-passed")
            sys.exit(1)
        citation_warnings = verify_harden_round_citations(root, ARCHIVE_DIR, rounds[-1])
        data["harden_status"] = "passed"
        rounds[-1]["ended_at"] = now_iso()
        rounds[-1]["outcome"] = "passed"
        data["harden_rounds"] = rounds
        data["updated"] = now_iso()
        spec.write_text(yaml.safe_dump(data, sort_keys=False))
        if json_mode:
            emit_command_json(
                "harden",
                task_id=args.task_id,
                state={"file": str(rel), "harden_status": "passed", "round": rounds[-1]["round"]},
                result={
                    "action": "round_passed",
                    "citation_warnings": citation_warnings,
                },
                warnings=citation_warnings,
            )
            return
        if citation_warnings:
            print(f"{c(C_YELLOW, 'warn')}: {len(citation_warnings)} harden citation(s) could not be resolved")
            for warning in citation_warnings:
                print(f"  - {warning}")
        print(f"{c(C_GREEN, 'hardened')}: {rel}")
        print(f"  harden_status: passed  round: {rounds[-1]['round']}")
        return

    prompt_path = resolve_prompt_path(root, "harden.md")
    if not prompt_path.exists():
        if json_mode:
            emit_command_json(
                "harden",
                ok=False,
                task_id=args.task_id,
                state={"file": str(rel)},
                error=error_payload(
                    f"harden prompt missing at {prompt_path}",
                    code=EC.PROMPT_MISSING,
                    exit_code=1,
                ),
            )
        else:
            print(f"{c(C_RED, 'error')}: harden prompt missing at {prompt_path}")
        sys.exit(1)

    next_round = len(rounds) + 1
    rounds.append({
        "round": next_round,
        "started_at": now_iso(),
        "outcome": "in_progress",
        "questions": [],
    })
    data["harden_status"] = "in_progress"
    data["harden_rounds"] = rounds
    data["updated"] = now_iso()
    spec.write_text(yaml.safe_dump(data, sort_keys=False))

    prompt_text = prompt_path.read_text()
    if json_mode:
        emit_command_json(
            "harden",
            task_id=args.task_id,
            state={"file": str(rel), "harden_status": "in_progress", "round": next_round},
            result={
                "action": "round_opened",
                "prompt": prompt_text,
                "mark_passed_command": f"scafld harden {args.task_id} --mark-passed",
            },
        )
        return

    print(prompt_text)
    print()
    print("---")
    print(f"spec: {rel}")
    print(f"round: {next_round}")
    print(f"when done, mark the round passed: {c(C_BOLD, f'scafld harden {args.task_id} --mark-passed')}")


def cmd_exec(args):
    """Run acceptance criteria commands and record results."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    text = spec.read_text()
    json_mode = bool(getattr(args, "json", False))

    status = yaml_read_field(text, "status")
    if status not in ("in_progress", "approved"):
        if json_mode:
            emit_command_json(
                "exec",
                ok=False,
                task_id=args.task_id,
                state={"status": status},
                error=error_payload(
                    f"spec must be in_progress or approved to exec (current: {status})",
                    code=EC.INVALID_SPEC_STATUS,
                    exit_code=1,
                ),
            )
        else:
            print(f"{c(C_RED, 'error')}: spec must be in_progress or approved to exec (current: {status})")
        sys.exit(1)

    spec_cwd = None
    match = re.search(r'^\s+context:.*?\n(?:\s+\S.*\n)*?\s+cwd:\s*"?([^"\n]+)"?', text, re.MULTILINE)
    if match:
        spec_cwd = match.group(1).strip().strip('"').strip("'")

    criteria = parse_acceptance_criteria(text)
    if not criteria:
        warning = "no acceptance criteria found in spec"
        if json_mode:
            emit_command_json(
                "exec",
                task_id=args.task_id,
                state={"status": status},
                result={"criteria": [], "summary": {"passed": 0, "failed": 0, "manual": 0, "skipped_resume": 0}},
                warnings=[warning],
            )
        else:
            print(f"{c(C_YELLOW, 'warn')}: {warning}")
        return

    if args.phase:
        criteria = [criterion for criterion in criteria if criterion.get("phase") == args.phase]
        if not criteria:
            warning = f"no criteria found for phase {args.phase}"
            if json_mode:
                emit_command_json(
                    "exec",
                    task_id=args.task_id,
                    state={"status": status},
                    result={"criteria": [], "summary": {"passed": 0, "failed": 0, "manual": 0, "skipped_resume": 0}},
                    warnings=[warning],
                )
            else:
                print(f"{c(C_YELLOW, 'warn')}: {warning}")
            return

    skipped_resume = 0
    if args.resume:
        before = len(criteria)
        criteria = [criterion for criterion in criteria if criterion.get("result") != "pass"]
        skipped_resume = before - len(criteria)

    runnable = [criterion for criterion in criteria if criterion.get("command") and criterion["command"] != "TODO"]
    manual = [criterion for criterion in criteria if not criterion.get("command") or criterion["command"] == "TODO"]

    if not runnable and not manual and not skipped_resume:
        warning = "no criteria found"
        if json_mode:
            emit_command_json(
                "exec",
                task_id=args.task_id,
                state={"status": status},
                result={"criteria": [], "summary": {"passed": 0, "failed": 0, "manual": 0, "skipped_resume": 0}},
                warnings=[warning],
            )
        else:
            print(f"{c(C_YELLOW, 'warn')}: {warning}")
        return

    if not json_mode:
        print(f"{c(C_BOLD, f'Executing acceptance criteria for {args.task_id}')}")
        if args.phase:
            print(f"  phase: {args.phase}")
        if skipped_resume:
            print(f"  {c(C_DIM, f'resume: skipping {skipped_resume} already-passed criteria')}")
        print()

    passed = 0
    failed = 0
    skipped = 0
    criterion_results = []

    for criterion in runnable:
        ac_id = criterion["id"]
        cmd = criterion["command"]
        expected = criterion.get("expected", "")
        description = criterion.get("description", ac_id)

        effective_cwd = criterion.get("cwd") or spec_cwd
        ac_cwd = root
        if effective_cwd:
            ac_cwd = (root / effective_cwd).resolve()
            if not str(ac_cwd).startswith(str(root.resolve())):
                message = f"cwd '{effective_cwd}' escapes workspace root"
                if not json_mode:
                    print(f"  {c(C_CYAN, ac_id)}: {description}")
                    print(f"    {c(C_RED, 'ERROR')}: {message}")
                text = record_exec_result(text, ac_id, False, message)
                criterion_results.append({
                    "id": ac_id,
                    "description": description,
                    "phase": criterion.get("phase"),
                    "command": cmd,
                    "cwd": effective_cwd,
                    "expected": expected,
                    "status": "fail",
                    "exit_code": None,
                    "output": message,
                })
                failed += 1
                continue
            if not ac_cwd.is_dir():
                message = f"cwd '{effective_cwd}' not found"
                if not json_mode:
                    print(f"  {c(C_CYAN, ac_id)}: {description}")
                    print(f"    {c(C_RED, 'ERROR')}: cwd '{effective_cwd}' is not a directory")
                text = record_exec_result(text, ac_id, False, message)
                criterion_results.append({
                    "id": ac_id,
                    "description": description,
                    "phase": criterion.get("phase"),
                    "command": cmd,
                    "cwd": effective_cwd,
                    "expected": expected,
                    "status": "fail",
                    "exit_code": None,
                    "output": message,
                })
                failed += 1
                continue
            cwd_suffix = f"  {c(C_DIM, '(in ' + effective_cwd + '/)')}"
        else:
            cwd_suffix = ""
        if not json_mode:
            print(f"  {c(C_CYAN, ac_id)}: {description}")
            print(f"    $ {c(C_DIM, cmd)}{cwd_suffix}")

        try:
            timeout_seconds = criterion_timeout_seconds(criterion)
        except ValueError as exc:
            if not json_mode:
                print(f"    {c(C_RED, 'ERROR')}: {exc}")
            text = record_exec_result(text, ac_id, False, str(exc))
            criterion_results.append({
                "id": ac_id,
                "description": description,
                "phase": criterion.get("phase"),
                "command": cmd,
                "cwd": effective_cwd,
                "expected": expected,
                "status": "fail",
                "exit_code": None,
                "output": str(exc),
            })
            failed += 1
            continue

        try:
            result = subprocess.run(
                cmd,
                shell=True,
                capture_output=True,
                text=True,
                timeout=timeout_seconds,
                cwd=str(ac_cwd),
            )
            output = (result.stdout + result.stderr).strip()
            ac_passed = check_expected(result.returncode, output, expected)

            if ac_passed:
                if not json_mode:
                    print(f"    {c(C_GREEN, 'PASS')}")
                passed += 1
            else:
                if not json_mode:
                    print(f"    {c(C_RED, 'FAIL')} (exit code {result.returncode})")
                    if expected:
                        print(f"    expected: {c(C_DIM, expected)}")
                    if output:
                        for line in output.splitlines()[:5]:
                            print(f"    {c(C_DIM, line)}")
                failed += 1

            text = record_exec_result(text, ac_id, ac_passed, output[:200])
            criterion_results.append({
                "id": ac_id,
                "description": description,
                "phase": criterion.get("phase"),
                "command": cmd,
                "cwd": effective_cwd,
                "expected": expected,
                "status": "pass" if ac_passed else "fail",
                "exit_code": result.returncode,
                "output": output[:200],
            })
        except subprocess.TimeoutExpired:
            message = f"Command timed out after {timeout_seconds}s"
            if not json_mode:
                print(f"    {c(C_RED, 'TIMEOUT')} ({timeout_seconds}s)")
            text = record_exec_result(text, ac_id, False, message)
            criterion_results.append({
                "id": ac_id,
                "description": description,
                "phase": criterion.get("phase"),
                "command": cmd,
                "cwd": effective_cwd,
                "expected": expected,
                "status": "fail",
                "exit_code": None,
                "output": message,
            })
            failed += 1
        except Exception as exc:
            if not json_mode:
                print(f"    {c(C_RED, 'ERROR')}: {exc}")
            text = record_exec_result(text, ac_id, False, str(exc))
            criterion_results.append({
                "id": ac_id,
                "description": description,
                "phase": criterion.get("phase"),
                "command": cmd,
                "cwd": effective_cwd,
                "expected": expected,
                "status": "fail",
                "exit_code": None,
                "output": str(exc),
            })
            failed += 1

    for criterion in manual:
        ac_id = criterion["id"]
        description = criterion.get("description", ac_id)
        if not json_mode:
            print(f"  {c(C_DIM, ac_id)}: {description} (manual - skipped)")
        criterion_results.append({
            "id": ac_id,
            "description": description,
            "phase": criterion.get("phase"),
            "command": criterion.get("command", ""),
            "cwd": criterion.get("cwd") or spec_cwd,
            "expected": criterion.get("expected", ""),
            "status": "manual",
            "exit_code": None,
            "output": "",
        })
        skipped += 1

    spec.write_text(text)

    summary = {
        "passed": passed,
        "failed": failed,
        "manual": skipped,
        "skipped_resume": skipped_resume,
    }

    if json_mode:
        emit_command_json(
            "exec",
            ok=failed == 0,
            task_id=args.task_id,
            state={"status": status},
            result={"criteria": criterion_results, "summary": summary},
            error=error_payload(
                f"{failed} acceptance criterion/criteria failed",
                code=EC.ACCEPTANCE_FAILED,
                exit_code=1,
            ) if failed else None,
        )
        if failed:
            sys.exit(1)
        return

    print()
    summary_parts = []
    if passed or skipped_resume:
        total_pass = passed + skipped_resume
        label = f"{total_pass} passed"
        if skipped_resume:
            label += f" ({skipped_resume} prior)"
        summary_parts.append(c(C_GREEN, label))
    if failed:
        summary_parts.append(c(C_RED, f"{failed} failed"))
    if skipped:
        summary_parts.append(c(C_DIM, f"{skipped} manual"))
    print(f"  {' / '.join(summary_parts)}")

    if failed:
        sys.exit(1)
