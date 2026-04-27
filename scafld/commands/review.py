import re
import sys

from scafld.command_runtime import require_root
from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.output import emit_command_json, error_payload
from scafld.review_runner import resolve_review_runner, run_external_review
from scafld.review_runtime import review_snapshot
from scafld.reviewing import (
    build_spec_review_block,
    parse_review_file,
)
from scafld.review_workflow import (
    apply_human_override,
    automated_review_pass_ids,
    check_self_eval,
    complete_review_round_from_result,
    collect_automated_review_passes,
    confirm_human_override,
    evaluate_review_gate,
    load_configured_review_topology,
    review_passes_are_not_run,
    upsert_review_block,
)
from scafld.runtime_bundle import REVIEWS_DIR
from scafld.runtime_guidance import existing_review_handoff
from scafld.runtime_contracts import archive_run_artifacts
from scafld.session_store import ensure_session, record_challenge_verdict, record_human_override, record_provider_invocation
from scafld.spec_parsing import parse_acceptance_criteria
from scafld.spec_store import move_spec, require_spec, yaml_read_field
from scafld.terminal import C_BOLD, C_CYAN, C_DIM, C_GREEN, C_RED, C_YELLOW, STATUS_COLORS, c


def move_result_payload(root, move_result):
    return {
        "from": str(move_result.source.relative_to(root)),
        "to": str(move_result.dest.relative_to(root)),
        "status": move_result.new_status,
    }


def diagnostic_path_from_details(details):
    for detail in details or []:
        if isinstance(detail, str) and detail.startswith("diagnostic: "):
            return detail.split("diagnostic: ", 1)[1]
    return ""


def record_external_review_invocation(root, task_id, provenance, *, status, diagnostic_path=""):
    record_provider_invocation(
        root,
        task_id,
        invocation_id=provenance.get("invocation_id") or None,
        role="challenger",
        gate="review",
        provider=provenance.get("provider") or "unknown",
        provider_bin=provenance.get("provider_bin") or provenance.get("provider") or "unknown",
        provider_requested=provenance.get("provider_requested") or provenance.get("provider") or "unknown",
        model_requested=provenance.get("model_requested") or "",
        model_observed=provenance.get("model_observed") or "",
        model_source=provenance.get("model_source") or "unknown",
        isolation_level=provenance.get("isolation_level") or "",
        isolation_downgraded=bool(provenance.get("isolation_downgraded")),
        fallback_policy=provenance.get("fallback_policy") or "",
        status=status,
        started_at=provenance.get("started_at") or "",
        completed_at=provenance.get("completed_at") or "",
        exit_code=provenance.get("exit_code"),
        timed_out=bool(provenance.get("timed_out")),
        timeout_seconds=provenance.get("timeout_seconds"),
        pid=provenance.get("pid"),
        provider_session_requested=provenance.get("provider_session_requested") or "",
        provider_session_observed=provenance.get("provider_session_observed") or "",
        command=provenance.get("command") or "",
        diagnostic_path=diagnostic_path,
        warning=provenance.get("warning") or "",
        review_packet=provenance.get("review_packet") or "",
        repair_handoff=provenance.get("repair_handoff") or "",
        repair_handoff_json=provenance.get("repair_handoff_json") or "",
    )


def print_move_result(root, move_result):
    transition = move_result_payload(root, move_result)
    print(f"{c(C_GREEN, '  moved')}: {transition['from']} -> {transition['to']}")
    print(f" {c(C_DIM, 'status')}: {c(STATUS_COLORS.get(transition['status'], ''), transition['status'])}")


def normalize_completed_archive_truth(text):
    """Stamp terminal completion truth into the spec before archival."""
    return _rewrite_completed_dod_statuses(_rewrite_completed_phase_statuses(text))


