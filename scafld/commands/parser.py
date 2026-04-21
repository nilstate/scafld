import argparse


def build_parser():
    parser = argparse.ArgumentParser(
        prog="scafld",
        description="Spec-driven development framework for AI coding agents",
    )
    parser.add_argument("--version", action="store_true", help="Show version")
    sub = parser.add_subparsers(dest="command")

    p_init = sub.add_parser("init", help="Bootstrap scafld workspace")
    p_init.add_argument("--json", action="store_true", help="Emit machine-readable initialization JSON")

    p_new = sub.add_parser("new", help="Create a new spec from template")
    p_new.add_argument("task_id", help="Task ID (kebab-case)")
    p_new.add_argument("-t", "--title", help="Task title")
    p_new.add_argument("-s", "--size", choices=["micro", "small", "medium", "large"])
    p_new.add_argument("-r", "--risk", choices=["low", "medium", "high"])
    p_new.add_argument("--json", action="store_true", help="Emit machine-readable spec creation JSON")

    p_status = sub.add_parser("status", help="Show spec status")
    p_status.add_argument("task_id", help="Task ID")
    p_status.add_argument("--json", action="store_true", help="Emit machine-readable JSON")

    p_list = sub.add_parser("list", help="List all specs")
    p_list.add_argument("filter", nargs="?", help="Filter by status or search term")

    p_approve = sub.add_parser("approve", help="Approve a draft spec")
    p_approve.add_argument("task_id", help="Task ID")
    p_approve.add_argument("--json", action="store_true", help="Emit machine-readable approval JSON")

    p_start = sub.add_parser("start", help="Start execution of approved spec")
    p_start.add_argument("task_id", help="Task ID")
    p_start.add_argument("--json", action="store_true", help="Emit machine-readable start JSON")

    p_branch = sub.add_parser("branch", help="Create or bind a working branch for a spec")
    p_branch.add_argument("task_id", help="Task ID")
    p_branch.add_argument("--name", help="Branch name to bind or create (default: task_id)")
    p_branch.add_argument("--base", help="Base ref for branch creation (default: repo default branch)")
    p_branch.add_argument("--bind-current", action="store_true", help="Record the current branch without creating or switching")
    p_branch.add_argument("--json", action="store_true", help="Emit machine-readable branch-binding JSON")

    p_sync = sub.add_parser("sync", help="Compare a spec's recorded origin binding with live git state")
    p_sync.add_argument("task_id", help="Task ID")
    p_sync.add_argument("--json", action="store_true", help="Emit machine-readable sync JSON")

    p_summary = sub.add_parser("summary", help="Render a concise markdown or JSON summary from spec state")
    p_summary.add_argument("task_id", help="Task ID")
    p_summary.add_argument("--json", action="store_true", help="Emit machine-readable summary JSON")

    p_checks = sub.add_parser("checks", help="Render CI-friendly check state from spec state")
    p_checks.add_argument("task_id", help="Task ID")
    p_checks.add_argument("--json", action="store_true", help="Emit machine-readable check JSON")

    p_pr_body = sub.add_parser("pr-body", help="Render a deterministic PR body from spec state")
    p_pr_body.add_argument("task_id", help="Task ID")
    p_pr_body.add_argument("--json", action="store_true", help="Emit machine-readable PR-body JSON")

    p_review = sub.add_parser("review", help="Run review passes and generate adversarial review prompt")
    p_review.add_argument("task_id", help="Task ID")
    p_review.add_argument("--json", action="store_true", help="Emit machine-readable review handoff JSON")

    p_complete = sub.add_parser("complete", help="Mark spec as completed")
    p_complete.add_argument("task_id", help="Task ID")
    p_complete.add_argument("--human-reviewed", action="store_true", help="Allow a human-confirmed override when the review gate is blocked")
    p_complete.add_argument("--reason", help="Required with --human-reviewed; records why the override was applied")
    p_complete.add_argument("--json", action="store_true", help="Emit machine-readable completion JSON")

    p_fail = sub.add_parser("fail", help="Mark spec as failed")
    p_fail.add_argument("task_id", help="Task ID")
    p_fail.add_argument("--json", action="store_true", help="Emit machine-readable failure-archive JSON")

    p_cancel = sub.add_parser("cancel", help="Cancel a spec")
    p_cancel.add_argument("task_id", help="Task ID")
    p_cancel.add_argument("--json", action="store_true", help="Emit machine-readable cancellation JSON")

    p_validate = sub.add_parser("validate", help="Validate spec against schema")
    p_validate.add_argument("task_id", help="Task ID")
    p_validate.add_argument("--json", action="store_true", help="Emit machine-readable validation JSON")

    p_harden = sub.add_parser("harden", help="Run HARDEN MODE prompt or mark a hardening round passed")
    p_harden.add_argument("task_id", help="Task ID")
    p_harden.add_argument(
        "--mark-passed",
        dest="mark_passed",
        action="store_true",
        help="Mark the latest hardening round as passed",
    )
    p_harden.add_argument("--json", action="store_true", help="Emit machine-readable harden JSON")

    p_exec = sub.add_parser("exec", help="Run acceptance criteria and record results")
    p_exec.add_argument("task_id", help="Task ID")
    p_exec.add_argument("-p", "--phase", help="Run only criteria for this phase (e.g. phase1)")
    p_exec.add_argument("-r", "--resume", action="store_true", help="Skip criteria that already passed")
    p_exec.add_argument("--json", action="store_true", help="Emit machine-readable execution JSON")

    p_audit = sub.add_parser("audit", help="Check spec changes vs current git changes")
    p_audit.add_argument("task_id", help="Task ID")
    p_audit.add_argument("-b", "--base", help="Git base ref for historical comparison (default: current working tree)")
    p_audit.add_argument("--json", action="store_true", help="Emit machine-readable audit JSON")

    p_diff = sub.add_parser("diff", help="Show git history and changes for a spec")
    p_diff.add_argument("task_id", help="Task ID")

    p_report = sub.add_parser("report", help="Aggregate stats across all specs")
    p_report.add_argument("--json", action="store_true", help="Emit machine-readable report JSON")

    p_update = sub.add_parser("update", help="Refresh the managed scafld bundle")
    p_update.add_argument("--scan-root", help="Recursively update all scafld workspaces under this path")
    p_update.add_argument("--self", action="store_true", help="git pull --ff-only the current scafld checkout before syncing workspaces")
    p_update.add_argument("--verbose", action="store_true", help="Show created/updated bundle files for each workspace")

    return parser
