import json

from scafld.errors import ScafldError
from scafld.error_codes import ErrorCode
from scafld.runtime_bundle import resolve_prompt_path
from scafld.runtime_contracts import (
    HANDOFF_SCHEMA_VERSION,
    archive_month_for_spec,
    ensure_run_dirs,
    expected_session_ref,
    handoff_json_path,
    handoff_path,
    load_llm_settings,
    normalize_handoff_identity,
    relative_path,
    session_ref,
)
from scafld.session_store import prior_phase_summary, session_summary_payload
from scafld.spec_parsing import now_iso
from scafld.spec_store import load_spec_document


def handoff_template_name(role, gate):
    return {
        ("executor", "phase"): "exec.md",
        ("executor", "recovery"): "recovery.md",
        ("challenger", "review"): "review.md",
        ("human", "harden"): "harden.md",
    }.get((role, gate), "exec.md")


def criterion_result_value(criterion):
    result = criterion.get("result")
    if isinstance(result, dict):
        return result.get("status")
    return result


def phase_definitions(spec_data):
    phases = spec_data.get("phases")
    return phases if isinstance(phases, list) else []


def find_phase(spec_data, phase_id):
    for phase in phase_definitions(spec_data):
        if phase.get("id") == phase_id:
            return phase
    return None


def criteria_for_phase(phase):
    criteria = phase.get("acceptance_criteria")
    return criteria if isinstance(criteria, list) else []


def ordered_phase_ids(spec_data):
    return [phase.get("id") for phase in phase_definitions(spec_data) if phase.get("id")]


def current_phase_id(spec_data):
    phases = phase_definitions(spec_data)
    if not phases:
        return None
    for phase in phases:
        criteria = criteria_for_phase(phase)
        if not criteria:
            return phase.get("id")
        if any(criterion_result_value(criterion) != "pass" for criterion in criteria):
            return phase.get("id")
    return phases[0].get("id")


def phase_change_lines(phase):
    changes = phase.get("changes") if isinstance(phase.get("changes"), list) else []
    lines = []
    for change in changes:
        if not isinstance(change, dict):
            continue
        path = change.get("file") or "(unknown)"
        action = change.get("action") or "update"
        content = str(change.get("content_spec") or "").strip()
        line = f"- `{path}` ({action})"
        if content:
            line += f": {content.splitlines()[0]}"
        lines.append(line)
    return lines or ["- No declared file changes in this phase."]


def phase_criteria_lines(phase):
    lines = []
    for criterion in criteria_for_phase(phase):
        if not isinstance(criterion, dict):
            continue
        ac_id = criterion.get("id") or criterion.get("dod_id") or "criterion"
        description = criterion.get("description") or ac_id
        command = criterion.get("command")
        expected = criterion.get("expected")
        result = criterion_result_value(criterion) or "pending"
        line = f"- `{ac_id}`: {description} [{result}]"
        if command:
            line += f"\n  command: `{command}`"
        if expected:
            line += f"\n  expected: `{expected}`"
        lines.append(line)
    return lines or ["- No acceptance criteria declared for this phase."]


def task_context_lines(task_block):
    context = task_block.get("context") if isinstance(task_block.get("context"), dict) else {}
    lines = []
    packages = context.get("packages") if isinstance(context.get("packages"), list) else []
    if packages:
        lines.append("Packages:")
        lines.extend(f"- `{package}`" for package in packages[:8])
    invariants = context.get("invariants") if isinstance(context.get("invariants"), list) else []
    if invariants:
        lines.append("Invariants:")
        lines.extend(f"- `{item}`" for item in invariants)
    files_impacted = context.get("files_impacted") if isinstance(context.get("files_impacted"), list) else []
    if files_impacted:
        lines.append("Grounded files:")
        for entry in files_impacted[:8]:
            if not isinstance(entry, dict):
                continue
            path = entry.get("path") or "(unknown)"
            reason = entry.get("reason") or ""
            line = f"- `{path}`"
            if reason:
                line += f": {reason}"
            lines.append(line)
    return lines or ["- No additional grounded context declared in the spec."]


