import json

from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError


DEFAULT_INIT_COMMANDS = {
    "compile_check": "echo 'Replace: your build command'",
    "targeted_tests": "echo 'Replace: your test command'",
    "full_test_suite": "echo 'Replace: your full test suite'",
    "linter_suite": "echo 'Replace: your linter'",
    "typecheck": "echo 'Replace: your typecheck'",
}
INIT_COMMAND_KEYS = tuple(DEFAULT_INIT_COMMANDS.keys())


def parse_yaml_value(raw):
    """Parse a YAML scalar value, handling quotes and escapes."""
    value = raw.strip()
    if not value:
        return ""
    if value.startswith('"') and value.endswith('"'):
        return value[1:-1].replace('\\"', '"')
    if value.startswith("'") and value.endswith("'"):
        return value[1:-1].replace("''", "'")
    return value


def deep_merge(base, overlay):
    """Deep merge overlay dict into base dict. Overlay wins on conflicts."""
    result = dict(base)
    for key, value in overlay.items():
        if key in result and isinstance(result[key], dict) and isinstance(value, dict):
            result[key] = deep_merge(result[key], value)
        else:
            result[key] = value
    return result


def load_config(root, framework_config_path, config_path, config_local_path):
    """Load config with local overlay support.

    YAML parsing is only needed when a config file is present. A malformed
    config file is a control-plane error: silently ignoring it can make
    review/build commands run with defaults instead of the operator's intended
    overrides.
    """
    config = {}
    candidates = [root / framework_config_path, root / config_path, root / config_local_path]
    existing_candidates = [candidate for candidate in candidates if candidate.exists()]
    if not existing_candidates:
        return config

    try:
        import yaml
    except ImportError as exc:
        raise ScafldError(
            "PyYAML is required to read scafld config files",
            [f"install package dependencies before reading {existing_candidates[0]}"],
            code=EC.MISSING_DEPENDENCY,
        ) from exc

    for candidate in existing_candidates:
        try:
            loaded = yaml.safe_load(candidate.read_text())
        except yaml.YAMLError as exc:
            raise ScafldError(
                "invalid scafld config YAML",
                [f"{candidate}: {exc}"],
                code=EC.INVALID_ARGUMENTS,
            ) from exc
        if isinstance(loaded, dict):
            config = deep_merge(config, loaded)
    return config


def default_init_detection():
    return {
        "summary": "no known Node or Python repo markers found",
        "markers": [],
        "commands": DEFAULT_INIT_COMMANDS.copy(),
    }


def node_package_manager(project_root, package_data):
    raw = package_data.get("packageManager")
    if isinstance(raw, str) and raw.strip():
        return raw.split("@", 1)[0]
    if (project_root / "pnpm-lock.yaml").exists():
        return "pnpm"
    if (project_root / "yarn.lock").exists():
        return "yarn"
    if (project_root / "bun.lock").exists() or (project_root / "bun.lockb").exists():
        return "bun"
    return "npm"


def node_script_command(package_manager, script_name):
    if package_manager == "npm":
        return f"npm {'test' if script_name == 'test' else f'run {script_name}'}"
    if package_manager == "yarn":
        return f"yarn {script_name}"
    if package_manager == "pnpm":
        return f"pnpm {script_name}"
    if package_manager == "bun":
        return f"bun run {script_name}"
    return f"{package_manager} run {script_name}"


