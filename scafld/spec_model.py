import datetime


def now_iso():
    return datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).strftime("%Y-%m-%dT%H:%M:%SZ")


def parse_iso8601_timestamp(value):
    if not value:
        return None
    try:
        return datetime.datetime.fromisoformat(str(value).replace("Z", "+00:00"))
    except ValueError:
        return None


def parse_numeric_scalar(value):
    if value in (None, "", "null", "{}"):
        return None
    try:
        return float(str(value).strip().strip('"').strip("'"))
    except (TypeError, ValueError):
        return None


def _as_model(source):
    if isinstance(source, dict):
        return source
    raise TypeError("spec model helpers require a normalized spec dict")


def task_block(spec):
    task = _as_model(spec).get("task")
    return task if isinstance(task, dict) else {}


def task_context(spec):
    context = task_block(spec).get("context")
    return context if isinstance(context, dict) else {}


def task_acceptance(spec):
    acceptance = task_block(spec).get("acceptance")
    return acceptance if isinstance(acceptance, dict) else {}


def phase_definitions(spec):
    phases = _as_model(spec).get("phases")
    return phases if isinstance(phases, list) else []


def get_status(spec):
    return str(_as_model(spec).get("status") or "")


def get_task_id(spec):
    return str(_as_model(spec).get("task_id") or "")


def get_title(spec):
    return str(task_block(spec).get("title") or "")


def get_size(spec):
    return str(task_block(spec).get("size") or "")


def get_risk(spec):
    return str(task_block(spec).get("risk_level") or "")


def get_updated(spec):
    return str(_as_model(spec).get("updated") or "")


def extract_spec_cwd(spec):
    return task_context(spec).get("cwd")


def extract_self_eval_score(spec):
    data = _as_model(spec)
    for key in ("self_eval", "perf_eval"):
        block = data.get(key)
        if isinstance(block, dict):
            for field in ("total", "score"):
                score = parse_numeric_scalar(block.get(field))
                if score is not None:
                    return score
        score = parse_numeric_scalar(block)
        if score is not None:
            return score
    return parse_numeric_scalar(data.get("score"))


def criterion_result_value(criterion):
    result = criterion.get("result")
    if isinstance(result, dict):
        return result.get("status")
    return result


def parse_phase_status_entries(spec):
    data = _as_model(spec)
    top_level_status = data.get("status")
    entries = []
    for phase in phase_definitions(data):
        if not isinstance(phase, dict):
            continue
        phase_id = phase.get("id")
        if not phase_id:
            continue
        status = phase.get("status") or "pending"
        if top_level_status == "completed" and status in ("pending", "in_progress"):
            status = "completed"
        entries.append({"id": phase_id, "status": status})
    return entries


def parse_phase_statuses(spec):
    return [entry["status"] for entry in parse_phase_status_entries(spec)]


def count_phases(spec):
    phase_statuses = parse_phase_statuses(spec)
    total = len(phase_statuses)
    completed = sum(1 for status in phase_statuses if status == "completed")
    failed = sum(1 for status in phase_statuses if status == "failed")
    in_progress = sum(1 for status in phase_statuses if status == "in_progress")
    return total, completed, failed, in_progress


def active_done_open(spec, status=None):
    status = status or get_status(spec)
    total, completed, failed, in_progress = count_phases(spec)
    return status == "in_progress" and total > 0 and completed == total and failed == 0 and in_progress == 0


def supersession_payload(spec):
    data = _as_model(spec)
    supersession = data.get("supersession")
    if not isinstance(supersession, dict):
        origin = data.get("origin")
        if isinstance(origin, dict):
            supersession = origin.get("supersession")
    if isinstance(supersession, dict):
        superseded_by = supersession.get("superseded_by") or ""
        superseded_at = supersession.get("superseded_at") or ""
        reason = supersession.get("reason") or ""
    else:
        superseded_by = ""
        superseded_at = ""
        reason = ""
    return {
        "superseded": bool(superseded_by),
        "superseded_by": superseded_by,
        "superseded_at": superseded_at,
        "reason": reason,
    }


def parse_acceptance_criteria(spec):
    criteria = []
    for phase in phase_definitions(spec):
        if not isinstance(phase, dict):
            continue
        phase_id = phase.get("id")
        blocks = []
        acceptance = phase.get("acceptance_criteria")
        if isinstance(acceptance, list):
            blocks.append(acceptance)
        validation = phase.get("validation")
        if isinstance(validation, list):
            blocks.append(validation)
        for block in blocks:
            for entry in block:
                if not isinstance(entry, dict):
                    continue
                criterion_id = entry.get("id") or entry.get("dod_id")
                if not criterion_id:
                    continue
                criterion = {"id": criterion_id, "phase": phase_id}
                for key in (
                    "type",
                    "description",
                    "command",
                    "expected",
                    "cwd",
                    "timeout_seconds",
                    "expected_kind",
                    "expected_exit_code",
                    "evidence_required",
                    "status",
                    "source_event",
                    "last_attempt",
                    "checked_at",
                    "evidence",
                ):
                    if key in entry:
                        criterion[key] = entry.get(key)
                result = entry.get("result")
                if isinstance(result, dict):
                    if "status" in result:
                        criterion["result"] = result.get("status")
                elif result not in (None, ""):
                    criterion["result"] = result
                if "result" not in criterion and entry.get("status") not in (None, "", "pending"):
                    criterion["result"] = entry.get("status")
                criteria.append(criterion)
    return criteria
