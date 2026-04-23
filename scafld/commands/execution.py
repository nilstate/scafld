import sys

from scafld.acceptance import evaluate_acceptance_criterion, record_exec_result
from scafld.command_runtime import require_root
from scafld.error_codes import ErrorCode as EC
from scafld.handoff_renderer import criterion_result_value, current_phase_id, phase_definitions, render_handoff
from scafld.hardening import verify_harden_round_citations
from scafld.output import emit_command_json, error_payload
from scafld.runtime_bundle import ARCHIVE_DIR, DRAFTS_DIR, resolve_prompt_path
from scafld.runtime_contracts import diagnostics_dir, load_llm_settings, relative_path, session_path
from scafld.session_store import (
    append_attempt,
    attempts_for_criterion,
    ensure_session,
    failed_attempts_for_criterion,
    load_session,
    phase_summary_map,
    record_phase_summary,
    set_criterion_state,
    set_phase_block,
    set_recovery_attempt,
    update_latest_attempt_status,
)
from scafld.spec_parsing import extract_spec_cwd, now_iso, parse_acceptance_criteria, require_pyyaml
from scafld.spec_store import load_spec_document, require_spec, yaml_read_field
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


def write_diagnostic_artifact(root, task_id, criterion_id, attempt_number, outcome):
    """Persist full command diagnostics under .ai/runs/{task-id}/diagnostics/."""
    diagnostic_root = diagnostics_dir(root, task_id)
    diagnostic_root.mkdir(parents=True, exist_ok=True)
    path = diagnostic_root / f"{criterion_id}-attempt{attempt_number}.txt"
    output = outcome.get("full_output") or outcome.get("output") or ""
    if not output:
        output = "(no command output captured)"
    path.write_text(output + ("\n" if not output.endswith("\n") else ""), encoding="utf-8")
    return path


def phase_completion_summary(phase):
    criteria = phase.get("acceptance_criteria") if isinstance(phase.get("acceptance_criteria"), list) else []
    passed = sum(1 for criterion in criteria if criterion_result_value(criterion) == "pass")
    files = []
    for change in phase.get("changes") or []:
        if isinstance(change, dict) and change.get("file"):
            files.append(change["file"])
    files_text = ", ".join(files[:4]) if files else "no declared files"
    return (
        f"{phase.get('name') or phase.get('id')}: {passed}/{len(criteria)} acceptance criteria passing; "
        f"files: {files_text}"
    )


