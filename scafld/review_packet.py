import hashlib
import json
import re
from pathlib import Path

from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.reviewing import FINDING_SEVERITIES, FINDING_TARGET_RE, review_pass_ids, review_passes_by_kind
from scafld.runtime_contracts import handoff_path, relative_path, review_packets_dir
from scafld.spec_parsing import now_iso


REVIEW_PACKET_SCHEMA_VERSION = "review_packet.v1"
REVIEW_PACKET_ARTIFACT_SCHEMA_VERSION = "review_packet_artifact.v1"
REVIEW_REPAIR_HANDOFF_SCHEMA_VERSION = 1
REVIEW_PACKET_VERDICTS = {"pass", "fail", "pass_with_issues"}
REVIEW_PACKET_PASS_RESULTS = {"pass", "fail", "pass_with_issues"}
REVIEW_PACKET_ALLOWED_KEYS = {
    "schema_version",
    "review_summary",
    "verdict",
    "pass_results",
    "checked_surfaces",
    "findings",
}
MAX_REVIEW_PACKET_FINDINGS = 10
FORBIDDEN_PROVIDER_PACKET_KEYS = {
    "canonical_response_sha256",
    "completed_at",
    "exit_code",
    "fallback_policy",
    "isolation",
    "isolation_downgraded",
    "isolation_level",
    "metadata",
    "provenance",
    "model",
    "model_observed",
    "model_requested",
    "model_source",
    "prompt_sha256",
    "provider",
    "provider_bin",
    "provider_env",
    "provider_requested",
    "provider_session_observed",
    "raw_response_sha256",
    "repair_handoff",
    "repair_handoff_json",
    "review_packet",
    "review_provenance",
    "reviewer_isolation",
    "reviewer_mode",
    "reviewer_session",
    "runner",
    "started_at",
    "timed_out",
    "timing",
    "timeout_seconds",
    "warning",
    "warnings",
}
GENERIC_CHECKED_TARGETS = {
    "all",
    "all files",
    "changes",
    "everything",
    "implementation",
    "the change",
    "the changes",
    "the code",
    "the diff",
    "the implementation",
}


def canonical_review_packet_json(packet):
    return json.dumps(packet, sort_keys=True, separators=(",", ":"))


def _packet_error(message, details=None):
    raise ScafldError(
        "external reviewer returned invalid ReviewPacket",
        [message, *(details or [])],
        code=EC.COMMAND_FAILED,
    )


def _strip_json_fence(text):
    stripped = str(text or "").strip()
    match = re.fullmatch(r"```(?:json)?\s*(.*?)\s*```", stripped, re.DOTALL)
    return match.group(1).strip() if match else stripped


def review_packet_from_text(text, topology, *, root=None):
    payload = _strip_json_fence(text)
    if not payload:
        raise ScafldError("external reviewer returned no ReviewPacket content", code=EC.COMMAND_FAILED)
    try:
        data = json.loads(payload)
    except json.JSONDecodeError as exc:
        preview = payload[:200].replace("\n", "\\n")
        if len(payload) > 200:
            preview += "..."
        raise ScafldError(
            "external reviewer returned invalid ReviewPacket JSON",
            [str(exc), f"received_preview: {preview}"],
            code=EC.COMMAND_FAILED,
        ) from exc
    return normalize_review_packet(data, topology, root=root)


def _as_nonempty_string(value, field, errors):
    if not isinstance(value, str) or not value.strip():
        errors.append(f"{field} must be a non-empty string")
        return ""
    normalized = value.strip()
    if "\n" in normalized or "\r" in normalized:
        errors.append(f"{field} must be a single-line string")
        return re.sub(r"\s+", " ", normalized).strip()
    return normalized


