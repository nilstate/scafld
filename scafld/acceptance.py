import subprocess

from scafld.spec_markdown import parse_spec_markdown, update_spec_markdown
from scafld.spec_model import now_iso


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
    # non-zero.
    if returncode != 0 or not (output or "").strip():
        return True, ""
    return False, "expected no matches, got output"


_KIND_MATCHERS = {
    "exit_code_zero": _match_exit_code_zero,
    "exit_code_nonzero": _match_exit_code_nonzero,
    "no_matches": _match_no_matches,
}


def resolve_kind(criterion):
    """Return the (kind, derived_fields) for a criterion."""
    if isinstance(criterion, dict):
        explicit = criterion.get("expected_kind")
        if isinstance(explicit, str) and explicit.strip():
            kind = explicit.strip()
            if kind in EXPECTED_KINDS:
                return kind, {}
            return "invalid_kind", {"declared_kind": kind}
    return "unset", {}


def check_kind(returncode, output, criterion):
    """Apply the structured matcher for the criterion's resolved kind.

    Returns (passed: bool, reason: str). Does not handle the `invalid_kind`
    path; callers route that before executing commands.
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
    data = parse_spec_markdown(text)
    status = "pass" if passed else "fail"
    executed_at = now_iso()
    snippet = output_snippet[:200].replace("\n", " ")

    phases = data.get("phases")
    if not isinstance(phases, list):
        return text

    for phase in phases:
        if not isinstance(phase, dict):
            continue
        block = phase.get("acceptance_criteria")
        if not isinstance(block, list):
            continue
        for entry in block:
            if not isinstance(entry, dict):
                continue
            if entry.get("id") != ac_id:
                continue
            entry["result"] = status
            entry["status"] = status
            entry["checked_at"] = executed_at
            if snippet:
                entry["evidence"] = snippet
            return update_spec_markdown(text, data)

    return text


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
    # Two pre-run rejection cases:
    #   - unset: no expected_kind declared
    #   - invalid_kind: an explicit `expected_kind` that's not in
    #     EXPECTED_KINDS (typically a typo like `exit_cod_zero`)
    kind, derived = resolve_kind(criterion)
    if kind in ("unset", "invalid_kind"):
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
