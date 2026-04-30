import datetime
import re
from collections import defaultdict

from scafld.command_runtime import require_root
from scafld.output import emit_command_json
from scafld.review_signal import review_signal_payload
from scafld.reviewing import parse_review_file, review_git_gate_reason
from scafld.review_workflow import capture_bound_review_git_state, load_review_topology
from scafld.runtime_bundle import REVIEWS_DIR
from scafld.session_store import load_session, session_summary_payload
from scafld.spec_model import (
    active_done_open,
    count_phases,
    extract_self_eval_score,
    get_risk,
    get_size,
    get_status,
    get_task_id,
    get_updated,
    parse_acceptance_criteria,
    parse_iso8601_timestamp,
    supersession_payload,
)
from scafld.spec_store import find_all_specs, load_spec_document
from scafld.terminal import C_BOLD, C_DIM, C_GREEN, C_RED, C_YELLOW, STATUS_COLORS, c


def report_triage_entry(root, spec_path, data):
    task_id = get_task_id(data) or spec_path.stem
    updated = parse_iso8601_timestamp(get_updated(data) or data.get("created"))
    age_days = None
    if updated is not None:
        age_days = max((datetime.datetime.now(datetime.timezone.utc) - updated).days, 0)
    return {
        "task_id": task_id,
        "path": str(spec_path.relative_to(root)),
        "age_days": age_days,
    }


def print_report_triage_section(title, items, formatter, empty_message="none"):
    print(f"  {c(C_BOLD, title)}")
    if not items:
        print(f"    {c(C_DIM, empty_message)}")
        return
    for item in items[:5]:
        print(f"    - {formatter(item)}")
    if len(items) > 5:
        print(f"    {c(C_DIM, f'+ {len(items) - 5} more')}")


def format_stale_active_entry(item):
    age = f"{item['age_days']}d" if item.get("age_days") is not None else "age unknown"
    return f"{item['task_id']} ({age}) — all phases done but still active; {item['path']}"


def format_superseded_entry(item):
    rendered = f"{item['task_id']} -> {item['superseded_by']} — {item['path']}"
    if item.get("reason"):
        rendered = f"{rendered} ({item['reason']})"
    return rendered


