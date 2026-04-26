import os
import shutil
import subprocess
import sys
from datetime import datetime, timezone

from scafld.handoff_renderer import render_handoff
from scafld.handoff_renderer import current_phase_id
from scafld.lifecycle_runtime import status_snapshot
from scafld.review_runtime import review_snapshot
from scafld.session_store import (
    attempts_for_criterion,
    failed_attempts_for_criterion,
    latest_failed_attempt,
    load_session,
    record_provider_invocation,
)
from scafld.spec_store import load_spec_document, require_spec


def resolve_provider(provider):
    if provider == "codex":
        return "codex", "SCAFLD_CODEX_BIN"
    if provider == "claude":
        return "claude", "SCAFLD_CLAUDE_BIN"
    raise ValueError(f"unknown provider '{provider}'")


def recovery_context(session, selector):
    failed_attempt = latest_failed_attempt(session, selector)
    if failed_attempt is None:
        raise ValueError(f"no failed attempt recorded for {selector}")
    return {
        "failed_attempt": failed_attempt,
        "diagnostic_rel": failed_attempt.get("diagnostic_path"),
        "criterion_attempts": attempts_for_criterion(session, selector),
        "recovery_attempt": max(len(failed_attempts_for_criterion(session, selector)), 1),
    }


def render_adapter_handoff(root, task_id, *, role, gate, selector=None):
    spec = require_spec(root, task_id)
    session = load_session(root, task_id, spec_path=spec)
    context = {}
    if gate == "phase" and not selector:
        selector = current_phase_id(load_spec_document(spec)) or "phase1"
    if gate == "recovery":
        if session is None:
            raise ValueError("recovery handoff requires an existing session")
        context = recovery_context(session, selector)
    rendered = render_handoff(
        root,
        task_id,
        spec,
        role=role,
        gate=gate,
        selector=selector,
        session=session,
        context=context,
    )
    return rendered["content"], rendered["path_rel"]


def build_prompt(root, task_id):
    spec = require_spec(root, task_id)
    snapshot = status_snapshot(root, spec, task_id)
    result = snapshot.get("result") or {}
    next_action = result.get("next_action") or {}
    current_handoff = result.get("current_handoff") or {}
    action_type = next_action.get("type")

    if action_type in {"phase_handoff", "recovery_handoff", "build"}:
        return render_adapter_handoff(
            root,
            task_id,
            role=current_handoff.get("role") or "executor",
            gate=current_handoff.get("gate") or "phase",
            selector=current_handoff.get("selector"),
        )
    if action_type == "review":
        review_payload, review_code = review_snapshot(root, task_id, use_color=False)
        if review_code != 0:
            error = review_payload.get("error") or {}
            raise ValueError(error.get("message") or f"review is blocked for {task_id}")
        review_result = review_payload.get("result") or {}
        return review_result.get("review_prompt") or "", review_result.get("handoff_file") or ""
    if action_type == "address_review_findings":
        handoff_file = current_handoff.get("handoff_file")
        if not handoff_file:
            raise ValueError("review findings are blocked, but no challenger handoff file is available")
        path = root / handoff_file
        if not path.exists():
            raise ValueError("review findings are blocked, but the challenger handoff file is missing")
        return path.read_text(), handoff_file
    if action_type == "complete":
        raise ValueError(f"review already passed; next step is 'scafld complete {task_id}'")
    if action_type == "human_required":
        raise ValueError("recovery is exhausted; a human must intervene before continuing")
    if action_type in {"harden", "approve"}:
        raise ValueError(next_action.get("message") or next_action.get("command") or "task is not executable yet")
    raise ValueError(f"no executable handoff is available for task '{task_id}'")


def review_prompt(root, task_id):
    payload, exit_code = review_snapshot(root, task_id, use_color=False)
    if exit_code != 0:
        error = payload.get("error") or {}
        raise ValueError(error.get("message") or f"review is blocked for {task_id}")
    result = payload.get("result") or {}
    return result.get("review_prompt") or "", result.get("handoff_file") or ""


def model_arg(provider_args):
    for index, arg in enumerate(provider_args):
        if arg in {"-m", "--model"} and index + 1 < len(provider_args):
            return provider_args[index + 1]
        if arg.startswith("--model="):
            return arg.split("=", 1)[1]
    return ""


def run_provider(root, provider, mode, task_id, provider_args):
    provider_label, env_name = resolve_provider(provider)
    provider_bin = os.environ.get(env_name, provider_label)
    if shutil.which(provider_bin) is None:
        raise ValueError(f"scafld-{provider_label}: '{provider_bin}' is not installed or not on PATH")

    if mode == "build":
        prompt, handoff_file = build_prompt(root, task_id)
    elif mode == "review":
        prompt, handoff_file = review_prompt(root, task_id)
    else:
        raise ValueError(f"unknown mode '{mode}'")

    print(f"scafld: feeding {handoff_file or 'generated handoff'} to {provider_label}", file=sys.stderr)
    started_at = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
    proc = subprocess.run(
        [provider_bin, *provider_args],
        input=(prompt if prompt.endswith("\n") else prompt + "\n"),
        text=True,
        cwd=str(root),
    )
    completed_at = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
    requested_model = model_arg(provider_args)
    record_provider_invocation(
        root,
        task_id,
        role="executor" if mode == "build" else "challenger",
        gate=mode,
        provider=provider_label,
        provider_bin=provider_bin,
        model_requested=requested_model,
        model_observed="",
        model_source="requested" if requested_model else "unknown",
        isolation_level="provider_adapter",
        status="completed" if proc.returncode == 0 else "failed",
        started_at=started_at,
        completed_at=completed_at,
        exit_code=proc.returncode,
    )
    return proc.returncode
