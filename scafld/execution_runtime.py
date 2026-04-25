import json
import re

from scafld.acceptance import evaluate_acceptance_criterion, record_exec_result
from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.handoff_renderer import criterion_result_value, current_phase_id, phase_definitions, render_handoff
from scafld.hardening import verify_harden_round_citations
from scafld.output import error_payload
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
from scafld.spec_parsing import extract_spec_cwd, now_iso, parse_acceptance_criteria
from scafld.spec_store import load_spec_document, require_spec, write_spec_document, yaml_read_field


def harden_open_snapshot(root, task_id):
    spec = require_spec(root, task_id)
    rel = spec.relative_to(root)
    if not str(rel).startswith(DRAFTS_DIR):
        raise ScafldError(
            f"harden only operates on drafts (current: {rel.parent})",
            [f"spec must live in {DRAFTS_DIR}/"],
            code=EC.INVALID_SPEC_LOCATION,
        )

    data = load_spec_document(spec)
    rounds = data.get("harden_rounds") or []
    prompt_path = resolve_prompt_path(root, "harden.md")
    if not prompt_path.exists():
        raise ScafldError(
            f"harden prompt missing at {prompt_path}",
            code=EC.PROMPT_MISSING,
        )

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
    write_spec_document(spec, data)
    prompt_text = prompt_path.read_text()
    return {
        "state": {"file": str(rel), "harden_status": "in_progress", "round": next_round},
        "result": {
            "action": "round_opened",
            "prompt": prompt_text,
            "mark_passed_command": f"scafld harden {task_id} --mark-passed",
        },
    }


def harden_pass_snapshot(root, task_id):
    spec = require_spec(root, task_id)
    rel = spec.relative_to(root)
    if not str(rel).startswith(DRAFTS_DIR):
        raise ScafldError(
            f"harden only operates on drafts (current: {rel.parent})",
            [f"spec must live in {DRAFTS_DIR}/"],
            code=EC.INVALID_SPEC_LOCATION,
        )

    data = load_spec_document(spec)
    rounds = data.get("harden_rounds") or []
    if not rounds:
        raise ScafldError(
            "no hardening round to mark passed",
            code=EC.MISSING_HARDEN_ROUND,
            next_action=f"scafld harden {task_id}",
        )
    citation_warnings = verify_harden_round_citations(root, ARCHIVE_DIR, rounds[-1])
    data["harden_status"] = "passed"
    rounds[-1]["ended_at"] = now_iso()
    rounds[-1]["outcome"] = "passed"
    data["harden_rounds"] = rounds
    data["updated"] = now_iso()
    write_spec_document(spec, data)
    return {
        "state": {"file": str(rel), "harden_status": "passed", "round": rounds[-1]["round"]},
        "result": {
            "action": "round_passed",
            "citation_warnings": citation_warnings,
        },
        "warnings": citation_warnings,
    }


def empty_exec_summary():
    return {"passed": 0, "failed": 0, "manual": 0, "skipped_resume": 0}


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
    text = spec.read_text()
    data = load_spec_document(spec)
    phases = data.get("phases")
    if not isinstance(phases, list):
        return data if isinstance(data, dict) else {}

    next_statuses = {}
    changed = False
    for phase in phases:
        if not isinstance(phase, dict):
            continue
        phase_id = phase.get("id")
        if not phase_id:
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
        next_statuses[phase_id] = next_status
        if phase.get("status") != next_status:
            phase["status"] = next_status
            changed = True

    if changed:
        spec.write_text(_rewrite_phase_status_fields(text, next_statuses))
    return data if isinstance(data, dict) else {}


