import sys

from scafld.command_runtime import require_root
from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.git_state import capture_review_git_state
from scafld.output import emit_command_json, error_payload
from scafld.review_artifacts import append_review_round, load_review_topology, upsert_review_block
from scafld.reviewing import (
    build_review_metadata,
    build_spec_review_block,
    normalize_review_pass_results,
    parse_review_file,
    review_git_gate_reason,
    review_pass_ids,
    review_passes_by_kind,
)
from scafld.review_workflow import (
    automated_pass_payload,
    check_self_eval,
    collect_automated_review_passes,
    confirm_human_override,
    render_adversarial_review_prompt,
    run_automated_review_pass,
)
from scafld.runtime_bundle import CONFIG_PATH, REVIEWS_DIR
from scafld.spec_lifecycle import move_result_payload
from scafld.spec_parsing import now_iso, parse_acceptance_criteria
from scafld.spec_store import move_spec, require_spec, yaml_read_field
from scafld.terminal import C_BOLD, C_CYAN, C_DIM, C_GREEN, C_RED, C_YELLOW, STATUS_COLORS, c


def print_move_result(root, move_result):
    transition = move_result_payload(root, move_result)
    print(f"{c(C_GREEN, '  moved')}: {transition['from']} -> {transition['to']}")
    print(f" {c(C_DIM, 'status')}: {c(STATUS_COLORS.get(transition['status'], ''), transition['status'])}")


def load_or_exit_review_topology(root):
    """Load the configured review topology or exit with a clear error."""
    try:
        return load_review_topology(root)
    except ValueError as exc:
        raise ScafldError(
            f"invalid review topology in {CONFIG_PATH}",
            [str(exc)],
        ) from exc


def review_passes_are_not_run(pass_results, pass_ids):
    return all(pass_results.get(pass_id) == "not_run" for pass_id in pass_ids)


