import re
from copy import deepcopy

from scafld.error_codes import ErrorCode
from scafld.errors import ScafldError

PHASE_SELECTOR_RE = re.compile(r"^[a-z][a-z0-9_-]*$")
FRONT_MATTER_RE = re.compile(r"\A---\n(.*?)\n---\n?", re.DOTALL)
HTML_COMMENT_RE = re.compile(r"<!--.*?-->", re.DOTALL)
PHASE_HEADING_RE = re.compile(r"^##\s+Phase\s+([1-9][0-9]*)\s*:\s*(.*?)\s*$", re.MULTILINE)
FENCE_RE = re.compile(r"^\s*(```+|~~~+)(.*)$")


def require_pyyaml():
    try:
        import yaml
    except ModuleNotFoundError as exc:
        raise ScafldError(
            "PyYAML is required for Markdown spec front matter and structured sections",
            ["install it with: python3 -m pip install PyYAML"],
            code=ErrorCode.MISSING_DEPENDENCY,
        ) from exc
    return yaml


def _yaml_load_any(text, *, path=None):
    yaml = require_pyyaml()
    try:
        data = yaml.safe_load(text) or {}
    except yaml.YAMLError as exc:
        details = []
        mark = getattr(exc, "problem_mark", None)
        if mark is not None:
            details.append(f"yaml parse error at line {mark.line + 1}, column {mark.column + 1}")
        problem = getattr(exc, "problem", None)
        if isinstance(problem, str) and problem:
            details.append(problem)
        name = getattr(path, "name", "spec")
        raise ScafldError(f"invalid spec document: {name}", details, code=ErrorCode.INVALID_SPEC_DOCUMENT) from exc
    return data


def _yaml_load(text, *, path=None):
    data = _yaml_load_any(text, path=path)
    if not isinstance(data, dict):
        name = getattr(path, "name", "spec")
        raise ScafldError(f"invalid spec document: {name}", ["spec root must be a mapping"], code=ErrorCode.INVALID_SPEC_DOCUMENT)
    return data


def _yaml_dump(data):
    yaml = require_pyyaml()

    class LiteralDumper(yaml.SafeDumper):
        pass

    def represent_str(dumper, value):
        style = "|" if "\n" in value else None
        return dumper.represent_scalar("tag:yaml.org,2002:str", value, style=style)

    LiteralDumper.add_representer(str, represent_str)
    return yaml.dump(data, Dumper=LiteralDumper, sort_keys=False).strip()


def parse_front_matter(text, *, path=None):
    match = FRONT_MATTER_RE.match(text)
    if not match:
        raise ScafldError(
            f"invalid Markdown spec: {getattr(path, 'name', 'spec')}",
            ["missing YAML front matter"],
            code=ErrorCode.INVALID_SPEC_DOCUMENT,
        )
    return _yaml_load(match.group(1), path=path), match.end()


def _line_spans(text):
    offset = 0
    for line in text.splitlines(keepends=True):
        yield offset, line.rstrip("\n\r")
        offset += len(line)


def _fence_state(line, active):
    match = FENCE_RE.match(line)
    if not match:
        return active
    marker = match.group(1)
    rest = match.group(2).strip()
    if active:
        closes = marker.startswith(active[0]) and len(marker) >= len(active) and not rest
        return None if closes else active
    return marker


def _find_next_heading_offset(text, start=0, level=2):
    active_fence = None
    heading_re = re.compile(r"^" + ("#" * level) + r"\s+")
    for offset, line in _line_spans(text[start:]):
        active_fence = _fence_state(line, active_fence)
        if active_fence:
            continue
        if heading_re.match(line):
            return start + offset
    return None


def _phase_heading_matches(text):
    active_fence = None
    matches = []
    for offset, line in _line_spans(text):
        active_fence = _fence_state(line, active_fence)
        if active_fence:
            continue
        match = PHASE_HEADING_RE.match(line)
        if match:
            number = int(match.group(1))
            name = match.group(2).strip() or f"Phase {number}"
            matches.append({"start": offset, "end": offset + len(line), "number": number, "name": name, "selector": f"phase{number}"})
    return matches


def _section_body(text, heading):
    pattern = re.compile(rf"^##\s+{re.escape(heading)}\s*$", re.MULTILINE)
    found = False
    end = 0
    active_fence = None
    for offset, line in _line_spans(text):
        active_fence = _fence_state(line, active_fence)
        if active_fence:
            continue
        candidate = pattern.match(line)
        if candidate:
            found = True
            end = offset + len(line)
            break
    if not found:
        return ""
    next_offset = _find_next_heading_offset(text, start=end)
    body_end = next_offset if next_offset is not None else len(text)
    body = text[end:body_end].strip("\n")
    return body.strip()


def _validate_markdown_fences(text):
    active_fence = None
    for line_no, raw in enumerate(text.splitlines(), start=1):
        match = FENCE_RE.match(raw)
        if not match:
            continue
        marker = match.group(1)
        info = match.group(2).strip().lower()
        if active_fence:
            closes = marker.startswith(active_fence[0]) and len(marker) >= len(active_fence) and not info
            if closes:
                active_fence = None
            continue
        active_fence = marker
    if active_fence:
        raise ScafldError(
            "invalid Markdown spec",
            ["unclosed Markdown code fence"],
            code=ErrorCode.INVALID_SPEC_DOCUMENT,
        )


def _reject_html_comments(text):
    if HTML_COMMENT_RE.search(text):
        raise ScafldError(
            "invalid Markdown spec",
            ["HTML comments are not part of task specs; use headings, labels, and lists"],
            code=ErrorCode.INVALID_SPEC_DOCUMENT,
        )


def _parse_label_block(payload):
    data = {}
    for raw in payload.splitlines():
        line = raw.strip()
        if not line or line.startswith("- "):
            continue
        if ":" not in line:
            continue
        key, value = line.split(":", 1)
        value = value.strip()
        if value.lower() == "none":
            parsed = None
        else:
            parsed = value.strip("`")
        data[key.strip().lower().replace(" ", "_").replace("-", "_")] = parsed
    return data


def _label_key(value):
    return str(value).strip().lower().replace(" ", "_").replace("-", "_")


def _none_value(value):
    return str(value or "").strip().strip("`").lower() in ("", "none", "null", "[]", "{}")