def _rewrite_phase_status_fields(text, phase_statuses):
    lines = text.splitlines(True)
    result = []
    i = 0
    in_phases = False

    while i < len(lines):
        line = lines[i]
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())

        if not in_phases:
            result.append(line)
            if re.match(r"^phases:\s*$", line):
                in_phases = True
            i += 1
            continue

        if stripped and indent == 0 and not stripped.startswith("- "):
            in_phases = False
            result.append(line)
            i += 1
            continue

        match = re.match(r'^(\s*)-\s+id:\s*"?(phase\d+)"?\s*$', line)
        if not match:
            result.append(line)
            i += 1
            continue

        phase_id = match.group(2)
        item_indent = len(match.group(1))
        field_indent = " " * (item_indent + 2)
        result.append(line)
        i += 1

        preserved = []
        insert_at = None
        while i < len(lines):
            field_line = lines[i]
            if not field_line.strip():
                preserved.append(field_line)
                i += 1
                continue

            field_indent_level = len(field_line) - len(field_line.lstrip())
            if field_indent_level <= item_indent:
                break
            if field_indent_level == item_indent and field_line.strip().startswith("- "):
                break

            if field_indent_level == len(field_indent) and re.match(r"^\s+status:\s*(.+)$", field_line):
                if insert_at is None:
                    insert_at = len(preserved)
                i += 1
                continue

            preserved.append(field_line)
            i += 1

        if insert_at is None:
            insert_at = len(preserved)
        preserved[insert_at:insert_at] = [f"{field_indent}status: {json.dumps(phase_statuses[phase_id])}\n"]
        result.extend(preserved)

    return "".join(result)