def cmd_complete(args):
    """Move spec from active to archive (completed). Reads review file and gates on verdict."""
    root = require_root()
    topology = load_or_exit_review_topology(root)
    spec = require_spec(root, args.task_id)
    text = spec.read_text()
    json_mode = bool(getattr(args, "json", False))

    has_results = any(ac.get("result") in ("pass", "fail") for ac in parse_acceptance_criteria(text))
    if not has_results and not json_mode:
        print(f"  {c(C_YELLOW, 'warn')}: no exec results recorded. Run '{c(C_BOLD, f'scafld exec {args.task_id}')}' first")

    review_file = root / REVIEWS_DIR / f"{args.task_id}.md"
    human_reviewed = bool(getattr(args, "human_reviewed", False))
    override_reason = (getattr(args, "reason", "") or "").strip()

    if override_reason and not human_reviewed:
        if json_mode:
            emit_command_json(
                "complete",
                ok=False,
                task_id=args.task_id,
                state={"status": yaml_read_field(text, "status")},
                error=error_payload(
                    "--reason requires --human-reviewed",
                    code=EC.INVALID_ARGUMENTS,
                    exit_code=1,
                ),
            )
        else:
            raise ScafldError("--reason requires --human-reviewed")
        sys.exit(1)
    if human_reviewed and not override_reason:
        if json_mode:
            emit_command_json(
                "complete",
                ok=False,
                task_id=args.task_id,
                state={"status": yaml_read_field(text, "status")},
                error=error_payload(
                    "--reason is required with --human-reviewed",
                    code=EC.INVALID_ARGUMENTS,
                    exit_code=1,
                ),
            )
        else:
            raise ScafldError("--reason is required with --human-reviewed")
        sys.exit(1)

    review_data = parse_review_file(review_file, topology)
    verdict = review_data["verdict"]
    blocking = review_data["blocking"]
    non_blocking = review_data["non_blocking"]
    pass_results = review_data["pass_results"]
    review_metadata = review_data.get("metadata") or {}
    gate_errors = list(review_data["errors"])
    override_applied = False
    override_confirmed_at = None
    current_git_state = None

    empty_adversarial = review_data["empty_adversarial"]
    automated_pass_ids = review_pass_ids(topology, "automated")
    override_section_bodies = {
        definition["id"]: "Override applied — this pass was not re-reviewed in the override round."
        for definition in review_passes_by_kind(topology, "adversarial")
    }

    gate_reason = None
    if not review_data["exists"]:
        gate_reason = "no review found"
    elif empty_adversarial:
        gate_reason = f"configured review sections incomplete — missing: {', '.join(empty_adversarial)}"
    elif gate_errors:
        gate_reason = "latest review round is malformed or incomplete"
    elif verdict == "fail":
        gate_reason = f"latest review failed with {len(blocking)} blocking finding(s)"
    elif verdict in (None, "incomplete") or review_data["round_status"] == "in_progress":
        gate_reason = "latest review is incomplete"
    else:
        current_git_state, current_git_error = capture_review_git_state(root, review_file.relative_to(root))
        if current_git_error:
            gate_reason = "current git state is unavailable for review binding"
            gate_errors.append(f"git state: {current_git_error}")
        else:
            gate_reason = review_git_gate_reason(current_git_state, review_metadata)

    if gate_reason:
        if not human_reviewed:
            if json_mode:
                emit_command_json(
                    "complete",
                    ok=False,
                    task_id=args.task_id,
                    state={"status": yaml_read_field(text, "status"), "review_verdict": verdict},
                    result={
                        "review_file": str(review_file.relative_to(root)),
                        "blocking_count": len(blocking),
                        "non_blocking_count": len(non_blocking),
                        "pass_results": pass_results,
                        "review_errors": gate_errors,
                        "blocking": blocking,
                    },
                    error=error_payload(
                        gate_reason,
                        code=EC.REVIEW_GATE_BLOCKED,
                        details=gate_errors or blocking,
                        next_action=f"scafld review {args.task_id}" if not review_data["exists"] else None,
                        exit_code=1,
                    ),
                )
            else:
                print(f"  {c(C_RED, 'error')}: {gate_reason}")
                if not review_data["exists"]:
                    print(f"         run '{c(C_BOLD, f'scafld review {args.task_id}')}' first")
                elif gate_errors:
                    for issue in gate_errors[:5]:
                        print(f"         {c(C_DIM, issue)}")
                elif blocking:
                    for finding in blocking[:5]:
                        print(f"         {finding}")
                print(f"         only a human can override with {c(C_BOLD, '--human-reviewed --reason <why>')}")
            sys.exit(1)

        if not review_data["exists"] or review_passes_are_not_run(pass_results, automated_pass_ids):
            pass_results = collect_automated_review_passes(root, args.task_id, text, topology)

        if json_mode:
            emit_command_json(
                "complete",
                ok=False,
                task_id=args.task_id,
                state={"status": yaml_read_field(text, "status"), "review_verdict": verdict},
                error=error_payload(
                    "human-reviewed override requires interactive terminal output; rerun without --json",
                    code=EC.INTERACTIVE_REQUIRED,
                    exit_code=1,
                ),
            )
            sys.exit(1)

        override_confirmed_at = confirm_human_override(args.task_id, gate_reason)
        if current_git_state is None:
            current_git_state, current_git_error = capture_review_git_state(root, review_file.relative_to(root))
            if current_git_error:
                raise ScafldError(
                    "current git state is unavailable for review binding",
                    [current_git_error],
                )
        override_metadata = build_review_metadata(
            topology,
            reviewer_mode="human_override",
            round_status="override",
            pass_results=pass_results,
            reviewed_at=override_confirmed_at,
            reviewer_session="",
            override_reason=override_reason,
            review_git_state=current_git_state,
        )
        append_review_round(
            review_file,
            args.task_id,
            text,
            topology,
            override_metadata,
            verdict=verdict or "incomplete",
            blocking=blocking,
            non_blocking=non_blocking,
            section_bodies=override_section_bodies,
        )
        review_data = parse_review_file(review_file, topology)
        verdict = review_data["verdict"]
        blocking = review_data["blocking"]
        non_blocking = review_data["non_blocking"]
        pass_results = review_data["pass_results"]
        override_applied = True
        print(f"  {c(C_YELLOW, 'override')}: human-reviewed override applied")
    elif human_reviewed:
        if json_mode:
            emit_command_json(
                "complete",
                ok=False,
                task_id=args.task_id,
                state={"status": yaml_read_field(text, "status")},
                error=error_payload(
                    "--human-reviewed is only for blocked completion",
                    code=EC.INVALID_ARGUMENTS,
                    exit_code=1,
                ),
            )
        else:
            raise ScafldError("--human-reviewed is only for blocked completion")
        sys.exit(1)

    if not json_mode:
        if verdict == "fail":
            print(f"  {c(C_RED, 'review')}: FAIL — {len(blocking)} blocking finding(s)")
            for finding in blocking[:5]:
                print(f"    {finding}")
        elif verdict == "pass_with_issues":
            print(f"  {c(C_YELLOW, 'review')}: pass with {len(non_blocking)} non-blocking finding(s)")
            for finding in non_blocking[:5]:
                print(f"    {c(C_DIM, finding)}")
        elif verdict == "pass":
            print(f"  {c(C_GREEN, 'review')}: pass")
        else:
            print(f"  {c(C_YELLOW, 'review')}: incomplete")

    review_block = build_spec_review_block(
        review_data,
        topology,
        override_applied=override_applied,
        override_reason=override_reason if override_applied else None,
        override_confirmed_at=override_confirmed_at,
    )
    text = upsert_review_block(text, review_block)
    spec.write_text(text)

    if not json_mode:
        check_self_eval(text, args.task_id)

    move_result = move_spec(root, spec, "completed")
    dest = move_result.dest
    if json_mode:
        emit_command_json(
            "complete",
            task_id=args.task_id,
            state={"status": move_result.new_status, "review_verdict": verdict},
            result={
                "archive_path": str(dest.relative_to(root)),
                "blocking_count": len(blocking),
                "non_blocking_count": len(non_blocking),
                "pass_results": pass_results,
                "override_applied": override_applied,
                "review_round": review_data.get("review_count", 0),
                "review_file": str(review_file.relative_to(root)),
                "transition": move_result_payload(root, move_result),
            },
        )
    else:
        print_move_result(root, move_result)