def _clean_value(value):
    value = str(value or "").strip()
    if _none_value(value):
        return None
    return value.strip("`")


def _parse_int_value(value):
    value = _clean_value(value)
    if value is None:
        return None
    try:
        return int(value)
    except ValueError:
        return None


def _parse_float_value(value):
    value = _clean_value(value)
    if value is None:
        return None
    try:
        return float(value)
    except ValueError:
        return None


def _parse_bool_value(value):
    value = _clean_value(value)
    if value is None:
        return None
    lowered = value.lower()
    if lowered in ("true", "yes"):
        return True
    if lowered in ("false", "no"):
        return False
    return None


def _first_payload_line(payload):
    for line in str(payload or "").splitlines():
        stripped = line.strip()
        if stripped:
            return stripped
    return ""


def _parse_bullets(payload):
    items = []
    for line in payload.splitlines():
        stripped = line.strip()
        if not stripped.startswith("- "):
            continue
        value = stripped[2:].strip()
        if value.lower() in ("none", "none."):
            continue
        items.append(value)
    return items


def _parse_key_bullets(payload):
    data = {}
    current_key = None
    for raw in str(payload or "").splitlines():
        stripped = raw.strip()
        match = re.match(r"^-\s+`?([^`:]+)`?:\s*(.*)$", stripped)
        if match:
            current_key = _label_key(match.group(1))
            data[current_key] = _clean_value(match.group(2))
            continue
        if current_key and stripped.startswith("- "):
            data.setdefault(current_key, [])
            if not isinstance(data[current_key], list):
                data[current_key] = []
            value = _clean_value(stripped[2:].strip())
            if value is not None:
                data[current_key].append(value)
    return data


def _parse_checklist(payload):
    items = []
    current = None
    for raw in payload.splitlines():
        line = raw.rstrip()
        match = re.match(r"^-\s+\[( |x|X)\]\s+`([^`]+)`\s*(.*)$", line.strip())
        if match:
            if current:
                items.append(current)
            status = "pass" if match.group(1).lower() == "x" else "pending"
            rest = match.group(3).strip()
            if rest.startswith("- "):
                rest = rest[2:].strip()
            item_type = None
            description = rest
            type_match = re.match(r"([A-Za-z_][\w_-]*)\s+-\s+(.*)$", rest)
            if type_match:
                item_type = type_match.group(1)
                description = type_match.group(2)
            elif rest and re.match(r"^[A-Za-z_][\w_-]*$", rest):
                item_type = rest
                description = ""
            current = {"id": match.group(2), "status": status, "description": description}
            if item_type:
                current["type"] = item_type
            continue
        detail = re.match(r"^\s+-\s+([^:]+):\s*(.*)$", line)
        if current and detail:
            key = detail.group(1).strip().lower().replace(" ", "_").replace("-", "_")
            value = detail.group(2).strip()
            if value.lower() == "none":
                continue
            current[key] = value.strip("`")
    if current:
        items.append(current)
    return items


def _parse_context(payload):
    context = {"packages": [], "files_impacted": [], "invariants": [], "related_docs": []}
    current = None
    for raw in payload.splitlines():
        line = raw.strip()
        lower = line.lower()
        if lower.startswith("cwd:"):
            context["cwd"] = line.split(":", 1)[1].strip().strip("`")
        elif lower.startswith("packages:"):
            current = "packages"
        elif lower.startswith("files impacted:"):
            current = "files_impacted"
        elif lower.startswith("invariants:"):
            current = "invariants"
        elif lower.startswith("related docs:"):
            current = "related_docs"
            if "none" in lower:
                context["related_docs"] = []
        elif line.startswith("- ") and current == "packages":
            value = line[2:].strip().strip("`")
            if value.lower() not in ("none", "none."):
                context["packages"].append(value)
        elif line.startswith("- ") and current == "invariants":
            value = line[2:].strip().strip("`")
            if value.lower() not in ("none", "none."):
                context["invariants"].append(value)
        elif line.startswith("- ") and current == "related_docs":
            value = line[2:].strip().strip("`")
            if value.lower() not in ("none", "none."):
                context["related_docs"].append(value)
        elif line.startswith("- ") and current == "files_impacted":
            match = re.match(r"-\s+`([^`]+)`(?:\s+\(([^)]*)\))?\s*-\s*(.*)", line)
            if match:
                item = {"path": match.group(1), "reason": match.group(3)}
                bits = [bit.strip() for bit in (match.group(2) or "").split(",") if bit.strip()]
                if bits:
                    item["lines"] = bits[0]
                if len(bits) > 1:
                    item["ownership"] = bits[1]
                context["files_impacted"].append(item)
    return context


def _parse_acceptance(payload):
    acceptance = {"validation_profile": "strict", "definition_of_done": [], "validation": []}
    lines = payload.splitlines()
    mode = None
    buffer = []
    for line in lines:
        stripped = line.strip()
        if stripped.lower().startswith("profile:"):
            acceptance["validation_profile"] = stripped.split(":", 1)[1].strip()
        elif stripped.lower().startswith("definition of done:"):
            if mode == "validation":
                acceptance["validation"] = _parse_checklist("\n".join(buffer))
            mode = "dod"
            buffer = []
        elif stripped.lower().startswith("validation:"):
            if mode == "dod":
                acceptance["definition_of_done"] = _parse_checklist("\n".join(buffer))
            mode = "validation"
            buffer = []
        elif mode:
            buffer.append(line)
    if mode == "dod":
        acceptance["definition_of_done"] = _parse_checklist("\n".join(buffer))
    elif mode == "validation":
        acceptance["validation"] = _parse_checklist("\n".join(buffer))
    for item in acceptance.get("definition_of_done") or []:
        if isinstance(item, dict) and item.get("status") == "pass":
            item["status"] = "done"
    return acceptance


def _parse_changes(payload):
    changes = []
    for line in payload.splitlines():
        stripped = line.strip()
        if not stripped.startswith("- "):
            continue
        match = re.match(r"-\s+`([^`]+)`(?:\s+\(([^)]*)\))?\s*-\s*(.*)", stripped)
        if not match:
            continue
        item = {"file": match.group(1), "action": "update", "content_spec": match.group(3)}
        bits = [bit.strip() for bit in (match.group(2) or "").split(",") if bit.strip()]
        if bits:
            item["lines"] = bits[0]
        if len(bits) > 1:
            item["ownership"] = bits[1]
        changes.append(item)
    return changes


