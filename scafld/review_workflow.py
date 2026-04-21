import re
import subprocess
import sys

from scafld.acceptance import evaluate_acceptance_criterion
from scafld.errors import ScafldError
from scafld.git_state import capture_review_git_state
from scafld.review_artifacts import append_review_round, load_review_topology
from scafld.reviewing import (
    build_review_metadata,
    normalize_review_pass_results,
    parse_review_file,
    review_git_gate_reason,
    review_pass_ids,
    review_passes_by_kind,
)
from scafld.spec_parsing import extract_self_eval_score, extract_spec_cwd, now_iso, parse_acceptance_criteria
from scafld.terminal import C_BOLD, C_CYAN, C_DIM, C_GREEN, C_RED, C_YELLOW, c

from .runtime_bundle import CONFIG_PATH, REVIEWS_DIR, scafld_source_root


def check_self_eval(text, task_id):
    """Warn if self-eval looks like a rubber stamp."""
    score = extract_self_eval_score(text)
    if score is None:
        print(f"  {c(C_YELLOW, 'warn')}: no self-eval score found in spec")
        return
    has_deviations = bool(re.search(r"deviations:", text, re.MULTILINE))
    has_improvements = bool(re.search(r"improvements:", text, re.MULTILINE))
    score_display = int(score) if float(score).is_integer() else score

    if score >= 9 and not has_deviations and not has_improvements:
        print(f"  {c(C_YELLOW, 'warn')}: self-eval {score_display}/10 with no deviations or improvements noted")
        print("         scores above 8 should document at least one deviation or improvement")
    elif score == 10:
        print(f"  {c(C_YELLOW, 'note')}: perfect 10/10 - are you sure? 10 means flawless with improvements beyond spec")


def automated_pass_payload(definition, outcome):
    return {
        "id": definition["id"],
        "title": definition["title"],
        "description": definition["description"],
        "result": outcome["result"],
        "lines": outcome.get("lines", []),
    }


def render_adversarial_review_prompt(task_id, spec_path_rel, review_path_rel, review_count, adversarial_passes, use_color=True):
    """Render the reviewer handoff prompt. JSON mode uses this as plain text."""

    def cc(code, text):
        return c(code, text) if use_color else text

    attack_vectors = []
    section_titles = []
    for idx, definition in enumerate(adversarial_passes, start=1):
        title = definition["title"]
        prompt = definition["prompt"]
        section_titles.append(f"`{definition['id']}` / `### {title}`")
        attack_vectors.append(
            f"    {cc(C_CYAN, f'{idx}. {title}')} — {prompt}\n"
            f"       {cc(C_DIM, f'Write findings under ### {title}')}"
        )
    attack_vectors_text = "\n\n".join(attack_vectors)
    section_titles_text = ", ".join(section_titles)

    return f"""{cc(C_BOLD, 'ADVERSARIAL REVIEW')}

  Your job is to find problems in the changes made for this spec. Not confirm
  success — find what's wrong. A review that finds zero issues is suspicious.

  {cc(C_BOLD, 'Read:')}
    - Spec: {cc(C_CYAN, spec_path_rel)}
    - Git diff of all changes made during execution
    - Project CONVENTIONS.md and AGENTS.md
    - Prior review rounds in {cc(C_CYAN, review_path_rel)} (if any — don't re-report fixed issues)

  {cc(C_BOLD, 'Configured review passes (all required — scafld complete will reject empty sections):')}

{attack_vectors_text}

  {cc(C_BOLD, 'Rules:')}
    - Every finding must cite a specific file and line number
    - Classify as blocking (must fix) or non-blocking (should fix)
    - Do not suggest improvements — only flag defects and omissions
    - Do not modify any code — review only
    - Each adversarial section must contain at least one finding or an explicit
      "No issues found" with a brief explanation of what was checked

  {cc(C_BOLD, 'Severity:')} critical (runtime/security) > high (common case) > medium (edge case) > low (smell)

  {cc(C_BOLD, 'Write findings to:')} {cc(C_CYAN, review_path_rel)}
    The latest review section (Review {review_count}) is already scaffolded with
    all required headings. Fill in each section:

    1. Update {cc(C_BOLD, '### Metadata')} JSON with reviewer provenance:
       - round_status: completed
       - reviewer_mode: fresh_agent | auto | executor
       - reviewer_session: session identifier or empty string
       - keep reviewed_head / reviewed_dirty / reviewed_diff unchanged
       - pass_results: keep automated results, and set each adversarial pass to pass | pass_with_issues | fail
    2. Fill {cc(C_BOLD, section_titles_text)} with findings from each vector
    3. Collect blocking/non-blocking findings into {cc(C_BOLD, '### Blocking')} and
       {cc(C_BOLD, '### Non-blocking')} using: - **severity** `file:line` — description
    4. Set {cc(C_BOLD, '### Verdict')}: pass | fail | pass_with_issues

  When done: {cc(C_BOLD, f'scafld complete {task_id}')}
"""