def cmd_review(args):
    """Run automated review passes and generate adversarial review prompt."""
    root = require_root()
    topology = load_or_exit_review_topology(root)
    spec = require_spec(root, args.task_id)
    text = spec.read_text()
    json_mode = bool(getattr(args, "json", False))

    status = yaml_read_field(text, "status")
    if status != "in_progress":
        if json_mode:
            emit_command_json(
                "review",
                ok=False,
                task_id=args.task_id,
                state={"status": status},
                error=error_payload(
                    f"spec must be in_progress to review (current: {status})",
                    code=EC.INVALID_SPEC_STATUS,
                    exit_code=1,
                ),
            )
        else:
            print(f"{c(C_RED, 'error')}: spec must be in_progress to review (current: {status})")
        sys.exit(1)

    if not json_mode:
        print(f"{c(C_BOLD, f'Review: {args.task_id}')}")
        print()

    pass_results = {}
    automated_results = []
    automated_passes = review_passes_by_kind(topology, "automated")
    adversarial_passes = review_passes_by_kind(topology, "adversarial")

    for definition in automated_passes:
        if not json_mode:
            print(f"  {c(C_CYAN, definition['id'])}: {definition['description']}")
        outcome = run_automated_review_pass(root, args.task_id, text, definition["id"])
        pass_results[definition["id"]] = outcome["result"]
        automated_results.append(automated_pass_payload(definition, outcome))
        if not json_mode:
            if outcome["result"] == "pass":
                print(f"    {c(C_GREEN, 'PASS')}")
            else:
                print(f"    {c(C_RED, 'FAIL')}")
                for line in outcome["lines"][-5:]:
                    print(f"    {c(C_DIM, line)}")

    normalized_passes = normalize_review_pass_results(topology, pass_results)
    passed = sum(1 for definition in automated_passes if normalized_passes[definition["id"]] == "pass")
    failed = sum(1 for definition in automated_passes if normalized_passes[definition["id"]] == "fail")

    if not json_mode:
        print()
        summary_parts = []
        if passed:
            summary_parts.append(c(C_GREEN, f"{passed} passed"))
        if failed:
            summary_parts.append(c(C_RED, f"{failed} failed"))
        print(f"  Automated passes: {' / '.join(summary_parts)}")
        print()

    if failed:
        if json_mode:
            emit_command_json(
                "review",
                ok=False,
                task_id=args.task_id,
                state={"status": status},
                result={
                    "automated_passes": automated_results,
                    "failed_count": failed,
                },
                error=error_payload(
                    f"{failed} automated pass(es) failed",
                    code=EC.AUTOMATED_REVIEW_FAILED,
                    next_action=f"scafld review {args.task_id}",
                    exit_code=1,
                ),
            )
        else:
            print(f"  {c(C_RED, 'error')}: {failed} automated pass(es) failed — fix before reviewing")
            print(f"  resolve failures, then re-run: {c(C_BOLD, f'scafld review {args.task_id}')}")
        sys.exit(1)

    reviews_dir = root / REVIEWS_DIR
    reviews_dir.mkdir(parents=True, exist_ok=True)
    review_file = reviews_dir / f"{args.task_id}.md"
    review_git_state, review_git_error = capture_review_git_state(root, review_file.relative_to(root))
    if review_git_error:
        if json_mode:
            emit_command_json(
                "review",
                ok=False,
                task_id=args.task_id,
                state={"status": status},
                error=error_payload(
                    f"could not capture reviewed git state: {review_git_error}",
                    code=EC.REVIEW_GIT_STATE_UNAVAILABLE,
                    exit_code=1,
                ),
            )
        else:
            print(f"  {c(C_RED, 'error')}: could not capture reviewed git state")
            print(f"         {c(C_DIM, review_git_error)}")
        sys.exit(1)
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
        args.task_id,
        text,
        topology,
        review_metadata,
        verdict="",
        blocking=[],
        non_blocking=[],
    )

    review_path_rel = str(review_file.relative_to(root))
    spec_path_rel = str(spec.relative_to(root))
    review_prompt = render_adversarial_review_prompt(
        args.task_id,
        spec_path_rel,
        review_path_rel,
        review_count,
        adversarial_passes,
        use_color=not json_mode,
    )
    if json_mode:
        emit_command_json(
            "review",
            task_id=args.task_id,
            state={"status": status, "review_round": review_count},
            result={
                "review_file": review_path_rel,
                "review_prompt": review_prompt,
                "automated_passes": automated_results,
                "required_sections": [
                    "Metadata",
                    "Pass Results",
                    *[definition["title"] for definition in adversarial_passes],
                    "Blocking",
                    "Non-blocking",
                    "Verdict",
                ],
                "complete_command": f"scafld complete {args.task_id}",
            },
        )
    else:
        print(review_prompt)
