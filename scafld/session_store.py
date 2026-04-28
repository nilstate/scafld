import hashlib
import json
import os
from collections import defaultdict
from copy import deepcopy

from scafld.runtime_contracts import (
    SESSION_SCHEMA_VERSION,
    ensure_run_dirs,
    locate_session_path,
    load_llm_settings,
    relative_path,
    session_path,
)
from scafld.spec_parsing import now_iso


PROVIDER_INVOCATION_STATUSES = (
    "running",
    "completed",
    "failed",
    "failed_model_unavailable",
    "failed_transient",
    "cancelled",
    "timed_out",
    "spawn_failed",
    "invalid_output",
    "invalid_artifact",
    "workspace_mutated",
)
PROVIDER_CONFIDENCE_VALUES = ("observed", "inferred", "requested_only", "unknown")
STRONG_REVIEW_ISOLATION_LEVELS = {"codex_read_only_ephemeral"}


def default_session(task_id, *, model_profile):
    timestamp = now_iso()
    return {
        "schema_version": SESSION_SCHEMA_VERSION,
        "task_id": task_id,
        "created_at": timestamp,
        "updated_at": timestamp,
        "model_profile": model_profile,
        "entries": [],
        "recovery_attempts": {},
        "criterion_states": {},
        "phases": [],
        "attempts": [],
        "phase_summaries": [],
        "workspace_baseline": None,
        "usage": {},
    }


def normalize_session(session):
    session["schema_version"] = SESSION_SCHEMA_VERSION
    session.setdefault("entries", [])
    session.setdefault("recovery_attempts", {})
    session.setdefault("criterion_states", {})
    session.setdefault("phases", [])
    session.setdefault("attempts", [])
    session.setdefault("phase_summaries", [])
    session.setdefault("workspace_baseline", None)
    session.setdefault("usage", {})
    return session


def snapshot_workspace_path(root, relative_path):
    path = root / relative_path
    try:
        if path.is_symlink():
            target = os.readlink(path)
            return {
                "kind": "symlink",
                "sha256": hashlib.sha256(target.encode("utf-8", errors="surrogateescape")).hexdigest(),
            }
        if not path.exists():
            return {"kind": "missing"}
        if path.is_dir():
            return {"kind": "directory"}
        payload = path.read_bytes()
        return {
            "kind": "file",
            "sha256": hashlib.sha256(payload).hexdigest(),
            "size": len(payload),
        }
    except OSError as exc:
        return {
            "kind": "unreadable",
            "error": str(exc),
        }


def snapshot_workspace_paths(root, paths):
    return {
        path: snapshot_workspace_path(root, path)
        for path in sorted({path for path in paths if isinstance(path, str) and path.strip()})
    }


def workspace_baseline_payload(root, paths, *, source):
    captured_at = now_iso()
    path_states = snapshot_workspace_paths(root, paths)
    return {
        "captured_at": captured_at,
        "source": source,
        "paths": path_states,
    }


def load_session(root, task_id, *, spec_path=None):
    path = locate_session_path(root, task_id, spec_path=spec_path)
    if not path.exists():
        return None
    return normalize_session(json.loads(path.read_text(encoding="utf-8")))


def write_session(root, task_id, session, *, spec_path=None):
    ensure_run_dirs(root, task_id, spec_path=spec_path)
    session["updated_at"] = now_iso()
    path = session_path(root, task_id, spec_path=spec_path)
    path.write_text(json.dumps(session, indent=2, sort_keys=False) + "\n", encoding="utf-8")
    return path


def ensure_session(root, task_id, *, model_profile=None, spec_path=None):
    session = load_session(root, task_id, spec_path=spec_path)
    if session is not None:
        return session
    settings = load_llm_settings(root)
    session = default_session(task_id, model_profile=model_profile or settings["model_profile"])
    write_session(root, task_id, session, spec_path=spec_path)
    return session


def record_workspace_baseline(root, task_id, *, paths, source, spec_path=None):
    baseline = workspace_baseline_payload(root, paths, source=source)

    def apply(session):
        session["workspace_baseline"] = baseline
        append_entry(
            session,
            "workspace_baseline",
            source=source,
            path_count=len(baseline["paths"]),
            recorded_at=baseline["captured_at"],
        )

    return mutate_session(root, task_id, apply, spec_path=spec_path)