def _split_label_blocks(payload, labels):
    labels = {label.lower().replace(" ", "_"): label for label in labels}
    blocks = {}
    current = None
    for raw in payload.splitlines():
        match = re.match(r"^([A-Za-z][A-Za-z _-]+):\s*(.*)$", raw.strip())
        key = match.group(1).lower().replace(" ", "_").replace("-", "_") if match else None
        if key in labels:
            current = key
            blocks.setdefault(current, [])
            rest = match.group(2).strip()
            if rest:
                blocks[current].append(rest)
            continue
        if current:
            blocks.setdefault(current, []).append(raw)
    return {key: "\n".join(value).strip() for key, value in blocks.items()}


def _parse_phase_body(payload):
    blocks = _split_label_blocks(payload, ("Goal", "Status", "Dependencies", "Changes", "Acceptance"))
    phase = {}
    goal = blocks.get("goal")
    if goal:
        phase["objective"] = goal.strip()
    status = blocks.get("status")
    if status:
        phase["status"] = status.splitlines()[0].strip()
    deps = blocks.get("dependencies")
    if deps:
        first = deps.splitlines()[0].strip()
        if first.lower() in ("none", "- none"):
            phase["dependencies"] = []
        elif first.startswith("- "):
            phase["dependencies"] = _parse_bullets(deps)
        else:
            phase["dependencies"] = [item.strip().strip("`") for item in first.split(",") if item.strip()]
    if blocks.get("changes"):
        phase["changes"] = _parse_changes(blocks["changes"])
    if blocks.get("acceptance"):
        phase["acceptance_criteria"] = _parse_checklist(blocks["acceptance"])
    return phase


def _parse_touchpoints(payload):
    items = []
    for bullet in _parse_bullets(payload):
        if ":" in bullet:
            area, description = bullet.split(":", 1)
            items.append({"area": area.strip(), "description": description.strip()})
        elif bullet.lower() not in ("none", "none."):
            items.append({"area": bullet, "description": ""})
    return items


def _parse_risks(payload):
    risks = []
    for bullet in _parse_bullets(payload):
        if bullet.lower() not in ("none", "none."):
            risks.append({"description": bullet})
    return risks


def _parse_rollback(payload):
    blocks = _split_label_blocks(payload, ("Strategy", "Commands"))
    strategy = _clean_value(_first_payload_line(blocks.get("strategy")))
    commands = {}
    for raw in blocks.get("commands", "").splitlines():
        match = re.match(r"^-\s+`?([^`:]+)`?:\s*(.*)$", raw.strip())
        if match:
            phase_id = match.group(1).strip()
            command = _clean_value(match.group(2))
            if phase_id and command:
                commands[phase_id] = command
    data = {}
    if strategy:
        data["strategy"] = strategy
    if commands:
        data["commands"] = commands
    return data


def _parse_review(payload):
    blocks = _split_label_blocks(
        payload,
        (
            "Status",
            "Verdict",
            "Timestamp",
            "Review rounds",
            "Reviewer mode",
            "Reviewer session",
            "Round status",
            "Override applied",
            "Override reason",
            "Override confirmed at",
            "Reviewed head",
            "Reviewed dirty",
            "Reviewed diff",
            "Blocking count",
            "Non-blocking count",
            "Findings",
            "Passes",
        ),
    )
    data = {}
    for label, key in (
        ("status", "status"),
        ("verdict", "verdict"),
        ("timestamp", "timestamp"),
        ("reviewer_mode", "reviewer_mode"),
        ("reviewer_session", "reviewer_session"),
        ("round_status", "round_status"),
        ("override_reason", "override_reason"),
        ("override_confirmed_at", "override_confirmed_at"),
        ("reviewed_head", "reviewed_head"),
        ("reviewed_diff", "reviewed_diff"),
    ):
        value = _clean_value(_first_payload_line(blocks.get(label)))
        if value:
            data[key] = value
    override_applied = _parse_bool_value(_first_payload_line(blocks.get("override_applied")))
    if override_applied is not None:
        data["override_applied"] = override_applied
    reviewed_dirty = _parse_bool_value(_first_payload_line(blocks.get("reviewed_dirty")))
    if reviewed_dirty is not None:
        data["reviewed_dirty"] = reviewed_dirty
    for label, key in (("review_rounds", "review_rounds"), ("blocking_count", "blocking_count"), ("non_blocking_count", "non_blocking_count")):
        value = _parse_int_value(_first_payload_line(blocks.get(label)))
        if value is not None:
            data[key] = value
    findings = []
    for bullet in _parse_bullets(blocks.get("findings", "")):
        if bullet.lower() not in ("none", "none."):
            findings.append({"summary": bullet})
    if findings or "findings" in blocks:
        data["findings"] = findings
    passes = []
    for raw in blocks.get("passes", "").splitlines():
        match = re.match(r"^-\s+`?([^`:]+)`?:\s*(.*)$", raw.strip())
        if match:
            result = _clean_value(match.group(2))
            if result:
                passes.append({"id": match.group(1).strip(), "result": result})
    if passes:
        data["passes"] = passes
    return data


def _parse_self_eval(payload):
    blocks = _split_label_blocks(
        payload,
        (
            "Status",
            "Completeness",
            "Architecture fidelity",
            "Spec alignment",
            "Validation depth",
            "Total",
            "Second pass performed",
            "Notes",
            "Improvements",
        ),
    )
    data = {}
    status = _clean_value(_first_payload_line(blocks.get("status")))
    if status:
        data["status"] = status
    for label, key in (
        ("completeness", "completeness"),
        ("architecture_fidelity", "architecture_fidelity"),
        ("spec_alignment", "spec_alignment"),
        ("validation_depth", "validation_depth"),
    ):
        value = _parse_int_value(_first_payload_line(blocks.get(label)))
        if value is not None:
            data[key] = value
    total = _parse_float_value(_first_payload_line(blocks.get("total")))
    if total is not None:
        data["total"] = total
    second_pass = _parse_bool_value(_first_payload_line(blocks.get("second_pass_performed")))
    if second_pass is not None:
        data["second_pass_performed"] = second_pass
    notes = _clean_value(blocks.get("notes"))
    if notes:
        data["notes"] = notes
    improvements = [item for item in _parse_bullets(blocks.get("improvements", "")) if item.lower() not in ("none", "none.")]
    if improvements:
        data["improvements"] = improvements
    return data


