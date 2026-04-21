import sys

from scafld.audit_scope import git_sync_excluded_paths
from scafld.command_runtime import require_root
from scafld.error_codes import ErrorCode as EC
from scafld.git_state import build_origin_sync_payload
from scafld.output import (
    emit_command_json,
    error_payload,
    projection_check,
    projection_metadata,
    render_projection_pr_body,
    render_projection_summary,
)
from scafld.projections import build_projection_model, origin_payload, phase_counts
from scafld.reviewing import load_review_state
from scafld.review_workflow import load_review_topology
from scafld.runtime_bundle import REVIEWS_DIR
from scafld.spec_parsing import count_phases, parse_acceptance_criteria, parse_phase_status_entries
from scafld.spec_store import load_spec_document, require_spec


def projection_model_for_task(root, spec, task_id):
    """Build one projection model from the current spec plus live review/sync state."""
    text = spec.read_text()
    data = load_spec_document(spec)
    counts = phase_counts(*count_phases(text))
    origin = origin_payload(data)
    sync = build_origin_sync_payload(root, origin, excluded_rels=git_sync_excluded_paths())
    review_path = root / REVIEWS_DIR / f"{task_id}.md"
    try:
        topology = load_review_topology(root)
    except Exception as exc:
        review_state = {"exists": False, "errors": [str(exc)]}
    else:
        review_state = load_review_state(review_path, topology)
    return build_projection_model(
        root,
        spec,
        task_id,
        data=data,
        phase_entries=parse_phase_status_entries(text),
        phase_counts_payload=counts,
        criteria=parse_acceptance_criteria(text),
        review_state=review_state,
        sync=sync,
    )


def cmd_summary(args):
    """Render a concise deterministic task summary for chat, issues, or logs."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    model = projection_model_for_task(root, spec, args.task_id)
    markdown = render_projection_summary(model)

    if getattr(args, "json", False):
        emit_command_json(
            "summary",
            task_id=args.task_id,
            state={"status": model.get("status")},
            result={
                "projection": projection_metadata("summary"),
                "model": model,
                "markdown": markdown,
            },
        )
        return

    print(markdown, end="")


def cmd_checks(args):
    """Render CI-friendly task check state from the projection model."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    model = projection_model_for_task(root, spec, args.task_id)
    check = projection_check(model)
    markdown = render_projection_summary(model)

    if getattr(args, "json", False):
        emit_command_json(
            "checks",
            task_id=args.task_id,
            ok=check["status"] != "failure",
            state={"status": model.get("status"), "check_status": check["status"]},
            result={
                "projection": projection_metadata("checks"),
                "model": model,
                "check": check,
                "markdown": markdown,
            },
            error=error_payload(
                check["summary"],
                code=EC.PROJECTION_CHECK_FAILED,
                details=check["details"],
                exit_code=1,
            ) if check["status"] == "failure" else None,
        )
        if check["status"] == "failure":
            sys.exit(1)
        return

    print(markdown, end="")
    print()
    print(f"Check: {check['status']} - {check['summary']}")
    if check["status"] == "failure":
        sys.exit(1)


def cmd_pr_body(args):
    """Render a deterministic PR body from the current spec state."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    model = projection_model_for_task(root, spec, args.task_id)
    markdown = render_projection_pr_body(model)

    if getattr(args, "json", False):
        emit_command_json(
            "pr-body",
            task_id=args.task_id,
            state={"status": model.get("status")},
            result={
                "projection": projection_metadata("pr-body"),
                "model": model,
                "markdown": markdown,
            },
        )
        return

    print(markdown, end="")
