import json
import re
import subprocess

from scafld.spec_parsing import now_iso, require_pyyaml


DEFAULT_ACCEPTANCE_TIMEOUT_SECONDS = 600

EXPECTED_KINDS = (
    "exit_code_zero",
    "exit_code_nonzero",
    "no_matches",
)


def _match_exit_code_zero(returncode, output, criterion):
    if returncode == 0:
        return True, ""
    return False, f"expected exit code 0, got {returncode}"


def _match_exit_code_nonzero(returncode, output, criterion):
    expected = criterion.get("expected_exit_code")
    if expected is not None:
        try:
            expected_int = int(expected)
        except (TypeError, ValueError):
            return False, f"criterion expected_exit_code is not an integer: {expected!r}"
        if returncode == expected_int:
            return True, ""
        return False, f"expected exit code {expected_int}, got {returncode}"
    if returncode != 0:
        return True, ""
    return False, "expected non-zero exit code, got 0"


def _match_no_matches(returncode, output, criterion):
    # `no matches` is satisfied when the command emits no output OR exits
    # non-zero. Mirrors the legacy `check_expected('no matches')` contract
    # so existing specs keep working under strict mode after auto-mapping.
    if returncode != 0 or not (output or "").strip():
        return True, ""
    return False, "expected no matches, got output"


_KIND_MATCHERS = {
    "exit_code_zero": _match_exit_code_zero,
    "exit_code_nonzero": _match_exit_code_nonzero,
    "no_matches": _match_no_matches,
}


def _legacy_expected_to_kind(expected):
    """Map a legacy `expected:` string into a (kind, fields) tuple.

    Returns ("legacy_substring", {"expected_substring": <verbatim>}) for
    strings not in the recognized set; that path falls through to
    `check_expected` substring matching in lenient mode.

    Returns ("unset", {}) when neither `expected_kind` nor `expected` is
    declared. Strict mode treats this as a hard failure (the criterion
    declares no contract for what passing looks like).
    """
    if not isinstance(expected, str) or not expected.strip():
        return "unset", {}
    exp_lower = expected.strip().lower()
    if exp_lower == "exit code 0":
        return "exit_code_zero", {}
    match = re.fullmatch(r"exit\s+code\s+(\d+)", exp_lower)
    if match:
        code = int(match.group(1))
        if code == 0:
            return "exit_code_zero", {}
        return "exit_code_nonzero", {"expected_exit_code": code}
    if exp_lower == "no matches":
        return "no_matches", {}
    return "legacy_substring", {"expected_substring": expected}


def resolve_kind(criterion):
    """Return the (kind, derived_fields) for a criterion.

    Explicit `expected_kind` wins. Otherwise the legacy `expected:` string
    is mapped. Unrecognized legacy strings yield kind == "legacy_substring"
    so callers can route them through the existing check_expected path
    (lenient mode) or refuse them (strict mode).
    """
    if isinstance(criterion, dict):
        explicit = criterion.get("expected_kind")
        if isinstance(explicit, str) and explicit.strip():
            kind = explicit.strip()
            if kind in EXPECTED_KINDS:
                return kind, {}
            return "invalid_kind", {"declared_kind": kind}
        return _legacy_expected_to_kind(criterion.get("expected", ""))
    return _legacy_expected_to_kind("")


def check_kind(returncode, output, criterion):
    """Apply the structured matcher for the criterion's resolved kind.

    Returns (passed: bool, reason: str). Does not handle the
    `legacy_substring` / `invalid_kind` paths — callers route those.
    """
    kind, derived = resolve_kind(criterion)
    if kind in _KIND_MATCHERS:
        merged = dict(criterion)
        for key, value in derived.items():
            merged.setdefault(key, value)
        return _KIND_MATCHERS[kind](returncode, output, merged)
    return False, f"check_kind cannot evaluate kind={kind!r}"


def record_exec_result(text, ac_id, passed, output_snippet=""):
    """Record execution result for an acceptance criterion in the spec."""
    yaml = require_pyyaml()
    data = yaml.safe_load(text) or {}
    if not isinstance(data, dict):
        return text

    status = "pass" if passed else "fail"
    executed_at = now_iso()
    snippet = output_snippet[:200].replace("\n", " ")
    nested_result = False

    phases = data.get("phases")
    if not isinstance(phases, list):
        return text

    for phase in phases:
        if not isinstance(phase, dict):
            continue
        for block_name in ("acceptance_criteria", "validation"):
            block = phase.get(block_name)
            if not isinstance(block, list):
                continue
            for entry in block:
                if not isinstance(entry, dict):
                    continue
                if entry.get("id") != ac_id and entry.get("dod_id") != ac_id:
                    continue
                nested_result = isinstance(entry.get("result"), dict)
                return _rewrite_exec_result_fields(
                    text=text,
                    ac_id=ac_id,
                    nested_result=nested_result,
                    status=status,
                    executed_at=executed_at,
                    output_snippet=snippet if output_snippet else "",
                )

    return text