def cmd_report(args):
    """Aggregate stats across archived specs."""
    root = require_root()
    specs = find_all_specs(root)
    json_mode = bool(getattr(args, "json", False))
    runtime_only = bool(getattr(args, "runtime_only", False))

    if not specs:
        if json_mode:
            emit_command_json(
                "report",
                result={"total_specs": 0},
                warnings=["no specs found"],
            )
        else:
            print(f"{c(C_DIM, 'No specs found.')}")
        return

    total = 0
    by_status = {}
    by_month = {}
    sizes = {}
    risks = {}
    self_eval_scores = []
    exec_pass = 0
    exec_fail = 0
    exec_total = 0
    specs_with_exec = 0
    specs_without_exec = 0
    phase_counts = []
    harden_status_counts = {"not_run": 0, "in_progress": 0, "passed": 0, "absent": 0}
    specs_with_rounds = 0
    stale_drafts = []
    approved_waiting = []
    stale_active = []
    superseded_specs = []
    active_without_exec = []
    review_drift = []
    runtime_sessions = 0
    first_attempt_passed = 0
    first_attempt_total = 0
    recovered_pass = 0
    recovered_total = 0
    challenge_overrides = 0
    challenge_blocked = 0
    provider_invocations = 0
    provider_invocations_by_role = defaultdict(int)
    provider_confidence = defaultdict(int)
    provider_statuses = defaultdict(int)
    provider_models_observed = 0
    provider_models_inferred = 0
    provider_models_unknown = 0
    provider_isolation_downgrades = 0
    provider_weaker_review_isolation = 0
    provider_model_separation = defaultdict(int)
    attempts_per_phase = defaultdict(int)
    usage_totals = defaultdict(float)
    per_task_runtime = {}
    review_signal = {
        "completed_rounds": 0,
        "blocking_verdicts": 0,
        "pass_with_issues_verdicts": 0,
        "format_compliant_clean_reviews": 0,
        "grounded_findings": 0,
    }
    per_task_review_signal = {}
    review_topology = None
    try:
        review_topology = load_review_topology(root)
    except ValueError:
        review_topology = None

    for spec_path, label in specs:
        data = load_spec_document(spec_path)

        status = get_status(data) or "unknown"
        triage_entry = report_triage_entry(root, spec_path, data)
        is_active_done_open = active_done_open(data, status)
        supersession = supersession_payload(data)
        session = load_session(root, triage_entry["task_id"], spec_path=spec_path)
        if runtime_only and session is None:
            continue
        if is_active_done_open:
            stale_active.append(triage_entry)
        if supersession.get("superseded"):
            superseded_specs.append({
                **triage_entry,
                "superseded_by": supersession.get("superseded_by"),
                "superseded_at": supersession.get("superseded_at"),
                "reason": supersession.get("reason"),
            })

        total += 1
        harden_status = data.get("harden_status")
        if harden_status and harden_status in harden_status_counts:
            harden_status_counts[harden_status] += 1
        else:
            harden_status_counts["absent"] += 1
        if isinstance(data.get("harden_rounds"), list) and data.get("harden_rounds"):
            specs_with_rounds += 1
        by_status[status] = by_status.get(status, 0) + 1

        if label.startswith("archive/"):
            month = label.split("/", 1)[1]
            by_month[month] = by_month.get(month, 0) + 1

        size = get_size(data) or "unknown"
        sizes[size] = sizes.get(size, 0) + 1

        risk = get_risk(data) or "unknown"
        risks[risk] = risks.get(risk, 0) + 1

        score = extract_self_eval_score(data)
        if score is not None:
            self_eval_scores.append(score)

        criteria = parse_acceptance_criteria(data)
        passes = sum(1 for criterion in criteria if criterion.get("result") == "pass")
        fails = sum(1 for criterion in criteria if criterion.get("result") == "fail")
        if passes or fails:
            exec_pass += passes
            exec_fail += fails
            exec_total += passes + fails
            specs_with_exec += 1
        else:
            specs_without_exec += 1
            if status == "in_progress" and not is_active_done_open:
                active_without_exec.append(triage_entry)

        total_phases, _, _, _ = count_phases(data)
        if total_phases > 0:
            phase_counts.append(total_phases)

        if status == "draft" and triage_entry["age_days"] is not None and triage_entry["age_days"] >= 7:
            stale_drafts.append(triage_entry)
        elif status == "approved":
            approved_waiting.append(triage_entry)

        if status == "in_progress" and review_topology is not None:
            review_file = root / REVIEWS_DIR / f"{triage_entry['task_id']}.md"
            review_data = parse_review_file(review_file, review_topology)
            if review_data.get("exists"):
                current_git_state, current_git_error = capture_bound_review_git_state(
                    root,
                    triage_entry["task_id"],
                    review_file.relative_to(root),
                )
                if current_git_error:
                    review_drift.append({
                        **triage_entry,
                        "reason": "current git state is unavailable for review binding",
                    })
                else:
                    drift_reason = review_git_gate_reason(current_git_state, review_data.get("metadata") or {})
                    if drift_reason:
                        review_drift.append({
                            **triage_entry,
                            "reason": drift_reason,
                        })

        if review_topology is not None:
            review_file = root / REVIEWS_DIR / f"{triage_entry['task_id']}.md"
            signal = review_signal_payload(parse_review_file(review_file, review_topology))
            per_task_review_signal[triage_entry["task_id"]] = signal
            if signal["completed_round"]:
                review_signal["completed_rounds"] += 1
            if signal["verdict"] == "fail":
                review_signal["blocking_verdicts"] += 1
            if signal["verdict"] == "pass_with_issues":
                review_signal["pass_with_issues_verdicts"] += 1
            if signal["format_compliant_clean_review"]:
                review_signal["format_compliant_clean_reviews"] += 1
            review_signal["grounded_findings"] += signal["grounded_findings"]

        if session is not None:
            runtime_sessions += 1
            runtime = session_summary_payload(session)
            per_task_runtime[triage_entry["task_id"]] = {
                "status": status,
                "spec_path": triage_entry["path"],
                "first_attempt_pass_rate": {
                    "passed": runtime["first_attempt_passed"],
                    "total": runtime["first_attempt_total"],
                    "rate": (
                        runtime["first_attempt_passed"] / runtime["first_attempt_total"]
                        if runtime["first_attempt_total"]
                        else None
                    ),
                },
                "recovery_convergence_rate": {
                    "recovered": runtime["recovered_pass"],
                    "total": runtime["recovered_total"],
                    "rate": (
                        runtime["recovered_pass"] / runtime["recovered_total"]
                        if runtime["recovered_total"]
                        else None
                    ),
                },
                "challenge_override_rate": {
                    "overrides": runtime["challenge_overrides"],
                    "total": runtime["challenge_blocked"],
                    "rate": runtime["challenge_override_rate"],
                },
                "attempts_per_phase": runtime["attempts_per_phase"],
                "failed_exhausted": runtime["failed_exhausted"],
                "provider_telemetry": {
                    "invocations": runtime["provider_invocations"],
                    "by_role": runtime["provider_invocations_by_role"],
                    "confidence": runtime["provider_confidence"],
                    "statuses": runtime["provider_statuses"],
                    "models_observed": runtime["provider_models_observed"],
                    "models_inferred": runtime["provider_models_inferred"],
                    "models_unknown": runtime["provider_models_unknown"],
                    "isolation_downgrades": runtime["provider_isolation_downgrades"],
                    "weaker_review_isolation": runtime["provider_weaker_review_isolation"],
                    "model_separation": runtime["provider_model_separation"],
                },
            }
            first_attempt_passed += runtime["first_attempt_passed"]
            first_attempt_total += runtime["first_attempt_total"]
            recovered_pass += runtime["recovered_pass"]
            recovered_total += runtime["recovered_total"]
            challenge_overrides += runtime["challenge_overrides"]
            challenge_blocked += runtime["challenge_blocked"]
            provider_invocations += runtime["provider_invocations"]
            provider_models_observed += runtime["provider_models_observed"]
            provider_models_inferred += runtime["provider_models_inferred"]
            provider_models_unknown += runtime["provider_models_unknown"]
            provider_isolation_downgrades += runtime["provider_isolation_downgrades"]
            provider_weaker_review_isolation += runtime["provider_weaker_review_isolation"]
            separation_state = runtime["provider_model_separation"]["state"]
            if separation_state != "none":
                provider_model_separation[separation_state] += 1
            for role, count in runtime["provider_invocations_by_role"].items():
                provider_invocations_by_role[role] += count
            for confidence, count in runtime["provider_confidence"].items():
                provider_confidence[confidence] += count
            for status_key, count in runtime["provider_statuses"].items():
                provider_statuses[status_key] += count
            for phase_id, count in runtime["attempts_per_phase"].items():
                attempts_per_phase[phase_id] += count
            usage = runtime.get("usage") if isinstance(runtime.get("usage"), dict) else {}
            for key, value in usage.items():
                try:
                    usage_totals[key] += float(value)
                except (TypeError, ValueError):
                    continue

    if total == 0:
        warning = "no runtime sessions found" if runtime_only else "no specs found"
        if json_mode:
            emit_command_json(
                "report",
                result={"total_specs": 0, "runtime_only": runtime_only},
                warnings=[warning],
            )
        else:
            print(f"{c(C_DIM, warning + '.')}")
        return

    if json_mode:
        avg_self_eval = sum(self_eval_scores) / len(self_eval_scores) if self_eval_scores else None
        avg_phases = sum(phase_counts) / len(phase_counts) if phase_counts else None
        emit_command_json(
            "report",
            result={
                "total_specs": total,
                "runtime_only": runtime_only,
                "by_status": by_status,
                "by_month": by_month,
                "sizes": sizes,
                "risks": risks,
                "self_eval": {
                    "scores": self_eval_scores,
                    "average": avg_self_eval,
                },
                "exec_results": {
                    "passed": exec_pass,
                    "failed": exec_fail,
                    "total": exec_total,
                    "specs_with_exec": specs_with_exec,
                    "specs_without_exec": specs_without_exec,
                },
                "phase_stats": {
                    "counts": phase_counts,
                    "average": avg_phases,
                },
                "harden_adoption": {
                    "counts": harden_status_counts,
                    "specs_with_rounds": specs_with_rounds,
                },
                "llm_runtime": {
                    "sessions_found": runtime_sessions,
                    "first_attempt_pass_rate": {
                        "passed": first_attempt_passed,
                        "total": first_attempt_total,
                        "rate": (first_attempt_passed / first_attempt_total) if first_attempt_total else None,
                    },
                    "recovery_convergence_rate": {
                        "recovered": recovered_pass,
                        "total": recovered_total,
                        "rate": (recovered_pass / recovered_total) if recovered_total else None,
                    },
                    "challenge_override_rate": {
                        "overrides": challenge_overrides,
                        "total": challenge_blocked,
                        "rate": (challenge_overrides / challenge_blocked) if challenge_blocked else None,
                    },
                    "attempts_per_phase": dict(sorted(attempts_per_phase.items())),
                    "usage_totals": dict(sorted(usage_totals.items())),
                    "provider_telemetry": {
                        "invocations": provider_invocations,
                        "by_role": dict(sorted(provider_invocations_by_role.items())),
                        "confidence": dict(sorted(provider_confidence.items())),
                        "statuses": dict(sorted(provider_statuses.items())),
                        "models_observed": provider_models_observed,
                        "models_inferred": provider_models_inferred,
                        "models_unknown": provider_models_unknown,
                        "isolation_downgrades": provider_isolation_downgrades,
                        "weaker_review_isolation": provider_weaker_review_isolation,
                        "model_separation": dict(sorted(provider_model_separation.items())),
                    },
                    "per_task": dict(sorted(per_task_runtime.items())),
                    "attribution_note": "Session metrics show outcomes only; external agents may ignore generated handoffs.",
                },
                "review_signal": {
                    **review_signal,
                    "per_task": dict(sorted(per_task_review_signal.items())),
                },
                "triage": {
                    "stale_drafts": stale_drafts,
                    "approved_waiting": approved_waiting,
                    "stale_active": stale_active,
                    "superseded": superseded_specs,
                    "active_without_exec": active_without_exec,
                    "review_drift": review_drift,
                },
            },
        )
        return

    title = "scafld Runtime Report" if runtime_only else "scafld Report"
    print(f"{c(C_BOLD, title)}")
    print(f"  {total} total specs")
    if runtime_only:
        print(f"  {c(C_DIM, 'runtime-only cohort')}")
    print()

    print(f"{c(C_BOLD, 'By status:')}")
    for status in ["completed", "failed", "cancelled", "in_progress", "approved", "draft"]:
        count = by_status.get(status, 0)
        if count:
            color = STATUS_COLORS.get(status, "")
            pct = count / total * 100
            bar = "#" * int(pct / 2)
            print(f"  {c(color, f'{status:14s}')} {count:4d}  {pct:5.1f}%  {c(C_DIM, bar)}")
    print()

    if by_month:
        print(f"{c(C_BOLD, 'By month:')}")
        for month in sorted(by_month.keys(), reverse=True)[:6]:
            print(f"  {month}  {by_month[month]:3d} specs")
        print()

    if any(value != "unknown" for value in sizes):
        print(f"{c(C_BOLD, 'Sizes:')}  ", end="")
        print("  ".join(f"{size}: {sizes.get(size, 0)}" for size in ["micro", "small", "medium", "large"] if sizes.get(size, 0)))
        print(f"{c(C_BOLD, 'Risks:')}  ", end="")
        print("  ".join(f"{risk}: {risks.get(risk, 0)}" for risk in ["low", "medium", "high"] if risks.get(risk, 0)))
        print()

    if self_eval_scores:
        avg = sum(self_eval_scores) / len(self_eval_scores)
        perfect = sum(1 for score in self_eval_scores if score == 10)
        below_7 = sum(1 for score in self_eval_scores if score < 7)
        print(f"{c(C_BOLD, 'Self-eval:')}")
        print(f"  avg: {avg:.1f}/10  ({len(self_eval_scores)} scored)")
        if perfect:
            pct = perfect / len(self_eval_scores) * 100
            color = C_YELLOW if pct > 50 else C_DIM
            print(f"  {c(color, f'perfect 10s: {perfect} ({pct:.0f}%)')}")
        if below_7:
            print(f"  below threshold: {below_7}")
        print()

    if exec_total:
        pass_rate = exec_pass / exec_total * 100
        print(f"{c(C_BOLD, 'Exec results:')}")
        print(f"  {c(C_GREEN, f'{exec_pass} passed')} / {c(C_RED, f'{exec_fail} failed')}  ({pass_rate:.0f}% pass rate)")
        print(f"  {specs_with_exec} specs with results, {specs_without_exec} without")
        print()

    if phase_counts:
        avg_phases = sum(phase_counts) / len(phase_counts)
        print(f"{c(C_BOLD, 'Phases:')}  avg {avg_phases:.1f} per spec, range {min(phase_counts)}-{max(phase_counts)}")
        print()

    print(f"{c(C_BOLD, 'Harden adoption:')}")
    for harden_status in ["passed", "in_progress", "not_run", "absent"]:
        count = harden_status_counts.get(harden_status, 0)
        if count:
            print(f"  {harden_status:14s} {count:4d}")
    print(f"  {'with rounds':14s} {specs_with_rounds:4d}")
    print()

    if runtime_sessions:
        print(f"{c(C_BOLD, 'LLM execution signals:')}")
        print(f"  {c(C_DIM, 'Session outcomes only; external agents may ignore handoffs.')}")
        if first_attempt_total:
            print(
                f"  first-attempt pass: {first_attempt_passed}/{first_attempt_total} "
                f"({first_attempt_passed / first_attempt_total * 100:.0f}%)"
            )
        else:
            print("  first-attempt pass: none recorded")
        if recovered_total:
            print(
                f"  recovery convergence: {recovered_pass}/{recovered_total} "
                f"({recovered_pass / recovered_total * 100:.0f}%)"
            )
        else:
            print("  recovery convergence: none recorded")
        if challenge_blocked:
            print(
                f"  challenge override: {challenge_overrides}/{challenge_blocked} "
                f"({challenge_overrides / challenge_blocked * 100:.0f}%)"
            )
        else:
            print("  challenge override: none recorded")
        if attempts_per_phase:
            rendered = ", ".join(f"{phase_id}={count}" for phase_id, count in sorted(attempts_per_phase.items()))
            print(f"  attempts per phase: {rendered}")
        if usage_totals:
            rendered_usage = ", ".join(f"{key}={value:g}" for key, value in sorted(usage_totals.items()))
            print(f"  usage totals: {rendered_usage}")
        if provider_invocations:
            by_role = ", ".join(f"{role}={count}" for role, count in sorted(provider_invocations_by_role.items()))
            separation = ", ".join(
                f"{state}={count}" for state, count in sorted(provider_model_separation.items())
            )
            print(
                f"  provider invocations: {provider_invocations} "
                f"(models observed {provider_models_observed}, inferred {provider_models_inferred}, unknown {provider_models_unknown})"
            )
            if by_role:
                print(f"  provider roles: {by_role}")
            if provider_confidence:
                rendered_confidence = ", ".join(
                    f"{key}={value}" for key, value in sorted(provider_confidence.items())
                )
                print(f"  provider confidence: {rendered_confidence}")
            if provider_statuses:
                rendered_statuses = ", ".join(f"{key}={value}" for key, value in sorted(provider_statuses.items()))
                print(f"  provider statuses: {rendered_statuses}")
            print(f"  isolation downgrades: {provider_isolation_downgrades}/{provider_invocations}")
            print(f"  weaker review isolation: {provider_weaker_review_isolation}/{provider_invocations}")
            print(f"  model separation: {separation}")
        print()

        print(f"  {c(C_BOLD, 'Per-task metrics')}")
        for task_id, runtime in sorted(per_task_runtime.items())[:5]:
            first = runtime["first_attempt_pass_rate"]
            recovery = runtime["recovery_convergence_rate"]
            challenge = runtime["challenge_override_rate"]
            rendered = (
                f"    - {task_id}: first_attempt_pass_rate "
                f"{first['passed']}/{first['total'] or 0}"
                f", recovery_convergence_rate {recovery['recovered']}/{recovery['total'] or 0}"
            )
            if challenge["total"]:
                rendered += f", challenge_override_rate {challenge['overrides']}/{challenge['total']}"
            if runtime["failed_exhausted"]:
                rendered += f", exhausted {runtime['failed_exhausted']}"
            print(rendered)
        if len(per_task_runtime) > 5:
            print(f"    {c(C_DIM, f'+ {len(per_task_runtime) - 5} more')}")
        print()

    if review_topology is not None:
        print(f"{c(C_BOLD, 'Review signal:')}")
        print(f"  completed rounds: {review_signal['completed_rounds']}")
        print(f"  blocking verdicts: {review_signal['blocking_verdicts']}")
        print(f"  pass_with_issues verdicts: {review_signal['pass_with_issues_verdicts']}")
        print(f"  format-compliant clean reviews: {review_signal['format_compliant_clean_reviews']}")
        print(f"  grounded findings: {review_signal['grounded_findings']}")
        print()

    print(f"{c(C_BOLD, 'Triage:')}")
    print_report_triage_section(
        "Stale drafts (>7d)",
        stale_drafts,
        lambda item: f"{item['task_id']} ({item['age_days']}d) — {item['path']}",
    )
    print_report_triage_section(
        "Approved waiting to start",
        approved_waiting,
        lambda item: f"{item['task_id']} — {item['path']}",
    )
    print_report_triage_section(
        "Stale active specs",
        stale_active,
        format_stale_active_entry,
    )
    print_report_triage_section(
        "Superseded specs",
        superseded_specs,
        format_superseded_entry,
    )
    print_report_triage_section(
        "Active with no exec evidence",
        active_without_exec,
        lambda item: f"{item['task_id']} — {item['path']}",
    )
    print_report_triage_section(
        "Review drift",
        review_drift,
        lambda item: f"{item['task_id']} — {item['reason']}",
    )
    print()
