from scafld.config import command_is_placeholder, detect_init_config, load_config
from scafld.spec_markdown import render_spec_markdown


def config_string(config, *keys):
    """Return one trimmed string value from a nested config mapping."""
    value = config
    for key in keys:
        if not isinstance(value, dict):
            return None
        value = value.get(key)
    if isinstance(value, str) and value.strip():
        return value.strip()
    return None


def resolved_validation_commands(config, detection):
    """Return repo-aware validation commands with local overrides applied."""
    commands = dict(detection["commands"])
    for key, path in (
        ("compile_check", ("validation", "per_phase", "compile_check")),
        ("targeted_tests", ("validation", "per_phase", "targeted_tests")),
        ("full_test_suite", ("validation", "pre_commit", "full_test_suite")),
        ("linter_suite", ("validation", "pre_commit", "linter_suite")),
        ("typecheck", ("validation", "pre_commit", "typecheck")),
    ):
        override = config_string(config, *path)
        if override and not (command_is_placeholder(override) and not command_is_placeholder(commands[key])):
            commands[key] = override
    return commands


def build_new_spec_scaffold(
    root,
    task_id,
    *,
    timestamp,
    title=None,
    size=None,
    risk=None,
    framework_config_path,
    config_path,
    config_local_path,
):
    """Build the repo-aware draft spec scaffold and companion metadata."""
    detection = detect_init_config(root)
    marker_text = ", ".join(detection["markers"]) if detection["markers"] else "none"
    config = load_config(root, framework_config_path, config_path, config_local_path)
    commands = resolved_validation_commands(config, detection)

    resolved_title = title or task_id.replace("-", " ").title()
    resolved_size = size or "small"
    resolved_risk = risk or "low"
    summary_prompt = (
        f'TODO: Describe the problem or goal for "{resolved_title}". '
        f"Repo context: {detection['summary']}."
    )
    packages_prompt = f"TODO: replace with affected package(s) or paths. Repo markers: {marker_text}."
    objective_prompt = f'TODO: State the primary outcome for "{resolved_title}".'
    touchpoint_area = f'TODO: primary area touched by "{resolved_title}"'
    touchpoint_description = (
        f'Replace with the main package, module, or workflow surface affected by "{resolved_title}".'
    )
    compile_description = "Run the repo's suggested compile or build check for this task."
    test_description = "Run the repo's suggested targeted test command for this task."
    phase_name = f'TODO: first implementation slice for "{resolved_title}"'
    phase_objective = f'TODO: Describe the first concrete slice of "{resolved_title}".'
    phase_content = f'TODO: Describe the changes for the first slice of "{resolved_title}".'
    phase_acceptance = "Run the repo's suggested targeted test command for this phase."

    data = {
        "spec_version": "2.0",
        "task_id": task_id,
        "created": timestamp,
        "updated": timestamp,
        "status": "draft",
        "harden_status": "not_run",
        "task": {
            "title": resolved_title,
            "summary": (
                f"{summary_prompt}\n\n"
                f"Repo context: {detection['summary']}. Repo markers: {marker_text}. "
                "Suggested validation commands come from repo detection and .scafld/config.local.yaml."
            ),
            "size": resolved_size,
            "risk_level": resolved_risk,
            "context": {
                "cwd": ".",
                "packages": [packages_prompt],
                "files_impacted": [{"path": "TODO", "reason": phase_content}],
                "invariants": ["domain_boundaries"],
            },
            "objectives": [objective_prompt],
            "touchpoints": [{"area": touchpoint_area, "description": touchpoint_description}],
            "acceptance": {
                "validation_profile": "strict",
                "definition_of_done": [
                    {"id": "dod1", "description": "TODO: define what completion means for this task.", "status": "pending"},
                ],
                "validation": [
                    {
                        "id": "v1",
                        "type": "compile",
                        "description": compile_description,
                        "command": commands["compile_check"],
                        "expected_kind": "exit_code_zero",
                    },
                    {
                        "id": "v2",
                        "type": "test",
                        "description": test_description,
                        "command": commands["targeted_tests"],
                        "expected_kind": "exit_code_zero",
                    },
                ],
            },
        },
        "planning_log": [{"timestamp": timestamp, "actor": "user", "summary": "Spec created via scafld plan"}],
        "phases": [
            {
                "id": "phase1",
                "name": phase_name,
                "objective": phase_objective,
                "changes": [{"file": "TODO", "action": "update", "content_spec": phase_content}],
                "acceptance_criteria": [
                    {
                        "id": "ac1_1",
                        "type": "test",
                        "description": phase_acceptance,
                        "command": commands["targeted_tests"],
                        "expected_kind": "exit_code_zero",
                    }
                ],
                "status": "pending",
            }
        ],
        "rollback": {"strategy": "per_phase", "commands": {"phase1": "git checkout HEAD -- TODO"}},
    }
    template = render_spec_markdown(data)

    return {
        "text": template,
        "title": resolved_title,
        "size": resolved_size,
        "risk": resolved_risk,
        "repo_context": {
            "summary": detection["summary"],
            "markers": list(detection["markers"]),
            "commands": {
                "compile_check": commands["compile_check"],
                "targeted_tests": commands["targeted_tests"],
                "full_test_suite": commands["full_test_suite"],
                "linter_suite": commands["linter_suite"],
                "typecheck": commands["typecheck"],
            },
        },
    }


