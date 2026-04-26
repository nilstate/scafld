import json
import re


REVIEW_SCHEMA_VERSION = 3
REVIEW_PASS_VALUES = {"pass", "fail", "pass_with_issues", "not_run"}
REVIEW_VERDICTS = {"pass", "fail", "pass_with_issues", "incomplete"}
REVIEW_ROUND_STATUSES = {"in_progress", "completed", "override"}
REVIEWER_MODES = {"challenger", "fresh_agent", "auto", "executor", "human_override"}
FINDING_SEVERITIES = ("critical", "high", "medium", "low")
FINDING_TARGET = r"(?:[^`\n]+:\d+|[^`\n]+\.(?:ya?ml|md|markdown)#[A-Za-z0-9_.:/-]+)"
FINDING_FORMAT_DESCRIPTION = "`file:line` or `doc.md#anchor`"
FINDING_LINE_RE = re.compile(
    rf"^- \*\*(critical|high|medium|low)\*\* `{FINDING_TARGET}` (?:—|--) .+$",
    re.IGNORECASE,
)
NO_ISSUES_RE = re.compile(
    r"^No(?: additional)? issues found(?: (?:—|--) checked (?P<checked>.+))?$",
    re.IGNORECASE,
)
GENERIC_CLEAN_TARGETS = {
    "all",
    "all changes",
    "all files",
    "changes",
    "everything",
    "implementation",
    "it",
    "nothing in particular",
    "the change",
    "the changes",
    "the code",
    "the codebase",
    "the diff",
    "the implementation",
    "the relevant code",
    "many things",
    "many files",
    "several things",
    "various things",
}
CONCRETE_CLEAN_RE = re.compile(r"(?:^|[\s`])[\w./-]+\.[A-Za-z0-9]{1,8}(?:\b|`)")
CONCRETE_CLEAN_TERMS = {
    "caller",
    "callers",
    "callee",
    "callees",
    "consumer",
    "consumers",
    "importer",
    "importers",
    "rule",
    "rules",
    "path",
    "paths",
    "schema",
    "schemas",
    "contract",
    "contracts",
    "endpoint",
    "endpoints",
    "null",
    "retry",
    "hardcode",
    "hardcodes",
    "race",
    "races",
}
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
        "description": "Compare the declared spec scope with the current workspace change set.",
        "prompt": "Compare the declared spec scope with the current workspace change set and flag undeclared work.",
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


def clean_no_issues_target(line):
    match = NO_ISSUES_RE.fullmatch(line.strip())
    if not match:
        return None
    checked = (match.group("checked") or "").strip()
    return checked or ""


def clean_no_issues_has_evidence(line):
    checked = clean_no_issues_target(line)
    if checked is None:
        return False
    normalized = checked.lower().strip(" .")
    if not normalized or normalized in GENERIC_CLEAN_TARGETS:
        return False
    if re.search(r"\b(many|various|several|relevant)\s+(things|code|files|areas|parts)\b", normalized):
        return False
    if CONCRETE_CLEAN_RE.search(checked):
        return True
    words = set(re.findall(r"[a-z0-9_]+", normalized))
    if len(words) <= 1:
        return False
    return bool(words & CONCRETE_CLEAN_TERMS)


