import json
import re


REVIEW_SCHEMA_VERSION = 3
REVIEW_PASS_VALUES = {"pass", "fail", "pass_with_issues", "not_run"}
REVIEW_VERDICTS = {"pass", "fail", "pass_with_issues", "incomplete"}
REVIEW_ROUND_STATUSES = {"in_progress", "completed", "override"}
REVIEWER_MODES = {"fresh_agent", "auto", "executor", "human_override"}
REVIEW_PASS_REGISTRY = {
    "spec_compliance": {
        "kind": "automated",
        "title": "Spec Compliance",
        "description": "Re-run acceptance criteria to verify code satisfies the spec.",
        "prompt": "Re-run acceptance criteria to confirm the implementation still satisfies the spec.",
    },
    "scope_drift": {
        "kind": "automated",
        "title": "Scope Drift",
        "description": "Compare the declared spec scope with the current git diff.",
        "prompt": "Compare the declared spec scope with the current git diff and flag undeclared work.",
    },
    "regression_hunt": {
        "kind": "adversarial",
        "title": "Regression Hunt",
        "description": "Trace callers/importers and downstream consumers for behavior regressions.",
        "prompt": "For each modified file, find every caller, importer, and downstream consumer. What assumptions do they make that this change violates? Check function signatures, return shapes, event schemas, and exports.",
    },
    "convention_check": {
        "kind": "adversarial",
        "title": "Convention Check",
        "description": "Check the diff against documented conventions and agent rules.",
        "prompt": "Read CONVENTIONS.md and AGENTS.md. Does the new code violate any documented rule? Cite the rule and the specific line.",
    },
    "dark_patterns": {
        "kind": "adversarial",
        "title": "Dark Patterns",
        "description": "Hunt for subtle bugs, hardcodes, races, copy-paste errors, and safety gaps.",
        "prompt": "Actively hunt for hardcoded values, off-by-one errors, missing null checks, race conditions, copy-paste errors, unhandled error paths, and security issues.",
    },
}


def build_review_topology(review_config, review_pass_registry=REVIEW_PASS_REGISTRY):
    """Build the configured review topology for built-in review passes."""
    topology = []
    seen_ids = set()
    seen_orders = {}

    for section_key, expected_kind in (("automated_passes", "automated"), ("adversarial_passes", "adversarial")):
        section = review_config.get(section_key)
        if not isinstance(section, dict):
            raise ValueError(f"config.review.{section_key} must be a mapping")

        for pass_id, configured in section.items():
            builtin = review_pass_registry.get(pass_id)
            if builtin is None:
                raise ValueError(f"unknown built-in review pass: {pass_id}")
            if builtin["kind"] != expected_kind:
                raise ValueError(f"review pass {pass_id} must be declared under review.{section_key}")
            if not isinstance(configured, dict):
                raise ValueError(f"config.review.{section_key}.{pass_id} must be a mapping")

            raw_order = configured.get("order")
            if raw_order in (None, ""):
                raise ValueError(f"config.review.{section_key}.{pass_id}.order is required")
            try:
                order = int(str(raw_order).strip())
            except (TypeError, ValueError) as exc:
                raise ValueError(f"config.review.{section_key}.{pass_id}.order must be an integer") from exc
            if order <= 0:
                raise ValueError(f"config.review.{section_key}.{pass_id}.order must be > 0")
            if order in seen_orders:
                raise ValueError(
                    f"review pass order {order} is duplicated by {seen_orders[order]} and {pass_id}"
                )

            title = str(configured.get("title") or builtin["title"]).strip()
            description = str(configured.get("description") or builtin["description"]).strip()
            if not title:
                raise ValueError(f"config.review.{section_key}.{pass_id}.title must be non-empty")
            if not description:
                raise ValueError(f"config.review.{section_key}.{pass_id}.description must be non-empty")

            topology.append({
                "id": pass_id,
                "kind": expected_kind,
                "order": order,
                "title": title,
                "description": description,
                "prompt": builtin["prompt"],
            })
            seen_ids.add(pass_id)
            seen_orders[order] = pass_id

    missing = sorted(set(review_pass_registry) - seen_ids)
    if missing:
        raise ValueError(f"review topology is missing built-in pass(es): {', '.join(missing)}")

    topology.sort(key=lambda item: (item["order"], item["id"]))
    return topology


def review_passes_by_kind(topology, kind):
    return [definition for definition in topology if definition["kind"] == kind]


def review_pass_ids(topology, kind=None):
    return [definition["id"] for definition in topology if kind is None or definition["kind"] == kind]


def normalize_review_pass_results(topology, raw_passes=None):
    """Normalize configured review pass results to pass/fail/pass_with_issues/not_run."""
    normalized = {definition["id"]: "not_run" for definition in topology}
    for key, value in (raw_passes or {}).items():
        if key not in normalized:
            continue
        if isinstance(value, bool):
            normalized[key] = "pass" if value else "fail"
        elif isinstance(value, str) and value in REVIEW_PASS_VALUES:
            normalized[key] = value
    return normalized


