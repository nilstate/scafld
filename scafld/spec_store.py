import datetime
import re
import shutil
from dataclasses import dataclass

from scafld.errors import ScafldError

AI_DIR = ".ai"
SPECS_DIR = f"{AI_DIR}/specs"
DRAFTS_DIR = f"{SPECS_DIR}/drafts"
APPROVED_DIR = f"{SPECS_DIR}/approved"
ACTIVE_DIR = f"{SPECS_DIR}/active"
ARCHIVE_DIR = f"{SPECS_DIR}/archive"

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
    "draft": ["under_review", "approved"],
    "under_review": ["draft", "approved"],
    "approved": ["in_progress"],
    "in_progress": ["completed", "failed", "cancelled"],
    "failed": ["cancelled"],
}


@dataclass(frozen=True)
class SpecMoveResult:
    source: object
    dest: object
    previous_status: str
    new_status: str


def now_iso():
    return datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).strftime("%Y-%m-%dT%H:%M:%SZ")


def yaml_read_field(text, field):
    """Read a top-level YAML scalar field (simple regex, no library needed)."""
    match = re.search(rf'^{field}:\s*"?([^"\n]+)"?', text, re.MULTILINE)
    if match:
        return match.group(1).strip().strip('"').strip("'")
    return None


def yaml_set_field(text, field, value):
    """Set a top-level YAML scalar field."""
    quoted = f'"{value}"'
    new_text, count = re.subn(
        rf'^{field}:\s*.*$',
        f'{field}: {quoted}',
        text,
        count=1,
        flags=re.MULTILINE,
    )
    if count == 0:
        new_text = f'{field}: {quoted}\n' + text
    return new_text


def yaml_read_nested(text, parent, field):
    """Read a field nested one level under a parent block."""
    in_parent = False
    for line in text.splitlines():
        if re.match(rf"^{parent}:", line):
            in_parent = True
            continue
        if in_parent:
            if line and not line[0].isspace():
                break
            match = re.match(rf'^\s+{field}:\s*"?([^"\n]+)"?', line)
            if match:
                return match.group(1).strip().strip('"').strip("'")
    return None


def append_planning_log(text, summary, actor="cli"):
    """Append a planning_log entry to the spec text."""
    entry = f'  - timestamp: "{now_iso()}"\n    actor: "{actor}"\n    summary: "{summary}"'
    if re.search(r"^planning_log:", text, re.MULTILINE):
        lines = text.splitlines(True)
        insert_idx = None
        in_log = False
        for index, line in enumerate(lines):
            if re.match(r"^planning_log:", line):
                in_log = True
                continue
            if in_log and line.strip() and not line[0].isspace():
                insert_idx = index
                break
        if insert_idx is None:
            return text.rstrip("\n") + "\n" + entry + "\n"
        lines.insert(insert_idx, entry + "\n")
        return "".join(lines)
    return text + f"\nplanning_log:\n{entry}\n"


def find_specs(root, task_id):
    """Find all spec files across draft, approved, active, and archive directories."""
    specs = []
    for folder in (DRAFTS_DIR, APPROVED_DIR, ACTIVE_DIR):
        candidate = root / folder / f"{task_id}.yaml"
        if candidate.exists():
            specs.append(candidate)
    archive_root = root / ARCHIVE_DIR
    if archive_root.is_dir():
        for month_dir in sorted(archive_root.iterdir(), reverse=True):
            if month_dir.is_dir():
                candidate = month_dir / f"{task_id}.yaml"
                if candidate.exists():
                    specs.append(candidate)
    return specs


def find_spec(root, task_id):
    """Return the first matching spec path across lifecycle directories."""
    specs = find_specs(root, task_id)
    return specs[0] if specs else None


def find_all_specs(root):
    """Return all specs with their lifecycle bucket labels."""
    specs = []
    for label, folder in (("drafts", DRAFTS_DIR), ("approved", APPROVED_DIR), ("active", ACTIVE_DIR)):
        folder_path = root / folder
        if folder_path.is_dir():
            for spec_path in sorted(folder_path.glob("*.yaml")):
                specs.append((spec_path, label))
    archive_root = root / ARCHIVE_DIR
    if archive_root.is_dir():
        for month_dir in sorted(archive_root.iterdir(), reverse=True):
            if month_dir.is_dir():
                for spec_path in sorted(month_dir.glob("*.yaml")):
                    specs.append((spec_path, f"archive/{month_dir.name}"))
    return specs


def require_spec(root, task_id):
    """Return one unambiguous spec path or raise a structured command error."""
    specs = find_specs(root, task_id)
    if not specs:
        raise ScafldError(
            f"spec not found: {task_id}",
            [f"searched: {DRAFTS_DIR}/, {APPROVED_DIR}/, {ACTIVE_DIR}/, {ARCHIVE_DIR}/"],
            code="spec_not_found",
        )
    if len(specs) > 1:
        details = ["matching specs:"]
        details.extend(f"  - {spec.relative_to(root)}" for spec in specs)
        details.append("resolve the duplicate task-id before continuing")
        raise ScafldError(f"ambiguous task-id: {task_id}", details, code="ambiguous_task_id")
    return specs[0]


def move_spec(root, spec_path, new_status):
    """Move a spec to the correct lifecycle directory and update its metadata."""
    text = spec_path.read_text(encoding="utf-8")
    current_status = yaml_read_field(text, "status")
    allowed = VALID_TRANSITIONS.get(current_status, [])
    if new_status not in allowed:
        allowed_display = ", ".join(allowed) if allowed else "none"
        raise ScafldError(
            f"cannot transition from '{current_status}' to '{new_status}'",
            [f"allowed transitions: {allowed_display}"],
            code="invalid_transition",
        )

    text = yaml_set_field(text, "status", new_status)
    text = yaml_set_field(text, "updated", now_iso())
    action_labels = {
        "approved": "Spec approved",
        "in_progress": "Execution started",
        "completed": "Spec completed",
        "failed": "Spec marked failed",
        "cancelled": "Spec cancelled",
    }
    text = append_planning_log(text, action_labels.get(new_status, f"Status changed to {new_status}"))

    if new_status in ("completed", "failed", "cancelled"):
        month = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m")
        dest_dir = root / ARCHIVE_DIR / month
    else:
        dest_dir = root / STATUS_FOLDERS[new_status]
    dest_dir.mkdir(parents=True, exist_ok=True)
    dest = dest_dir / spec_path.name

    spec_path.write_text(text, encoding="utf-8")
    shutil.move(str(spec_path), str(dest))
    return SpecMoveResult(
        source=spec_path,
        dest=dest,
        previous_status=current_status,
        new_status=new_status,
    )