def _rewrite_completed_phase_statuses(text):
    lines = text.splitlines(True)
    result = []
    i = 0
    in_phases = False

    while i < len(lines):
        line = lines[i]
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())

        if not in_phases:
            result.append(line)
            if re.match(r"^phases:\s*$", line):
                in_phases = True
            i += 1
            continue

        if stripped and indent == 0 and not stripped.startswith("- "):
            in_phases = False
            continue

        match = re.match(r'^(\s*)-\s+id:\s*"?(phase\d+)"?\s*$', line)
        if not match:
            result.append(line)
            i += 1
            continue

        item_indent = len(match.group(1))
        field_indent = " " * (item_indent + 2)
        result.append(line)
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
            if field_indent_level <= item_indent:
                break
            if field_indent_level == item_indent and field_line.strip().startswith("- "):
                break

            if field_indent_level == len(field_indent) and re.match(
                r'^\s+status:\s*"?(pending|in_progress|completed|failed|skipped)"?\s*$',
                field_line,
            ):
                if insert_at is None:
                    insert_at = len(preserved)
                i += 1
                continue

            preserved.append(field_line)
            i += 1

        if insert_at is None:
            insert_at = len(preserved)
        preserved[insert_at:insert_at] = [f'{field_indent}status: "completed"\n']
        result.extend(preserved)

    return "".join(result)


def _rewrite_completed_dod_statuses(text):
    lines = text.splitlines(True)
    result = []
    i = 0
    in_definition_of_done = False
    block_indent = 0

    while i < len(lines):
        line = lines[i]
        stripped = line.strip()
        indent = len(line) - len(line.lstrip())

        if not in_definition_of_done:
            result.append(line)
            if re.match(r"^\s*definition_of_done:\s*$", line):
                in_definition_of_done = True
                block_indent = indent
            i += 1
            continue

        if stripped and indent <= block_indent:
            in_definition_of_done = False
            continue

        match = re.match(r'^\s*-\s+id:\s*"?(dod\d+)"?\s*$', line)
        if not match:
            result.append(line)
            i += 1
            continue

        item_indent = len(line) - len(line.lstrip())
        field_indent = " " * (item_indent + 2)
        result.append(line)
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
            if field_indent_level <= item_indent:
                break
            if field_indent_level == item_indent and field_line.strip().startswith("- "):
                break

            if field_indent_level == len(field_indent) and re.match(
                r'^\s+status:\s*"?(pending|in_progress|done)"?\s*$',
                field_line,
            ):
                if insert_at is None:
                    insert_at = len(preserved)
                i += 1
                continue

            preserved.append(field_line)
            i += 1

        if insert_at is None:
            insert_at = len(preserved)
        preserved[insert_at:insert_at] = [f'{field_indent}status: "done"\n']
        result.extend(preserved)

    return "".join(result)