def review_pass_label(value):
    """Human label for review pass values."""
    return {
        "pass": "PASS",
        "fail": "FAIL",
        "pass_with_issues": "PASS WITH ISSUES",
        "not_run": "NOT RUN",
    }.get(value, str(value).upper())


def build_review_metadata(
    topology,
    reviewer_mode,
    round_status,
    pass_results,
    reviewed_at,
    reviewer_session="",
    override_reason=None,
    review_git_state=None,
):
    """Build Review Artifact v3 metadata."""
    metadata = {
        "schema_version": REVIEW_SCHEMA_VERSION,
        "round_status": round_status,
        "reviewer_mode": reviewer_mode,
        "reviewer_session": reviewer_session,
        "reviewed_at": reviewed_at,
        "override_reason": override_reason,
        "pass_results": normalize_review_pass_results(topology, pass_results),
    }
    metadata.update(review_git_state or {
        "reviewed_head": None,
        "reviewed_dirty": None,
        "reviewed_diff": None,
    })
    return metadata


def validate_review_metadata(metadata, topology):
    """Validate Review Artifact v3 metadata. Returns list of errors."""
    if not isinstance(metadata, dict):
        return ["metadata is not an object"]

    errors = []
    if metadata.get("schema_version") != REVIEW_SCHEMA_VERSION:
        errors.append(f"schema_version must be {REVIEW_SCHEMA_VERSION}")
    if metadata.get("round_status") not in REVIEW_ROUND_STATUSES:
        errors.append("round_status must be in_progress, completed, or override")
    if metadata.get("reviewer_mode") not in REVIEWER_MODES:
        errors.append("reviewer_mode is invalid")
    if not isinstance(metadata.get("reviewer_session"), str):
        errors.append("reviewer_session must be a string")
    if not isinstance(metadata.get("reviewed_at"), str) or not metadata.get("reviewed_at"):
        errors.append("reviewed_at must be a non-empty string")

    git_fields = ("reviewed_head", "reviewed_dirty", "reviewed_diff")
    missing_git_fields = [field for field in git_fields if field not in metadata]
    if missing_git_fields:
        errors.append(f"metadata missing reviewed git field(s): {', '.join(missing_git_fields)}")
    else:
        reviewed_head = metadata.get("reviewed_head")
        reviewed_dirty = metadata.get("reviewed_dirty")
        reviewed_diff = metadata.get("reviewed_diff")
        trio = (reviewed_head, reviewed_dirty, reviewed_diff)
        if any(value is None for value in trio) and not all(value is None for value in trio):
            errors.append("reviewed_head, reviewed_dirty, and reviewed_diff must all be set or all be null")
        elif reviewed_head is not None:
            if not isinstance(reviewed_head, str) or not reviewed_head:
                errors.append("reviewed_head must be a non-empty string or null")
            if not isinstance(reviewed_dirty, bool):
                errors.append("reviewed_dirty must be a boolean or null")
            if not isinstance(reviewed_diff, str) or not re.fullmatch(r"[0-9a-f]{64}", reviewed_diff):
                errors.append("reviewed_diff must be a 64-character lowercase hex string or null")

    override_reason = metadata.get("override_reason")
    if override_reason is not None and not isinstance(override_reason, str):
        errors.append("override_reason must be a string or null")

    raw_pass_results = metadata.get("pass_results")
    if not isinstance(raw_pass_results, dict):
        errors.append("pass_results must be an object")
    else:
        expected_ids = set(review_pass_ids(topology))
        provided_ids = set(raw_pass_results)
        missing = sorted(expected_ids - provided_ids)
        unexpected = sorted(provided_ids - expected_ids)
        if missing:
            errors.append(f"pass_results missing configured pass(es): {', '.join(missing)}")
        if unexpected:
            errors.append(f"pass_results includes unknown pass(es): {', '.join(unexpected)}")

        for definition in topology:
            pass_id = definition["id"]
            value = raw_pass_results.get(pass_id)
            if value not in REVIEW_PASS_VALUES:
                errors.append(f"pass_results.{pass_id} must be one of: {', '.join(sorted(REVIEW_PASS_VALUES))}")
            elif definition["kind"] == "automated" and value == "pass_with_issues":
                errors.append(f"pass_results.{pass_id} cannot be pass_with_issues for an automated pass")

        if metadata.get("round_status") == "completed":
            incomplete = [pass_id for pass_id, value in raw_pass_results.items() if value == "not_run"]
            if incomplete:
                errors.append(f"completed review round cannot leave pass_results as not_run: {', '.join(sorted(incomplete))}")

    if metadata.get("round_status") == "override":
        if metadata.get("reviewer_mode") != "human_override":
            errors.append("override round must use reviewer_mode=human_override")
        if not isinstance(override_reason, str) or not override_reason.strip():
            errors.append("override round must include a non-empty override_reason")
    elif override_reason is not None and override_reason != "":
        errors.append("override_reason must be null unless round_status=override")

    return errors


