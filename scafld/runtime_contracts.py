import re
import shutil
from pathlib import Path

from scafld.runtime_bundle import ARCHIVE_DIR, RUNS_DIR, load_runtime_config


HANDOFF_SCHEMA_VERSION = 3
SESSION_SCHEMA_VERSION = 3
HANDOFFS_DIRNAME = "handoffs"
DIAGNOSTICS_DIRNAME = "diagnostics"
RUNS_ARCHIVE_DIRNAME = "archive"

HANDOFF_ROLE_VALUES = ("executor", "challenger", "human")
HANDOFF_GATE_VALUES = ("harden", "phase", "recovery", "review")

DEFAULT_MODEL_PROFILE = "default"
DEFAULT_CONTEXT_BUDGET_TOKENS = 12000
DEFAULT_RECOVERY_MAX_ATTEMPTS = 1


def parse_int_setting(value, default):
    try:
        parsed = int(str(value).strip())
    except (TypeError, ValueError):
        return default
    return parsed if parsed > 0 else default


def load_llm_settings(root):
    """Return the minimal LLM runtime settings from merged workspace config."""
    config = load_runtime_config(root)
    llm = config.get("llm") if isinstance(config.get("llm"), dict) else {}
    context = llm.get("context") if isinstance(llm.get("context"), dict) else {}
    recovery = llm.get("recovery") if isinstance(llm.get("recovery"), dict) else {}
    return {
        "model_profile": str(llm.get("model_profile") or DEFAULT_MODEL_PROFILE),
        "context_budget_tokens": parse_int_setting(
            context.get("budget_tokens"),
            DEFAULT_CONTEXT_BUDGET_TOKENS,
        ),
        "recovery_max_attempts": parse_int_setting(
            recovery.get("max_attempts"),
            DEFAULT_RECOVERY_MAX_ATTEMPTS,
        ),
    }


def normalize_selector(value, fallback="current"):
    selector = re.sub(r"[^A-Za-z0-9_.-]+", "-", str(value or "")).strip("-")
    return selector or fallback


def normalize_handoff_identity(*, role=None, gate=None):
    if role is None or gate is None:
        raise ValueError("handoff identity requires role+gate")
    if role not in HANDOFF_ROLE_VALUES:
        raise ValueError(f"invalid handoff role: {role}")
    if gate not in HANDOFF_GATE_VALUES:
        raise ValueError(f"invalid handoff gate: {gate}")
    return role, gate


def archive_month_for_spec(root, spec_path):
    if spec_path is None:
        return None
    try:
        rel = spec_path.relative_to(root)
    except ValueError:
        return None
    prefix = f"{ARCHIVE_DIR}/"
    rel_text = rel.as_posix()
    if not rel_text.startswith(prefix):
        return None
    parts = rel_text.split("/")
    if len(parts) < 4:
        return None
    return parts[3]


def current_task_run_dir(root, task_id):
    return root / RUNS_DIR / task_id


def archived_task_run_dir(root, task_id, month):
    return root / RUNS_DIR / RUNS_ARCHIVE_DIRNAME / month / task_id


def task_run_dir(root, task_id, *, spec_path=None):
    month = archive_month_for_spec(root, spec_path)
    if month:
        return archived_task_run_dir(root, task_id, month)
    return current_task_run_dir(root, task_id)


def find_task_run_dir(root, task_id, *, spec_path=None):
    month = archive_month_for_spec(root, spec_path)
    if month:
        archived = archived_task_run_dir(root, task_id, month)
        if archived.exists():
            return archived

    current = current_task_run_dir(root, task_id)
    if current.exists():
        return current

    archive_root = root / RUNS_DIR / RUNS_ARCHIVE_DIRNAME
    if archive_root.is_dir():
        matches = sorted(archive_root.glob(f"*/{task_id}"))
        if matches:
            return matches[-1]
    return current


def handoffs_dir(root, task_id, *, spec_path=None):
    return task_run_dir(root, task_id, spec_path=spec_path) / HANDOFFS_DIRNAME


def diagnostics_dir(root, task_id, *, spec_path=None):
    return task_run_dir(root, task_id, spec_path=spec_path) / DIAGNOSTICS_DIRNAME


def session_path(root, task_id, *, spec_path=None):
    return task_run_dir(root, task_id, spec_path=spec_path) / "session.json"


def locate_session_path(root, task_id, *, spec_path=None):
    return find_task_run_dir(root, task_id, spec_path=spec_path) / "session.json"


def ensure_run_dirs(root, task_id, *, spec_path=None):
    run_dir = task_run_dir(root, task_id, spec_path=spec_path)
    handoff_dir = handoffs_dir(root, task_id, spec_path=spec_path)
    diagnostic_dir = diagnostics_dir(root, task_id, spec_path=spec_path)
    handoff_dir.mkdir(parents=True, exist_ok=True)
    diagnostic_dir.mkdir(parents=True, exist_ok=True)
    return {
        "run_dir": run_dir,
        "handoffs_dir": handoff_dir,
        "diagnostics_dir": diagnostic_dir,
        "session_path": session_path(root, task_id, spec_path=spec_path),
    }


def handoff_filename(*, role=None, gate=None, selector=None, attempt=None):
    role, gate = normalize_handoff_identity(role=role, gate=gate)
    safe_selector = normalize_selector(selector, fallback="current")
    stem = f"{role}-{gate}"
    if gate not in {"review", "harden"}:
        stem += f"-{safe_selector}"
    if attempt not in (None, ""):
        stem += f"-{attempt}"
    return f"{stem}.md"


def handoff_path(root, task_id, *, role=None, gate=None, selector=None, attempt=None, spec_path=None):
    return handoffs_dir(root, task_id, spec_path=spec_path) / handoff_filename(
        role=role,
        gate=gate,
        selector=selector,
        attempt=attempt,
    )


def handoff_json_path(root, task_id, *, role=None, gate=None, selector=None, attempt=None, spec_path=None):
    return handoff_path(
        root,
        task_id,
        role=role,
        gate=gate,
        selector=selector,
        attempt=attempt,
        spec_path=spec_path,
    ).with_suffix(".json")


def relative_path(root, path):
    try:
        return str(path.relative_to(root))
    except ValueError:
        return str(path)


def session_ref(root, task_id, *, spec_path=None):
    return relative_path(root, session_path(root, task_id, spec_path=spec_path))


def expected_session_ref(task_id, *, archive_month=None):
    if archive_month:
        return str(Path(RUNS_DIR) / RUNS_ARCHIVE_DIRNAME / archive_month / task_id / "session.json")
    return str(Path(RUNS_DIR) / task_id / "session.json")


def archive_run_artifacts(root, task_id, month):
    current = current_task_run_dir(root, task_id)
    destination = archived_task_run_dir(root, task_id, month)
    if not current.exists():
        return destination if destination.exists() else None
    destination.parent.mkdir(parents=True, exist_ok=True)
    if destination.exists():
        shutil.rmtree(destination)
    shutil.move(str(current), str(destination))
    return destination