def ensure_workspace_baseline(root, task_id, *, paths, source, spec_path=None):
    session = ensure_session(root, task_id, spec_path=spec_path)
    baseline = session.get("workspace_baseline")
    if isinstance(baseline, dict) and isinstance(baseline.get("paths"), dict):
        return session
    return record_workspace_baseline(root, task_id, paths=paths, source=source, spec_path=spec_path)


def effective_changed_paths(root, actual_paths, session):
    baseline = session.get("workspace_baseline") if isinstance(session, dict) else None
    baseline_paths = baseline.get("paths") if isinstance(baseline, dict) else None
    if not isinstance(baseline_paths, dict):
        return sorted({path for path in actual_paths if isinstance(path, str) and path.strip()})

    effective = []
    for path in sorted({path for path in actual_paths if isinstance(path, str) and path.strip()}):
        baseline_state = baseline_paths.get(path)
        if baseline_state is None:
            effective.append(path)
            continue
        current_state = snapshot_workspace_path(root, path)
        if current_state != baseline_state:
            effective.append(path)
    return effective


def mutate_session(root, task_id, mutator, *, model_profile=None, spec_path=None):
    session = ensure_session(root, task_id, model_profile=model_profile, spec_path=spec_path)
    mutable = deepcopy(session)
    mutator(mutable)
    write_session(root, task_id, mutable, spec_path=spec_path)
    return mutable


def phase_entry(session, phase_id):
    phases = session.setdefault("phases", [])
    for entry in phases:
        if entry.get("phase_id") == phase_id:
            return entry

    entry = {
        "phase_id": phase_id,
        "attempt_count": 0,
        "criterion_ids": [],
        "completed_at": None,
        "blocked_at": None,
        "blocked_reason": None,
    }
    phases.append(entry)
    return entry


def append_entry(
    session,
    entry_type,
    *,
    replace_keys=None,
    recorded_at=None,
    **fields,
):
    entry = {
        "type": entry_type,
        "recorded_at": recorded_at or now_iso(),
        **fields,
    }
    entries = session.setdefault("entries", [])
    if replace_keys:
        for index, existing in enumerate(entries):
            if existing.get("type") != entry_type:
                continue
            if all(existing.get(key) == value for key, value in replace_keys.items()):
                merged = dict(existing)
                merged.update(entry)
                entries[index] = merged
                return merged
    entries.append(entry)
    return entry


def append_attempt(
    root,
    task_id,
    *,
    criterion_id,
    phase_id,
    status,
    command,
    expected,
    cwd,
    exit_code,
    output_snippet,
    diagnostic_path=None,
    spec_path=None,
):
    def apply(session):
        attempts = session.setdefault("attempts", [])
        criterion_attempts = sum(1 for item in attempts if item.get("criterion_id") == criterion_id)
        attempt = {
            "attempt_index": len(attempts) + 1,
            "criterion_id": criterion_id,
            "phase_id": phase_id,
            "criterion_attempt": criterion_attempts + 1,
            "status": status,
            "command": command,
            "expected": expected,
            "cwd": cwd,
            "exit_code": exit_code,
            "output_snippet": output_snippet,
            "diagnostic_path": relative_path(root, diagnostic_path) if diagnostic_path else None,
            "recorded_at": now_iso(),
        }
        attempts.append(attempt)
        append_entry(
            session,
            "attempt",
            criterion_id=criterion_id,
            phase_id=phase_id,
            attempt_index=attempt["attempt_index"],
            criterion_attempt=attempt["criterion_attempt"],
            status=status,
            command=command,
            expected=expected,
            cwd=cwd,
            exit_code=exit_code,
            output_snippet=output_snippet,
            diagnostic_path=attempt["diagnostic_path"],
        )

        phase = phase_entry(session, phase_id)
        phase["attempt_count"] = phase.get("attempt_count", 0) + 1
        criterion_ids = phase.setdefault("criterion_ids", [])
        if criterion_id not in criterion_ids:
            criterion_ids.append(criterion_id)

    session = mutate_session(root, task_id, apply, spec_path=spec_path)
    return session.get("attempts", [])[-1], session


def set_recovery_attempt(root, task_id, criterion_id, count, *, spec_path=None):
    def apply(session):
        session.setdefault("recovery_attempts", {})[criterion_id] = count

    return mutate_session(root, task_id, apply, spec_path=spec_path)