def _parse_deviations(payload):
    deviations = []
    current = None
    for raw in str(payload or "").splitlines():
        stripped = raw.strip()
        bullet = re.match(r"^-\s+`?([^`:-]+)`?\s*-\s*(.*)$", stripped)
        if bullet:
            if current:
                deviations.append(current)
            rule = bullet.group(1).strip()
            if rule.lower() in ("none", "none."):
                current = None
            else:
                current = {"rule": rule, "reason": bullet.group(2).strip()}
            continue
        detail = re.match(r"^\s+-\s+([^:]+):\s*(.*)$", raw)
        if current and detail:
            value = _clean_value(detail.group(2))
            if value is not None:
                current[_label_key(detail.group(1))] = value
    if current:
        deviations.append(current)
    return deviations


def _parse_metadata(payload):
    blocks = _split_label_blocks(payload, ("Estimated effort hours", "Actual effort hours", "AI model", "React cycles", "Tags"))
    data = {}
    for label, key in (("estimated_effort_hours", "estimated_effort_hours"), ("actual_effort_hours", "actual_effort_hours")):
        value = _parse_float_value(_first_payload_line(blocks.get(label)))
        if value is not None:
            data[key] = value
    react_cycles = _parse_int_value(_first_payload_line(blocks.get("react_cycles")))
    if react_cycles is not None:
        data["react_cycles"] = react_cycles
    ai_model = _clean_value(_first_payload_line(blocks.get("ai_model")))
    if ai_model:
        data["ai_model"] = ai_model
    tags = [item.strip("`") for item in _parse_bullets(blocks.get("tags", "")) if item.lower() not in ("none", "none.")]
    if tags or "tags" in blocks:
        data["tags"] = tags
    return data


def _parse_origin(payload):
    blocks = _split_label_blocks(payload, ("Source", "Repo", "Git", "Sync", "Supersession"))
    data = {}
    for section in ("source", "repo", "git", "supersession"):
        block = blocks.get(section)
        if not block or _none_value(_first_payload_line(block)):
            continue
        parsed = {key: value for key, value in _parse_key_bullets(block).items() if value is not None}
        if parsed:
            data[section] = parsed
    sync_block = blocks.get("sync")
    if sync_block and not _none_value(_first_payload_line(sync_block)):
        parsed = _parse_key_bullets(sync_block)
        sync = {}
        actual = {}
        for key, value in parsed.items():
            if key == "reasons":
                sync["reasons"] = value if isinstance(value, list) else ([] if value is None else [value])
            elif key.startswith("actual_"):
                actual[key[len("actual_"):]] = value
            elif value is not None:
                sync[key] = value
        if actual:
            sync["actual"] = actual
        if sync:
            data["sync"] = sync
    return data


def _parse_harden_rounds(payload):
    rounds = []
    current = None
    current_question = None
    for raw in str(payload or "").splitlines():
        stripped = raw.strip()
        round_match = re.match(r"^-\s+Round\s+([0-9]+)\s*$", stripped)
        if round_match:
            if current_question and current is not None:
                current.setdefault("questions", []).append(current_question)
            if current:
                rounds.append(current)
            current = {"round": int(round_match.group(1)), "questions": []}
            current_question = None
            continue
        if current is None:
            continue
        question_match = re.match(r"^\s+-\s+Question:\s*(.*)$", raw)
        if question_match:
            if current_question:
                current.setdefault("questions", []).append(current_question)
            current_question = {"question": _clean_value(question_match.group(1)) or ""}
            continue
        detail = re.match(r"^\s+-\s+([^:]+):\s*(.*)$", raw)
        if not detail:
            continue
        key = _label_key(detail.group(1))
        value = _clean_value(detail.group(2))
        if value is None:
            continue
        if current_question is not None and key in ("grounded_in", "recommended_answer", "if_unanswered", "answered_with"):
            current_question[key] = value
        elif key in ("started_at", "ended_at", "outcome"):
            current[key] = value
    if current_question and current is not None:
        current.setdefault("questions", []).append(current_question)
    if current:
        rounds.append(current)
    return rounds


def parse_spec_markdown(text, *, path=None):
    front_matter, body_start = parse_front_matter(text, path=path)
    body = text[body_start:]
    _reject_html_comments(body)
    _validate_markdown_fences(body)
    data = deepcopy(front_matter)
    title_match = re.search(r"^#\s+(.+?)\s*$", body, re.MULTILINE)
    task = data.setdefault("task", {})
    if title_match:
        task["title"] = title_match.group(1).strip()
    for key, target in (("size", "size"), ("risk_level", "risk_level")):
        if key in data:
            task[target] = data.get(key)

    summary = _section_body(body, "Summary")
    if summary:
        task["summary"] = summary
    for section, target in (("Objectives", "objectives"), ("Dependencies", "dependencies"), ("Assumptions", "assumptions")):
        section_text = _section_body(body, section)
        if section_text:
            task[target] = _parse_bullets(section_text)
    scope = _section_body(body, "Scope")
    if scope:
        task["scope"] = {"summary": scope}

    current = _section_body(body, "Current State")
    if current:
        data["current_state"] = _parse_label_block(current)
    context = _section_body(body, "Context")
    if context:
        task["context"] = _parse_context(context)
    touchpoints = _section_body(body, "Touchpoints")
    if touchpoints:
        task["touchpoints"] = _parse_touchpoints(touchpoints)
    risks = _section_body(body, "Risks")
    if risks:
        task["risks"] = _parse_risks(risks)
    acceptance = _section_body(body, "Acceptance")
    if acceptance:
        task["acceptance"] = _parse_acceptance(acceptance)

    phases = []
    phase_matches = _phase_heading_matches(body)
    seen = set()
    for index, match in enumerate(phase_matches):
        name = match["name"].strip()
        selector = match["selector"].strip()
        if not PHASE_SELECTOR_RE.match(selector):
            raise ScafldError("invalid Markdown spec", [f"invalid phase selector: {selector}"], code=ErrorCode.INVALID_SPEC_DOCUMENT)
        if selector in seen:
            raise ScafldError("invalid Markdown spec", [f"duplicate phase selector: {selector}"], code=ErrorCode.INVALID_SPEC_DOCUMENT)
        seen.add(selector)
        phase = {"id": selector, "name": name}
        next_heading = _find_next_heading_offset(body, start=match["end"], level=2)
        end = next_heading if next_heading is not None else len(body)
        section = body[match["end"]:end]
        phase.update(_parse_phase_body(section))
        phases.append(phase)
    if phases:
        data["phases"] = phases

    section_parsers = {
        "rollback": ("Rollback", _parse_rollback),
        "review": ("Review", _parse_review),
        "self_eval": ("Self Eval", _parse_self_eval),
        "deviations": ("Deviations", _parse_deviations),
        "metadata": ("Metadata", _parse_metadata),
        "origin": ("Origin", _parse_origin),
        "harden_rounds": ("Harden Rounds", _parse_harden_rounds),
    }
    for kind, (heading, parser) in section_parsers.items():
        section = _section_body(body, heading)
        if section:
            data[kind] = parser(section)
    planning = _section_body(body, "Planning Log")
    if planning:
        log = []
        for line in planning.splitlines():
            match = re.match(r"-\s+([0-9T:Z+-]+)\s+-\s+([^-]+?)\s+-\s+(.*)", line.strip())
            if match:
                log.append({"timestamp": match.group(1), "actor": match.group(2).strip(), "summary": match.group(3)})
        data["planning_log"] = log
    return data