def cmd_complete(args):
    """Move spec from active to archive (completed). Reads review file and gates on verdict."""
    root = require_root()
    topology = load_configured_review_topology(root)
    spec = require_spec(root, args.task_id)
    text = spec.read_text()
    json_mode = bool(getattr(args, "json", False))

    has_results = any(ac.get("result") in ("pass", "fail") for ac in parse_acceptance_criteria(text))
    if not has_results and not json_mode:
        print(f"  {c(C_YELLOW, 'warn')}: no build results recorded. Run '{c(C_BOLD, f'scafld build {args.task_id}')}' first")

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
    override_ready = (
        review_data["exists"]
        and review_data.get("review_count", 0) > 0
        and review_data.get("round_status") == "completed"
        and verdict is not None
    )
    session = ensure_session(root, args.task_id, spec_path=spec)
    review_metadata = review_data.get("metadata") or {}
    if review_data["exists"] and verdict is not None:
        session = record_challenge_verdict(
            root,
            args.task_id,
            gate="review",
            review_round=review_data.get("review_count", 0),
            verdict=verdict,
            blocked=bool(gate_reason),
            blocking_count=len(blocking),
            non_blocking_count=len(non_blocking),
            reviewer_mode=review_metadata.get("reviewer_mode"),
            review_file=str(review_file.relative_to(root)),
            handoff_file=review_metadata.get("review_handoff"),
            spec_path=spec,
        )

    if gate_reason:
        if human_reviewed and not override_ready:
            message = "cannot override before a completed challenger review exists"
            next_step = f"scafld review {args.task_id}"
            if json_mode:
                emit_command_json(
                    "complete",
                    ok=False,
                    task_id=args.task_id,
                    state={"status": yaml_read_field(text, "status"), "review_verdict": verdict},
                    result={
                        "review_file": str(review_file.relative_to(root)),
                        "current_handoff": existing_review_handoff(root, args.task_id, review_data.get("metadata") or {}),
                        "next_action": {
                            "type": "review",
                            "command": next_step,
                            "message": "Run and complete the challenger review gate before applying a human override.",
                            "followup_command": None,
                            "blocked": True,
                        },
                    },
                    error=error_payload(
                        message,
                        code=EC.REVIEW_GATE_BLOCKED,
                        next_action=next_step,
                        exit_code=1,
                    ),
                )
            else:
                print(f"  {c(C_RED, 'error')}: {message}")
                print(f"         run '{c(C_BOLD, next_step)}' and finish the challenger round first")
            sys.exit(1)

        if not human_reviewed:
            if json_mode:
                review_handoff = existing_review_handoff(root, args.task_id, review_data.get("metadata") or {})
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
                        "current_handoff": review_handoff,
                        "next_action": {
                            "type": "review" if not review_data["exists"] else "address_review_findings",
                            "command": f"scafld review {args.task_id}" if not review_data["exists"] else None,
                            "message": (
                                "Run the challenger review gate."
                                if not review_data["exists"]
                                else "Fix the blocking review findings, then rerun review."
                            ),
                            "followup_command": None if not review_data["exists"] else f"scafld review {args.task_id}",
                            "blocked": True,
                        },
                    },
                    error=error_payload(
                        gate_reason,
                        code=EC.REVIEW_GATE_BLOCKED,
                        details=gate_errors or blocking,
                        next_action=f"scafld review {args.task_id}" if not review_data["exists"] else f"scafld review {args.task_id}",
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
        session = record_human_override(
            root,
            args.task_id,
            gate="review",
            review_round=review_data.get("review_count", 0) - 1,
            reason=override_reason,
            confirmed_at=override_confirmed_at,
            review_file=str(review_file.relative_to(root)),
            spec_path=spec,
        )
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
    text = normalize_completed_archive_truth(text)
    spec.write_text(text)

    if not json_mode:
        check_self_eval(text, args.task_id)

    move_result = move_spec(root, spec, "completed")
    dest = move_result.dest
    archive_month = dest.parent.name
    archived_run_dir = archive_run_artifacts(root, args.task_id, archive_month)
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
                "handoff_file": review_data.get("metadata", {}).get("review_handoff"),
                "run_archive_dir": str(archived_run_dir.relative_to(root)) if archived_run_dir else None,
                "transition": move_result_payload(root, move_result),
            },
        )
    else:
        print_move_result(root, move_result)
        if archived_run_dir:
            print(f"  run archive: {c(C_DIM, str(archived_run_dir.relative_to(root)))}")


