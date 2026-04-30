import datetime
import shutil
from dataclasses import dataclass

from scafld.error_codes import ErrorCode
from scafld.errors import ScafldError
from scafld.spec_markdown import parse_spec_markdown, render_spec_markdown, update_spec_markdown
from scafld.spec_model import now_iso

SCAFLD_DIR = ".scafld"
SPECS_DIR = f"{SCAFLD_DIR}/specs"
DRAFTS_DIR = f"{SPECS_DIR}/drafts"
APPROVED_DIR = f"{SPECS_DIR}/approved"
ACTIVE_DIR = f"{SPECS_DIR}/active"
ARCHIVE_DIR = f"{SPECS_DIR}/archive"
SPEC_EXTENSION = ".md"

STATUS_FOLDERS = {
    "draft": DRAFTS_DIR,
    "under_review": DRAFTS_DIR,
    "approved": APPROVED_DIR,
    "in_progress": ACTIVE_DIR,
    "completed": ARCHIVE_DIR,
    "failed": ARCHIVE_DIR,
    "cancelled": ARCHIVE_DIR,
}

VALID_TRANSITIONS = {
    "draft": ["under_review", "approved", "cancelled"],
    "under_review": ["draft", "approved", "cancelled"],
    "approved": ["in_progress", "cancelled"],
    "in_progress": ["completed", "failed", "cancelled"],
    "failed": ["cancelled"],
}


@dataclass(frozen=True)
class SpecMoveResult:
    source: object
    dest: object
    previous_status: str
    new_status: str


def load_spec_document(spec_path):
    """Load a Markdown task spec into the normalized runtime model."""
    if spec_path.suffix != SPEC_EXTENSION:
        raise ScafldError(
            f"unsupported spec format: {spec_path.name}",
            [f"scafld v2 only loads *{SPEC_EXTENSION} task specs"],
            code=ErrorCode.INVALID_SPEC_DOCUMENT,
        )
    return parse_spec_markdown(spec_path.read_text(encoding="utf-8"), path=spec_path)


def write_spec_document(spec_path, data):
    """Write Markdown runner sections while preserving human-owned prose."""
    if spec_path.suffix != SPEC_EXTENSION:
        raise ScafldError(
            f"unsupported spec format: {spec_path.name}",
            [f"scafld v2 only writes *{SPEC_EXTENSION} task specs"],
            code=ErrorCode.INVALID_SPEC_DOCUMENT,
        )
    if spec_path.exists():
        current = spec_path.read_text(encoding="utf-8")
        spec_path.write_text(update_spec_markdown(current, data), encoding="utf-8")
    else:
        spec_path.parent.mkdir(parents=True, exist_ok=True)
        spec_path.write_text(render_spec_markdown(data), encoding="utf-8")


def prune_empty(value):
    """Drop empty strings/nulls/empty containers while preserving False/0."""
    if isinstance(value, dict):
        pruned = {key: prune_empty(item) for key, item in value.items()}
        return {key: item for key, item in pruned.items() if item not in (None, "", [], {})}
    if isinstance(value, list):
        pruned = [prune_empty(item) for item in value]
        return [item for item in pruned if item not in (None, "", [], {})]
    return value


def append_planning_entry(data, summary, actor="cli"):
    entry = {"timestamp": now_iso(), "actor": actor, "summary": summary}
    entries = data.get("planning_log")
    if not isinstance(entries, list):
        entries = []
    entries.append(entry)
    data["planning_log"] = entries
    return data


def find_specs(root, task_id):
    """Find all v2 Markdown spec files across lifecycle directories."""
    specs = []
    for folder in (DRAFTS_DIR, APPROVED_DIR, ACTIVE_DIR):
        candidate = root / folder / f"{task_id}{SPEC_EXTENSION}"
        if candidate.exists():
            specs.append(candidate)
    archive_root = root / ARCHIVE_DIR
    if archive_root.is_dir():
        for month_dir in sorted(archive_root.iterdir(), reverse=True):
            if month_dir.is_dir():
                candidate = month_dir / f"{task_id}{SPEC_EXTENSION}"
                if candidate.exists():
                    specs.append(candidate)
    return specs


def find_spec(root, task_id):
    """Return the first matching v2 spec path across lifecycle directories."""
    specs = find_specs(root, task_id)
    return specs[0] if specs else None


def find_all_specs(root):
    """Return all v2 specs with their lifecycle bucket labels."""
    specs = []
    for label, folder in (("drafts", DRAFTS_DIR), ("approved", APPROVED_DIR), ("active", ACTIVE_DIR)):
        folder_path = root / folder
        if folder_path.is_dir():
            for spec_path in sorted(folder_path.glob(f"*{SPEC_EXTENSION}")):
                specs.append((spec_path, label))
    archive_root = root / ARCHIVE_DIR
    if archive_root.is_dir():
        for month_dir in sorted(archive_root.iterdir(), reverse=True):
            if month_dir.is_dir():
                for spec_path in sorted(month_dir.glob(f"*{SPEC_EXTENSION}")):
                    specs.append((spec_path, f"archive/{month_dir.name}"))
    return specs


def require_spec(root, task_id):
    """Return one unambiguous Markdown spec path or raise a structured command error."""
    specs = find_specs(root, task_id)
    if not specs:
        raise ScafldError(
            f"spec not found: {task_id}",
            [f"searched: {DRAFTS_DIR}/, {APPROVED_DIR}/, {ACTIVE_DIR}/, {ARCHIVE_DIR}/"],
            code=ErrorCode.SPEC_NOT_FOUND,
        )
    if len(specs) > 1:
        details = ["matching specs:"]
        details.extend(f"  - {spec.relative_to(root)}" for spec in specs)
        details.append("resolve the duplicate task-id before continuing")
        raise ScafldError(f"ambiguous task-id: {task_id}", details, code=ErrorCode.AMBIGUOUS_TASK_ID)
    return specs[0]


def move_spec(root, spec_path, new_status):
    """Move a Markdown spec to the correct lifecycle directory and update managed state."""
    data = load_spec_document(spec_path)
    current_status = data.get("status")
    allowed = VALID_TRANSITIONS.get(current_status, [])
    if new_status not in allowed:
        allowed_display = ", ".join(allowed) if allowed else "none"
        raise ScafldError(
            f"cannot transition from '{current_status}' to '{new_status}'",
            [f"allowed transitions: {allowed_display}"],
            code=ErrorCode.INVALID_TRANSITION,
        )

    data["status"] = new_status
    data["updated"] = now_iso()
    action_labels = {
        "approved": "Spec approved",
        "in_progress": "Execution started",
        "completed": "Spec completed",
        "failed": "Spec marked failed",
        "cancelled": "Spec cancelled",
    }
    append_planning_entry(data, action_labels.get(new_status, f"Status changed to {new_status}"))
    write_spec_document(spec_path, data)

    if new_status in ("completed", "failed", "cancelled"):
        month = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m")
        dest_dir = root / ARCHIVE_DIR / month
    else:
        dest_dir = root / STATUS_FOLDERS[new_status]
    dest_dir.mkdir(parents=True, exist_ok=True)
    dest = dest_dir / spec_path.name

    shutil.move(str(spec_path), str(dest))
    return SpecMoveResult(
        source=spec_path,
        dest=dest,
        previous_status=current_status,
        new_status=new_status,
    )