def exec_snapshot(root, task_id, *, phase=None, resume=False):
    spec = require_spec(root, task_id)
    text = spec.read_text()
    spec_data = load_spec_document(spec)
    status = yaml_read_field(text, "status")
    if status not in ("in_progress", "approved"):
        return ({
            "ok": False,
            "command": "exec",
            "task_id": task_id,
            "state": {"status": status},
            "result": None,
            "warnings": [],
            "error": error_payload(
                f"spec must be in_progress or approved to exec (current: {status})",
                code=EC.INVALID_SPEC_STATUS,
                exit_code=1,
            ),
        }, 1)

    spec_cwd = extract_spec_cwd(text)
    criteria = parse_acceptance_criteria(text)
    resolved_phase = phase or current_phase_id(spec_data)
    if not criteria:
        warning = "no acceptance criteria found in spec"
        return ({
            "ok": True,
            "command": "exec",
            "task_id": task_id,
            "state": {"status": status},
            "result": {"criteria": [], "summary": empty_exec_summary()},
            "warnings": [warning],
            "error": None,
        }, 0)

    if resolved_phase:
        criteria = [criterion for criterion in criteria if criterion.get("phase") == resolved_phase]
        if not criteria:
            return ({
                "ok": False,
                "command": "exec",
                "task_id": task_id,
                "state": {"status": status},
                "result": None,
                "warnings": [],
                "error": error_payload(
                    f"phase not found or has no criteria: {resolved_phase}",
                    code=EC.PHASE_NOT_FOUND,
                    exit_code=1,
                ),
            }, 1)

    session = load_session(root, task_id, spec_path=spec) or {"attempts": []}
    skipped_resume = 0
    if resume:
        before = len(criteria)
        criteria = [criterion for criterion in criteria if criterion_result_value(criterion) != "pass"]
        skipped_resume = before - len(criteria)

    runnable = [criterion for criterion in criteria if criterion.get("command")]
    manual = [criterion for criterion in criteria if not criterion.get("command")]
    if not runnable and not manual and not skipped_resume:
        warning = "no runnable acceptance criteria found"
        return ({
            "ok": True,
            "command": "exec",
            "task_id": task_id,
            "state": {"status": status},
            "result": {"criteria": [], "summary": empty_exec_summary()},
            "warnings": [warning],
            "error": None,
        }, 0)
    if not runnable and not manual:
        return ({
            "ok": True,
            "command": "exec",
            "task_id": task_id,
            "state": {"status": status},
            "result": {"criteria": [], "summary": {"passed": 0, "failed": 0, "manual": 0, "skipped_resume": skipped_resume}},
            "warnings": [],
            "error": None,
        }, 0)

    session = ensure_session(root, task_id, spec_path=spec)
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
        phase_id = criterion.get("phase")
        effective_cwd = criterion.get("cwd") or spec_cwd

        attempt_number = len(attempts_for_criterion(session, ac_id)) + 1
        outcome = evaluate_acceptance_criterion(root, criterion, spec_cwd=spec_cwd)
        ac_passed = outcome["status"] == "pass"
        diagnostic_path = None

        if ac_passed:
            passed += 1
        else:
            diagnostic_path = write_diagnostic_artifact(root, task_id, ac_id, attempt_number, outcome)
            failed += 1

        attempt_entry, session = append_attempt(
            root,
            task_id,
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
                task_id,
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
                session = set_recovery_attempt(root, task_id, ac_id, failure_count, spec_path=spec)
                session = set_criterion_state(
                    root,
                    task_id,
                    ac_id,
                    status="recovery_pending",
                    phase_id=phase_id,
                    reason="awaiting_recovery_handoff",
                    spec_path=spec,
                )
                rendered_recovery = render_handoff(
                    root,
                    task_id,
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
                session = update_latest_attempt_status(root, task_id, ac_id, status="failed_exhausted", spec_path=spec)
                session = set_criterion_state(
                    root,
                    task_id,
                    ac_id,
                    status="failed_exhausted",
                    phase_id=phase_id,
                    reason="recovery_cap_reached",
                    spec_path=spec,
                )
                session = set_phase_block(
                    root,
                    task_id,
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
        criterion_results.append({
            "id": criterion["id"],
            "description": criterion.get("description", criterion["id"]),
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
    session = load_session(root, task_id, spec_path=spec) or session
    recorded_summaries = phase_summary_map(session)

    for phase_definition in phase_definitions(spec_data):
        phase_id = phase_definition.get("id")
        criteria_for_phase = phase_definition.get("acceptance_criteria") if isinstance(phase_definition.get("acceptance_criteria"), list) else []
        if not phase_id or not criteria_for_phase:
            continue
        if all(criterion_result_value(criterion) == "pass" for criterion in criteria_for_phase):
            if phase_id not in recorded_summaries:
                record_phase_summary(root, task_id, phase_id, phase_completion_summary(phase_definition), spec_path=spec)
                completed_phase_ids.append(phase_id)
    session = load_session(root, task_id, spec_path=spec) or session

    next_handoff = None
    next_action = None
    if failed == 0:
        next_phase = current_phase_id(spec_data)
        if next_phase:
            phase_definition = next((item for item in phase_definitions(spec_data) if item.get("id") == next_phase), None)
            criteria_for_phase = phase_definition.get("acceptance_criteria") if phase_definition and isinstance(phase_definition.get("acceptance_criteria"), list) else []
            if phase_definition and any(criterion_result_value(criterion) != "pass" for criterion in criteria_for_phase):
                rendered_phase = render_handoff(
                    root,
                    task_id,
                    spec,
                    role="executor",
                    gate="phase",
                    selector=next_phase,
                    session=session,
                )
                next_handoff = {
                    "role": rendered_phase["role"],
                    "gate": rendered_phase["gate"],
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
    error = None
    exit_code = 0
    if failed:
        exit_code = 1
        error = error_payload(
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
                    f"scafld handoff {task_id} --recovery {recovery_handoffs[0]['criterion_id']}"
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
        )

    payload = {
        "ok": failed == 0,
        "command": "exec",
        "task_id": task_id,
        "state": {
            "status": status,
            "executed_phase": resolved_phase,
        },
        "result": {
            "executed_phase": resolved_phase,
            "criteria": criterion_results,
            "summary": summary,
            "session_file": relative_path(root, session_path(root, task_id, spec_path=spec)),
            "recovery_handoffs": recovery_handoffs,
            "next_handoff": next_handoff,
            "next_action": next_action,
        },
        "warnings": [],
        "error": error,
    }
    return payload, exit_code