def build_review_metadata(
    topology,
    reviewer_mode,
    round_status,
    pass_results,
    reviewed_at,
    reviewer_session="",
    override_reason=None,
    review_git_state=None,
    review_handoff=None,
    reviewer_isolation=None,
    review_provenance=None,
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
    if review_handoff is not None:
        metadata["review_handoff"] = review_handoff
    if reviewer_isolation is not None:
        metadata["reviewer_isolation"] = reviewer_isolation
    if review_provenance is not None:
        metadata["review_provenance"] = review_provenance
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

    review_provenance = metadata.get("review_provenance")
    if review_provenance is not None and not isinstance(review_provenance, dict):
        errors.append("review_provenance must be an object when present")

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


def parse_review_file(review_path, topology):
    """Parse the latest review round and validate the Review Artifact v3 contract."""
    adversarial_passes = review_passes_by_kind(topology, "adversarial")
    adversarial_titles = [definition["title"] for definition in adversarial_passes]
    parsed = {
        "exists": review_path.exists(),
        "review_count": 0,
        "verdict": None,
        "blocking": [],
        "non_blocking": [],
        "metadata": None,
        "pass_results": normalize_review_pass_results(topology),
        "round_status": None,
        "reviewer_mode": None,
        "empty_adversarial": list(adversarial_titles),
        "adversarial_sections": {},
        "adversarial_findings": [],
        "quality_errors": [],
        "errors": [],
    }
    if not review_path.exists():
        return parsed

    text = review_path.read_text()
    matches = list(re.finditer(r'^## Review \d+\s+—.*$', text, re.MULTILINE))
    parsed["review_count"] = len(matches)
    if not matches:
        parsed["errors"].append("review file has no review rounds")
        return parsed

    last_section = text[matches[-1].end():]

    def section_body(heading):
        match = re.search(
            rf'^### {re.escape(heading)}\s*\n(.*?)(?=^### |\Z)',
            last_section,
            re.MULTILINE | re.DOTALL,
        )
        return match.group(1) if match else None

    required_sections = ("Metadata", "Pass Results", "Blocking", "Non-blocking", "Verdict")

    sections = {}
    for heading in (*required_sections, *adversarial_titles):
        body = section_body(heading)
        sections[heading] = body
        if body is None and heading in required_sections:
            parsed["errors"].append(f"latest review round is missing ### {heading}")

    empty_adversarial = []
    for heading in adversarial_titles:
        body = sections.get(heading)
        if body is None or not body.strip():
            empty_adversarial.append(heading)
    parsed["empty_adversarial"] = empty_adversarial

    blocking = []
    non_blocking = []
    for heading, bucket in (("Blocking", blocking), ("Non-blocking", non_blocking)):
        body = sections.get(heading)
        if body is None:
            continue
        for line in body.strip().splitlines():
            stripped = line.strip()
            if not stripped:
                continue
            normalized = stripped.lower().strip(".")
            if normalized in {"none", "- none", "n/a", "(none)"}:
                continue
            if stripped.startswith("* "):
                stripped = "- " + stripped[2:].strip()
            if stripped.startswith("- "):
                bucket.append(stripped)
                continue
            parsed["quality_errors"].append(
                f"{heading} contains malformed content; use finding bullets or None."
            )
            break
    parsed["blocking"] = blocking
    parsed["non_blocking"] = non_blocking

    adversarial_findings = []
    for heading in adversarial_titles:
        body = (sections.get(heading) or "").strip()
        section_state = {
            "state": "empty",
            "checked": False,
            "evidence": False,
            "finding_count": 0,
        }
        if not body:
            parsed["adversarial_sections"][heading] = section_state
            continue
        meaningful_lines = [line.strip() for line in body.splitlines() if line.strip()]
        if not meaningful_lines:
            parsed["adversarial_sections"][heading] = section_state
            continue
        if len(meaningful_lines) == 1 and NO_ISSUES_RE.fullmatch(meaningful_lines[0]):
            section_state["state"] = "no_issues"
            checked_target = clean_no_issues_target(meaningful_lines[0])
            section_state["checked"] = bool(checked_target)
            section_state["evidence"] = clean_no_issues_has_evidence(meaningful_lines[0])
            parsed["adversarial_sections"][heading] = section_state
            if not checked_target:
                parsed["quality_errors"].append(
                    f"{heading} must say what was checked when reporting no issues found"
                )
            elif not section_state["evidence"]:
                parsed["quality_errors"].append(
                    f"{heading} clean no-issues note must name concrete files, callers, rules, or paths checked"
                )
            continue
        section_state["state"] = "findings"
        for line in meaningful_lines:
            normalized = line.lower().strip(".")
            if normalized in {"none", "- none", "n/a", "(none)"}:
                parsed["quality_errors"].append(
                    f"{heading} cannot be empty; use a grounded finding or an explicit no-issues note"
                )
                continue
            if line.startswith("* "):
                line = "- " + line[2:].strip()
            if FINDING_LINE_RE.fullmatch(line):
                section_state["finding_count"] += 1
                adversarial_findings.append(line)
            if not FINDING_LINE_RE.fullmatch(line):
                parsed["quality_errors"].append(
                    f"{heading} findings must use '- **severity** {FINDING_FORMAT_DESCRIPTION} — explanation'"
                )
                break
        parsed["adversarial_sections"][heading] = section_state
    parsed["adversarial_findings"] = adversarial_findings

    for heading, findings in (("Blocking", blocking), ("Non-blocking", non_blocking)):
        for finding in findings:
            if not FINDING_LINE_RE.fullmatch(finding):
                parsed["quality_errors"].append(
                    f"{heading} findings must use '- **severity** {FINDING_FORMAT_DESCRIPTION} — explanation'"
                )
                break

    metadata_body = sections.get("Metadata")
    if metadata_body is not None:
        metadata_match = re.search(r'```json\s*\n(.*?)\n```', metadata_body, re.DOTALL)
        if not metadata_match:
            parsed["errors"].append("latest review metadata must contain a fenced json block")
        else:
            try:
                metadata = json.loads(metadata_match.group(1))
            except json.JSONDecodeError as e:
                parsed["errors"].append(f"latest review metadata is invalid JSON: {e}")
            else:
                parsed["metadata"] = metadata
                parsed["pass_results"] = normalize_review_pass_results(topology, metadata.get("pass_results"))
                parsed["round_status"] = metadata.get("round_status")
                parsed["reviewer_mode"] = metadata.get("reviewer_mode")
                for issue in validate_review_metadata(metadata, topology):
                    parsed["errors"].append(f"metadata: {issue}")

    verdict_body = sections.get("Verdict")
    verdict = None
    if verdict_body is None:
        parsed["errors"].append("latest review round is missing a verdict body")
    else:
        normalized_verdict = re.sub(r'[`*_]+', ' ', verdict_body.lower())
        normalized_verdict = re.sub(r'\s+', ' ', normalized_verdict).strip()
        if normalized_verdict in {"pass with issues", "pass_with_issues"}:
            verdict = "pass_with_issues"
        elif normalized_verdict == "fail":
            verdict = "fail"
        elif normalized_verdict == "pass":
            verdict = "pass"
        elif normalized_verdict == "incomplete":
            verdict = "incomplete"
        elif normalized_verdict:
            parsed["errors"].append("latest review verdict is invalid")
        else:
            parsed["errors"].append("latest review verdict is empty")

    if verdict is None:
        if blocking:
            verdict = "fail"
        elif non_blocking:
            verdict = "pass_with_issues"

    if verdict == "fail" and not blocking:
        parsed["errors"].append("verdict fail requires at least one blocking finding")
    if verdict == "pass_with_issues":
        if blocking:
            parsed["errors"].append("pass_with_issues cannot include blocking findings")
        if not non_blocking:
            parsed["errors"].append("pass_with_issues requires at least one non-blocking finding")
    if verdict == "pass":
        if blocking:
            parsed["errors"].append("pass verdict cannot include blocking findings")
        if non_blocking:
            parsed["errors"].append("pass verdict cannot include non-blocking findings")
        if adversarial_findings:
            parsed["errors"].append("pass verdict cannot include adversarial findings")
    if adversarial_findings and not (blocking or non_blocking):
        parsed["errors"].append("adversarial findings must be collected into Blocking or Non-blocking")
    if verdict == "incomplete" and parsed["round_status"] == "completed":
        parsed["errors"].append("completed review round cannot use verdict=incomplete")

    adversarial_ids = review_pass_ids(topology, "adversarial")
    configured_results = parsed["pass_results"]
    if verdict == "pass":
        bad = [
            pass_id
            for pass_id in adversarial_ids
            if configured_results.get(pass_id) in {"fail", "pass_with_issues"}
        ]
        if bad:
            parsed["errors"].append(
                "pass verdict cannot mark adversarial passes as fail/pass_with_issues: "
                + ", ".join(sorted(bad))
            )
    if verdict == "pass_with_issues":
        bad = [pass_id for pass_id in adversarial_ids if configured_results.get(pass_id) == "fail"]
        if bad:
            parsed["errors"].append(
                f"pass_with_issues cannot mark adversarial passes as fail: {', '.join(sorted(bad))}"
            )
        if not any(configured_results.get(pass_id) == "pass_with_issues" for pass_id in adversarial_ids):
            parsed["errors"].append(
                "pass_with_issues requires at least one adversarial pass result of pass_with_issues"
            )
    if verdict == "fail" and parsed["round_status"] == "completed":
        failing = [pass_id for pass_id, result in configured_results.items() if result == "fail"]
        if not failing:
            parsed["errors"].append("fail verdict requires at least one review pass result of fail")

    parsed["errors"].extend(parsed["quality_errors"])
    parsed["verdict"] = verdict
    return parsed


def review_data_payload(review_data):
    """Return a compact JSON-safe review state for status/automation callers."""
    metadata = review_data.get("metadata") or {}
    return {
        "exists": review_data.get("exists", False),
        "review_rounds": review_data.get("review_count", 0),
        "verdict": review_data.get("verdict"),
        "round_status": review_data.get("round_status"),
        "reviewer_mode": review_data.get("reviewer_mode"),
        "review_handoff": metadata.get("review_handoff"),
        "reviewer_isolation": metadata.get("reviewer_isolation"),
        "review_provenance": metadata.get("review_provenance"),
        "reviewed_head": metadata.get("reviewed_head"),
        "reviewed_dirty": metadata.get("reviewed_dirty"),
        "reviewed_diff": metadata.get("reviewed_diff"),
        "blocking_count": len(review_data.get("blocking", [])),
        "non_blocking_count": len(review_data.get("non_blocking", [])),
        "pass_results": review_data.get("pass_results", {}),
        "empty_adversarial": review_data.get("empty_adversarial", []),
        "adversarial_sections": review_data.get("adversarial_sections", {}),
        "adversarial_findings": review_data.get("adversarial_findings", []),
        "quality_errors": review_data.get("quality_errors", []),
        "errors": review_data.get("errors", []),
    }


def load_review_state(review_path, topology):
    """Load one parsed review payload without raising review-parse exceptions."""
    try:
        return review_data_payload(parse_review_file(review_path, topology))
    except Exception as exc:
        return {"exists": False, "errors": [str(exc)]}