def build_slim_spec_scaffold(
    root,
    task_id,
    *,
    timestamp,
    title,
    command,
    files,
    size="small",
    risk_level="low",
):
    """Build a compact Markdown spec scaffold.

    Caller supplies real values for `title`, `command`, and `files`
    (a list). The result has no `TODO` placeholders: every required
    field is populated, so `validate_spec` passes immediately and
    `scafld approve` can advance without a manual fill round.

    The criterion always carries explicit `expected_kind:
    exit_code_zero` so `evaluate_acceptance_criterion`'s
    strict-unset-reject doesn't fire.

    The grammar still emits every standard section, including empty
    state/projection sections, so the document remains deterministic for
    runner updates. For complex multi-phase work, operators extend the
    Markdown spec by hand or use the verbose scaffold.
    """
    resolved_title = title or task_id.replace("-", " ").title()
    resolved_size = size or "small"
    resolved_risk = risk_level or "low"
    file_paths = [str(path).strip() for path in (files or []) if str(path).strip()]
    if not file_paths:
        # Slim plan must declare at least one file so audit_scope and
        # validate_spec stay meaningful.
        raise ValueError(
            "build_slim_spec_scaffold requires --files; the slim "
            "criterion must declare which files the work touches"
        )
    if not command or not str(command).strip():
        raise ValueError(
            "build_slim_spec_scaffold requires --command; the slim "
            "criterion must declare an executable verification command"
        )

    detection = detect_init_config(root)

    data = {
        "spec_version": "2.0",
        "task_id": task_id,
        "created": timestamp,
        "updated": timestamp,
        "status": "draft",
        "harden_status": "not_run",
        "task": {
            "title": resolved_title,
            "summary": resolved_title,
            "size": resolved_size,
            "risk_level": resolved_risk,
            "context": {
                "cwd": ".",
                "files_impacted": [{"path": path, "reason": "See task summary."} for path in file_paths],
            },
        },
        "planning_log": [{"timestamp": timestamp, "actor": "user", "summary": "Spec created via scafld plan --command"}],
        "phases": [
            {
                "id": "phase1",
                "name": resolved_title,
                "objective": f"Implement {resolved_title}",
                "changes": [{"file": path, "action": "update", "content_spec": "See task summary."} for path in file_paths],
                "acceptance_criteria": [
                    {
                        "id": "ac1_1",
                        "type": "test",
                        "command": str(command),
                        "expected_kind": "exit_code_zero",
                    }
                ],
                "status": "pending",
            }
        ],
    }
    template = render_spec_markdown(data)

    return {
        "text": template,
        "title": resolved_title,
        "size": resolved_size,
        "risk": resolved_risk,
        "repo_context": {
            "summary": detection["summary"],
            "markers": list(detection["markers"]),
            "commands": {},
        },
    }