def _rewrite_exec_result_fields(*, text, ac_id, nested_result, status, executed_at, output_snippet):
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

            i += 1
            preserved = []
            insert_at = None
            while i < len(lines):
                field_line = lines[i]
                if not field_line.strip():
                    preserved.append(field_line)
                    i += 1
                    continue

                field_indent_level = len(field_line) - len(field_line.lstrip())
                expected_indent = len(field_indent)
                if field_indent_level < expected_indent:
                    break
                if field_indent_level == expected_indent - 2 and field_line.strip().startswith("- "):
                    break

                if field_indent_level == expected_indent and re.match(
                    r"^\s+(result|result_output|executed_at):(?:\s|$)",
                    field_line,
                ):
                    if insert_at is None:
                        insert_at = len(preserved)
                    i = _skip_yaml_field(lines, i, field_indent_level, expected_indent - 2)
                    continue

                preserved.append(field_line)
                i += 1

            runtime_lines = _render_exec_result_fields(
                field_indent=field_indent,
                nested_result=nested_result,
                status=status,
                executed_at=executed_at,
                output_snippet=output_snippet,
            )
            if insert_at is None:
                insert_at = len(preserved)
            preserved[insert_at:insert_at] = runtime_lines
            result.extend(preserved)
            continue

        result.append(line)
        i += 1

    return "".join(result)


def _skip_yaml_field(lines, start_index, field_indent_level, item_indent_level):
    i = start_index + 1
    while i < len(lines):
        next_line = lines[i]
        if not next_line.strip():
            i += 1
            continue
        next_indent = len(next_line) - len(next_line.lstrip())
        if next_indent <= field_indent_level:
            break
        if next_indent <= item_indent_level and next_line.strip().startswith("- "):
            break
        i += 1
    return i


def _render_exec_result_fields(*, field_indent, nested_result, status, executed_at, output_snippet):
    if nested_result:
        lines = [
            f"{field_indent}result:\n",
            f"{field_indent}  status: {json.dumps(status)}\n",
            f"{field_indent}  timestamp: {json.dumps(executed_at)}\n",
        ]
        if output_snippet:
            lines.append(f"{field_indent}  output: {json.dumps(output_snippet)}\n")
        return lines

    lines = [
        f"{field_indent}result: {json.dumps(status)}\n",
        f"{field_indent}executed_at: {json.dumps(executed_at)}\n",
    ]
    if output_snippet:
        lines.append(f"{field_indent}result_output: {json.dumps(output_snippet)}\n")
    return lines


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
                "full_output": f"cwd '{effective_cwd}' escapes workspace root",
            }
        if not ac_cwd.is_dir():
            return {
                **base,
                "status": "fail",
                "exit_code": None,
                "output": f"cwd '{effective_cwd}' not found",
                "full_output": f"cwd '{effective_cwd}' not found",
            }

    try:
        timeout_seconds = criterion_timeout_seconds(criterion)
    except ValueError as exc:
        return {
            **base,
            "status": "fail",
            "exit_code": None,
            "output": str(exc),
            "full_output": str(exc),
        }

    # Reject criteria whose kind we cannot evaluate BEFORE running the
    # command. The command can have side effects (file writes, network
    # calls); running it only to throw the result away is a footgun.
    # Three pre-run rejection cases:
    #   - unset: no expected_kind and no legacy `expected:` string
    #   - legacy_substring: a legacy `expected:` string that doesn't
    #     auto-map to one of the three kinds
    #   - invalid_kind: an explicit `expected_kind` that's not in
    #     EXPECTED_KINDS (typically a typo like `exit_cod_zero`)
    kind, derived = resolve_kind(criterion)
    if kind in ("legacy_substring", "unset", "invalid_kind"):
        if kind == "unset":
            reason = (
                f"criterion {ac_id} has no expected_kind declared; "
                f"add expected_kind: exit_code_zero (or one of "
                f"exit_code_nonzero, no_matches) to the criterion"
            )
        elif kind == "invalid_kind":
            declared = derived.get("declared_kind")
            reason = (
                f"criterion {ac_id} declares unknown expected_kind={declared!r}; "
                f"valid kinds are: {', '.join(EXPECTED_KINDS)}"
            )
        else:
            reason = (
                f"criterion {ac_id} has only a legacy `expected: {expected!r}` string; "
                f"add an explicit expected_kind"
            )
        return {
            **base,
            "status": "fail",
            "exit_code": None,
            "output": f"[acceptance] {reason}"[:200],
            "full_output": f"[acceptance] {reason}",
            "fail_reason": reason,
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
            "full_output": f"Command timed out after {timeout_seconds}s",
        }
    except Exception as exc:
        return {
            **base,
            "status": "fail",
            "exit_code": None,
            "output": str(exc),
            "full_output": str(exc),
        }

    output = (result.stdout + result.stderr).strip()
    stdout_only = (result.stdout or "").strip()
    passed, reason = check_kind(result.returncode, output, criterion)

    # `evidence_required: true` rejects criteria whose command exits 0 with
    # empty stdout. Stops the "compile + unittest pass with no real work"
    # pattern: a phase whose acceptance is `python -m unittest` against an
    # empty test suite emits zero stdout and now fails.
    if passed and criterion.get("evidence_required") and not stdout_only:
        passed = False
        reason = "evidence_required: command stdout is empty"

    # Surface the failure reason in the recorded output so the spec,
    # diagnostic, and live --json all carry the same explanation.
    surfaced_output = output
    if not passed and reason:
        prefix = f"[acceptance] {reason}"
        surfaced_output = f"{prefix}\n{output}" if output else prefix
    return {
        **base,
        "status": "pass" if passed else "fail",
        "exit_code": result.returncode,
        "output": surfaced_output[:200],
        "full_output": surfaced_output,
        "fail_reason": "" if passed else reason,
    }
