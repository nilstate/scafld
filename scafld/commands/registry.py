from .audit import cmd_audit, cmd_diff
from .execution import cmd_exec, cmd_harden
from .lifecycle import (
    cmd_approve,
    cmd_branch,
    cmd_cancel,
    cmd_fail,
    cmd_list,
    cmd_new,
    cmd_start,
    cmd_status,
    cmd_sync,
    cmd_validate,
)
from .projections import cmd_checks, cmd_pr_body, cmd_summary
from .reporting import cmd_report
from .review import cmd_complete, cmd_review
from .workspace import cmd_init, cmd_update, cmd_version


def command_registry():
    return {
        "version": cmd_version,
        "init": cmd_init,
        "new": cmd_new,
        "status": cmd_status,
        "list": cmd_list,
        "approve": cmd_approve,
        "start": cmd_start,
        "branch": cmd_branch,
        "sync": cmd_sync,
        "summary": cmd_summary,
        "checks": cmd_checks,
        "pr-body": cmd_pr_body,
        "review": cmd_review,
        "complete": cmd_complete,
        "fail": cmd_fail,
        "cancel": cmd_cancel,
        "validate": cmd_validate,
        "harden": cmd_harden,
        "exec": cmd_exec,
        "audit": cmd_audit,
        "diff": cmd_diff,
        "report": cmd_report,
        "update": cmd_update,
    }
