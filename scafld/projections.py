from scafld.spec_store import prune_empty


def origin_block(data, key):
    """Return one origin sub-block as a mapping."""
    origin = data.get("origin") if isinstance(data.get("origin"), dict) else {}
    value = origin.get(key)
    return value if isinstance(value, dict) else {}


def origin_payload(data):
    """Return the structured origin metadata stored in the spec."""
    return prune_empty({
        "source": origin_block(data, "source"),
        "repo": origin_block(data, "repo"),
        "git": origin_block(data, "git"),
        "sync": origin_block(data, "sync"),
    })


def stored_sync_snapshot(sync_payload, *, checked_at):
    """Reduce a live sync payload to the portable facts worth persisting in the spec."""
    actual = sync_payload.get("actual") if isinstance(sync_payload.get("actual"), dict) else {}
    return prune_empty({
        "status": sync_payload.get("status"),
        "last_checked_at": checked_at,
        "reasons": list(sync_payload.get("reasons") or []),
        "actual": {
            "branch": actual.get("branch"),
            "head_sha": actual.get("head_sha"),
            "upstream": actual.get("upstream"),
            "remote": actual.get("remote"),
            "remote_url": actual.get("remote_url"),
            "default_base_ref": actual.get("default_base_ref"),
            "dirty": actual.get("dirty"),
            "detached": actual.get("detached"),
        },
    })


def build_origin_binding(existing_origin, live_state, branch_name, base_ref, mode, *, bound_at):
    """Merge live branch binding facts into the spec's provider-neutral origin block."""
    existing_origin = existing_origin if isinstance(existing_origin, dict) else {}
    return prune_empty({
        "source": existing_origin.get("source") if isinstance(existing_origin.get("source"), dict) else {},
        "repo": {
            "root": ".",
            "remote": live_state.get("remote"),
            "remote_url": live_state.get("remote_url"),
        },
        "git": {
            "branch": branch_name,
            "base_ref": base_ref,
            "upstream": live_state.get("upstream"),
            "mode": mode,
            "bound_at": bound_at,
        },
        "sync": existing_origin.get("sync") if isinstance(existing_origin.get("sync"), dict) else {},
    })


def phase_counts(total, completed, failed, in_progress):
    """Return one normalized phase-count payload."""
    return {
        "total": total,
        "completed": completed,
        "failed": failed,
        "in_progress": in_progress,
        "pending": max(total - completed - failed - in_progress, 0),
    }


def acceptance_summary(criteria):
    """Summarize acceptance results for projection surfaces."""
    passed = sum(1 for criterion in criteria if criterion.get("result") == "pass")
    failed = sum(1 for criterion in criteria if criterion.get("result") == "fail")
    total = len(criteria)
    pending = max(total - passed - failed, 0)
    manual = sum(1 for criterion in criteria if criterion.get("type") in ("documentation", "custom"))
    return {
        "total": total,
        "passed": passed,
        "failed": failed,
        "pending": pending,
        "manual": manual,
    }


def build_projection_model(root, spec_path, task_id, *, data, phase_entries, phase_counts_payload, criteria, review_state, sync, runtime=None):
    """Build one deterministic projection model from the current spec state."""
    rel = spec_path.relative_to(root)
    task_block = data.get("task") if isinstance(data.get("task"), dict) else {}
    risks = task_block.get("risks") or []
    risk_descriptions = [
        entry.get("description")
        for entry in risks
        if isinstance(entry, dict) and entry.get("description")
    ]

    return prune_empty({
        "task_id": task_id,
        "file": str(rel),
        "title": task_block.get("title") or task_id,
        "summary": task_block.get("summary") or "",
        "status": data.get("status") or "unknown",
        "size": task_block.get("size") or "",
        "risk": task_block.get("risk_level") or "",
        "updated_at": data.get("updated") or "",
        "objectives": list(task_block.get("objectives") or []),
        "risks": risk_descriptions,
        "phases": {
            "entries": list(phase_entries or []),
            **phase_counts_payload,
        },
        "acceptance": acceptance_summary(criteria),
        "review": review_state,
        "origin": origin_payload(data),
        "sync": sync,
        "runtime": runtime or {},
    })


def summarize_origin_source(origin):
    """Render one concise human-readable origin source summary."""
    source = origin.get("source") if isinstance(origin.get("source"), dict) else {}
    if not source:
        return None

    system = source.get("system")
    kind = source.get("kind")
    identifier = source.get("id")
    title = source.get("title")
    url = source.get("url")

    parts = []
    if isinstance(system, str) and system:
        parts.append(system)
    if isinstance(kind, str) and kind:
        parts.append(kind)
    if identifier not in (None, ""):
        ident_text = str(identifier).strip()
        if kind in ("issue", "pr") and ident_text.isdigit():
            ident_text = f"#{ident_text}"
        if ident_text:
            parts.append(ident_text)

    summary = " ".join(parts)
    if isinstance(title, str) and title:
        return f"{summary} - {title}" if summary else title
    if summary:
        return summary
    if isinstance(url, str) and url:
        return url
    return None


def humanize_binding_mode(mode):
    """Render one branch-binding mode for humans."""
    labels = {
        "created_branch": "created branch",
        "checked_out_existing": "checked out existing branch",
        "bound_current": "bound current branch",
    }
    return labels.get(mode, mode)
