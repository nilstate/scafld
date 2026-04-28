from scafld.handoff_renderer import criteria_for_phase, phase_definitions
from scafld.review_workflow import evaluate_review_gate, load_review_topology
from scafld.reviewing import load_review_state, parse_review_file
from scafld.runtime_bundle import REVIEWS_DIR
from scafld.runtime_contracts import handoff_json_path, handoff_path, relative_path
from scafld.session_store import provider_invocation_process_alive


def criterion_result_value(criterion):
    result = criterion.get("result")
    if isinstance(result, dict):
        return result.get("status")
    return result


def phase_ids(spec_data):
    return [phase.get("id") for phase in phase_definitions(spec_data) if phase.get("id")]


def next_open_phase_id(spec_data):
    for phase in phase_definitions(spec_data):
        phase_id = phase.get("id")
        if not phase_id:
            continue
        criteria = criteria_for_phase(phase)
        if not criteria:
            return phase_id
        if any(criterion_result_value(criterion) != "pass" for criterion in criteria):
            return phase_id
    return None


def ordered_criterion_ids(spec_data):
    ordered = []
    for phase in phase_definitions(spec_data):
        for criterion in criteria_for_phase(phase):
            criterion_id = criterion.get("id")
            if criterion_id:
                ordered.append(criterion_id)
    return ordered


def _criterion_states(session):
    if not isinstance(session, dict):
        return {}
    states = session.get("criterion_states")
    return states if isinstance(states, dict) else {}


def active_review_provider_invocation(session):
    if not isinstance(session, dict):
        return None
    entries = session.get("entries")
    if not isinstance(entries, list):
        return None
    for entry in reversed(entries):
        if not isinstance(entry, dict):
            continue
        if entry.get("type") != "provider_invocation":
            continue
        if entry.get("role") != "challenger" or entry.get("gate") != "review":
            continue
        return entry if entry.get("status") == "running" else None
    return None


def first_recovery_selector(spec_data, session):
    states = _criterion_states(session)
    for criterion_id in ordered_criterion_ids(spec_data):
        entry = states.get(criterion_id)
        if isinstance(entry, dict) and entry.get("status") == "recovery_pending":
            return criterion_id
    for criterion_id, entry in states.items():
        if isinstance(entry, dict) and entry.get("status") == "recovery_pending":
            return criterion_id
    return None


def exhausted_criteria(spec_data, session):
    states = _criterion_states(session)
    ordered = []
    for criterion_id in ordered_criterion_ids(spec_data):
        entry = states.get(criterion_id)
        if isinstance(entry, dict) and entry.get("status") == "failed_exhausted":
            ordered.append(criterion_id)
    for criterion_id, entry in states.items():
        if criterion_id in ordered:
            continue
        if isinstance(entry, dict) and entry.get("status") == "failed_exhausted":
            ordered.append(criterion_id)
    return ordered


def handoff_command(task_id, gate, selector=None):
    command = f"scafld handoff {task_id}"
    if gate == "phase" and selector:
        return f"{command} --phase {selector}"
    if gate == "recovery" and selector:
        return f"{command} --recovery {selector}"
    if gate == "review":
        return f"{command} --review"
    return command


def predicted_handoff(root, task_id, spec_path, *, role, gate, selector=None):
    handoff_file = handoff_path(
        root,
        task_id,
        role=role,
        gate=gate,
        selector=selector,
        spec_path=spec_path,
    )
    handoff_json_file = handoff_json_path(
        root,
        task_id,
        role=role,
        gate=gate,
        selector=selector,
        spec_path=spec_path,
    )
    return {
        "role": role,
        "gate": gate,
        "selector": selector,
        "command": handoff_command(task_id, gate, selector),
        "handoff_file": relative_path(root, handoff_file),
        "handoff_json_file": relative_path(root, handoff_json_file),
    }


def existing_review_handoff(root, task_id, review_state):
    if not isinstance(review_state, dict):
        return None
    handoff_file = review_state.get("review_handoff")
    if not isinstance(handoff_file, str) or not handoff_file:
        return None
    json_file = handoff_file[:-3] + ".json" if handoff_file.endswith(".md") else None
    payload = {
        "role": "challenger",
        "gate": "review",
        "selector": "review",
        "command": None,
        "handoff_file": handoff_file,
        "handoff_json_file": json_file,
    }
    return payload