def _as_optional_string(value, field, errors):
    if value is None:
        return ""
    if not isinstance(value, str):
        errors.append(f"{field} must be a string")
        return ""
    normalized = value.strip()
    if not normalized:
        return ""
    if "\n" in normalized or "\r" in normalized:
        errors.append(f"{field} must be a single-line string")
        return re.sub(r"\s+", " ", normalized).strip()
    return normalized


def _as_string_list(value, field, errors, *, require_nonempty=True):
    if not isinstance(value, list):
        errors.append(f"{field} must be a list")
        return []
    result = []
    for index, item in enumerate(value):
        if not isinstance(item, str) or not item.strip():
            errors.append(f"{field}[{index}] must be a non-empty string")
            continue
        normalized = item.strip()
        if "\n" in normalized or "\r" in normalized:
            errors.append(f"{field}[{index}] must be a single-line string")
            normalized = re.sub(r"\s+", " ", normalized).strip()
        result.append(normalized)
    if require_nonempty and not result:
        errors.append(f"{field} must contain at least one item")
    return result


def _is_concrete_checked_target(target):
    normalized = re.sub(r"\s+", " ", str(target or "").strip().lower()).strip(".")
    if not normalized or normalized in GENERIC_CHECKED_TARGETS:
        return False
    return bool(re.search(r"[\w./-]+\.[A-Za-z0-9]{1,8}", normalized) or ":" in normalized or "#" in normalized)


def _validate_file_line_target(target, root, field, errors):
    if root is None:
        return
    match = re.fullmatch(r"([^#`\n]+):(\d+)", str(target or ""))
    if not match:
        return
    rel_text = match.group(1)
    line_number = int(match.group(2))
    rel_path = Path(rel_text)
    if rel_path.is_absolute() or ".." in rel_path.parts:
        errors.append(f"{field} must cite a relative path inside the repository")
        return
    root_path = Path(root).resolve()
    path = (root_path / rel_path).resolve()
    try:
        path.relative_to(root_path)
    except ValueError:
        errors.append(f"{field} must cite a path inside the repository")
        return
    if not path.is_file():
        errors.append(f"{field} cites a file that does not exist: {rel_text}")
        return
    with path.open("r", encoding="utf-8", errors="replace") as handle:
        line_count = sum(1 for _line in handle)
    if line_number < 1 or line_number > line_count:
        errors.append(f"{field} cites line {line_number} outside {rel_text} line count {line_count}")


def _validate_anchor_target(target, root, field, errors):
    if root is None:
        return
    match = re.fullmatch(r"([^#`\n]+\.(?:ya?ml|md|markdown))#[A-Za-z0-9_.:/-]+", str(target or ""), re.IGNORECASE)
    if not match:
        return
    rel_text = match.group(1)
    rel_path = Path(rel_text)
    if rel_path.is_absolute() or ".." in rel_path.parts:
        errors.append(f"{field} must cite a relative anchor path inside the repository")
        return
    root_path = Path(root).resolve()
    path = (root_path / rel_path).resolve()
    try:
        path.relative_to(root_path)
    except ValueError:
        errors.append(f"{field} must cite an anchor path inside the repository")
        return
    if not path.is_file():
        errors.append(f"{field} cites an anchor file that does not exist: {rel_text}")


def _validate_grounded_target(target, root, field, errors):
    _validate_file_line_target(target, root, field, errors)
    _validate_anchor_target(target, root, field, errors)


def _normalize_spec_update_suggestions(value, field, errors):
    if value is None:
        return []
    if not isinstance(value, list):
        errors.append(f"{field} must be a list")
        return []
    suggestions = []
    for index, item in enumerate(value):
        item_field = f"{field}[{index}]"
        if not isinstance(item, dict):
            errors.append(f"{item_field} must be an object")
            continue
        suggestion = {
            "kind": _as_nonempty_string(item.get("kind"), f"{item_field}.kind", errors),
            "suggested_text": _as_nonempty_string(
                item.get("suggested_text"),
                f"{item_field}.suggested_text",
                errors,
            ),
            "reason": _as_optional_string(item.get("reason"), f"{item_field}.reason", errors),
            "phase_id": _as_optional_string(item.get("phase_id"), f"{item_field}.phase_id", errors),
            "validation_command": _as_optional_string(
                item.get("validation_command"),
                f"{item_field}.validation_command",
                errors,
            ),
        }
        suggestions.append(suggestion)
    return suggestions