def increment_recovery_attempt(root, task_id, criterion_id, *, spec_path=None):
    counts = ensure_session(root, task_id, spec_path=spec_path).setdefault("recovery_attempts", {})
    next_count = int(counts.get(criterion_id, 0)) + 1
    session = set_recovery_attempt(root, task_id, criterion_id, next_count, spec_path=spec_path)
    return next_count, session


def record_phase_summary(root, task_id, phase_id, summary, *, spec_path=None):
    def apply(session):
        summaries = session.setdefault("phase_summaries", [])
        timestamp = now_iso()
        for entry in summaries:
            if entry.get("phase_id") == phase_id:
                entry["summary"] = summary
                entry["created_at"] = timestamp
                break
        else:
            summaries.append({
                "phase_id": phase_id,
                "summary": summary,
                "created_at": timestamp,
            })

        phase = phase_entry(session, phase_id)
        phase["completed_at"] = timestamp
        phase["blocked_at"] = None
        phase["blocked_reason"] = None
        append_entry(
            session,
            "phase_summary",
            replace_keys={"phase_id": phase_id},
            phase_id=phase_id,
            summary=summary,
            recorded_at=timestamp,
        )

    return mutate_session(root, task_id, apply, spec_path=spec_path)


def set_criterion_state(root, task_id, criterion_id, *, status, phase_id=None, reason=None, spec_path=None):
    def apply(session):
        session.setdefault("criterion_states", {})[criterion_id] = {
            "status": status,
            "phase_id": phase_id,
            "reason": reason,
            "updated_at": now_iso(),
        }

    return mutate_session(root, task_id, apply, spec_path=spec_path)


def set_phase_block(root, task_id, phase_id, *, reason, spec_path=None):
    def apply(session):
        phase = phase_entry(session, phase_id)
        blocked_at = now_iso()
        phase["blocked_at"] = blocked_at
        phase["blocked_reason"] = reason
        append_entry(
            session,
            "phase_block",
            phase_id=phase_id,
            reason=reason,
            recorded_at=blocked_at,
        )

    return mutate_session(root, task_id, apply, spec_path=spec_path)


def update_latest_attempt_status(root, task_id, criterion_id, *, status, spec_path=None):
    def apply(session):
        for attempt in reversed(session.setdefault("attempts", [])):
            if attempt.get("criterion_id") == criterion_id:
                attempt["status"] = status
                break
        for entry in reversed(session.setdefault("entries", [])):
            if entry.get("type") == "attempt" and entry.get("criterion_id") == criterion_id:
                entry["status"] = status
                break

    return mutate_session(root, task_id, apply, spec_path=spec_path)


def record_approval(root, task_id, *, gate, actor="human", note=None, spec_path=None):
    def apply(session):
        append_entry(
            session,
            "approval",
            gate=gate,
            actor=actor,
            note=note,
        )

    return mutate_session(root, task_id, apply, spec_path=spec_path)


def record_challenge_verdict(
    root,
    task_id,
    *,
    gate,
    review_round,
    verdict,
    blocked,
    blocking_count,
    non_blocking_count,
    reviewer_mode,
    review_file,
    handoff_file=None,
    spec_path=None,
):
    def apply(session):
        append_entry(
            session,
            "challenge_verdict",
            replace_keys={"gate": gate, "review_round": review_round},
            gate=gate,
            review_round=review_round,
            verdict=verdict,
            blocked=bool(blocked),
            blocking_count=blocking_count,
            non_blocking_count=non_blocking_count,
            reviewer_mode=reviewer_mode,
            review_file=review_file,
            handoff_file=handoff_file,
        )

    return mutate_session(root, task_id, apply, spec_path=spec_path)


def record_human_override(
    root,
    task_id,
    *,
    gate,
    review_round,
    reason,
    confirmed_at,
    review_file,
    spec_path=None,
):
    def apply(session):
        append_entry(
            session,
            "human_override",
            gate=gate,
            review_round=review_round,
            reason=reason,
            review_file=review_file,
            recorded_at=confirmed_at,
        )

    return mutate_session(root, task_id, apply, spec_path=spec_path)