def session_summary_lines(session):
    if not session:
        return ["- Session not initialized yet."]
    summary = session_summary_payload(session)
    lines = [
        f"- Attempts recorded: {summary['attempt_count']}",
        f"- Ledger entries: {summary['entry_count']}",
        f"- Phase summaries: {summary['phase_summaries']}",
    ]
    if summary["first_attempt_total"]:
        lines.append(
            f"- First-attempt pass: {summary['first_attempt_passed']}/{summary['first_attempt_total']}"
        )
    if summary["recovered_total"]:
        lines.append(
            f"- Recovery convergence: {summary['recovered_pass']}/{summary['recovered_total']}"
        )
    if summary["failed_exhausted"]:
        lines.append(f"- Recovery exhausted: {summary['failed_exhausted']}")
    if summary["challenge_verdicts"]:
        lines.append(f"- Review challenges recorded: {summary['challenge_verdicts']}")
    if summary["challenge_blocked"]:
        lines.append(
            f"- Challenge overrides: {summary['challenge_overrides']}/{summary['challenge_blocked']}"
        )
    if summary["attempts_per_phase"]:
        rendered = ", ".join(
            f"{phase_id}={count}" for phase_id, count in summary["attempts_per_phase"].items()
        )
        lines.append(f"- Attempts per phase: {rendered}")
    return lines


def format_section(title, lines):
    return {
        "title": title,
        "lines": list(lines or ["- None."]),
    }


def render_section_markdown(section):
    body = "\n".join(section.get("lines") or ["- None."])
    return f"## {section.get('title')}\n{body}"


def load_template(root, role, gate):
    name = handoff_template_name(role, gate)
    path = resolve_prompt_path(root, name)
    if not path.exists():
        raise ScafldError(
            f"handoff template missing at {path}",
            code=ErrorCode.PROMPT_MISSING,
        )
    return path, path.read_text(encoding="utf-8")


def front_matter(role, gate, task_id, selector, generated_at, model_profile, template_rel, session_rel):
    lines = [
        "---",
        f"schema_version: {HANDOFF_SCHEMA_VERSION}",
        f"role: {json.dumps(role)}",
        f"gate: {json.dumps(gate)}",
        f"task_id: {json.dumps(task_id)}",
        f"selector: {json.dumps(selector)}",
        f"generated_at: {json.dumps(generated_at)}",
        f"model_profile: {json.dumps(model_profile)}",
        f"template: {json.dumps(template_rel)}",
        f"session_ref: {json.dumps(session_rel)}",
        "---",
        "",
    ]
    return "\n".join(lines)


def render_phase_sections(root, task_id, spec_path, spec_data, session, selector, llm_settings):
    task_block = spec_data.get("task") if isinstance(spec_data.get("task"), dict) else {}
    phase = find_phase(spec_data, selector)
    if phase is None:
        raise ScafldError(f"phase not found: {selector}", code=ErrorCode.INVALID_ARGUMENTS)

    prior_summary = prior_phase_summary(session or {}, ordered_phase_ids(spec_data), selector)
    return [
        format_section(
            "Task Contract",
            [
                f"- Task: `{task_id}`",
                f"- Spec: `{relative_path(root, spec_path)}`",
                f"- Title: {task_block.get('title') or task_id}",
                f"- Summary: {task_block.get('summary') or '(none)'}",
                f"- Model profile: `{llm_settings['model_profile']}`",
                f"- Context budget: {llm_settings['context_budget_tokens']} tokens",
            ],
        ),
        format_section(
            "Curated Context",
            task_context_lines(task_block),
        ),
        format_section(
            "Phase Objective",
            [
                f"- Phase: `{selector}`",
                f"- Name: {phase.get('name') or selector}",
                f"- Objective: {phase.get('objective') or '(none)'}",
            ],
        ),
        format_section(
            "Declared Changes",
            phase_change_lines(phase),
        ),
        format_section(
            "Acceptance Criteria",
            phase_criteria_lines(phase),
        ),
        format_section(
            "Prior Phase Summary",
            [f"- {prior_summary.get('summary')}"] if prior_summary else ["- No prior phase summary recorded."],
        ),
        format_section(
            "Session Snapshot",
            session_summary_lines(session),
        ),
    ]