def _normalize_checked_surfaces(packet, pass_ids, errors, *, root=None):
    value = packet.get("checked_surfaces")
    if not isinstance(value, list):
        errors.append("checked_surfaces must be a list")
        return []
    surfaces = []
    seen = set()
    duplicates = set()
    for index, item in enumerate(value):
        field = f"checked_surfaces[{index}]"
        if not isinstance(item, dict):
            errors.append(f"{field} must be an object")
            continue
        pass_id = _as_nonempty_string(item.get("pass_id"), f"{field}.pass_id", errors)
        if pass_id and pass_id not in pass_ids:
            errors.append(f"{field}.pass_id is not configured: {pass_id}")
        targets = _as_string_list(item.get("targets"), f"{field}.targets", errors)
        for target_index, target in enumerate(targets):
            if not _is_concrete_checked_target(target):
                errors.append(f"{field}.targets[{target_index}] must name a concrete file, symbol, rule, path, or anchor")
            _validate_grounded_target(target, root, f"{field}.targets[{target_index}]", errors)
        summary = _as_nonempty_string(item.get("summary"), f"{field}.summary", errors)
        limitations = _as_string_list(
            item.get("limitations", []),
            f"{field}.limitations",
            errors,
            require_nonempty=False,
        )
        if pass_id:
            if pass_id in seen:
                duplicates.add(pass_id)
            seen.add(pass_id)
        surfaces.append({
            "pass_id": pass_id,
            "targets": targets,
            "summary": summary,
            "limitations": limitations,
        })
    if duplicates:
        errors.append(f"checked_surfaces contains duplicate pass ids: {', '.join(sorted(duplicates))}")
    missing = sorted(set(pass_ids) - seen)
    if missing:
        errors.append(f"checked_surfaces missing pass ids: {', '.join(missing)}")
    return surfaces


def _normalize_findings(packet, pass_ids, errors, *, root=None):
    value = packet.get("findings", [])
    if not isinstance(value, list):
        errors.append("findings must be a list")
        return []
    if len(value) > MAX_REVIEW_PACKET_FINDINGS:
        errors.append(f"findings must contain at most {MAX_REVIEW_PACKET_FINDINGS} items")
    findings = []
    seen_ids = set()
    for index, item in enumerate(value):
        field = f"findings[{index}]"
        if not isinstance(item, dict):
            errors.append(f"{field} must be an object")
            continue
        finding_id = _as_nonempty_string(item.get("id"), f"{field}.id", errors)
        if finding_id:
            if not re.fullmatch(r"[A-Za-z0-9_.-]+", finding_id):
                errors.append(f"{field}.id must contain only letters, numbers, dot, underscore, or hyphen")
            if finding_id in seen_ids:
                errors.append(f"{field}.id is duplicated: {finding_id}")
            seen_ids.add(finding_id)
        pass_id = _as_nonempty_string(item.get("pass_id"), f"{field}.pass_id", errors)
        if pass_id and pass_id not in pass_ids:
            errors.append(f"{field}.pass_id is not configured: {pass_id}")
        severity = _as_nonempty_string(item.get("severity"), f"{field}.severity", errors).lower()
        if severity and severity not in FINDING_SEVERITIES:
            errors.append(f"{field}.severity must be one of: {', '.join(FINDING_SEVERITIES)}")
        blocking = item.get("blocking")
        if not isinstance(blocking, bool):
            errors.append(f"{field}.blocking must be true or false")
            blocking = False
        target = _as_nonempty_string(item.get("target"), f"{field}.target", errors)
        if target and not FINDING_TARGET_RE.fullmatch(target):
            errors.append(f"{field}.target must use file:line or doc.md#anchor")
        _validate_grounded_target(target, root, f"{field}.target", errors)
        finding = {
            "id": finding_id,
            "pass_id": pass_id,
            "severity": severity,
            "blocking": blocking,
            "target": target,
            "summary": _as_nonempty_string(item.get("summary"), f"{field}.summary", errors),
            "failure_mode": _as_nonempty_string(item.get("failure_mode"), f"{field}.failure_mode", errors),
            "why_it_matters": _as_nonempty_string(item.get("why_it_matters"), f"{field}.why_it_matters", errors),
            "evidence": _as_string_list(item.get("evidence"), f"{field}.evidence", errors),
            "suggested_fix": _as_nonempty_string(item.get("suggested_fix"), f"{field}.suggested_fix", errors),
            "tests_to_add": _as_string_list(item.get("tests_to_add"), f"{field}.tests_to_add", errors),
            "spec_update_suggestions": _normalize_spec_update_suggestions(
                item.get("spec_update_suggestions", []),
                f"{field}.spec_update_suggestions",
                errors,
            ),
        }
        findings.append(finding)
    return findings