def detect_node_init_commands(project_root):
    package_json = project_root / "package.json"
    if not package_json.exists():
        return None

    try:
        package_data = json.loads(package_json.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        package_data = {}

    scripts = package_data.get("scripts")
    if not isinstance(scripts, dict):
        scripts = {}

    package_manager = node_package_manager(project_root, package_data)
    markers = ["package.json"]
    for marker in ("package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb", "tsconfig.json"):
        if (project_root / marker).exists():
            markers.append(marker)

    commands = default_init_detection()["commands"].copy()
    tsc_command = {
        "npm": "npx tsc --noEmit",
        "yarn": "yarn tsc --noEmit",
        "pnpm": "pnpm exec tsc --noEmit",
        "bun": "bunx tsc --noEmit",
    }.get(package_manager, "npx tsc --noEmit")

    if scripts.get("build"):
        commands["compile_check"] = node_script_command(package_manager, "build")
    elif scripts.get("typecheck"):
        commands["compile_check"] = node_script_command(package_manager, "typecheck")
    elif (project_root / "tsconfig.json").exists():
        commands["compile_check"] = tsc_command

    if scripts.get("test"):
        test_command = node_script_command(package_manager, "test")
        commands["targeted_tests"] = test_command
        commands["full_test_suite"] = test_command

    if scripts.get("lint"):
        commands["linter_suite"] = node_script_command(package_manager, "lint")

    if scripts.get("typecheck"):
        commands["typecheck"] = node_script_command(package_manager, "typecheck")
    elif (project_root / "tsconfig.json").exists():
        commands["typecheck"] = tsc_command

    summary = f"Node repo detected ({package_manager})"
    dependencies = {}
    for key in ("dependencies", "devDependencies"):
        value = package_data.get(key)
        if isinstance(value, dict):
            dependencies.update(value)
    if "next" in dependencies:
        summary += ", Next.js"
    elif "react" in dependencies:
        summary += ", React"
    elif "vue" in dependencies:
        summary += ", Vue"
    elif "svelte" in dependencies:
        summary += ", Svelte"
    if (project_root / "tsconfig.json").exists():
        summary += ", TypeScript"

    return {
        "summary": summary,
        "markers": markers,
        "commands": commands,
    }


def detect_python_init_commands(project_root):
    pyproject = project_root / "pyproject.toml"
    requirements = project_root / "requirements.txt"
    setup_py = project_root / "setup.py"
    if not any(path.exists() for path in (pyproject, requirements, setup_py)):
        return None

    markers = []
    for marker in (
        "pyproject.toml",
        "uv.lock",
        "poetry.lock",
        "requirements.txt",
        "requirements-dev.txt",
        "setup.py",
        "pytest.ini",
        "mypy.ini",
        ".mypy.ini",
        "pyrightconfig.json",
        ".ruff.toml",
        "ruff.toml",
    ):
        if (project_root / marker).exists():
            markers.append(marker)

    pyproject_text = ""
    if pyproject.exists():
        try:
            pyproject_text = pyproject.read_text(encoding="utf-8").lower()
        except OSError:
            pyproject_text = ""

    requirements_text = ""
    for requirement_file in (requirements, project_root / "requirements-dev.txt"):
        if requirement_file.exists():
            try:
                requirements_text += "\n" + requirement_file.read_text(encoding="utf-8").lower()
            except OSError:
                pass

    tool_text = "\n".join((pyproject_text, requirements_text))
    runner = "uv run" if (project_root / "uv.lock").exists() else "poetry run" if (project_root / "poetry.lock").exists() else "python -m"
    if runner == "python -m":
        pytest_command = "python -m pytest"
        ruff_command = "python -m ruff check ."
        mypy_command = "python -m mypy ."
        pyright_command = "python -m pyright"
        compile_command = "python -m compileall ."
    else:
        pytest_command = f"{runner} pytest"
        ruff_command = f"{runner} ruff check ."
        mypy_command = f"{runner} mypy ."
        pyright_command = f"{runner} pyright"
        compile_command = f"{runner} python -m compileall ."

    commands = default_init_detection()["commands"].copy()
    commands["compile_check"] = compile_command

    has_pytest = (
        (project_root / "tests").exists()
        or (project_root / "pytest.ini").exists()
        or "pytest" in tool_text
    )
    if has_pytest:
        commands["targeted_tests"] = pytest_command
        commands["full_test_suite"] = pytest_command

    has_ruff = (
        (project_root / ".ruff.toml").exists()
        or (project_root / "ruff.toml").exists()
        or "tool.ruff" in pyproject_text
        or "\nruff" in tool_text
    )
    if has_ruff:
        commands["linter_suite"] = ruff_command

    has_mypy = (
        (project_root / "mypy.ini").exists()
        or (project_root / ".mypy.ini").exists()
        or "tool.mypy" in pyproject_text
        or "\nmypy" in tool_text
    )
    has_pyright = (project_root / "pyrightconfig.json").exists() or "pyright" in tool_text
    if has_mypy:
        commands["typecheck"] = mypy_command
    elif has_pyright:
        commands["typecheck"] = pyright_command

    summary = "Python repo detected"
    if (project_root / "uv.lock").exists():
        summary += " (uv)"
    elif (project_root / "poetry.lock").exists():
        summary += " (poetry)"
    if "fastapi" in tool_text:
        summary += ", FastAPI"
    elif "django" in tool_text:
        summary += ", Django"
    elif "flask" in tool_text:
        summary += ", Flask"

    return {
        "summary": summary,
        "markers": markers,
        "commands": commands,
    }


def command_is_placeholder(command):
    return not isinstance(command, str) or command.strip() in DEFAULT_INIT_COMMANDS.values()


def detection_signal_score(detection):
    commands = detection.get("commands") if isinstance(detection, dict) else {}
    if not isinstance(commands, dict):
        return 0
    return sum(1 for key in INIT_COMMAND_KEYS if not command_is_placeholder(commands.get(key)))


def merge_markers(*marker_lists):
    merged = []
    seen = set()
    for marker_list in marker_lists:
        for marker in marker_list or []:
            if marker in seen:
                continue
            seen.add(marker)
            merged.append(marker)
    return merged


def relabel_detection_summary(summary, stack_name):
    prefix = f"{stack_name} repo detected"
    if summary.startswith(prefix):
        return f"{stack_name}{summary[len(prefix):]}"
    return summary


def merge_command_value(primary_command, secondary_command, *, default_key):
    concrete = []
    for command in (primary_command, secondary_command):
        if command_is_placeholder(command):
            continue
        if command not in concrete:
            concrete.append(command)
    if concrete:
        return " && ".join(concrete)
    for command in (primary_command, secondary_command):
        if isinstance(command, str) and command.strip():
            return command.strip()
    return DEFAULT_INIT_COMMANDS[default_key]


def merge_stack_detections(primary, secondary):
    primary_commands = primary.get("commands") if isinstance(primary.get("commands"), dict) else {}
    secondary_commands = secondary.get("commands") if isinstance(secondary.get("commands"), dict) else {}
    commands = {
        key: merge_command_value(
            primary_commands.get(key),
            secondary_commands.get(key),
            default_key=key,
        )
        for key in INIT_COMMAND_KEYS
    }
    primary_label = relabel_detection_summary(primary.get("summary", "primary"), "Python" if "Python" in primary.get("summary", "") else "Node")
    secondary_label = relabel_detection_summary(secondary.get("summary", "secondary"), "Python" if "Python" in secondary.get("summary", "") else "Node")
    return {
        "summary": f"Mixed repo detected: {primary_label} + {secondary_label}",
        "markers": merge_markers(primary.get("markers"), secondary.get("markers")),
        "commands": commands,
    }


def detect_init_config(project_root):
    node = detect_node_init_commands(project_root)
    python_detection = detect_python_init_commands(project_root)
    if node and python_detection:
        ordered = sorted(
            (python_detection, node),
            key=lambda item: (detection_signal_score(item), item.get("summary", "").startswith("Python")),
            reverse=True,
        )
        return merge_stack_detections(ordered[0], ordered[1])
    if node:
        return node
    if python_detection:
        return python_detection

    return default_init_detection()


def render_init_local_config(detection):
    commands = detection["commands"]
    marker_text = ", ".join(detection["markers"]) if detection["markers"] else "none"
    return f"""# Project-specific config overlay
# Values here merge on top of the managed bundle (.scafld/core/config.yaml)
# and any project-level .scafld/config.yaml overrides.
# Only include sections you want to override - everything else inherits.
#
# Suggested commands generated from repo markers. Review them before relying on them.
# Detection: {detection['summary']}
# Markers: {marker_text}

validation:
  per_phase:
    compile_check: {json.dumps(commands['compile_check'])}
    targeted_tests: {json.dumps(commands['targeted_tests'])}
  pre_commit:
    full_test_suite: {json.dumps(commands['full_test_suite'])}
    linter_suite: {json.dumps(commands['linter_suite'])}
    typecheck: {json.dumps(commands['typecheck'])}

# CUSTOMIZE: Replace with your actual tech stack
tech_stack:
  backend:
    language: "Your language (e.g., Python 3.11, Ruby 3.2)"
    framework: "Your framework (e.g., Django, Rails, FastAPI)"
  frontend:
    framework: "Your framework (e.g., React, Vue, Next.js)"

# CUSTOMIZE: Replace with your actual directory layout
repo_layout:
  backend: "backend/"
  frontend: "frontend/"
"""
