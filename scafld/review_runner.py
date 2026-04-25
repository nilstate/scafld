import hashlib
import json
import os
import re
import shutil
import subprocess
import tempfile
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.reviewing import review_pass_ids, review_passes_by_kind
from scafld.runtime_bundle import CONFIG_PATH, load_runtime_config


REVIEW_RUNNER_VALUES = ("external", "local", "manual")
REVIEW_PROVIDER_VALUES = ("auto", "codex", "claude")
EXTERNAL_REVIEWER_MODES = {"fresh_agent", "auto"}
EXTERNAL_REVIEW_VERDICTS = {"pass", "fail", "pass_with_issues"}


@dataclass(frozen=True)
class ReviewRunnerConfig:
    runner: str
    provider: str
    model: str


@dataclass(frozen=True)
class ResolvedReviewRunner:
    runner: str
    provider: str | None
    model: str


@dataclass(frozen=True)
class ExternalReviewResult:
    reviewer_mode: str
    reviewer_session: str
    reviewer_isolation: str
    pass_results: dict[str, str]
    sections: dict[str, str]
    blocking: list[str]
    non_blocking: list[str]
    verdict: str
    provenance: dict


def _review_config(root):
    config = load_runtime_config(root)
    review_config = config.get("review")
    if not isinstance(review_config, dict):
        raise ValueError(f"{CONFIG_PATH}: review must be a mapping")
    return review_config


def _normalize_choice(value, *, allowed, field):
    normalized = str(value or "").strip()
    if not normalized:
        raise ValueError(f"{CONFIG_PATH}: review.{field} must be one of: {', '.join(allowed)}")
    if normalized not in allowed:
        raise ValueError(f"{CONFIG_PATH}: review.{field} must be one of: {', '.join(allowed)}")
    return normalized


def _normalize_model(value):
    return str(value or "").strip()


def _provider_model(external_config, provider):
    if provider == "auto":
        return ""
    provider_entry = external_config.get(provider) or {}
    if provider_entry and not isinstance(provider_entry, dict):
        raise ValueError(f"{CONFIG_PATH}: review.external.{provider} must be a mapping")
    return _normalize_model(provider_entry.get("model", ""))


def configured_provider_model(root, provider):
    review_config = _review_config(root)
    external_config = review_config.get("external") or {}
    if external_config and not isinstance(external_config, dict):
        raise ValueError(f"{CONFIG_PATH}: review.external must be a mapping")
    return _provider_model(external_config, provider)


def load_review_runner_config(root):
    review_config = _review_config(root)
    runner = _normalize_choice(
        review_config.get("runner", "external"),
        allowed=REVIEW_RUNNER_VALUES,
        field="runner",
    )

    external_config = review_config.get("external") or {}
    if external_config and not isinstance(external_config, dict):
        raise ValueError(f"{CONFIG_PATH}: review.external must be a mapping")

    provider = _normalize_choice(
        external_config.get("provider", "auto"),
        allowed=REVIEW_PROVIDER_VALUES,
        field="external.provider",
    )

    provider_model = _provider_model(external_config, provider)

    return ReviewRunnerConfig(runner=runner, provider=provider, model=provider_model)


def resolve_review_runner(root, *, runner_override=None, provider_override=None, model_override=None):
    review_config = _review_config(root)
    external_config = review_config.get("external") or {}
    if external_config and not isinstance(external_config, dict):
        raise ValueError(f"{CONFIG_PATH}: review.external must be a mapping")

    config = load_review_runner_config(root)
    runner = config.runner
    if runner_override is not None:
        runner = _normalize_choice(
            runner_override,
            allowed=REVIEW_RUNNER_VALUES,
            field="runner override",
        )

    if runner != "external":
        if provider_override is not None:
            raise ValueError("--provider is only valid with --runner external")
        if model_override is not None:
            raise ValueError("--model is only valid with --runner external")
        return ResolvedReviewRunner(runner=runner, provider=None, model="")

    provider = config.provider
    if provider_override is not None:
        provider = _normalize_choice(
            provider_override,
            allowed=REVIEW_PROVIDER_VALUES,
            field="provider override",
        )

    if model_override is not None:
        model = _normalize_model(model_override)
    else:
        model = _provider_model(external_config, provider)
    return ResolvedReviewRunner(runner=runner, provider=provider, model=model)


def _provider_env_name(provider):
    if provider == "codex":
        return "SCAFLD_CODEX_BIN"
    if provider == "claude":
        return "SCAFLD_CLAUDE_BIN"
    raise ValueError(f"unknown provider '{provider}'")


def _provider_binary(provider):
    env_name = _provider_env_name(provider)
    return os.environ.get(env_name, provider), env_name