def _validate_pass_result_relations(verdict, pass_results, findings, pass_ids, errors):
    findings_by_pass = {pass_id: [] for pass_id in pass_ids}
    for finding in findings:
        if finding.get("pass_id") in findings_by_pass:
            findings_by_pass[finding["pass_id"]].append(finding)

    for pass_id in pass_ids:
        result = pass_results.get(pass_id)
        pass_findings = findings_by_pass.get(pass_id, [])
        blocking = [finding for finding in pass_findings if finding.get("blocking")]
        non_blocking = [finding for finding in pass_findings if not finding.get("blocking")]
        if result == "pass" and pass_findings:
            errors.append(f"pass result {pass_id}=pass cannot include findings")
        if result == "fail" and not blocking:
            errors.append(f"pass result {pass_id}=fail requires a blocking finding")
        if result == "pass_with_issues" and (blocking or not non_blocking):
            errors.append(f"pass result {pass_id}=pass_with_issues requires non-blocking findings and no blocking findings")

    blocking_all = [finding for finding in findings if finding.get("blocking")]
    non_blocking_all = [finding for finding in findings if not finding.get("blocking")]
    if verdict == "pass" and findings:
        errors.append("verdict pass cannot include findings")
    if verdict == "pass_with_issues" and (blocking_all or not non_blocking_all):
        errors.append("verdict pass_with_issues requires non-blocking findings and no blocking findings")
    if verdict == "fail" and not blocking_all:
        errors.append("verdict fail requires at least one blocking finding")