def record_provider_invocation(
    root,
    task_id,
    *,
    invocation_id=None,
    role,
    gate,
    provider,
    provider_bin=None,
    provider_requested=None,
    model_requested="",
    model_observed="",
    model_source="unknown",
    isolation_level="",
    isolation_downgraded=False,
    fallback_policy="",
    confidence=None,
    status="completed",
    started_at="",
    completed_at="",
    exit_code=None,
    timed_out=False,
    timeout_seconds=None,
    pid=None,
    provider_session_requested="",
    provider_session_observed="",
    command="",
    diagnostic_path=None,
    warning="",
    review_packet="",
    repair_handoff="",
    repair_handoff_json="",
    schema_arg_attached=False,
    schema_load_error="",
    spec_path=None,
):
    if confidence is None:
        if model_observed and model_source == "inferred":
            confidence = "inferred"
        elif model_observed:
            confidence = "observed"
        elif model_requested:
            confidence = "requested_only"
        else:
            confidence = "unknown"
    status = _normalize_provider_invocation_value(
        status,
        allowed=PROVIDER_INVOCATION_STATUSES,
        field="provider invocation status",
        default="completed",
    )
    confidence = _normalize_provider_invocation_value(
        confidence,
        allowed=PROVIDER_CONFIDENCE_VALUES,
        field="provider invocation confidence",
        default="unknown",
    )

    def apply(session):
        fields = {
            "invocation_id": invocation_id or "",
            "role": role,
            "gate": gate,
            "provider": provider,
            "provider_bin": provider_bin or provider,
            "provider_requested": provider_requested or provider,
            "model_requested": model_requested or "",
            "model_observed": model_observed or "",
            "model_source": model_source or "unknown",
            "isolation_level": isolation_level or "",
            "isolation_downgraded": bool(isolation_downgraded),
            "fallback_policy": fallback_policy or "",
            "confidence": confidence,
            "status": status or "completed",
            "timed_out": bool(timed_out),
        }
        if not fields["invocation_id"]:
            fields.pop("invocation_id")
        if started_at:
            fields["started_at"] = started_at
        if completed_at:
            fields["completed_at"] = completed_at
        if exit_code is not None:
            fields["exit_code"] = exit_code
        if timeout_seconds is not None:
            fields["timeout_seconds"] = timeout_seconds
        if pid is not None:
            fields["pid"] = pid
        if provider_session_requested:
            fields["provider_session_requested"] = provider_session_requested
        if provider_session_observed:
            fields["provider_session_observed"] = provider_session_observed
        if command:
            fields["command"] = command
        if diagnostic_path:
            fields["diagnostic_path"] = diagnostic_path
        if warning:
            fields["warning"] = warning
        if review_packet:
            fields["review_packet"] = review_packet
        if repair_handoff:
            fields["repair_handoff"] = repair_handoff
        if repair_handoff_json:
            fields["repair_handoff_json"] = repair_handoff_json
        fields["schema_arg_attached"] = bool(schema_arg_attached)
        if schema_load_error:
            fields["schema_load_error"] = schema_load_error
        replace_keys = {"invocation_id": invocation_id} if invocation_id else None
        append_entry(session, "provider_invocation", replace_keys=replace_keys, **fields)

    return mutate_session(root, task_id, apply, spec_path=spec_path)


def _normalize_provider_invocation_value(value, *, allowed, field, default):
    normalized = str(value or default).strip() or default
    if normalized not in allowed:
        raise ValueError(f"{field} must be one of: {', '.join(allowed)}")
    return normalized


def _trusted_observed_provider_model(entry):
    model = entry.get("model_observed") or ""
    if not model:
        return ""
    confidence = entry.get("confidence")
    if confidence in {None, "", "observed"}:
        return model
    return ""


