import datetime
import re

from scafld.config import parse_yaml_value
from scafld.spec_store import yaml_read_field, yaml_read_nested


def require_pyyaml():
    try:
        import yaml
    except ModuleNotFoundError as exc:
        raise RuntimeError("PyYAML is required") from exc
    return yaml


def now_iso():
    return datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).strftime("%Y-%m-%dT%H:%M:%SZ")


def parse_numeric_scalar(value):
    """Parse a YAML scalar that should contain a numeric value."""
    if value in (None, "", "null", "{}"):
        return None
    try:
        return float(str(value).strip().strip('"').strip("'"))
    except (TypeError, ValueError):
        return None


def extract_self_eval_score(text):
    """Read the recorded self-eval score from supported spec shapes."""
    for parent in ("self_eval", "perf_eval"):
        for field in ("total", "score"):
            score = parse_numeric_scalar(yaml_read_nested(text, parent, field))
            if score is not None:
                return score
        score = parse_numeric_scalar(yaml_read_field(text, parent))
        if score is not None:
            return score

    return parse_numeric_scalar(yaml_read_field(text, "score"))


def parse_phase_status_entries(text):
    """Parse phase ids/statuses from the phases block without counting unrelated statuses."""
    lines = text.splitlines()
    entries = []
    in_phases = False
    i = 0

    while i < len(lines):
        line = lines[i]
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())

        if not in_phases:
            if re.match(r"^phases:\s*$", line):
                in_phases = True
            i += 1
            continue

        if stripped and indent == 0:
            break

        match = re.match(r'^\s*-\s+id:\s*"?(phase\d+)"?', line)
        if not match:
            i += 1
            continue

        phase_id = match.group(1)
        item_indent = len(match.group(0)) - len(match.group(0).lstrip())
        status = "pending"
        i += 1

        while i < len(lines):
            field_line = lines[i]
            field_stripped = field_line.strip()
            field_indent = len(field_line) - len(field_line.lstrip())

            if field_stripped and field_indent <= item_indent:
                break

            status_match = re.match(
                r'^\s+status:\s*"?(pending|in_progress|completed|failed|skipped)"?',
                field_line,
            )
            if status_match:
                status = status_match.group(1)

            i += 1

        entries.append({"id": phase_id, "status": status})

    return entries


def parse_phase_statuses(text):
    """Parse phase statuses from the phases block without counting unrelated statuses."""
    return [entry["status"] for entry in parse_phase_status_entries(text)]


def count_phases(text):
    """Count phases and their statuses."""
    phase_statuses = parse_phase_statuses(text)
    total = len(phase_statuses)
    completed = sum(1 for status in phase_statuses if status == "completed")
    failed = sum(1 for status in phase_statuses if status == "failed")
    in_progress = sum(1 for status in phase_statuses if status == "in_progress")
    return total, completed, failed, in_progress


def parse_acceptance_criteria(text):
    """Extract acceptance criteria from all phases using indent-aware parsing."""
    criteria = []
    lines = text.splitlines()
    current_phase = None
    i = 0

    while i < len(lines):
        line = lines[i]

        match = re.match(r"^(\s+)-\s+id:\s*(.+)$", line)
        if match:
            value = parse_yaml_value(match.group(2))
            if re.match(r"^phase\d+$", value):
                current_phase = value
                i += 1
                continue

        if current_phase and re.match(r"^(\s+)(acceptance_criteria|validation)\s*:", line):
            block_indent = len(line) - len(line.lstrip())
            i += 1
            while i < len(lines):
                item_line = lines[i]
                if not item_line.strip():
                    i += 1
                    continue
                item_indent = len(item_line) - len(item_line.lstrip())
                if item_indent <= block_indent and item_line.strip():
                    break
                match = re.match(r"^(\s+)-\s+(id|dod_id):\s*(.+)$", item_line)
                if match:
                    item_base_indent = len(match.group(1))
                    criterion = {"id": parse_yaml_value(match.group(3)), "phase": current_phase}
                    result_block_indent = None
                    i += 1
                    while i < len(lines):
                        field_line = lines[i]
                        if not field_line.strip():
                            i += 1
                            continue
                        field_indent = len(field_line) - len(field_line.lstrip())
                        if field_indent <= item_base_indent and field_line.strip():
                            break
                        if field_line.strip().startswith("- ") and field_indent == item_base_indent:
                            break
                        if result_block_indent is not None and field_indent <= result_block_indent:
                            result_block_indent = None
                        field_match = re.match(r"^\s+([\w_]+):\s*(.*)$", field_line)
                        if field_match:
                            key = field_match.group(1)
                            value = parse_yaml_value(field_match.group(2))
                            if key in ("type", "description", "command", "expected", "cwd", "timeout_seconds"):
                                criterion[key] = value
                            elif key == "result":
                                if value:
                                    criterion[key] = value
                                else:
                                    result_block_indent = field_indent
                            elif key == "status" and result_block_indent is not None and field_indent > result_block_indent:
                                criterion["result"] = value
                        i += 1
                    criteria.append(criterion)
                else:
                    i += 1
            continue

        i += 1

    return criteria


def extract_spec_cwd(text):
    match = re.search(r'^\s+context:.*?\n(?:\s+\S.*\n)*?\s+cwd:\s*"?([^"\n]+)"?', text, re.MULTILINE)
    if not match:
        return None
    return match.group(1).strip().strip('"').strip("'")


def parse_iso8601_timestamp(value):
    if not value:
        return None
    try:
        return datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