def _render_list(items):
    return "\n".join(f"- {item}" for item in (items or [])) or "- None."


def _render_context(context):
    context = context if isinstance(context, dict) else {}
    lines = [f"CWD: `{context.get('cwd') or '.'}`", "", "Packages:"]
    packages = context.get("packages") if isinstance(context.get("packages"), list) else []
    lines.extend(f"- `{item}`" for item in packages) if packages else lines.append("- none")
    lines.extend(["", "Files impacted:"])
    files = context.get("files_impacted") if isinstance(context.get("files_impacted"), list) else []
    if files:
        for item in files:
            if not isinstance(item, dict):
                continue
            bits = [item.get("lines") or "all"]
            if item.get("ownership"):
                bits.append(item.get("ownership"))
            reason = str(item.get("reason") or item.get("content_spec") or "").replace("\n", " ").strip()
            lines.append(f"- `{item.get('path')}` ({', '.join(bits)}) - {reason}".rstrip())
    else:
        lines.append("- none")
    lines.extend(["", "Invariants:"])
    invariants = context.get("invariants") if isinstance(context.get("invariants"), list) else []
    lines.extend(f"- `{item}`" for item in invariants) if invariants else lines.append("- none")
    lines.extend(["", "Related docs:"])
    docs = context.get("related_docs") if isinstance(context.get("related_docs"), list) else []
    lines.extend(f"- `{item}`" for item in docs) if docs else lines.append("- none")
    return "\n".join(lines)


def _checkbox(status):
    return "x" if status in ("passed", "pass", "completed", "done") else " "


def _render_criterion(item):
    item = item if isinstance(item, dict) else {}
    prefix = f"- [{_checkbox(item.get('status') or item.get('result'))}] `{item.get('id')}`"
    item_type = item.get("type")
    description = item.get("description") or ""
    if item_type:
        prefix += f" {item_type}"
    if description:
        prefix += f" - {description}"
    lines = [prefix]
    for label, key in (
        ("Command", "command"),
        ("Expected kind", "expected_kind"),
        ("Timeout seconds", "timeout_seconds"),
        ("Result", "result"),
        ("Status", "status"),
        ("Evidence", "evidence"),
        ("Source event", "source_event"),
        ("Last attempt", "last_attempt"),
        ("Checked at", "checked_at"),
    ):
        value = item.get(key)
        if key in ("command", "expected_kind") and value:
            rendered = f"`{value}`"
        else:
            rendered = str(value) if value not in (None, "") else "none"
        lines.append(f"  - {label}: {rendered}")
    return "\n".join(lines)


def _render_acceptance(acceptance):
    acceptance = acceptance if isinstance(acceptance, dict) else {}
    lines = [f"Profile: {acceptance.get('validation_profile') or 'strict'}", "", "Definition of done:"]
    dod = acceptance.get("definition_of_done") if isinstance(acceptance.get("definition_of_done"), list) else []
    lines.extend(f"- [{_checkbox(item.get('status'))}] `{item.get('id')}` {item.get('description') or ''}".rstrip() for item in dod) if dod else lines.append("- none")
    lines.extend(["", "Validation:"])
    validation = acceptance.get("validation") if isinstance(acceptance.get("validation"), list) else []
    lines.extend(_render_criterion(item) for item in validation) if validation else lines.append("- none")
    return "\n".join(lines)


def _render_phase_changes(changes):
    lines = []
    for item in changes or []:
        if not isinstance(item, dict):
            continue
        bits = [item.get("lines") or "all"]
        if item.get("ownership"):
            bits.append(item.get("ownership"))
        description = str(item.get("content_spec") or item.get("reason") or item.get("action") or "").replace("\n", " ").strip()
        lines.append(f"- `{item.get('file')}` ({', '.join(bits)}) - {description}".rstrip())
    return "\n".join(lines) if lines else "- none"


def _render_planning_log(entries):
    lines = []
    for item in entries or []:
        if isinstance(item, dict):
            lines.append(f"- {item.get('timestamp')} - {item.get('actor') or 'agent'} - {item.get('summary') or ''}".rstrip())
    return "\n".join(lines) if lines else "- none"


def _render_scalar(value):
    if value in (None, "", [], {}):
        return "none"
    if isinstance(value, bool):
        return "true" if value else "false"
    return str(value)


def _render_inline_code(value):
    value = _render_scalar(value)
    return value if value == "none" else f"`{value}`"


def _render_simple_list(items):
    values = [item for item in (items or []) if item not in (None, "")]
    return "\n".join(f"- {item}" for item in values) if values else "- none"