def existing_review_repair_handoff(root, task_id, review_state):
    if not isinstance(review_state, dict):
        return None
    provenance = review_state.get("review_provenance")
    if not isinstance(provenance, dict):
        return None
    handoff_file = provenance.get("repair_handoff")
    if not isinstance(handoff_file, str) or not handoff_file:
        return None
    json_file = provenance.get("repair_handoff_json")
    if not isinstance(json_file, str) or not json_file:
        json_file = handoff_file[:-3] + ".json" if handoff_file.endswith(".md") else None
    return {
        "role": "executor",
        "gate": "review_repair",
        "selector": "review_repair",
        "command": None,
        "handoff_file": handoff_file,
        "handoff_json_file": json_file,
    }


def review_gate_snapshot(root, task_id):
    review_file = root / REVIEWS_DIR / f"{task_id}.md"
    try:
        topology = load_review_topology(root)
    except Exception as exc:
        return {
            "review_state": {"exists": False, "errors": [str(exc)]},
            "review_gate": {"exists": False, "gate_reason": None, "gate_errors": [str(exc)]},
        }

    review_data = parse_review_file(review_file, topology)
    review_state = load_review_state(review_file, topology)
    gate = evaluate_review_gate(root, review_file, review_data)
    return {
        "review_state": review_state,
        "review_gate": {
            "exists": bool(review_state.get("exists")),
            "gate_reason": gate.get("gate_reason"),
            "gate_errors": gate.get("gate_errors") or [],
            "gate_threshold": gate.get("gate_threshold"),
            "gate_blocking_count": gate.get("gate_blocking_count"),
            "gate_advisory_count": gate.get("gate_advisory_count"),
        },
    }


def _guidance(action_type, *, command=None, message=None, followup_command=None, blocked=False, **extra):
    payload = {
        "type": action_type,
        "command": command,
        "message": message,
        "followup_command": followup_command,
        "blocked": bool(blocked),
    }
    payload.update(extra)
    return payload