def render_recovery_sections(root, task_id, spec_path, spec_data, session, selector, llm_settings, context):
    if session is None:
        raise ScafldError("recovery handoff requires an execution session", code=ErrorCode.INVALID_ARGUMENTS)
    failed_attempt = context.get("failed_attempt") or {}
    phase_id = failed_attempt.get("phase_id")
    phase = find_phase(spec_data, phase_id) if phase_id else None
    prior_summary = prior_phase_summary(session, ordered_phase_ids(spec_data), phase_id) if phase_id else None
    diagnostic_rel = context.get("diagnostic_rel")
    max_attempts = llm_settings["recovery_max_attempts"]
    return [
        format_section(
            "Task Contract",
            [
                f"- Task: `{task_id}`",
                f"- Spec: `{relative_path(root, spec_path)}`",
                f"- Recovery target: `{selector}`",
                f"- Model profile: `{llm_settings['model_profile']}`",
                f"- Recovery budget: {max_attempts} attempt(s)",
            ],
        ),
        format_section(
            "Failure Summary",
            [
                f"- Criterion: `{failed_attempt.get('criterion_id') or selector}`",
                f"- Phase: `{phase_id or '(unknown)'}`",
                f"- Command: `{failed_attempt.get('command') or ''}`",
                f"- Expected: `{failed_attempt.get('expected') or ''}`",
                f"- Exit code: {failed_attempt.get('exit_code')}",
                f"- Latest snippet: {failed_attempt.get('output_snippet') or '(none)'}",
                (
                    f"- Diagnostics: `{diagnostic_rel}`"
                    if diagnostic_rel
                    else "- Diagnostics: not available"
                ),
            ],
        ),
        format_section(
            "Current Phase Slice",
            [
                f"- Name: {phase.get('name') or phase_id}" if phase else "- Phase metadata unavailable.",
                f"- Objective: {phase.get('objective') or '(none)'}" if phase else "- Objective unavailable.",
                *phase_change_lines(phase or {}),
                *phase_criteria_lines(phase or {}),
            ],
        ),
        format_section(
            "Prior Attempts",
            [
                (
                    f"- Attempt {attempt.get('criterion_attempt')}: {attempt.get('status')} "
                    f"(exit={attempt.get('exit_code')})"
                )
                for attempt in context.get("criterion_attempts", [])
            ] or ["- No prior attempts recorded."],
        ),
        format_section(
            "Prior Phase Summary",
            [f"- {prior_summary.get('summary')}"] if prior_summary else ["- No prior phase summary recorded."],
        ),
        format_section(
            "Session Snapshot",
            session_summary_lines(session),
        ),
    ]


def render_review_sections(root, task_id, spec_path, spec_data, session, selector, llm_settings, context):
    task_block = spec_data.get("task") if isinstance(spec_data.get("task"), dict) else {}
    automated_results = context.get("automated_results") or []
    required_sections = context.get("required_sections") or []
    changed_files = []
    for phase in phase_definitions(spec_data):
        changed_files.extend(phase_change_lines(phase))
    review_file_rel = context.get("review_file_rel") or ".ai/reviews/{task_id}.md"
    return [
        format_section(
            "Challenge Contract",
            [
                f"- Task: `{task_id}`",
                f"- Spec: `{relative_path(root, spec_path)}`",
                f"- Review file: `{review_file_rel}`",
                f"- Review round: {context.get('review_count')}",
                f"- Challenger isolation: `{context.get('reviewer_isolation') or 'fresh_context_handoff'}`",
                f"- Model profile: `{llm_settings['model_profile']}`",
            ],
        ),
        format_section(
            "Task Summary",
            [
                f"- Title: {task_block.get('title') or task_id}",
                f"- Summary: {task_block.get('summary') or '(none)'}",
                *task_context_lines(task_block),
            ],
        ),
        format_section(
            "Automated Review Results",
            [
                f"- `{entry.get('id')}`: {entry.get('result')}"
                for entry in automated_results
            ] or ["- No automated review results recorded."],
        ),
        format_section(
            "Changed Areas",
            changed_files,
        ),
        format_section(
            "Session Snapshot",
            session_summary_lines(session),
        ),
        format_section(
            "Required Review Sections",
            [f"- ### {section}" for section in required_sections] or ["- Review scaffolding determines the final required headings."],
        ),
    ]