def normalize_review_packet(packet, topology, *, root=None):
    if not isinstance(packet, dict):
        _packet_error("ReviewPacket must be a JSON object")
    forbidden = sorted(key for key in packet if key in FORBIDDEN_PROVIDER_PACKET_KEYS)
    unexpected = sorted(set(packet) - REVIEW_PACKET_ALLOWED_KEYS)
    errors = []
    if forbidden:
        errors.append(f"ReviewPacket contains scafld-owned fields: {', '.join(forbidden)}")
    non_forbidden_unexpected = sorted(set(unexpected) - set(forbidden))
    if non_forbidden_unexpected:
        errors.append(f"ReviewPacket contains unexpected fields: {', '.join(non_forbidden_unexpected)}")
    schema_version = packet.get("schema_version")
    if schema_version != REVIEW_PACKET_SCHEMA_VERSION:
        errors.append(f"schema_version must be {REVIEW_PACKET_SCHEMA_VERSION}")

    pass_ids = review_pass_ids(topology, "adversarial")
    pass_results_value = packet.get("pass_results")
    pass_results = {}
    if not isinstance(pass_results_value, dict):
        errors.append("pass_results must be an object")
    else:
        unexpected = sorted(set(pass_results_value) - set(pass_ids))
        missing = sorted(set(pass_ids) - set(pass_results_value))
        if missing:
            errors.append(f"pass_results missing pass ids: {', '.join(missing)}")
        if unexpected:
            errors.append(f"pass_results contains unexpected pass ids: {', '.join(unexpected)}")
        for pass_id in pass_ids:
            value = pass_results_value.get(pass_id)
            if value not in REVIEW_PACKET_PASS_RESULTS:
                errors.append(f"pass_results.{pass_id} must be one of: pass, fail, pass_with_issues")
            else:
                pass_results[pass_id] = value

    verdict = packet.get("verdict")
    if verdict not in REVIEW_PACKET_VERDICTS:
        errors.append("verdict must be one of: pass, fail, pass_with_issues")
        verdict = ""

    review_summary = _as_nonempty_string(packet.get("review_summary"), "review_summary", errors)
    checked_surfaces = _normalize_checked_surfaces(packet, pass_ids, errors, root=root)
    findings = _normalize_findings(packet, pass_ids, errors, root=root)
    if verdict:
        _validate_pass_result_relations(verdict, pass_results, findings, pass_ids, errors)

    if errors:
        _packet_error(errors[0], errors[1:8])

    return {
        "schema_version": REVIEW_PACKET_SCHEMA_VERSION,
        "review_summary": review_summary,
        "verdict": verdict,
        "pass_results": {pass_id: pass_results[pass_id] for pass_id in pass_ids},
        "checked_surfaces": checked_surfaces,
        "findings": findings,
    }


def finding_markdown_line(finding):
    return f"- **{finding['severity']}** `{finding['target']}` — {finding['summary']}"


def _checked_line(surface):
    targets = ", ".join(surface.get("targets") or [])
    if surface.get("summary"):
        return f"No issues found — checked {surface['summary']} ({targets})."
    return f"No issues found — checked {targets}."


def review_packet_projection(packet, topology):
    surfaces_by_pass = {}
    for surface in packet.get("checked_surfaces", []):
        surfaces_by_pass.setdefault(surface["pass_id"], surface)
    findings_by_pass = {}
    for finding in packet.get("findings", []):
        findings_by_pass.setdefault(finding["pass_id"], []).append(finding)

    sections = {}
    for definition in review_passes_by_kind(topology, "adversarial"):
        pass_id = definition["id"]
        findings = findings_by_pass.get(pass_id, [])
        if findings:
            sections[pass_id] = "\n".join(finding_markdown_line(finding) for finding in findings)
        else:
            sections[pass_id] = _checked_line(surfaces_by_pass[pass_id])

    blocking = [finding_markdown_line(finding) for finding in packet.get("findings", []) if finding["blocking"]]
    non_blocking = [finding_markdown_line(finding) for finding in packet.get("findings", []) if not finding["blocking"]]
    return {
        "pass_results": packet["pass_results"],
        "sections": sections,
        "blocking": blocking,
        "non_blocking": non_blocking,
        "verdict": packet["verdict"],
        "canonical": {
            "packet": packet,
            "projection": {
                "pass_results": packet["pass_results"],
                "sections": sections,
                "blocking": blocking,
                "non_blocking": non_blocking,
                "verdict": packet["verdict"],
            },
        },
    }


def compute_canonical_response_sha256(packet):
    """Recompute the canonical response sha256 from a parsed packet.

    Mirrors the writer at review_runner.py — json-dumps the packet
    with sorted keys and compact separators and returns the hex
    sha256. Topology-independent: the seal binds to the packet
    contents directly, so editing `.ai/config.yaml` between review
    and complete cannot produce false-positive mismatches.
    """
    canonical_payload = json.dumps(packet, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canonical_payload.encode("utf-8")).hexdigest()


