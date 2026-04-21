import argparse
from dataclasses import dataclass, field
from functools import lru_cache
from importlib import import_module


@dataclass(frozen=True)
class ArgumentSpec:
    flags: tuple[str, ...]
    kwargs: dict = field(default_factory=dict)


@dataclass(frozen=True)
class CommandSpec:
    name: str
    help: str
    handler_path: str
    args: tuple[ArgumentSpec, ...] = ()


VERSION_HANDLER_PATH = "scafld.commands.workspace:cmd_version"

COMMAND_SPECS = (
    CommandSpec(
        "init",
        "Bootstrap scafld workspace",
        "scafld.commands.workspace:cmd_init",
        (
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable initialization JSON"}),
        ),
    ),
    CommandSpec(
        "new",
        "Create a new spec from template",
        "scafld.commands.lifecycle:cmd_new",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID (kebab-case)"}),
            ArgumentSpec(("-t", "--title"), {"help": "Task title"}),
            ArgumentSpec(("-s", "--size"), {"choices": ["micro", "small", "medium", "large"]}),
            ArgumentSpec(("-r", "--risk"), {"choices": ["low", "medium", "high"]}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable spec creation JSON"}),
        ),
    ),
    CommandSpec(
        "status",
        "Show spec status",
        "scafld.commands.lifecycle:cmd_status",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable JSON"}),
        ),
    ),
    CommandSpec(
        "list",
        "List all specs",
        "scafld.commands.lifecycle:cmd_list",
        (
            ArgumentSpec(("filter",), {"nargs": "?", "help": "Filter by status or search term"}),
        ),
    ),
    CommandSpec(
        "approve",
        "Approve a draft spec",
        "scafld.commands.lifecycle:cmd_approve",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable approval JSON"}),
        ),
    ),
    CommandSpec(
        "start",
        "Start execution of approved spec",
        "scafld.commands.lifecycle:cmd_start",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable start JSON"}),
        ),
    ),
    CommandSpec(
        "branch",
        "Create or bind a working branch for a spec",
        "scafld.commands.lifecycle:cmd_branch",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--name",), {"help": "Branch name to bind or create (default: task_id)"}),
            ArgumentSpec(("--base",), {"help": "Base ref for branch creation (default: repo default branch)"}),
            ArgumentSpec(("--bind-current",), {"action": "store_true", "help": "Record the current branch without creating or switching"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable branch-binding JSON"}),
        ),
    ),
    CommandSpec(
        "sync",
        "Compare a spec's recorded origin binding with live git state",
        "scafld.commands.lifecycle:cmd_sync",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable sync JSON"}),
        ),
    ),
    CommandSpec(
        "summary",
        "Render a concise markdown or JSON summary from spec state",
        "scafld.commands.projections:cmd_summary",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable summary JSON"}),
        ),
    ),
    CommandSpec(
        "checks",
        "Render CI-friendly check state from spec state",
        "scafld.commands.projections:cmd_checks",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable check JSON"}),
        ),
    ),
    CommandSpec(
        "pr-body",
        "Render a deterministic PR body from spec state",
        "scafld.commands.projections:cmd_pr_body",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable PR-body JSON"}),
        ),
    ),
    CommandSpec(
        "review",
        "Run review passes and generate adversarial review prompt",
        "scafld.commands.review:cmd_review",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable review handoff JSON"}),
        ),
    ),
    CommandSpec(
        "complete",
        "Mark spec as completed",
        "scafld.commands.review:cmd_complete",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--human-reviewed",), {"action": "store_true", "help": "Allow a human-confirmed override when the review gate is blocked"}),
            ArgumentSpec(("--reason",), {"help": "Required with --human-reviewed; records why the override was applied"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable completion JSON"}),
        ),
    ),
    CommandSpec(
        "fail",
        "Mark spec as failed",
        "scafld.commands.lifecycle:cmd_fail",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable failure-archive JSON"}),
        ),
    ),
    CommandSpec(
        "cancel",
        "Cancel a spec",
        "scafld.commands.lifecycle:cmd_cancel",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable cancellation JSON"}),
        ),
    ),
    CommandSpec(
        "validate",
        "Validate spec against schema",
        "scafld.commands.lifecycle:cmd_validate",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable validation JSON"}),
        ),
    ),
    CommandSpec(
        "harden",
        "Run HARDEN MODE prompt or mark a hardening round passed",
        "scafld.commands.execution:cmd_harden",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--mark-passed",), {"dest": "mark_passed", "action": "store_true", "help": "Mark the latest hardening round as passed"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable harden JSON"}),
        ),
    ),
    CommandSpec(
        "exec",
        "Run acceptance criteria and record results",
        "scafld.commands.execution:cmd_exec",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("-p", "--phase"), {"help": "Run only criteria for this phase (e.g. phase1)"}),
            ArgumentSpec(("-r", "--resume"), {"action": "store_true", "help": "Skip criteria that already passed"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable execution JSON"}),
        ),
    ),
    CommandSpec(
        "audit",
        "Check spec changes vs current git changes",
        "scafld.commands.audit:cmd_audit",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("-b", "--base"), {"help": "Git base ref for historical comparison (default: current working tree)"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable audit JSON"}),
        ),
    ),
    CommandSpec(
        "diff",
        "Show git history and changes for a spec",
        "scafld.commands.audit:cmd_diff",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
        ),
    ),
    CommandSpec(
        "report",
        "Aggregate stats across all specs",
        "scafld.commands.reporting:cmd_report",
        (
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable report JSON"}),
        ),
    ),
    CommandSpec(
        "update",
        "Refresh the managed scafld bundle",
        "scafld.commands.workspace:cmd_update",
        (
            ArgumentSpec(("--scan-root",), {"help": "Recursively update all scafld workspaces under this path"}),
            ArgumentSpec(("--self",), {"action": "store_true", "help": "git pull --ff-only the current scafld checkout before syncing workspaces"}),
            ArgumentSpec(("--verbose",), {"action": "store_true", "help": "Show created/updated bundle files for each workspace"}),
        ),
    ),
)

COMMAND_SPECS_BY_NAME = {spec.name: spec for spec in COMMAND_SPECS}


@lru_cache(maxsize=None)
def load_handler(handler_path):
    module_name, func_name = handler_path.split(":", 1)
    module = import_module(module_name)
    return getattr(module, func_name)


def resolve_command(name):
    if name == "version":
        return load_handler(VERSION_HANDLER_PATH)

    spec = COMMAND_SPECS_BY_NAME.get(name)
    if spec is None:
        return None
    return load_handler(spec.handler_path)


def build_parser():
    parser = argparse.ArgumentParser(
        prog="scafld",
        description="Spec-driven development framework for AI coding agents",
    )
    parser.add_argument("--version", action="store_true", help="Show version")
    sub = parser.add_subparsers(dest="command")

    for spec in COMMAND_SPECS:
        command_parser = sub.add_parser(spec.name, help=spec.help)
        for argument in spec.args:
            command_parser.add_argument(*argument.flags, **argument.kwargs)

    return parser