def confirm_human_override(task_id, gate_reason):
    """Require interactive confirmation before allowing a human-reviewed override."""
    if not sys.stdin.isatty():
        print(f"  {c(C_RED, 'error')}: {c(C_BOLD, '--human-reviewed')} requires an interactive terminal")
        sys.exit(1)

    print(f"  {c(C_YELLOW, 'review gate blocked')}: {gate_reason}")
    try:
        confirm = input(f"  Type '{task_id}' to confirm a human-reviewed override: ").strip()
    except EOFError:
        print(f"  {c(C_RED, 'error')}: confirmation aborted")
        sys.exit(1)

    if confirm != task_id:
        print(f"  {c(C_RED, 'error')}: confirmation failed")
        sys.exit(1)
    return now_iso()


def run_spec_compliance_check(root, text):
    """Run acceptance criteria in read-only review mode."""
    spec_cwd = extract_spec_cwd(text)
    criteria = parse_acceptance_criteria(text)
    if not criteria:
        return {"result": "pass", "lines": ["no acceptance criteria found"]}

    before = len(criteria)
    criteria = [criterion for criterion in criteria if criterion.get("result") != "pass"]
    skipped_resume = before - len(criteria)
    runnable = [criterion for criterion in criteria if criterion.get("command") and criterion["command"] != "TODO"]
    manual = [criterion for criterion in criteria if not criterion.get("command") or criterion["command"] == "TODO"]

    if not runnable and not manual:
        lines = [f"resume: skipping {skipped_resume} already-passed criteria"] if skipped_resume else ["no runnable criteria found"]
        return {"result": "pass", "lines": lines}

    failure_lines = []
    for criterion in runnable:
        outcome = evaluate_acceptance_criterion(root, criterion, spec_cwd=spec_cwd)
        if outcome["status"] == "pass":
            continue
        failure_lines.append(f"{outcome['id']}: exit {outcome['exit_code']}" if outcome["exit_code"] is not None else f"{outcome['id']}: {outcome['output']}")
        if outcome.get("expected"):
            failure_lines.append(f"{outcome['id']}: expected {outcome['expected']}")
        for line in outcome["output"].splitlines()[:3]:
            failure_lines.append(f"{outcome['id']}: {line}")

    if failure_lines:
        return {"result": "fail", "lines": failure_lines}

    lines = []
    if skipped_resume:
        lines.append(f"resume: skipping {skipped_resume} already-passed criteria")
    if manual:
        lines.append(f"manual criteria skipped: {len(manual)}")
    if not lines:
        lines.append("all runnable criteria passed")
    return {"result": "pass", "lines": lines}


def run_scope_drift_check(root, task_id):
    """Run scope drift in read-only review mode."""
    try:
        result = subprocess.run(
            [sys.executable, str(scafld_source_root() / "cli" / "scafld"), "audit", task_id],
            capture_output=True,
            text=True,
            timeout=60,
            cwd=str(root),
        )
    except (subprocess.TimeoutExpired, Exception) as exc:
        return {"result": "fail", "lines": [str(exc)]}

    output = (result.stdout + result.stderr).strip()
    lines = output.splitlines()[-5:] if output else []
    if result.returncode == 0:
        return {"result": "pass", "lines": lines}
    return {"result": "fail", "lines": lines}


def run_automated_review_pass(root, task_id, text, pass_id):
    """Run one built-in automated review pass."""
    if pass_id == "spec_compliance":
        return run_spec_compliance_check(root, text)
    if pass_id == "scope_drift":
        return run_scope_drift_check(root, task_id)
    return {"result": "fail", "lines": [f"unknown automated review pass: {pass_id}"]}


def collect_automated_review_passes(root, task_id, text, topology):
    """Collect the configured automated review pass states without mutating the spec."""
    pass_results = {}
    for definition in review_passes_by_kind(topology, "automated"):
        outcome = run_automated_review_pass(root, task_id, text, definition["id"])
        pass_results[definition["id"]] = outcome["result"]
    return normalize_review_pass_results(topology, pass_results)


def load_configured_review_topology(root):
    try:
        return load_review_topology(root)
    except ValueError as exc:
        raise ScafldError(
            f"invalid review topology in {CONFIG_PATH}",
            [str(exc)],
        ) from exc


def review_passes_are_not_run(pass_results, pass_ids):
    return all(pass_results.get(pass_id) == "not_run" for pass_id in pass_ids)


def automated_review_pass_ids(topology):
    return review_pass_ids(topology, "automated")