def render_handoff(
    root,
    task_id,
    spec_path,
    *,
    role=None,
    gate=None,
    selector=None,
    session=None,
    context=None,
):
    spec_data = load_spec_document(spec_path)
    settings = load_llm_settings(root)
    role, gate = normalize_handoff_identity(role=role, gate=gate)
    template_path, template_text = load_template(root, role, gate)
    generated_at = now_iso()
    selector = selector or ("review" if gate == "review" else current_phase_id(spec_data) or "current")
    context = context or {}

    if gate == "phase":
        sections = render_phase_sections(root, task_id, spec_path, spec_data, session, selector, settings)
        attempt = None
    elif gate == "recovery":
        sections = render_recovery_sections(root, task_id, spec_path, spec_data, session, selector, settings, context)
        attempt = context.get("recovery_attempt")
    elif gate == "review":
        sections = render_review_sections(root, task_id, spec_path, spec_data, session, selector, settings, context)
        attempt = None
    else:
        sections = [
            format_section(
                "Task Contract",
                [f"- Task: `{task_id}`", f"- Spec: `{relative_path(root, spec_path)}`"],
            ),
        ]
        attempt = None

    template_rel = relative_path(root, template_path)
    archive_month = archive_month_for_spec(root, spec_path)
    session_rel = (
        session_ref(root, task_id, spec_path=spec_path)
        if session is not None
        else expected_session_ref(task_id, archive_month=archive_month)
    )
    handoff_file = handoff_path(
        root,
        task_id,
        role=role,
        gate=gate,
        selector=selector,
        attempt=attempt,
        spec_path=spec_path,
    )
    handoff_file_json = handoff_json_path(
        root,
        task_id,
        role=role,
        gate=gate,
        selector=selector,
        attempt=attempt,
        spec_path=spec_path,
    )
    ensure_run_dirs(root, task_id, spec_path=spec_path)
    section_markdown = [render_section_markdown(section) for section in sections]
    content = (
        front_matter(role, gate, task_id, selector, generated_at, settings["model_profile"], template_rel, session_rel)
        + template_text.rstrip()
        + "\n\n---\n\n"
        + "\n\n".join(section_markdown)
        + "\n"
    )
    handoff_file.write_text(content, encoding="utf-8")
    payload = {
        "schema_version": HANDOFF_SCHEMA_VERSION,
        "role": role,
        "gate": gate,
        "task_id": task_id,
        "selector": selector,
        "generated_at": generated_at,
        "model_profile": settings["model_profile"],
        "template": template_rel,
        "session_ref": session_rel,
        "markdown_file": relative_path(root, handoff_file),
        "sections": sections,
        "context": context,
    }
    handoff_file_json.write_text(json.dumps(payload, indent=2, sort_keys=False) + "\n", encoding="utf-8")
    return {
        "path": handoff_file,
        "path_rel": relative_path(root, handoff_file),
        "json_path": handoff_file_json,
        "json_path_rel": relative_path(root, handoff_file_json),
        "content": content,
        "payload": payload,
        "role": role,
        "gate": gate,
        "selector": selector,
        "generated_at": generated_at,
        "template": template_rel,
        "session_ref": session_rel,
    }
