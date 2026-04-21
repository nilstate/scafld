import datetime
import re

from scafld.command_runtime import require_root
from scafld.git_state import capture_review_git_state
from scafld.output import emit_command_json
from scafld.reviewing import parse_review_file, review_git_gate_reason
from scafld.spec_store import find_all_specs, yaml_read_field, yaml_read_nested

from .shared import (
    REVIEWS_DIR,
    STATUS_COLORS,
    c,
    count_phases,
    extract_self_eval_score,
    load_review_topology,
    parse_acceptance_criteria,
    parse_iso8601_timestamp,
    C_BOLD,
    C_DIM,
    C_GREEN,
    C_RED,
    C_YELLOW,
)


def report_triage_entry(root, spec_path, text):
    task_id = yaml_read_field(text, "task_id") or spec_path.stem
    updated = parse_iso8601_timestamp(yaml_read_field(text, "updated") or yaml_read_field(text, "created"))
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


def cmd_report(args):
    """Aggregate stats across archived specs."""
    root = require_root()
    specs = find_all_specs(root)
    json_mode = bool(getattr(args, "json", False))

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
    active_without_exec = []
    review_drift = []
    review_topology = None
    try:
        review_topology = load_review_topology(root)
    except ValueError:
        review_topology = None

    for spec_path, label in specs:
        text = spec_path.read_text()
        total += 1

        status = yaml_read_field(text, "status") or "unknown"
        triage_entry = report_triage_entry(root, spec_path, text)
        harden_status = yaml_read_field(text, "harden_status")
        if harden_status and harden_status in harden_status_counts:
            harden_status_counts[harden_status] += 1
        else:
            harden_status_counts["absent"] += 1
        if re.search(r'^harden_rounds:\s*\n\s*-\s+round:', text, re.MULTILINE):
            specs_with_rounds += 1
        by_status[status] = by_status.get(status, 0) + 1

        if label.startswith("archive/"):
            month = label.split("/", 1)[1]
            by_month[month] = by_month.get(month, 0) + 1

        size = yaml_read_nested(text, "task", "size") or "unknown"
        sizes[size] = sizes.get(size, 0) + 1

        risk = yaml_read_nested(text, "task", "risk_level") or "unknown"
        risks[risk] = risks.get(risk, 0) + 1

        score = extract_self_eval_score(text)
        if score is not None:
            self_eval_scores.append(score)

        criteria = parse_acceptance_criteria(text)
        passes = sum(1 for criterion in criteria if criterion.get("result") == "pass")
        fails = sum(1 for criterion in criteria if criterion.get("result") == "fail")
        if passes or fails:
            exec_pass += passes
            exec_fail += fails
            exec_total += passes + fails
            specs_with_exec += 1
        else:
            specs_without_exec += 1
            if status == "in_progress":
                active_without_exec.append(triage_entry)

        total_phases, _, _, _ = count_phases(text)
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
                current_git_state, current_git_error = capture_review_git_state(root, review_file.relative_to(root))
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

    if json_mode:
        avg_self_eval = sum(self_eval_scores) / len(self_eval_scores) if self_eval_scores else None
        avg_phases = sum(phase_counts) / len(phase_counts) if phase_counts else None
        emit_command_json(
            "report",
            result={
                "total_specs": total,
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
                "triage": {
                    "stale_drafts": stale_drafts,
                    "approved_waiting": approved_waiting,
                    "active_without_exec": active_without_exec,
                    "review_drift": review_drift,
                },
            },
        )
        return

    print(f"{c(C_BOLD, 'scafld Report')}")
    print(f"  {total} total specs")
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