def resolve_external_provider(provider):
    candidates = ("codex", "claude") if provider == "auto" else (provider,)
    for candidate in candidates:
        provider_bin, env_name = _provider_binary(candidate)
        if shutil.which(provider_bin) is not None:
            return candidate, provider_bin, env_name
    if provider == "auto":
        raise ScafldError(
            "no external review provider is installed",
            [
                "expected `codex` first or `claude` as fallback on PATH",
                "use --runner local or --runner manual for an explicit degraded path",
            ],
            code=EC.MISSING_DEPENDENCY,
        )
    provider_bin, _env_name = _provider_binary(provider)
    raise ScafldError(
        f"external review provider '{provider_bin}' is not installed or not on PATH",
        ["use --runner local or --runner manual for an explicit degraded path"],
        code=EC.MISSING_DEPENDENCY,
    )


def _strip_json_fence(text):
    stripped = text.strip()
    match = re.fullmatch(r"```(?:json)?\s*(.*?)\s*```", stripped, re.DOTALL)
    if match:
        return match.group(1).strip()
    return stripped


def _extract_json_payload(text):
    stripped = _strip_json_fence(text)
    if not stripped:
        raise ScafldError("external reviewer returned no content", code=EC.COMMAND_FAILED)

    try:
        return json.loads(stripped)
    except json.JSONDecodeError:
        start = stripped.find("{")
        end = stripped.rfind("}")
        if start != -1 and end != -1 and end > start:
            try:
                return json.loads(stripped[start:end + 1])
            except json.JSONDecodeError:
                pass
    raise ScafldError(
        "external reviewer did not return valid JSON",
        ["review runner output must be a single JSON object"],
        code=EC.COMMAND_FAILED,
    )


def _validate_external_result(payload, topology):
    if not isinstance(payload, dict):
        raise ScafldError("external reviewer returned a non-object payload", code=EC.COMMAND_FAILED)

    pass_ids = review_pass_ids(topology, "adversarial")
    pass_results = payload.get("pass_results")
    if not isinstance(pass_results, dict):
        raise ScafldError("external reviewer payload is missing pass_results", code=EC.COMMAND_FAILED)
    missing_passes = sorted(set(pass_ids) - set(pass_results))
    unexpected_passes = sorted(set(pass_results) - set(pass_ids))
    if missing_passes or unexpected_passes:
        details = []
        if missing_passes:
            details.append(f"missing adversarial pass results: {', '.join(missing_passes)}")
        if unexpected_passes:
            details.append(f"unexpected adversarial pass results: {', '.join(unexpected_passes)}")
        raise ScafldError("external reviewer returned the wrong pass result set", details, code=EC.COMMAND_FAILED)
    for pass_id in pass_ids:
        value = pass_results.get(pass_id)
        if value not in {"pass", "pass_with_issues", "fail"}:
            raise ScafldError(
                f"external reviewer returned invalid pass_results.{pass_id}",
                [f"expected one of: pass, pass_with_issues, fail; got {value!r}"],
                code=EC.COMMAND_FAILED,
            )

    sections = payload.get("sections")
    if not isinstance(sections, dict):
        raise ScafldError("external reviewer payload is missing sections", code=EC.COMMAND_FAILED)
    missing_sections = sorted(set(pass_ids) - set(sections))
    unexpected_sections = sorted(set(sections) - set(pass_ids))
    if missing_sections or unexpected_sections:
        details = []
        if missing_sections:
            details.append(f"missing adversarial sections: {', '.join(missing_sections)}")
        if unexpected_sections:
            details.append(f"unexpected adversarial sections: {', '.join(unexpected_sections)}")
        raise ScafldError("external reviewer returned the wrong section set", details, code=EC.COMMAND_FAILED)
    normalized_sections = {}
    for pass_id in pass_ids:
        body = sections.get(pass_id)
        if not isinstance(body, str) or not body.strip():
            raise ScafldError(
                f"external reviewer returned an empty section for {pass_id}",
                code=EC.COMMAND_FAILED,
            )
        normalized_sections[pass_id] = body.strip()

    def normalize_bucket(name):
        bucket = payload.get(name)
        if bucket in (None, ""):
            return []
        if not isinstance(bucket, list):
            raise ScafldError(f"external reviewer payload {name} must be a list", code=EC.COMMAND_FAILED)
        normalized = []
        for item in bucket:
            if not isinstance(item, str) or not item.strip():
                raise ScafldError(f"external reviewer payload {name} must contain non-empty strings", code=EC.COMMAND_FAILED)
            normalized.append(item.strip())
        return normalized

    verdict = payload.get("verdict")
    if verdict not in EXTERNAL_REVIEW_VERDICTS:
        raise ScafldError(
            "external reviewer returned an invalid verdict",
            [f"expected one of: {', '.join(sorted(EXTERNAL_REVIEW_VERDICTS))}"],
            code=EC.COMMAND_FAILED,
        )

    reviewer_mode = str(payload.get("reviewer_mode") or "fresh_agent").strip()
    if reviewer_mode not in EXTERNAL_REVIEWER_MODES:
        raise ScafldError(
            "external reviewer returned an invalid reviewer_mode",
            [f"expected one of: {', '.join(sorted(EXTERNAL_REVIEWER_MODES))}"],
            code=EC.COMMAND_FAILED,
        )

    reviewer_session = str(payload.get("reviewer_session") or "").strip()
    return {
        "reviewer_mode": reviewer_mode,
        "reviewer_session": reviewer_session,
        "pass_results": {pass_id: str(pass_results[pass_id]) for pass_id in pass_ids},
        "sections": normalized_sections,
        "blocking": normalize_bucket("blocking"),
        "non_blocking": normalize_bucket("non_blocking"),
        "verdict": verdict,
    }