def _render_rollback(rollback):
    rollback = rollback if isinstance(rollback, dict) else {}
    lines = [f"Strategy: {_render_scalar(rollback.get('strategy') or 'per_phase')}", "", "Commands:"]
    commands = rollback.get("commands") if isinstance(rollback.get("commands"), dict) else {}
    if commands:
        for phase_id, command in commands.items():
            lines.append(f"- `{phase_id}`: {_render_inline_code(command)}")
    else:
        lines.append("- none")
    return "\n".join(lines)


def _render_findings(findings):
    if not findings:
        return "- none"
    lines = []
    for item in findings:
        if isinstance(item, dict):
            summary = item.get("summary") or item.get("title") or item.get("id") or "finding"
            lines.append(f"- {summary}")
            for label, key in (("Severity", "severity"), ("Blocking", "blocking"), ("File", "file"), ("Line", "line"), ("Evidence", "evidence")):
                if item.get(key) not in (None, "", [], {}):
                    lines.append(f"  - {label}: {_render_scalar(item.get(key))}")
        elif item not in (None, ""):
            lines.append(f"- {item}")
    return "\n".join(lines) if lines else "- none"


def _render_review(review):
    review = review if isinstance(review, dict) else {}
    lines = [
        f"Status: {_render_scalar(review.get('status') or 'not_started')}",
        f"Verdict: {_render_scalar(review.get('verdict'))}",
        f"Timestamp: {_render_scalar(review.get('timestamp'))}",
        f"Review rounds: {_render_scalar(review.get('review_rounds'))}",
        f"Reviewer mode: {_render_scalar(review.get('reviewer_mode'))}",
        f"Reviewer session: {_render_scalar(review.get('reviewer_session'))}",
        f"Round status: {_render_scalar(review.get('round_status'))}",
        f"Override applied: {_render_scalar(review.get('override_applied'))}",
        f"Override reason: {_render_scalar(review.get('override_reason'))}",
        f"Override confirmed at: {_render_scalar(review.get('override_confirmed_at'))}",
        f"Reviewed head: {_render_scalar(review.get('reviewed_head'))}",
        f"Reviewed dirty: {_render_scalar(review.get('reviewed_dirty'))}",
        f"Reviewed diff: {_render_scalar(review.get('reviewed_diff'))}",
        f"Blocking count: {_render_scalar(review.get('blocking_count'))}",
        f"Non-blocking count: {_render_scalar(review.get('non_blocking_count'))}",
        "",
        "Findings:",
        _render_findings(review.get("findings")),
    ]
    passes = review.get("passes") if isinstance(review.get("passes"), list) else []
    lines.extend(["", "Passes:"])
    if passes:
        for item in passes:
            if isinstance(item, dict):
                lines.append(f"- `{item.get('id')}`: {_render_scalar(item.get('result'))}")
    else:
        lines.append("- none")
    return "\n".join(lines)


def _render_self_eval(self_eval):
    self_eval = self_eval if isinstance(self_eval, dict) else {}
    lines = [
        f"Status: {_render_scalar(self_eval.get('status') or 'not_started')}",
        f"Completeness: {_render_scalar(self_eval.get('completeness'))}",
        f"Architecture fidelity: {_render_scalar(self_eval.get('architecture_fidelity'))}",
        f"Spec alignment: {_render_scalar(self_eval.get('spec_alignment'))}",
        f"Validation depth: {_render_scalar(self_eval.get('validation_depth'))}",
        f"Total: {_render_scalar(self_eval.get('total'))}",
        f"Second pass performed: {_render_scalar(self_eval.get('second_pass_performed'))}",
        "",
        "Notes:",
        _render_scalar(self_eval.get("notes")),
        "",
        "Improvements:",
        _render_simple_list(self_eval.get("improvements")),
    ]
    return "\n".join(lines)


def _render_deviations(deviations):
    lines = []
    for item in deviations or []:
        if not isinstance(item, dict):
            continue
        lines.append(f"- `{item.get('rule')}` - {item.get('reason') or ''}".rstrip())
        for label, key in (("Mitigation", "mitigation"), ("Approved by", "approved_by")):
            if item.get(key):
                lines.append(f"  - {label}: {item.get(key)}")
    return "\n".join(lines) if lines else "- none"


def _render_metadata(metadata):
    metadata = metadata if isinstance(metadata, dict) else {}
    return "\n".join([
        f"Estimated effort hours: {_render_scalar(metadata.get('estimated_effort_hours'))}",
        f"Actual effort hours: {_render_scalar(metadata.get('actual_effort_hours'))}",
        f"AI model: {_render_scalar(metadata.get('ai_model'))}",
        f"React cycles: {_render_scalar(metadata.get('react_cycles'))}",
        "",
        "Tags:",
        _render_simple_list(metadata.get("tags")),
    ])


def _render_key_bullets(data, keys):
    data = data if isinstance(data, dict) else {}
    if not data:
        return "- none"
    lines = []
    for label, key in keys:
        if key in data:
            lines.append(f"- {label}: {_render_scalar(data.get(key))}")
    return "\n".join(lines) if lines else "- none"


def _render_origin(origin):
    origin = origin if isinstance(origin, dict) else {}
    source_keys = (("System", "system"), ("Kind", "kind"), ("ID", "id"), ("URL", "url"), ("Title", "title"))
    repo_keys = (("Root", "root"), ("Remote", "remote"), ("Remote URL", "remote_url"))
    git_keys = (("Branch", "branch"), ("Base ref", "base_ref"), ("Upstream", "upstream"), ("Mode", "mode"), ("Bound at", "bound_at"))
    sync = origin.get("sync") if isinstance(origin.get("sync"), dict) else {}
    actual = sync.get("actual") if isinstance(sync.get("actual"), dict) else {}
    sync_lines = _render_key_bullets(sync, (("Status", "status"), ("Last checked at", "last_checked_at"))).splitlines()
    reasons = sync.get("reasons") if isinstance(sync.get("reasons"), list) else []
    sync_lines.extend(["- Reasons:"])
    sync_lines.extend(f"  - {reason}" for reason in reasons) if reasons else sync_lines.append("  - none")
    for label, key in (
        ("Actual branch", "branch"),
        ("Actual head sha", "head_sha"),
        ("Actual upstream", "upstream"),
        ("Actual remote", "remote"),
        ("Actual remote URL", "remote_url"),
        ("Actual default base ref", "default_base_ref"),
        ("Actual dirty", "dirty"),
        ("Actual detached", "detached"),
    ):
        if key in actual:
            sync_lines.append(f"- {label}: {_render_scalar(actual.get(key))}")
    return "\n".join([
        "Source:",
        _render_key_bullets(origin.get("source"), source_keys),
        "",
        "Repo:",
        _render_key_bullets(origin.get("repo"), repo_keys),
        "",
        "Git:",
        _render_key_bullets(origin.get("git"), git_keys),
        "",
        "Sync:",
        "\n".join(sync_lines) if sync else "- none",
        "",
        "Supersession:",
        _render_key_bullets(origin.get("supersession"), (("Superseded by", "superseded_by"), ("Superseded at", "superseded_at"), ("Reason", "reason"))),
    ])


