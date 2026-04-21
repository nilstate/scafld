import sys

from scafld.command_runtime import require_root
from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.output import emit_command_json, error_payload
from scafld.reviewing import (
    build_spec_review_block,
    parse_review_file,
)
from scafld.review_workflow import (
    apply_human_override,
    automated_review_pass_ids,
    check_self_eval,
    collect_automated_review_passes,
    confirm_human_override,
    evaluate_review_gate,
    load_configured_review_topology,
    open_review_round,
    review_passes_are_not_run,
    run_automated_review_suite,
    upsert_review_block,
)
from scafld.runtime_bundle import REVIEWS_DIR
from scafld.spec_parsing import parse_acceptance_criteria
from scafld.spec_store import move_spec, require_spec, yaml_read_field
from scafld.terminal import C_BOLD, C_CYAN, C_DIM, C_GREEN, C_RED, C_YELLOW, STATUS_COLORS, c


def move_result_payload(root, move_result):
    return {
        "from": str(move_result.source.relative_to(root)),
        "to": str(move_result.dest.relative_to(root)),
        "status": move_result.new_status,
    }


def print_move_result(root, move_result):
    transition = move_result_payload(root, move_result)
    print(f"{c(C_GREEN, '  moved')}: {transition['from']} -> {transition['to']}")
    print(f" {c(C_DIM, 'status')}: {c(STATUS_COLORS.get(transition['status'], ''), transition['status'])}")


def cmd_complete(args):
    """Move spec from active to archive (completed). Reads review file and gates on verdict."""
    root = require_root()
    topology = load_configured_review_topology(root)
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
    override_applied = False
    override_confirmed_at = None
    gate = evaluate_review_gate(root, review_file, review_data)
    gate_reason = gate["gate_reason"]
    gate_errors = gate["gate_errors"]
    current_git_state = gate["current_git_state"]

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

        if not review_data["exists"] or review_passes_are_not_run(pass_results, automated_review_pass_ids(topology)):
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
        review_data = apply_human_override(
            root,
            args.task_id,
            text,
            topology,
            review_file,
            review_data,
            pass_results,
            override_reason,
            current_git_state=current_git_state,
        )
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
    topology = load_configured_review_topology(root)
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

    suite = run_automated_review_suite(root, args.task_id, text, topology)
    automated_results = suite["automated_results"]
    automated_passes = suite["automated_passes"]
    normalized_passes = suite["normalized_passes"]
    passed = suite["passed"]
    failed = suite["failed"]

    for definition, outcome in zip(automated_passes, automated_results):
        if not json_mode:
            print(f"  {c(C_CYAN, definition['id'])}: {definition['description']}")
            if outcome["result"] == "pass":
                print(f"    {c(C_GREEN, 'PASS')}")
            else:
                print(f"    {c(C_RED, 'FAIL')}")
                for line in outcome["lines"][-5:]:
                    print(f"    {c(C_DIM, line)}")

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

    try:
        review_round = open_review_round(
            root,
            args.task_id,
            spec,
            text,
            topology,
            normalized_passes,
            use_color=not json_mode,
        )
    except ScafldError as exc:
        if json_mode:
            emit_command_json(
                "review",
                ok=False,
                task_id=args.task_id,
                state={"status": status},
                error=error_payload(
                    f"{exc.message}: {exc.details[0]}" if exc.details else exc.message,
                    code=EC.REVIEW_GIT_STATE_UNAVAILABLE,
                    exit_code=1,
                ),
            )
        else:
            print(f"  {c(C_RED, 'error')}: {exc.message}")
            for detail in exc.details:
                print(f"         {c(C_DIM, detail)}")
        sys.exit(1)
    if json_mode:
        emit_command_json(
            "review",
            task_id=args.task_id,
            state={"status": status, "review_round": review_round["review_count"]},
            result={
                "review_file": review_round["review_path_rel"],
                "review_prompt": review_round["review_prompt"],
                "automated_passes": automated_results,
                "required_sections": review_round["required_sections"],
                "complete_command": f"scafld complete {args.task_id}",
            },
        )
    else:
        print(review_round["review_prompt"])
