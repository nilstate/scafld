import json
import re

from scafld.audit_scope import CHANGE_OWNERSHIP_VALUES
from scafld.runtime_bundle import resolve_schema_path
from scafld.spec_store import yaml_read_field, yaml_read_nested


def move_result_payload(root, move_result):
    """Return a structured representation of a spec move/transition."""
    return {
        "from": str(move_result.source.relative_to(root)),
        "to": str(move_result.dest.relative_to(root)),
        "status": move_result.new_status,
    }


def validate_spec(root, spec):
    """Validate a spec against the JSON schema. Returns list of errors (empty = valid)."""
    schema_path = resolve_schema_path(root)
    if not schema_path.exists():
        return [f"schema not found at {schema_path.relative_to(root)}"]

    try:
        schema = json.loads(schema_path.read_text())
    except json.JSONDecodeError as exc:
        return [f"invalid schema JSON: {exc}"]

    text = spec.read_text()
    errors = []

    required_top = schema.get("required", [])
    for field in required_top:
        if not yaml_read_field(text, field):
            if not re.search(rf"^{field}:", text, re.MULTILINE):
                errors.append(f"missing required field: {field}")

    task_id = yaml_read_field(text, "task_id")
    if task_id and not re.match(r"^[a-z0-9-]+$", task_id):
        errors.append(f"task_id must be kebab-case: got '{task_id}'")

    status = yaml_read_field(text, "status")
    valid_statuses = ["draft", "blocked", "under_review", "approved", "in_progress", "completed", "failed", "cancelled"]
    if status and status not in valid_statuses:
        errors.append(f"invalid status: '{status}' (expected one of: {', '.join(valid_statuses)})")

    spec_version = yaml_read_field(text, "spec_version")
    if spec_version and not re.match(r"^\d+\.\d+$", spec_version):
        errors.append(f"spec_version must be semver: got '{spec_version}'")

    if not re.search(r"^phases:", text, re.MULTILINE):
        errors.append("missing required field: phases")
    else:
        phase_ids = re.findall(r'^\s+-\s+id:\s*"?(phase\d+)"?', text, re.MULTILINE)
        if not phase_ids:
            errors.append("phases array is empty (at least 1 phase required)")

    if not re.search(r"^planning_log:", text, re.MULTILINE):
        errors.append("missing required field: planning_log")

    if re.search(r"^task:", text, re.MULTILINE):
        for field in ["title", "summary", "size", "risk_level"]:
            if not yaml_read_nested(text, "task", field):
                errors.append(f"missing required task field: task.{field}")
    else:
        errors.append("missing required field: task")

    todo_patterns = [
        (r'^\s+(?:command|content_spec|description|file):\s*"?TODO', "has TODO placeholder"),
        (r'^\s+-\s+"?TODO', "has TODO list item"),
    ]
    for pattern, message in todo_patterns:
        matches = re.findall(pattern, text, re.MULTILINE)
        if matches:
            count = len(matches)
            errors.append(f"{message} ({count} occurrence{'s' if count > 1 else ''})")
            break

    ownership_values = re.findall(r'^\s+ownership:\s*"?([^"\n]+)"?', text, re.MULTILINE)
    invalid_ownership = sorted({value for value in ownership_values if value not in CHANGE_OWNERSHIP_VALUES})
    for value in invalid_ownership:
        errors.append(
            f"invalid change ownership: '{value}' (expected one of: {', '.join(sorted(CHANGE_OWNERSHIP_VALUES))})"
        )

    return errors