def run_automated_review_suite(root, task_id, text, topology):
    automated_results = []
    pass_results = {}
    automated_passes = review_passes_by_kind(topology, "automated")
    adversarial_passes = review_passes_by_kind(topology, "adversarial")

    for definition in automated_passes:
        outcome = run_automated_review_pass(root, task_id, text, definition["id"])
        pass_results[definition["id"]] = outcome["result"]
        automated_results.append(automated_pass_payload(definition, outcome))

    normalized_passes = normalize_review_pass_results(topology, pass_results)
    passed = sum(1 for definition in automated_passes if normalized_passes[definition["id"]] == "pass")
    failed = sum(1 for definition in automated_passes if normalized_passes[definition["id"]] == "fail")

    return {
        "automated_results": automated_results,
        "automated_passes": automated_passes,
        "adversarial_passes": adversarial_passes,
        "normalized_passes": normalized_passes,
        "passed": passed,
        "failed": failed,
    }


def open_review_round(root, task_id, spec, text, topology, normalized_passes, *, use_color=True):
    reviews_dir = root / REVIEWS_DIR
    reviews_dir.mkdir(parents=True, exist_ok=True)
    review_file = reviews_dir / f"{task_id}.md"
    review_git_state, review_git_error = capture_review_git_state(root, review_file.relative_to(root))
    if review_git_error:
        raise ScafldError(
            "could not capture reviewed git state",
            [review_git_error],
        )

    review_metadata = build_review_metadata(
        topology,
        reviewer_mode="executor",
        round_status="in_progress",
        pass_results=normalized_passes,
        reviewed_at=now_iso(),
        reviewer_session="",
        review_git_state=review_git_state,
    )
    review_count = append_review_round(
        review_file,
        task_id,
        text,
        topology,
        review_metadata,
        verdict="",
        blocking=[],
        non_blocking=[],
    )

    review_path_rel = str(review_file.relative_to(root))
    spec_path_rel = str(spec.relative_to(root))
    adversarial_passes = review_passes_by_kind(topology, "adversarial")
    review_prompt = render_adversarial_review_prompt(
        task_id,
        spec_path_rel,
        review_path_rel,
        review_count,
        adversarial_passes,
        use_color=use_color,
    )
    return {
        "review_file": review_file,
        "review_path_rel": review_path_rel,
        "review_count": review_count,
        "review_prompt": review_prompt,
        "required_sections": [
            "Metadata",
            "Pass Results",
            *[definition["title"] for definition in adversarial_passes],
            "Blocking",
            "Non-blocking",
            "Verdict",
        ],
    }


def evaluate_review_gate(root, review_file, review_data):
    gate_errors = list(review_data["errors"])
    current_git_state = None
    review_metadata = review_data.get("metadata") or {}

    gate_reason = None
    if not review_data["exists"]:
        gate_reason = "no review found"
    elif review_data["empty_adversarial"]:
        gate_reason = f"configured review sections incomplete — missing: {', '.join(review_data['empty_adversarial'])}"
    elif gate_errors:
        gate_reason = "latest review round is malformed or incomplete"
    elif review_data["verdict"] == "fail":
        gate_reason = f"latest review failed with {len(review_data['blocking'])} blocking finding(s)"
    elif review_data["verdict"] in (None, "incomplete") or review_data["round_status"] == "in_progress":
        gate_reason = "latest review is incomplete"
    else:
        current_git_state, current_git_error = capture_review_git_state(root, review_file.relative_to(root))
        if current_git_error:
            gate_reason = "current git state is unavailable for review binding"
            gate_errors.append(f"git state: {current_git_error}")
        else:
            gate_reason = review_git_gate_reason(current_git_state, review_metadata)

    return {
        "gate_reason": gate_reason,
        "gate_errors": gate_errors,
        "current_git_state": current_git_state,
        "review_metadata": review_metadata,
    }


def apply_human_override(root, task_id, text, topology, review_file, review_data, pass_results, override_reason, current_git_state=None):
    if current_git_state is None:
        current_git_state, current_git_error = capture_review_git_state(root, review_file.relative_to(root))
        if current_git_error:
            raise ScafldError(
                "current git state is unavailable for review binding",
                [current_git_error],
            )

    override_section_bodies = {
        definition["id"]: "Override applied — this pass was not re-reviewed in the override round."
        for definition in review_passes_by_kind(topology, "adversarial")
    }
    override_metadata = build_review_metadata(
        topology,
        reviewer_mode="human_override",
        round_status="override",
        pass_results=pass_results,
        reviewed_at=now_iso(),
        reviewer_session="",
        override_reason=override_reason,
        review_git_state=current_git_state,
    )
    append_review_round(
        review_file,
        task_id,
        text,
        topology,
        override_metadata,
        verdict=review_data["verdict"] or "incomplete",
        blocking=review_data["blocking"],
        non_blocking=review_data["non_blocking"],
        section_bodies=override_section_bodies,
    )
    return parse_review_file(review_file, topology)