def _provider_model_separation(provider_invocations):
    latest_by_role = {}
    for entry in provider_invocations:
        role = entry.get("role")
        if role in {"executor", "challenger"}:
            latest_by_role[role] = entry

    executor = latest_by_role.get("executor")
    challenger = latest_by_role.get("challenger")
    if executor is None and challenger is None:
        return {
            "state": "none",
            "executor_model_observed": "",
            "challenger_model_observed": "",
        }
    if executor is None:
        return {
            "state": "unknown_executor",
            "executor_model_observed": "",
            "challenger_model_observed": _trusted_observed_provider_model(challenger),
        }
    if challenger is None:
        return {
            "state": "unknown_challenger",
            "executor_model_observed": _trusted_observed_provider_model(executor),
            "challenger_model_observed": "",
        }

    executor_model = _trusted_observed_provider_model(executor)
    challenger_model = _trusted_observed_provider_model(challenger)
    if not executor_model and not challenger_model:
        state = "unknown_both"
    elif not executor_model:
        state = "unknown_executor"
    elif not challenger_model:
        state = "unknown_challenger"
    elif executor_model == challenger_model:
        state = "same_model"
    else:
        state = "separated"
    return {
        "state": state,
        "executor_model_observed": executor_model,
        "challenger_model_observed": challenger_model,
    }


def attempts_for_criterion(session, criterion_id):
    return [attempt for attempt in session.get("attempts", []) if attempt.get("criterion_id") == criterion_id]


def latest_failed_attempt(session, criterion_id=None):
    attempts = session.get("attempts", [])
    for attempt in reversed(attempts):
        if attempt.get("status") not in {"fail", "failed_exhausted"}:
            continue
        if criterion_id and attempt.get("criterion_id") != criterion_id:
            continue
        return attempt
    return None


def failed_attempts_for_criterion(session, criterion_id):
    return [
        attempt
        for attempt in attempts_for_criterion(session, criterion_id)
        if attempt.get("status") in {"fail", "failed_exhausted"}
    ]


def phase_summary_map(session):
    return {
        entry.get("phase_id"): entry
        for entry in session.get("phase_summaries", [])
        if entry.get("phase_id")
    }


def prior_phase_summary(session, ordered_phase_ids, current_phase_id):
    if current_phase_id not in ordered_phase_ids:
        return None
    summaries = phase_summary_map(session)
    current_index = ordered_phase_ids.index(current_phase_id)
    for phase_id in reversed(ordered_phase_ids[:current_index]):
        summary = summaries.get(phase_id)
        if summary:
            return summary
    return None


def provider_invocation_process_alive(entry):
    if not isinstance(entry, dict) or entry.get("status") != "running":
        return None
    try:
        pid = int(entry.get("pid"))
    except (TypeError, ValueError):
        return None
    if pid <= 0:
        return None
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    except OSError:
        return False
    return True


def provider_invocation_status_payload(entry):
    if not isinstance(entry, dict):
        return None
    keys = (
        "invocation_id",
        "role",
        "gate",
        "provider",
        "provider_requested",
        "model_requested",
        "model_observed",
        "model_source",
        "confidence",
        "status",
        "started_at",
        "completed_at",
        "pid",
        "provider_session_requested",
        "provider_session_observed",
        "command",
        "timeout_seconds",
        "timed_out",
        "exit_code",
        "diagnostic_path",
        "warning",
        "review_packet",
        "repair_handoff",
        "repair_handoff_json",
    )
    payload = {key: entry[key] for key in keys if key in entry}
    if entry.get("status") == "running":
        payload["process_alive"] = provider_invocation_process_alive(entry)
    return payload