def metadata_canonical_sha256(metadata):
    """Read the seal hash from metadata.

    The writer (`build_review_metadata` + `review_provenance` payload)
    nests the hash inside `metadata.review_provenance.canonical_response_sha256`.
    This helper centralises the lookup so callers don't read the wrong
    level. Returns the hex string when present, or None.
    """
    if not isinstance(metadata, dict):
        return None
    provenance = metadata.get("review_provenance")
    if isinstance(provenance, dict):
        sha = provenance.get("canonical_response_sha256")
        if isinstance(sha, str) and sha:
            return sha
    # Fallback: tolerate seals written at the top level (older write paths
    # or hand-crafted fixtures). Empty/missing returns None.
    sha = metadata.get("canonical_response_sha256")
    return sha if isinstance(sha, str) and sha else None


def verify_review_seal(metadata, packet):
    """Verify a parsed packet's canonical sha256 against the review metadata.

    Returns:
      (True, "")           — metadata hash matches the recomputed packet hash
      (False, "missing_seal") — metadata lacks `canonical_response_sha256`;
                               caller decides what to do (1.7.0 cutover:
                               reject; pre-1.7 files surface as missing).
      (False, "hash mismatch: ...") — metadata hash and recomputed hash
                                       differ. The review file or packet was
                                       edited after the seal was written.
    """
    expected = metadata_canonical_sha256(metadata)
    if not expected:
        return False, "missing_seal"
    recomputed = compute_canonical_response_sha256(packet)
    if recomputed == expected:
        return True, ""
    return False, (
        f"hash mismatch: metadata={expected[:12]}, recomputed={recomputed[:12]}"
    )


def read_review_packet_artifact(root, task_id, review_count, *, spec_path=None):
    """Read the canonical packet body persisted alongside a review round.

    Returns the parsed `packet` dict, or None when the artifact is
    missing (local / manual runner paths, or pre-1.7 review files that
    pre-date the artifact write).
    """
    path = review_packet_artifact_path(root, task_id, review_count, spec_path=spec_path)
    if not path.exists():
        return None
    try:
        artifact = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None
    if not isinstance(artifact, dict):
        return None
    packet = artifact.get("packet")
    return packet if isinstance(packet, dict) else None


def review_packet_artifact_path(root, task_id, review_count, *, spec_path=None):
    return review_packets_dir(root, task_id, spec_path=spec_path) / f"review-{review_count}.json"


def review_packet_artifact_rel(root, task_id, review_count, *, spec_path=None):
    return relative_path(root, review_packet_artifact_path(root, task_id, review_count, spec_path=spec_path))


def write_review_packet_artifact(root, task_id, review_count, packet, *, spec_path=None):
    path = review_packet_artifact_path(root, task_id, review_count, spec_path=spec_path)
    path.parent.mkdir(parents=True, exist_ok=True)
    artifact = {
        "schema_version": REVIEW_PACKET_ARTIFACT_SCHEMA_VERSION,
        "task_id": task_id,
        "review_round": review_count,
        "generated_at": now_iso(),
        "packet": packet,
    }
    path.write_text(json.dumps(artifact, indent=2) + "\n", encoding="utf-8")
    return review_packet_artifact_rel(root, task_id, review_count, spec_path=spec_path)


def repair_handoff_paths(root, task_id, *, spec_path=None):
    markdown_path = handoff_path(root, task_id, role="executor", gate="review_repair", spec_path=spec_path)
    return markdown_path, markdown_path.with_suffix(".json")


def repair_handoff_rels(root, task_id, *, spec_path=None):
    markdown_path, json_path = repair_handoff_paths(root, task_id, spec_path=spec_path)
    return relative_path(root, markdown_path), relative_path(root, json_path)


def _render_list(items):
    items = list(items or [])
    if not items:
        return "- None."
    return "\n".join(f"- {item}" for item in items)


