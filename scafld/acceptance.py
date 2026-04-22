import re
import subprocess

from scafld.spec_parsing import now_iso


DEFAULT_ACCEPTANCE_TIMEOUT_SECONDS = 600
GENERIC_PASS_EXPECTATIONS = {"all pass", "all tests pass", "all specs pass"}


def record_exec_result(text, ac_id, passed, output_snippet=""):
    """Record execution result for an acceptance criterion in the spec."""
    lines = text.splitlines(True)
    result = []
    i = 0
    while i < len(lines):
        line = lines[i]
        if re.search(rf'(?:id|dod_id):\s*"?{re.escape(ac_id)}"?\s*$', line):
            result.append(line)
            item_match = re.match(r"^(\s*)-\s+", line)
            if item_match:
                field_indent = " " * (len(item_match.group(1)) + 2)
            else:
                field_indent = " " * (len(line) - len(line.lstrip()))
            nested_result = False
            i += 1
            while i < len(lines):
                field_line = lines[i]
                if not field_line.strip():
                    break
                field_indent_level = len(field_line) - len(field_line.lstrip())
                expected_indent = len(field_indent)
                if field_indent_level < expected_indent:
                    break
                if field_indent_level == expected_indent - 2 and field_line.strip().startswith("- "):
                    break
                if field_indent_level == expected_indent and re.match(r"^\s+result:\s*$", field_line):
                    nested_result = True
                    i += 1
                    while i < len(lines):
                        nested = lines[i]
                        if not nested.strip():
                            i += 1
                            continue
                        nested_indent = len(nested) - len(nested.lstrip())
                        if nested_indent <= expected_indent:
                            break
                        i += 1
                    continue
                if field_indent_level == expected_indent and re.match(r"^\s+(result|result_output|executed_at):", field_line):
                    i += 1
                    continue
                result.append(field_line)
                i += 1
            status = "pass" if passed else "fail"
            executed_at = now_iso()
            snippet = output_snippet[:200].replace('"', '\\"').replace("\n", " ")
            if nested_result:
                result.append(f"{field_indent}result:\n")
                result.append(f'{field_indent}  status: "{status}"\n')
                result.append(f'{field_indent}  timestamp: "{executed_at}"\n')
                if output_snippet:
                    result.append(f'{field_indent}  output: "{snippet}"\n')
            else:
                result.append(f'{field_indent}result: "{status}"\n')
                result.append(f'{field_indent}executed_at: "{executed_at}"\n')
                if output_snippet:
                    result.append(f'{field_indent}result_output: "{snippet}"\n')
            continue
        result.append(line)
        i += 1

    return "".join(result)


def criterion_timeout_seconds(criterion):
    raw = criterion.get("timeout_seconds", "")
    if raw in ("", None):
        return DEFAULT_ACCEPTANCE_TIMEOUT_SECONDS

    try:
        seconds = int(str(raw).strip())
    except (TypeError, ValueError) as exc:
        raise ValueError(f"invalid timeout_seconds '{raw}'") from exc

    if seconds <= 0:
        raise ValueError(f"timeout_seconds must be > 0 (got {raw})")

    return seconds


def check_expected(returncode, output, expected):
    """Check command result against expected outcome."""
    if not expected:
        return returncode == 0

    exp = expected.strip()
    exp_lower = exp.lower()

    if exp_lower == "no matches":
        return returncode != 0 or not output

    match = re.match(r"^exit\s+code\s+(\d+)$", exp_lower)
    if match:
        return returncode == int(match.group(1))

    if exp_lower == "0 failures":
        if returncode != 0:
            return False
        fail_match = re.search(r"(\d+)\s+failures?", output, re.IGNORECASE)
        if fail_match and int(fail_match.group(1)) > 0:
            return False
        return True

    if exp_lower in GENERIC_PASS_EXPECTATIONS:
        if returncode != 0:
            return False
        fail_match = re.search(r"(\d+)\s+failures?", output, re.IGNORECASE)
        if fail_match and int(fail_match.group(1)) > 0:
            return False
        return True

    if returncode != 0:
        return False
    return exp_lower in output.lower()


def evaluate_acceptance_criterion(root, criterion, spec_cwd=None):
    """Run one acceptance criterion and return a stable result payload."""
    ac_id = criterion["id"]
    command = criterion["command"]
    expected = criterion.get("expected", "")
    effective_cwd = criterion.get("cwd") or spec_cwd
    base = {
        "id": ac_id,
        "description": criterion.get("description", ac_id),
        "phase": criterion.get("phase"),
        "command": command,
        "cwd": effective_cwd,
        "expected": expected,
    }

    ac_cwd = root
    if effective_cwd:
        ac_cwd = (root / effective_cwd).resolve()
        if not str(ac_cwd).startswith(str(root.resolve())):
            return {
                **base,
                "status": "fail",
                "exit_code": None,
                "output": f"cwd '{effective_cwd}' escapes workspace root",
            }
        if not ac_cwd.is_dir():
            return {
                **base,
                "status": "fail",
                "exit_code": None,
                "output": f"cwd '{effective_cwd}' not found",
            }

    try:
        timeout_seconds = criterion_timeout_seconds(criterion)
    except ValueError as exc:
        return {
            **base,
            "status": "fail",
            "exit_code": None,
            "output": str(exc),
        }

    try:
        result = subprocess.run(
            command,
            shell=True,
            capture_output=True,
            text=True,
            timeout=timeout_seconds,
            cwd=str(ac_cwd),
        )
    except subprocess.TimeoutExpired:
        return {
            **base,
            "status": "fail",
            "exit_code": None,
            "output": f"Command timed out after {timeout_seconds}s",
        }
    except Exception as exc:
        return {
            **base,
            "status": "fail",
            "exit_code": None,
            "output": str(exc),
        }

    output = (result.stdout + result.stderr).strip()
    return {
        **base,
        "status": "pass" if check_expected(result.returncode, output, expected) else "fail",
        "exit_code": result.returncode,
        "output": output[:200],
    }