def render_review_pass_results(topology, pass_states):
    """Render the human-readable pass results section."""
    normalized = normalize_review_pass_results(topology, pass_states)
    return "\n".join(
        f"- {definition['id']}: {review_pass_label(normalized[definition['id']])}"
        for definition in topology
    )


def review_git_gate_reason(current_git_state, metadata):
    """Return a blocking reason when the current workspace diverges from reviewed git state."""
    if current_git_state.get("reviewed_head") is None:
        return None

    reviewed_head = metadata.get("reviewed_head")
    reviewed_dirty = metadata.get("reviewed_dirty")
    reviewed_diff = metadata.get("reviewed_diff")
    if reviewed_head is None and reviewed_dirty is None and reviewed_diff is None:
        return "latest review is not bound to git state"

    if current_git_state["reviewed_head"] != reviewed_head:
        return (
            f"current HEAD {current_git_state['reviewed_head'][:12]} does not match "
            f"reviewed commit {reviewed_head[:12]}"
        )

    if (
        current_git_state["reviewed_dirty"] != reviewed_dirty
        or current_git_state["reviewed_diff"] != reviewed_diff
    ):
        return "current workspace no longer matches the reviewed git state"

    return None


def build_spec_review_block(review_data, topology, override_applied=False, override_reason=None, override_confirmed_at=None):
    """Render the top-level spec review block from parsed review data."""
    metadata = review_data.get("metadata") or {}
    verdict = review_data.get("verdict") or "incomplete"
    pass_results = normalize_review_pass_results(topology, review_data.get("pass_results"))
    timestamp = override_confirmed_at or metadata.get("reviewed_at")
    reviewer_mode = "human_override" if override_applied else (metadata.get("reviewer_mode") or "")
    reviewer_session = "" if override_applied else (metadata.get("reviewer_session") or "")
    round_status = "override" if override_applied else (metadata.get("round_status") or "")
    applied_override_reason = override_reason if override_reason is not None else metadata.get("override_reason")
    reviewed_head = metadata.get("reviewed_head")
    reviewed_dirty = metadata.get("reviewed_dirty")
    reviewed_diff = metadata.get("reviewed_diff")

    lines = [
        "review:",
        f"  timestamp: {json.dumps(timestamp)}",
        f"  verdict: {json.dumps(verdict)}",
        f"  review_rounds: {review_data.get('review_count', 0)}",
        f"  reviewer_mode: {json.dumps(reviewer_mode)}",
        f"  reviewer_session: {json.dumps(reviewer_session)}",
        f"  round_status: {json.dumps(round_status)}",
        f"  override_applied: {'true' if override_applied else 'false'}",
        f"  override_reason: {json.dumps(applied_override_reason) if applied_override_reason is not None else 'null'}",
        f"  override_confirmed_at: {json.dumps(override_confirmed_at) if override_confirmed_at is not None else 'null'}",
        f"  reviewed_head: {json.dumps(reviewed_head) if reviewed_head is not None else 'null'}",
        f"  reviewed_dirty: {json.dumps(reviewed_dirty) if reviewed_dirty is not None else 'null'}",
        f"  reviewed_diff: {json.dumps(reviewed_diff) if reviewed_diff is not None else 'null'}",
        "  passes:",
    ]
    for definition in topology:
        lines.append(
            f"    - id: {definition['id']}\n"
            f"      result: {json.dumps(pass_results[definition['id']])}"
        )
    lines.extend([
        f"  blocking_count: {len(review_data.get('blocking', []))}",
        f"  non_blocking_count: {len(review_data.get('non_blocking', []))}",
    ])
    return "\n".join(lines)


def review_data_payload(review_data):
    """Return a compact JSON-safe review state for status/automation callers."""
    metadata = review_data.get("metadata") or {}
    return {
        "exists": review_data.get("exists", False),
        "review_rounds": review_data.get("review_count", 0),
        "verdict": review_data.get("verdict"),
        "round_status": review_data.get("round_status"),
        "reviewer_mode": review_data.get("reviewer_mode"),
        "reviewed_head": metadata.get("reviewed_head"),
        "reviewed_dirty": metadata.get("reviewed_dirty"),
        "reviewed_diff": metadata.get("reviewed_diff"),
        "blocking_count": len(review_data.get("blocking", [])),
        "non_blocking_count": len(review_data.get("non_blocking", [])),
        "pass_results": review_data.get("pass_results", {}),
        "empty_adversarial": review_data.get("empty_adversarial", []),
        "errors": review_data.get("errors", []),
    }