def _render_harden_rounds(rounds):
    lines = []
    for item in rounds or []:
        if not isinstance(item, dict):
            continue
        lines.append(f"- Round {item.get('round')}")
        lines.append(f"  - Started at: {_render_scalar(item.get('started_at'))}")
        lines.append(f"  - Ended at: {_render_scalar(item.get('ended_at'))}")
        lines.append(f"  - Outcome: {_render_scalar(item.get('outcome'))}")
        lines.append("  - Questions:")
        questions = item.get("questions") if isinstance(item.get("questions"), list) else []
        if questions:
            for question in questions:
                if not isinstance(question, dict):
                    continue
                lines.append(f"    - Question: {_render_scalar(question.get('question'))}")
                for label, key in (
                    ("Grounded in", "grounded_in"),
                    ("Recommended answer", "recommended_answer"),
                    ("If unanswered", "if_unanswered"),
                    ("Answered with", "answered_with"),
                ):
                    lines.append(f"      - {label}: {_render_scalar(question.get(key))}")
        else:
            lines.append("    - none")
    return "\n".join(lines) if lines else "- none"


def _front_matter_payload(data):
    task = data.get("task") if isinstance(data.get("task"), dict) else {}
    return {
        "spec_version": "2.0",
        "task_id": data.get("task_id"),
        "created": data.get("created"),
        "updated": data.get("updated"),
        "status": data.get("status") or "draft",
        "harden_status": data.get("harden_status") or "not_run",
        "size": task.get("size") or data.get("size") or "small",
        "risk_level": task.get("risk_level") or data.get("risk_level") or "low",
    }


def _render_current_state(data):
    current = data.get("current_state") if isinstance(data.get("current_state"), dict) else {}
    return "\n".join([
        f"Status: {data.get('status') or current.get('status') or 'draft'}",
        f"Current phase: {current.get('current_phase') or 'none'}",
        f"Next: {current.get('next') or 'none'}",
        f"Reason: {current.get('reason') or 'none'}",
        f"Blockers: {current.get('blockers') or 'none'}",
        f"Allowed follow-up command: {current.get('allowed_follow_up_command') or 'none'}",
        f"Latest runner update: {current.get('latest_runner_update') or 'none'}",
        f"Review gate: {current.get('review_gate') or 'not_started'}",
    ])


def _section_payloads(data):
    task = data.get("task") if isinstance(data.get("task"), dict) else {}
    payloads = {
        "current_state": _render_current_state(data),
        "context": _render_context(task.get("context")),
        "acceptance": _render_acceptance(task.get("acceptance")),
        "rollback": _render_rollback(data.get("rollback")),
        "review": _render_review(data.get("review")),
        "self_eval": _render_self_eval(data.get("self_eval")),
        "deviations": _render_deviations(data.get("deviations")),
        "metadata": _render_metadata(data.get("metadata")),
        "origin": _render_origin(data.get("origin")),
        "harden_rounds": _render_harden_rounds(data.get("harden_rounds")),
        "planning_log": _render_planning_log(data.get("planning_log")),
    }
    return payloads


def _render_phase_number(index, phase):
    phase_id = phase.get("id") if isinstance(phase, dict) else None
    match = re.match(r"^phase([1-9][0-9]*)$", str(phase_id or ""))
    return int(match.group(1)) if match else index


def _render_phase_block(index, phase):
    phase = phase if isinstance(phase, dict) else {}
    number = _render_phase_number(index, phase)
    name = phase.get("name") or f"Phase {number}"
    dependencies = ", ".join(phase.get("dependencies") or []) if phase.get("dependencies") else "none"
    criteria = []
    if isinstance(phase.get("acceptance_criteria"), list):
        criteria.extend(phase.get("acceptance_criteria"))
    if isinstance(phase.get("validation"), list):
        criteria.extend(phase.get("validation"))
    acceptance = "\n".join(_render_criterion(item) for item in criteria) if criteria else "- none"
    return "\n".join([
        f"## Phase {number}: {name}",
        "",
        f"Goal: {phase.get('objective') or 'none'}",
        "",
        f"Status: {phase.get('status') or 'pending'}",
        f"Dependencies: {dependencies}",
        "",
        "Changes:",
        _render_phase_changes(phase.get("changes")),
        "",
        "Acceptance:",
        acceptance,
        "",
    ])


def render_spec_markdown(data):
    task = data.get("task") if isinstance(data.get("task"), dict) else {}
    front = _front_matter_payload(data)
    payloads = _section_payloads(data)
    lines = ["---", _yaml_dump(front), "---", "", f"# {task.get('title') or data.get('task_id')}", ""]
    lines.extend([
        "## Current State",
        "",
        payloads["current_state"],
        "",
        "## Summary",
        "",
        task.get("summary") or "",
        "",
        "## Context",
        "",
        payloads["context"],
        "",
        "## Objectives",
        "",
        _render_list(task.get("objectives")),
        "",
        "## Scope",
        "",
    ])
    scope = task.get("scope")
    lines.append(scope.get("summary", "") if isinstance(scope, dict) else (scope or ""))
    lines.extend(["", "## Dependencies", "", _render_list(task.get("dependencies")), "", "## Assumptions", "", _render_list(task.get("assumptions")), "", "## Touchpoints", "", _render_list([f"{item.get('area')}: {item.get('description')}" for item in task.get("touchpoints", []) if isinstance(item, dict)]), "", "## Risks", "", _render_list([item.get("description") if isinstance(item, dict) else item for item in (task.get("risks") or [])]), "", "## Acceptance", "", payloads["acceptance"], ""])
    for index, phase in enumerate(data.get("phases") or [], start=1):
        lines.append(_render_phase_block(index, phase).rstrip("\n"))
        lines.append("")
    lines.extend([
        "## Rollback",
        "",
        payloads["rollback"],
        "",
        "## Review",
        "",
        payloads["review"],
        "",
        "## Self Eval",
        "",
        payloads["self_eval"],
        "",
        "## Deviations",
        "",
        payloads["deviations"],
        "",
        "## Metadata",
        "",
        payloads["metadata"],
        "",
        "## Origin",
        "",
        payloads["origin"],
        "",
        "## Harden Rounds",
        "",
        payloads["harden_rounds"],
        "",
        "## Planning Log",
        "",
        payloads["planning_log"],
        "",
    ])
    return "\n".join(lines)


