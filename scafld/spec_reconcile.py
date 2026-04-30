from scafld.spec_markdown import parse_spec_markdown, update_spec_markdown

PHASE_BLOCK_FIELDS = {
    "status": "projected phase status",
    "reason": "optional blocked/completion reason",
    "updated_at": "ISO timestamp for the latest phase state event",
}


def project_session_state(spec_model, session):
    """Return a spec model with session-derived runner state applied.

    The session ledger is authoritative for execution facts. The spec is the
    human-readable projection, so reconciliation only writes facts that can be
    derived from durable session entries. Session phase state lives in
    ``phase_blocks[phase_id]`` using ``PHASE_BLOCK_FIELDS`` and is the only
    source used to project phase status into the spec.
    """
    data = dict(spec_model or {})
    session = session if isinstance(session, dict) else {}
    entries = session.get("entries") if isinstance(session.get("entries"), list) else []
    criterion_states = session.get("criterion_states") if isinstance(session.get("criterion_states"), dict) else {}
    phase_blocks = session.get("phase_blocks") if isinstance(session.get("phase_blocks"), dict) else {}

    phases = []
    for phase in data.get("phases") or []:
        if not isinstance(phase, dict):
            continue
        next_phase = dict(phase)
        phase_id = next_phase.get("id")
        if phase_id and phase_id in phase_blocks:
            block = phase_blocks.get(phase_id) or {}
            if block.get("status"):
                next_phase["status"] = block.get("status")
            if block.get("reason"):
                next_phase["block_reason"] = block.get("reason")

        criteria = []
        for criterion in next_phase.get("acceptance_criteria") or []:
            if not isinstance(criterion, dict):
                continue
            next_criterion = dict(criterion)
            criterion_id = next_criterion.get("id")
            state = criterion_states.get(criterion_id) if criterion_id else None
            if isinstance(state, dict):
                if state.get("status"):
                    next_criterion["result"] = state.get("status")
                    next_criterion["status"] = state.get("status")
                if state.get("reason"):
                    next_criterion["reason"] = state.get("reason")
                if state.get("phase_id"):
                    next_criterion["phase"] = state.get("phase_id")
            criteria.append(next_criterion)
        next_phase["acceptance_criteria"] = criteria
        phases.append(next_phase)

    data["phases"] = phases
    latest_attempt = next((entry for entry in reversed(entries) if entry.get("type") == "attempt"), None)
    data["current_state"] = {
        "status": data.get("status") or "draft",
        "current_phase": next((phase.get("id") for phase in phases if phase.get("status") == "in_progress"), None),
        "latest_runner_update": latest_attempt.get("recorded_at") if isinstance(latest_attempt, dict) else None,
        "review_gate": "not_started",
    }
    return data


def rebuild_spec_from_session(spec_text, session):
    """Rebuild runner-derived spec sections from session truth."""
    return update_spec_markdown(spec_text, project_session_state(parse_spec_markdown(spec_text), session))


def projection_matches(spec_text, session):
    """Return True when rebuilding from the session makes no byte changes."""
    return rebuild_spec_from_session(spec_text, session) == spec_text