def build_external_review_prompt(review_prompt, topology):
    adversarial = review_passes_by_kind(topology, "adversarial")
    pass_ids = ", ".join(definition["id"] for definition in adversarial)
    pass_shape = ",\n".join(f'    "{definition["id"]}": "pass"' for definition in adversarial)
    section_shape = ",\n".join(
        f'    "{definition["id"]}": "No issues found — checked ..."' for definition in adversarial
    )
    return (
        f"{review_prompt.rstrip()}\n\n"
        "Return only JSON.\n"
        "Do not edit any files.\n"
        "Use this exact shape:\n"
        "{\n"
        '  "reviewer_mode": "fresh_agent",\n'
        '  "reviewer_session": "",\n'
        '  "pass_results": {\n'
        f"{pass_shape}\n"
        "  },\n"
        '  "sections": {\n'
        f"{section_shape}\n"
        "  },\n"
        '  "blocking": [],\n'
        '  "non_blocking": [],\n'
        '  "verdict": "pass"\n'
        "}\n"
        "Rules:\n"
        f"- pass_results keys must be exactly: {pass_ids}\n"
        f"- sections keys must be exactly: {pass_ids}\n"
        '- if a section is clean, use "No issues found — checked ..." and say what you checked\n'
        "- blocking and non_blocking items must already be final finding lines using '- **severity** `file:line` — explanation'\n"
        "- verdict must be pass, fail, or pass_with_issues\n"
    )


def _codex_args(root, output_path, model):
    args = [
        "exec",
        "--sandbox",
        "read-only",
        "--skip-git-repo-check",
        "--cd",
        str(root),
        "--ephemeral",
        "--ignore-user-config",
        "--ignore-rules",
        "--color",
        "never",
        "-o",
        str(output_path),
    ]
    if model:
        args.extend(["-m", model])
    return args


def _claude_args(model):
    args = [
        "-p",
        "--output-format",
        "text",
        "--allowedTools",
        "Read,Grep,Glob",
    ]
    if model:
        args.extend(["--model", model])
    return args


def run_external_review(root, task_id, review_prompt, topology, resolved_runner):
    provider, provider_bin, env_name = resolve_external_provider(resolved_runner.provider or "auto")
    model = resolved_runner.model or configured_provider_model(root, provider)
    prompt = build_external_review_prompt(review_prompt, topology)
    started_at = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
    raw_output = ""
    return_code = 0

    with tempfile.NamedTemporaryFile(prefix=f"scafld-review-{task_id}-", suffix=".txt", delete=False) as tmp:
        output_path = Path(tmp.name)
    try:
        if provider == "codex":
            argv = [provider_bin, *_codex_args(root, output_path, model)]
            proc = subprocess.run(
                argv,
                input=(prompt if prompt.endswith("\n") else prompt + "\n"),
                text=True,
                capture_output=True,
                cwd=str(root),
            )
            return_code = proc.returncode
            raw_output = output_path.read_text() if output_path.exists() else proc.stdout
        else:
            argv = [provider_bin, *_claude_args(model)]
            proc = subprocess.run(
                argv,
                input=(prompt if prompt.endswith("\n") else prompt + "\n"),
                text=True,
                capture_output=True,
                cwd=str(root),
            )
            return_code = proc.returncode
            raw_output = proc.stdout
        stderr = (proc.stderr or "").strip()
        if return_code != 0:
            details = [stderr] if stderr else []
            raise ScafldError(
                f"external review runner failed via {provider}",
                details,
                code=EC.COMMAND_FAILED,
            )
    finally:
        try:
            output_path.unlink()
        except OSError:
            pass

    completed_at = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
    normalized = _validate_external_result(_extract_json_payload(raw_output), topology)
    provenance = {
        "runner": "external",
        "provider": provider,
        "provider_bin": provider_bin,
        "provider_env": env_name,
        "model": model,
        "started_at": started_at,
        "completed_at": completed_at,
        "exit_code": return_code,
        "response_sha256": hashlib.sha256(raw_output.encode("utf-8")).hexdigest(),
    }
    return ExternalReviewResult(
        reviewer_mode=normalized["reviewer_mode"],
        reviewer_session=normalized["reviewer_session"],
        reviewer_isolation="fresh_process_subprocess",
        pass_results=normalized["pass_results"],
        sections=normalized["sections"],
        blocking=normalized["blocking"],
        non_blocking=normalized["non_blocking"],
        verdict=normalized["verdict"],
        provenance=provenance,
    )