def replace_front_matter(text, data):
    match = FRONT_MATTER_RE.match(text)
    if not match:
        raise ScafldError("invalid Markdown spec", ["missing YAML front matter"], code=ErrorCode.INVALID_SPEC_DOCUMENT)
    return "---\n" + _yaml_dump(_front_matter_payload(data)) + "\n---\n" + text[match.end():]


def _iter_line_records(text):
    offset = 0
    for raw in text.splitlines(keepends=True):
        content = raw.rstrip("\n\r")
        yield offset, content, offset + len(raw)
        offset += len(raw)


def _heading_range(text, heading):
    pattern = re.compile(rf"^##\s+{re.escape(heading)}\s*$")
    matches = []
    active_fence = None
    for offset, line, line_end in _iter_line_records(text):
        active_fence = _fence_state(line, active_fence)
        if active_fence:
            continue
        if pattern.match(line):
            next_offset = _find_next_heading_offset(text, start=line_end)
            matches.append((offset, line_end, next_offset if next_offset is not None else len(text)))
    if not matches:
        raise ScafldError("invalid Markdown spec", [f"missing section: ## {heading}"], code=ErrorCode.INVALID_SPEC_DOCUMENT)
    if len(matches) > 1:
        raise ScafldError("invalid Markdown spec", [f"duplicate section: ## {heading}"], code=ErrorCode.INVALID_SPEC_DOCUMENT)
    return matches[0]


def _replace_heading_section(text, heading, payload):
    start, _line_end, end = _heading_range(text, heading)
    replacement = f"## {heading}\n\n{str(payload or '').strip()}\n\n"
    return text[:start] + replacement + text[end:].lstrip("\n")


def _replace_h1(text, title):
    active_fence = None
    for offset, line, line_end in _iter_line_records(text):
        active_fence = _fence_state(line, active_fence)
        if active_fence:
            continue
        if re.match(r"^#\s+", line):
            return text[:offset] + f"# {title}\n" + text[line_end:]
    raise ScafldError("invalid Markdown spec", ["missing document title"], code=ErrorCode.INVALID_SPEC_DOCUMENT)


def _replace_phase_sections(text, phases):
    matches = _phase_heading_matches(text)
    phase_ids = [f"phase{match['number']}" for match in matches]
    wanted = [phase.get("id") for phase in phases or [] if isinstance(phase, dict)]
    if phase_ids != wanted:
        return None
    replacements = []
    for index, (match, phase) in enumerate(zip(matches, phases or []), start=1):
        next_offset = _find_next_heading_offset(text, start=match["end"], level=2)
        end = next_offset if next_offset is not None else len(text)
        replacements.append((match["start"], end, _render_phase_block(index, phase).rstrip("\n") + "\n\n"))
    for start, end, replacement in reversed(replacements):
        text = text[:start] + replacement + text[end:].lstrip("\n")
    return text


def _values_equal(left, right):
    return prune_for_compare(left) == prune_for_compare(right)


def prune_for_compare(value):
    if isinstance(value, dict):
        return {key: prune_for_compare(item) for key, item in value.items() if item not in (None, "", [], {})}
    if isinstance(value, list):
        return [prune_for_compare(item) for item in value if item not in (None, "", [], {})]
    return value


def update_spec_markdown(text, data):
    current = parse_spec_markdown(text)
    rendered = render_spec_markdown(data)
    rendered_front, rendered_body_start = parse_front_matter(rendered)
    del rendered_front
    rendered_body = rendered[rendered_body_start:]
    updated = replace_front_matter(text, data)
    task = data.get("task") if isinstance(data.get("task"), dict) else {}
    current_task = current.get("task") if isinstance(current.get("task"), dict) else {}

    if task.get("title") != current_task.get("title"):
        updated = _replace_h1(updated, task.get("title") or data.get("task_id"))

    human_sections = {
        "Summary": (current_task.get("summary"), task.get("summary")),
        "Objectives": (current_task.get("objectives"), task.get("objectives")),
        "Scope": (
            current_task.get("scope"),
            task.get("scope"),
        ),
        "Dependencies": (current_task.get("dependencies"), task.get("dependencies")),
        "Assumptions": (current_task.get("assumptions"), task.get("assumptions")),
        "Touchpoints": (current_task.get("touchpoints"), task.get("touchpoints")),
        "Risks": (current_task.get("risks"), task.get("risks")),
    }
    runner_sections = [
        "Current State",
        "Context",
        "Acceptance",
        "Rollback",
        "Review",
        "Self Eval",
        "Deviations",
        "Metadata",
        "Origin",
        "Harden Rounds",
        "Planning Log",
    ]
    for heading in runner_sections:
        updated = _replace_heading_section(updated, heading, _section_body(rendered_body, heading))
    for heading, (before, after) in human_sections.items():
        if not _values_equal(before, after):
            updated = _replace_heading_section(updated, heading, _section_body(rendered_body, heading))

    phase_updated = _replace_phase_sections(updated, data.get("phases") or [])
    if phase_updated is None:
        disk_ids = [
            f"phase{match['number']}"
            for match in _phase_heading_matches(updated)
        ]
        model_ids = [
            phase.get("id")
            for phase in data.get("phases") or []
            if isinstance(phase, dict)
        ]
        raise ScafldError(
            "invalid Markdown spec",
            [f"phase headings do not match model phases: on-disk={disk_ids}, model={model_ids}"],
            code=ErrorCode.INVALID_SPEC_DOCUMENT,
        )
    return phase_updated