def sync_phase_statuses(spec):
    """Update per-phase status fields from current acceptance results."""
    try:
        yaml = require_pyyaml()
    except RuntimeError:
        return load_spec_document(spec)

    data = yaml.safe_load(spec.read_text()) or {}
    phases = data.get("phases")
    if not isinstance(phases, list):
        return data if isinstance(data, dict) else {}

    changed = False
    for phase in phases:
        if not isinstance(phase, dict):
            continue
        criteria = phase.get("acceptance_criteria")
        if not isinstance(criteria, list) or not criteria:
            next_status = "pending"
        else:
            results = [criterion_result_value(criterion) for criterion in criteria]
            if all(result == "pass" for result in results):
                next_status = "completed"
            elif any(result == "failed_exhausted" for result in results):
                next_status = "failed"
            elif any(result in {"pass", "fail", "recovery_pending"} for result in results):
                next_status = "in_progress"
            else:
                next_status = "pending"
        if phase.get("status") != next_status:
            phase["status"] = next_status
            changed = True

    if changed:
        spec.write_text(yaml.safe_dump(data, sort_keys=False))
    return data if isinstance(data, dict) else {}


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

    spec_cwd = extract_spec_cwd(text)

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

    session = ensure_session(root, args.task_id, spec_path=spec)
    llm_settings = load_llm_settings(root)

    passed = 0
    failed = 0
    skipped = 0
    criterion_results = []
    recovery_handoffs = []
    exhausted_criteria = []
    completed_phase_ids = []

    for criterion in runnable:
        ac_id = criterion["id"]
        cmd = criterion["command"]
        expected = criterion.get("expected", "")
        description = criterion.get("description", ac_id)
        phase_id = criterion.get("phase")

        effective_cwd = criterion.get("cwd") or spec_cwd
        cwd_suffix = f"  {c(C_DIM, '(in ' + effective_cwd + '/)')}" if effective_cwd else ""
        if not json_mode:
            print(f"  {c(C_CYAN, ac_id)}: {description}")
            print(f"    $ {c(C_DIM, cmd)}{cwd_suffix}")

        attempt_number = len(attempts_for_criterion(session, ac_id)) + 1
        outcome = evaluate_acceptance_criterion(root, criterion, spec_cwd=spec_cwd)
        ac_passed = outcome["status"] == "pass"
        diagnostic_path = None

        if ac_passed:
            if not json_mode:
                print(f"    {c(C_GREEN, 'PASS')}")
            passed += 1
        else:
            diagnostic_path = write_diagnostic_artifact(root, args.task_id, ac_id, attempt_number, outcome)
            if not json_mode:
                if outcome["exit_code"] is None and outcome["output"].startswith("Command timed out"):
                    print(f"    {c(C_RED, 'TIMEOUT')} ({outcome['output'].split()[-1]})")
                elif outcome["exit_code"] is None:
                    print(f"    {c(C_RED, 'ERROR')}: {outcome['output']}")
                else:
                    print(f"    {c(C_RED, 'FAIL')} (exit code {outcome['exit_code']})")
                if expected:
                    print(f"    expected: {c(C_DIM, expected)}")
                if outcome["output"]:
                    for line in outcome["output"].splitlines()[:5]:
                        print(f"    {c(C_DIM, line)}")
            failed += 1

        attempt_entry, session = append_attempt(
            root,
            args.task_id,
            criterion_id=ac_id,
            phase_id=phase_id,
            status=outcome["status"],
            command=cmd,
            expected=expected,
            cwd=effective_cwd,
            exit_code=outcome["exit_code"],
            output_snippet=outcome["output"],
            diagnostic_path=diagnostic_path,
            spec_path=spec,
        )

        if ac_passed:
            session = set_criterion_state(
                root,
                args.task_id,
                ac_id,
                status="pass",
                phase_id=phase_id,
                reason=None,
                spec_path=spec,
            )
        else:
            failure_attempts = failed_attempts_for_criterion(session, ac_id)
            failure_count = len(failure_attempts)
            max_attempts = llm_settings["recovery_max_attempts"]
            if failure_count <= max_attempts:
                session = set_recovery_attempt(
                    root,
                    args.task_id,
                    ac_id,
                    failure_count,
                    spec_path=spec,
                )
                session = set_criterion_state(
                    root,
                    args.task_id,
                    ac_id,
                    status="recovery_pending",
                    phase_id=phase_id,
                    reason="awaiting_recovery_handoff",
                    spec_path=spec,
                )
                rendered_recovery = render_handoff(
                    root,
                    args.task_id,
                    spec,
                    role="executor",
                    gate="recovery",
                    selector=ac_id,
                    session=session,
                    context={
                        "failed_attempt": attempt_entry,
                        "diagnostic_rel": attempt_entry.get("diagnostic_path"),
                        "criterion_attempts": attempts_for_criterion(session, ac_id),
                        "recovery_attempt": failure_count,
                    },
                )
                recovery_handoffs.append({
                    "criterion_id": ac_id,
                    "role": rendered_recovery["role"],
                    "gate": rendered_recovery["gate"],
                    "handoff_file": rendered_recovery["path_rel"],
                    "handoff_json_file": rendered_recovery["json_path_rel"],
                    "diagnostic_file": attempt_entry.get("diagnostic_path"),
                    "recovery_attempt": failure_count,
                    "max_attempts": max_attempts,
                })
            else:
                exhausted_criteria.append(ac_id)
                attempt_entry["status"] = "failed_exhausted"
                outcome["status"] = "failed_exhausted"
                session = update_latest_attempt_status(
                    root,
                    args.task_id,
                    ac_id,
                    status="failed_exhausted",
                    spec_path=spec,
                )
                session = set_criterion_state(
                    root,
                    args.task_id,
                    ac_id,
                    status="failed_exhausted",
                    phase_id=phase_id,
                    reason="recovery_cap_reached",
                    spec_path=spec,
                )
                session = set_phase_block(
                    root,
                    args.task_id,
                    phase_id,
                    reason=f"recovery exhausted for {ac_id}",
                    spec_path=spec,
                )

        text = record_exec_result(text, ac_id, ac_passed, outcome["output"])
        criterion_payload = dict(outcome)
        criterion_payload["criterion_attempt"] = attempt_entry["criterion_attempt"]
        if attempt_entry.get("diagnostic_path"):
            criterion_payload["diagnostic_path"] = attempt_entry["diagnostic_path"]
        criterion_results.append(criterion_payload)

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
    spec_data = sync_phase_statuses(spec)
    session = load_session(root, args.task_id, spec_path=spec) or session
    recorded_summaries = phase_summary_map(session)

    for phase in phase_definitions(spec_data):
        phase_id = phase.get("id")
        criteria_for_phase = phase.get("acceptance_criteria") if isinstance(phase.get("acceptance_criteria"), list) else []
        if not phase_id or not criteria_for_phase:
            continue
        if all(criterion_result_value(criterion) == "pass" for criterion in criteria_for_phase):
            if phase_id not in recorded_summaries:
                record_phase_summary(root, args.task_id, phase_id, phase_completion_summary(phase), spec_path=spec)
                completed_phase_ids.append(phase_id)
    session = load_session(root, args.task_id, spec_path=spec) or session

    next_handoff = None
    next_action = None
    if failed == 0:
        next_phase = current_phase_id(spec_data)
        if next_phase:
            phase = next((item for item in phase_definitions(spec_data) if item.get("id") == next_phase), None)
            criteria_for_phase = phase.get("acceptance_criteria") if phase and isinstance(phase.get("acceptance_criteria"), list) else []
            if phase and any(criterion_result_value(criterion) != "pass" for criterion in criteria_for_phase):
                rendered_phase = render_handoff(
                    root,
                    args.task_id,
                    spec,
                    role="executor",
                    gate="phase",
                    selector=next_phase,
                    session=session,
                )
                next_handoff = {
                    "role": rendered_phase["role"],
                    "gate": rendered_phase["gate"],
                    "kind": rendered_phase["kind"],
                    "selector": rendered_phase["selector"],
                    "handoff_file": rendered_phase["path_rel"],
                    "handoff_json_file": rendered_phase["json_path_rel"],
                }
                next_action = {
                    "type": "phase_handoff",
                    "role": rendered_phase["role"],
                    "gate": rendered_phase["gate"],
                    "selector": rendered_phase["selector"],
                    "handoff_file": rendered_phase["path_rel"],
                    "handoff_json_file": rendered_phase["json_path_rel"],
                }
    elif exhausted_criteria:
        next_action = {
            "type": "human_required",
            "reason": "recovery_exhausted",
            "criterion_ids": exhausted_criteria,
            "message": "Recovery cap reached; human intervention required.",
        }
    elif recovery_handoffs:
        first_recovery = recovery_handoffs[0]
        next_action = {
            "type": "recovery_handoff",
            "criterion_id": first_recovery["criterion_id"],
            "role": first_recovery["role"],
            "gate": first_recovery["gate"],
            "handoff_file": first_recovery["handoff_file"],
            "handoff_json_file": first_recovery["handoff_json_file"],
        }

    summary = {
        "passed": passed,
        "failed": failed,
        "manual": skipped,
        "skipped_resume": skipped_resume,
        "completed_phases": completed_phase_ids,
        "failed_exhausted": len(exhausted_criteria),
    }

    if json_mode:
        emit_command_json(
            "exec",
            ok=failed == 0,
            task_id=args.task_id,
            state={"status": status},
            result={
                "criteria": criterion_results,
                "summary": summary,
                "session_file": relative_path(root, session_path(root, args.task_id, spec_path=spec)),
                "recovery_handoffs": recovery_handoffs,
                "next_handoff": next_handoff,
                "next_action": next_action,
            },
            error=error_payload(
                (
                    f"{len(exhausted_criteria)} acceptance criterion/criteria exhausted recovery and require human intervention"
                    if exhausted_criteria
                    else f"{failed} acceptance criterion/criteria failed"
                ),
                code=EC.RECOVERY_EXHAUSTED if exhausted_criteria else EC.ACCEPTANCE_FAILED,
                next_action=(
                    "human intervention required"
                    if exhausted_criteria
                    else (
                        f"scafld handoff {args.task_id} --recovery {recovery_handoffs[0]['criterion_id']}"
                        if recovery_handoffs
                        else None
                    )
                ),
                details=(
                    [f"recovery cap reached for: {', '.join(exhausted_criteria)}"]
                    if exhausted_criteria
                    else None
                ),
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
    if completed_phase_ids:
        print(f"  phase summaries: {c(C_DIM, ', '.join(completed_phase_ids))}")
    if exhausted_criteria:
        print(f"  human required: recovery exhausted for {', '.join(exhausted_criteria)}")
    elif recovery_handoffs:
        first = recovery_handoffs[0]
        print(f"  recovery handoff: {first['handoff_file']}")
    elif next_handoff:
        print(f"  next handoff: {next_handoff['handoff_file']}")

    if failed:
        sys.exit(1)