def _render_spec_suggestions(suggestions):
    if not suggestions:
        return "- None."
    lines = []
    for suggestion in suggestions:
        bits = [suggestion["suggested_text"]]
        if suggestion.get("kind"):
            bits.append(f"kind: `{suggestion['kind']}`")
        if suggestion.get("phase_id"):
            bits.append(f"phase: `{suggestion['phase_id']}`")
        if suggestion.get("validation_command"):
            bits.append(f"validation: `{suggestion['validation_command']}`")
        if suggestion.get("reason"):
            bits.append(f"reason: {suggestion['reason']}")
        lines.append("- " + " | ".join(bits))
    return "\n".join(lines)


def render_executor_repair_handoff(task_id, review_count, packet, packet_rel):
    lines = [
        "---",
        f"schema_version: {REVIEW_REPAIR_HANDOFF_SCHEMA_VERSION}",
        'role: "executor"',
        'gate: "review_repair"',
        f"task_id: {json.dumps(task_id)}",
        f"review_round: {review_count}",
        f"review_packet: {json.dumps(packet_rel)}",
        "---",
        "",
        "# Executor Review Repair",
        "",
        "Use this packet-derived repair brief to update the spec and implementation. Do not apply spec suggestions blindly; treat them as reviewer proposals to reconcile with the task contract.",
        "",
        "## Review Summary",
        packet["review_summary"],
        "",
        "## Verdict",
        f"- Verdict: `{packet['verdict']}`",
        "- Pass results: "
        + ", ".join(f"{key}={value}" for key, value in packet["pass_results"].items()),
        "",
        "## Checked Surfaces",
    ]
    for surface in packet.get("checked_surfaces", []):
        lines.extend([
            f"### {surface['pass_id']}",
            surface["summary"],
            "",
            "Targets:",
            _render_list(f"`{target}`" for target in surface.get("targets", [])),
            "",
            "Limitations:",
            _render_list(surface.get("limitations", [])),
            "",
        ])

    blocking = [finding for finding in packet.get("findings", []) if finding["blocking"]]
    non_blocking = [finding for finding in packet.get("findings", []) if not finding["blocking"]]
    for title, findings in (("Blocking Findings", blocking), ("Non-blocking Findings", non_blocking)):
        lines.append(f"## {title}")
        if not findings:
            lines.extend(["- None.", ""])
            continue
        for finding in findings:
            lines.extend([
                f"### {finding['id']} — {finding['severity']} — `{finding['target']}`",
                finding["summary"],
                "",
                f"- Pass: `{finding['pass_id']}`",
                f"- Failure mode: {finding['failure_mode']}",
                f"- Why it matters: {finding['why_it_matters']}",
                f"- Suggested fix: {finding['suggested_fix']}",
                "",
                "Evidence:",
                _render_list(finding.get("evidence", [])),
                "",
                "Tests to add or update:",
                _render_list(finding.get("tests_to_add", [])),
                "",
                "Spec update suggestions:",
                _render_spec_suggestions(finding.get("spec_update_suggestions", [])),
                "",
            ])
    return "\n".join(lines).rstrip() + "\n"


def write_executor_repair_handoff(root, task_id, review_count, packet, packet_rel, *, spec_path=None):
    markdown_path, json_path = repair_handoff_paths(root, task_id, spec_path=spec_path)
    markdown_path.parent.mkdir(parents=True, exist_ok=True)
    markdown_path.write_text(
        render_executor_repair_handoff(task_id, review_count, packet, packet_rel),
        encoding="utf-8",
    )
    payload = {
        "schema_version": REVIEW_REPAIR_HANDOFF_SCHEMA_VERSION,
        "role": "executor",
        "gate": "review_repair",
        "task_id": task_id,
        "review_round": review_count,
        "review_packet": packet_rel,
        "finding_count": len(packet.get("findings", [])),
        "blocking_count": len([finding for finding in packet.get("findings", []) if finding.get("blocking")]),
    }
    json_path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
    return repair_handoff_rels(root, task_id, spec_path=spec_path)