def session_summary_payload(session):
    attempts = session.get("attempts", [])
    entries = session.get("entries", []) if isinstance(session.get("entries"), list) else []
    first_attempts = {}
    recovered_total = 0
    recovered_pass = 0
    phase_attempts = defaultdict(int)

    for attempt in attempts:
        criterion_id = attempt.get("criterion_id")
        phase_id = attempt.get("phase_id") or "unknown"
        phase_attempts[phase_id] += 1
        first_attempts.setdefault(criterion_id, []).append(attempt)

    first_attempt_passed = 0
    first_attempt_total = 0
    for criterion_id, criterion_attempts in first_attempts.items():
        if not criterion_attempts:
            continue
        ordered = sorted(criterion_attempts, key=lambda item: item.get("criterion_attempt", 0))
        first_attempt_total += 1
        if ordered[0].get("status") == "pass":
            first_attempt_passed += 1
            continue

        recovered_total += 1
        if any(item.get("status") == "pass" for item in ordered[1:]):
            recovered_pass += 1

    usage = session.get("usage") if isinstance(session.get("usage"), dict) else {}
    criterion_states = session.get("criterion_states") if isinstance(session.get("criterion_states"), dict) else {}
    failed_exhausted = sum(
        1
        for entry in criterion_states.values()
        if isinstance(entry, dict) and entry.get("status") == "failed_exhausted"
    )
    latest_challenges = {}
    for entry in entries:
        if entry.get("type") != "challenge_verdict":
            continue
        key = (entry.get("gate"), entry.get("review_round"))
        latest_challenges[key] = entry
    challenge_verdicts = list(latest_challenges.values())
    blocked_challenge_keys = {
        (entry.get("gate"), entry.get("review_round"))
        for entry in challenge_verdicts
        if entry.get("blocked")
    }
    override_entries = [entry for entry in entries if entry.get("type") == "human_override"]
    provider_invocations = [entry for entry in entries if entry.get("type") == "provider_invocation"]
    running_provider_invocations = [entry for entry in provider_invocations if entry.get("status") == "running"]
    latest_provider_invocation = provider_invocations[-1] if provider_invocations else None
    active_provider_invocation = (
        latest_provider_invocation
        if latest_provider_invocation and latest_provider_invocation.get("status") == "running"
        else None
    )
    provider_invocations_by_role = defaultdict(int)
    provider_confidence = defaultdict(int)
    provider_statuses = defaultdict(int)
    provider_weaker_review_isolation = 0
    provider_models_observed = 0
    provider_models_inferred = 0
    provider_models_unknown = 0
    for entry in provider_invocations:
        provider_invocations_by_role[entry.get("role") or "unknown"] += 1
        provider_confidence[entry.get("confidence") or "unknown"] += 1
        provider_statuses[entry.get("status") or "unknown"] += 1
        if entry.get("model_observed") and entry.get("confidence") == "inferred":
            provider_models_inferred += 1
        elif entry.get("model_observed") and (entry.get("confidence") in {None, "", "observed"}):
            provider_models_observed += 1
        elif not entry.get("model_observed"):
            provider_models_unknown += 1
        if (
            entry.get("role") == "challenger"
            and entry.get("gate") == "review"
            and entry.get("isolation_level")
            and entry.get("isolation_level") not in STRONG_REVIEW_ISOLATION_LEVELS
        ):
            provider_weaker_review_isolation += 1
    provider_model_separation = _provider_model_separation(provider_invocations) if provider_invocations else {
        "state": "none",
        "executor_model_observed": "",
        "challenger_model_observed": "",
    }
    override_keys = {
        (entry.get("gate"), entry.get("review_round"))
        for entry in override_entries
    }
    challenge_blocked = len(blocked_challenge_keys | override_keys)
    challenge_overrides = len(override_entries)
    return {
        "attempt_count": len(attempts),
        "entry_count": len(entries),
        "phase_count": len(session.get("phases", [])),
        "first_attempt_passed": first_attempt_passed,
        "first_attempt_total": first_attempt_total,
        "recovered_pass": recovered_pass,
        "recovered_total": recovered_total,
        "failed_exhausted": failed_exhausted,
        "attempts_per_phase": dict(sorted(phase_attempts.items())),
        "phase_summaries": len(session.get("phase_summaries", [])),
        "challenge_verdicts": len(challenge_verdicts),
        "challenge_blocked": challenge_blocked,
        "challenge_overrides": challenge_overrides,
        "challenge_override_rate": (
            challenge_overrides / challenge_blocked
            if challenge_blocked
            else None
        ),
        "provider_invocations": len(provider_invocations),
        "running_provider_invocations": len(running_provider_invocations),
        "provider_invocations_by_role": dict(sorted(provider_invocations_by_role.items())),
        "provider_confidence": dict(sorted(provider_confidence.items())),
        "provider_statuses": dict(sorted(provider_statuses.items())),
        "latest_provider_invocation": provider_invocation_status_payload(latest_provider_invocation),
        "active_provider_invocation": provider_invocation_status_payload(active_provider_invocation),
        "provider_models_observed": provider_models_observed,
        "provider_models_inferred": provider_models_inferred,
        "provider_models_unknown": provider_models_unknown,
        "provider_isolation_downgrades": sum(1 for entry in provider_invocations if entry.get("isolation_downgraded")),
        "provider_weaker_review_isolation": provider_weaker_review_isolation,
        "provider_model_separation": provider_model_separation,
        "usage": usage,
    }
