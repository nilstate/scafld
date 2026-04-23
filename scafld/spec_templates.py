import json

from scafld.config import command_is_placeholder, detect_init_config, load_config


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

    template = f'''# Repo context: {detection["summary"]}
# Repo markers: {marker_text}
# Suggested validation commands come from repo detection (including mixed Python+Node repos)
# and .ai/config.local.yaml when available.
spec_version: "1.1"
task_id: {json.dumps(task_id)}
created: {json.dumps(timestamp)}
updated: {json.dumps(timestamp)}
status: "draft"
harden_status: "not_run"

task:
  title: {json.dumps(resolved_title)}
  summary: >
    {summary_prompt}
  size: {json.dumps(resolved_size)}
  risk_level: {json.dumps(resolved_risk)}
  context:
    packages:
      - {json.dumps(packages_prompt)}
    invariants:
      - "domain_boundaries"
  objectives:
    - {json.dumps(objective_prompt)}
  touchpoints:
    - area: {json.dumps(touchpoint_area)}
      description: {json.dumps(touchpoint_description)}
  acceptance:
    definition_of_done:
      - id: "dod1"
        description: "TODO: define what completion means for this task."
        status: "pending"
    validation:
      - id: "v1"
        type: "compile"
        description: {json.dumps(compile_description)}
        command: {json.dumps(commands["compile_check"])}
        expected: "exit code 0"
      - id: "v2"
        type: "test"
        description: {json.dumps(test_description)}
        command: {json.dumps(commands["targeted_tests"])}
        expected: "exit code 0"

planning_log:
  - timestamp: {json.dumps(timestamp)}
    actor: "user"
    summary: "Spec created via scafld new"

phases:
  - id: "phase1"
    name: {json.dumps(phase_name)}
    objective: {json.dumps(phase_objective)}
    changes:
      - file: "TODO"
        action: "update"
        content_spec: |
          {phase_content}
    acceptance_criteria:
      - id: "ac1_1"
        type: "test"
        description: {json.dumps(phase_acceptance)}
        command: {json.dumps(commands["targeted_tests"])}
        expected: "exit code 0"
    status: "pending"

rollback:
  strategy: "per_phase"
  commands:
    phase1: "git checkout HEAD -- TODO"
'''

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