def derive_task_guidance(root, task_id, spec_path, spec_data, status, session, review_state=None, review_gate=None):
    review_state = review_state or {}
    review_gate = review_gate or {}
    current_handoff = None
    block_reason = review_gate.get("gate_reason")
    open_phase_id = next_open_phase_id(spec_data)
    exhausted = exhausted_criteria(spec_data, session)
    recovery_selector = first_recovery_selector(spec_data, session)

    if status == "draft":
        harden_status = spec_data.get("harden_status")
        if harden_status == "in_progress":
            next_action = _guidance(
                "harden",
                command=f"scafld harden {task_id} --mark-passed",
                message="Finish the open harden round, then approve the draft.",
            )
        else:
            next_action = _guidance(
                "approve",
                command=f"scafld approve {task_id}",
                message="Approve the reviewed draft to start governed execution.",
            )
        return {
            "next_action": next_action,
            "current_handoff": None,
            "block_reason": None,
        }

    if status == "approved":
        selector = open_phase_id or "phase1"
        current_handoff = predicted_handoff(
            root,
            task_id,
            spec_path,
            role="executor",
            gate="phase",
            selector=selector,
        )
        next_action = _guidance(
            "build",
            command=f"scafld build {task_id}",
            message="Start approved work and validate to the next handoff or block.",
            handoff_command=current_handoff["command"],
        )
        return {
            "next_action": next_action,
            "current_handoff": current_handoff,
            "block_reason": None,
        }

    if status != "in_progress":
        return {
            "next_action": None,
            "current_handoff": None,
            "block_reason": None,
        }

    if exhausted:
        next_action = _guidance(
            "human_required",
            message="Recovery cap reached; a human has to intervene before the task can continue.",
            blocked=True,
            reason="recovery_exhausted",
            criterion_ids=exhausted,
        )
        return {
            "next_action": next_action,
            "current_handoff": None,
            "block_reason": "recovery exhausted",
        }

    if recovery_selector:
        current_handoff = predicted_handoff(
            root,
            task_id,
            spec_path,
            role="executor",
            gate="recovery",
            selector=recovery_selector,
        )
        next_action = _guidance(
            "recovery_handoff",
            command=current_handoff["command"],
            message=f"Read the recovery handoff for {recovery_selector}, fix it, then rerun build.",
            followup_command=f"scafld build {task_id}",
            criterion_id=recovery_selector,
            handoff_command=current_handoff["command"],
        )
        return {
            "next_action": next_action,
            "current_handoff": current_handoff,
            "block_reason": None,
        }

    if open_phase_id:
        current_handoff = predicted_handoff(
            root,
            task_id,
            spec_path,
            role="executor",
            gate="phase",
            selector=open_phase_id,
        )
        next_action = _guidance(
            "phase_handoff",
            command=current_handoff["command"],
            message=f"Read the executor handoff for {open_phase_id}, make the phase change, then rerun build.",
            followup_command=f"scafld build {task_id}",
            phase_id=open_phase_id,
            handoff_command=current_handoff["command"],
        )
        return {
            "next_action": next_action,
            "current_handoff": current_handoff,
            "block_reason": None,
        }

    active_review = active_review_provider_invocation(session)
    if active_review:
        process_alive = provider_invocation_process_alive(active_review)
        current_handoff = existing_review_handoff(root, task_id, review_state) or predicted_handoff(
            root,
            task_id,
            spec_path,
            role="challenger",
            gate="review",
            selector="review",
        )
        common = {
            "provider": active_review.get("provider"),
            "pid": active_review.get("pid"),
            "process_alive": process_alive,
            "invocation_id": active_review.get("invocation_id"),
            "started_at": active_review.get("started_at"),
            "provider_session_requested": active_review.get("provider_session_requested"),
        }
        if process_alive is False:
            next_action = _guidance(
                "review_stale",
                command=f"scafld review {task_id}",
                message="An external review was recorded as running, but its subprocess is no longer alive; rerun review.",
                blocked=True,
                reason="external_review_process_not_alive",
                **common,
            )
            return {
                "next_action": next_action,
                "current_handoff": current_handoff,
                "block_reason": "external review process not alive",
            }

        next_action = _guidance(
            "review_running",
            command=f"scafld status {task_id} --json",
            message="External review is running; wait for the original review command to finish.",
            followup_command=f"scafld complete {task_id}",
            **common,
        )
        return {
            "next_action": next_action,
            "current_handoff": current_handoff,
            "block_reason": None,
        }

    if review_state.get("exists"):
        if not review_gate.get("gate_reason"):
            next_action = _guidance(
                "complete",
                command=f"scafld complete {task_id}",
                message="Review passed; archive the task.",
            )
            return {
                "next_action": next_action,
                "current_handoff": existing_review_handoff(root, task_id, review_state),
                "block_reason": None,
            }

        if review_state.get("verdict") == "fail":
            current_handoff = (
                existing_review_repair_handoff(root, task_id, review_state)
                or existing_review_handoff(root, task_id, review_state)
            )
            next_action = _guidance(
                "address_review_findings",
                command=current_handoff["command"] if current_handoff else None,
                message="Read the review repair handoff, fix the blocking findings, rerun build if needed, then rerun review.",
                followup_command=f"scafld review {task_id}",
                blocked=True,
                reason=review_gate.get("gate_reason"),
            )
            return {
                "next_action": next_action,
                "current_handoff": current_handoff,
                "block_reason": review_gate.get("gate_reason"),
            }

        next_action = _guidance(
            "review",
            command=f"scafld review {task_id}",
            message="Rerun review to regenerate a valid challenger round.",
            blocked=True,
            reason=review_gate.get("gate_reason"),
        )
        return {
            "next_action": next_action,
            "current_handoff": existing_review_handoff(root, task_id, review_state),
            "block_reason": review_gate.get("gate_reason"),
        }

    next_action = _guidance(
        "review",
        command=f"scafld review {task_id}",
        message="Execution is complete; run the challenger review gate.",
    )
    return {
        "next_action": next_action,
        "current_handoff": None,
        "block_reason": None,
    }