def cmd_review(args):
    """Run automated review passes and generate adversarial review prompt."""
    root = require_root()
    json_mode = bool(getattr(args, "json", False))
    try:
        resolved_runner = resolve_review_runner(
            root,
            runner_override=getattr(args, "runner", None),
            provider_override=getattr(args, "provider", None),
            model_override=getattr(args, "model", None),
        )
    except ValueError as exc:
        if json_mode:
            emit_command_json(
                "review",
                ok=False,
                task_id=args.task_id,
                error=error_payload(str(exc), code=EC.INVALID_ARGUMENTS, exit_code=1),
            )
            sys.exit(1)
        raise ScafldError(str(exc), code=EC.INVALID_ARGUMENTS)

    payload, exit_code = review_snapshot(
        root,
        args.task_id,
        use_color=not json_mode,
        resolved_runner=resolved_runner,
    )
    result = payload.get("result") or {}
    state = payload.get("state") or {}
    automated_results = result.get("automated_passes") or []

    if json_mode:
        emit_command_json(
            "review",
            ok=payload.get("ok", False),
            task_id=args.task_id,
            state=state,
            result=result,
            error=payload.get("error"),
            warnings=payload.get("warnings") or [],
        )
        if exit_code:
            sys.exit(exit_code)
        return

    if not payload.get("ok"):
        status = state.get("status") or "unknown"
        error = payload.get("error") or {}
        if status != "in_progress":
            print(f"{c(C_RED, 'error')}: {error.get('message') or f'spec must be in_progress to review (current: {status})'}")
        else:
            print(f"  {c(C_RED, 'error')}: {error.get('message') or 'review failed'}")
            next_action = error.get("next_action")
            if next_action:
                print(f"  resolve failures, then re-run: {c(C_BOLD, next_action)}")
        sys.exit(exit_code)

    print(f"{c(C_BOLD, f'Review: {args.task_id}')}")
    print()
    passed = 0
    failed = 0
    for outcome in automated_results:
        if outcome["result"] == "pass":
            passed += 1
        else:
            failed += 1
        print(f"  {c(C_CYAN, outcome['id'])}: {outcome['description']}")
        if outcome["result"] == "pass":
            print(f"    {c(C_GREEN, 'PASS')}")
        else:
            print(f"    {c(C_RED, 'FAIL')}")
            for line in outcome["lines"][-5:]:
                print(f"    {c(C_DIM, line)}")

    print()
    summary_parts = []
    if passed:
        summary_parts.append(c(C_GREEN, f"{passed} passed"))
    if failed:
        summary_parts.append(c(C_RED, f"{failed} failed"))
    print(f"  Automated passes: {' / '.join(summary_parts)}")
    print()
    print(f"  challenger handoff: {c(C_DIM, result['handoff_file'])}")
    print(f"  review runner: {c(C_DIM, resolved_runner.runner)}")
    if resolved_runner.runner == "external":
        topology = load_configured_review_topology(root)
        review_file = root / result["review_file"]
        spec = require_spec(root, args.task_id)
        review_data = parse_review_file(review_file, topology)

        def print_external_start(event):
            provider = event.get("provider") or "unknown"
            pid = event.get("pid")
            timeout = event.get("timeout_seconds")
            session_id = event.get("provider_session_requested") or ""
            invocation_id = event.get("invocation_id") or ""
            print()
            print(f"  external runner: {c(C_DIM, provider)}")
            if pid:
                print(f"  subprocess pid: {c(C_DIM, str(pid))}")
            if session_id:
                print(f"  provider session: {c(C_DIM, session_id)}")
            if invocation_id:
                print(f"  invocation id: {c(C_DIM, invocation_id)}")
            if timeout:
                print(f"  timeout: {c(C_DIM, str(timeout) + 's')}")
            print(f"  track: {c(C_BOLD, f'scafld status {args.task_id} --json')}  ({c(C_DIM, f'.ai/runs/{args.task_id}/session.json')})")
            sys.stdout.flush()

        try:
            runner_result = run_external_review(
                root,
                args.task_id,
                result["review_prompt"],
                topology,
                resolved_runner,
                on_start=print_external_start,
            )
        except ScafldError as exc:
            print()
            print(f"  {c(C_RED, 'error')}: {exc.message}")
            for detail in exc.details:
                print(f"    {c(C_DIM, detail)}")
            print(f"  review file: {c(C_DIM, result['review_file'])}")
            print(f"  fallback: {c(C_BOLD, f'scafld review {args.task_id} --runner local')} or {c(C_BOLD, f'scafld review {args.task_id} --runner manual')}")
            sys.exit(exc.exit_code)

        try:
            review_data = complete_review_round_from_result(
                root,
                review_file,
                args.task_id,
                spec.read_text(),
                topology,
                review_data,
                runner_result,
            )
        except ScafldError as exc:
            record_external_review_invocation(
                root,
                args.task_id,
                runner_result.provenance,
                status="invalid_artifact",
                diagnostic_path=diagnostic_path_from_details(exc.details),
            )
            print()
            print(f"  {c(C_RED, 'error')}: {exc.message}")
            for detail in exc.details:
                print(f"    {c(C_DIM, detail)}")
            print(f"  review file: {c(C_DIM, result['review_file'])}")
            print(f"  fallback: {c(C_BOLD, f'scafld review {args.task_id} --runner local')} or {c(C_BOLD, f'scafld review {args.task_id} --runner manual')}")
            sys.exit(exc.exit_code)
        record_external_review_invocation(root, args.task_id, runner_result.provenance, status="completed")
        verdict = review_data.get("verdict") or "incomplete"
        print(f"  provider: {c(C_DIM, runner_result.provenance.get('provider', 'unknown'))}")
        if runner_result.provenance.get("model_observed"):
            label = "model inferred" if runner_result.provenance.get("model_source") == "inferred" else "model observed"
            print(f"  {label}: {c(C_DIM, runner_result.provenance['model_observed'])}")
        elif runner_result.provenance.get("model_requested"):
            print(f"  model requested: {c(C_DIM, runner_result.provenance['model_requested'])}")
        print(f"  isolation: {c(C_DIM, runner_result.provenance.get('isolation_level', 'unknown'))}")
        if runner_result.provenance.get("review_packet"):
            print(f"  review packet: {c(C_DIM, runner_result.provenance['review_packet'])}")
        if runner_result.provenance.get("repair_handoff"):
            print(f"  repair handoff: {c(C_DIM, runner_result.provenance['repair_handoff'])}")
        for runner_warning in runner_result.provenance.get("warnings") or []:
            if runner_warning.startswith("provider=auto "):
                continue
            print(f"  {c(C_YELLOW, 'warning')}: {runner_warning}")
        if runner_result.provenance.get("isolation_downgraded"):
            fallback_policy = runner_result.provenance.get("fallback_policy") or "warn"
            if fallback_policy != "warn":
                if runner_result.provenance.get("provider_selection_reason") == "avoid_codex_self_review":
                    print(f"  {c(C_DIM, 'note')}: provider=auto selected Claude to avoid Codex self-review; Claude isolation is weaker than Codex sandboxing")
                else:
                    print(f"  {c(C_DIM, 'note')}: provider=auto fell back to weaker Claude isolation")
        print(f"  review file: {c(C_DIM, result['review_file'])}")
        print()
        if verdict == "fail":
            print(f"  {c(C_RED, 'review')}: FAIL — {len(review_data.get('blocking', []))} blocking finding(s)")
            print(f"  next: fix the blocking findings, then rerun {c(C_BOLD, f'scafld review {args.task_id}')}")
        elif verdict == "pass_with_issues":
            print(f"  {c(C_YELLOW, 'review')}: pass with {len(review_data.get('non_blocking', []))} non-blocking finding(s)")
            print(f"  next: {c(C_BOLD, f'scafld complete {args.task_id}')}")
        else:
            print(f"  {c(C_GREEN, 'review')}: pass")
            print(f"  next: {c(C_BOLD, f'scafld complete {args.task_id}')}")
        return

    print()
    if resolved_runner.runner == "local":
        print(f"  {c(C_YELLOW, 'warning')}: local review uses the current shared runtime; it is an explicit degraded path")
    else:
        print(f"  {c(C_DIM, 'manual')}: handoff only; no external challenger was spawned")
    print()
    print(result["review_prompt"])
