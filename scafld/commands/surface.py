import argparse
from dataclasses import dataclass, field
from functools import lru_cache
from importlib import import_module

from scafld.review_runner import REVIEW_PROVIDER_VALUES, REVIEW_RUNNER_VALUES


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
    public: bool = True
    help_visible: bool = True


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
        "plan",
        "Create a draft spec or reopen an existing draft harden round",
        "scafld.commands.workflow:cmd_plan",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID (kebab-case)"}),
            ArgumentSpec(("-t", "--title"), {"help": "Task title"}),
            ArgumentSpec(("-s", "--size"), {"choices": ["micro", "small", "medium", "large"]}),
            ArgumentSpec(("-r", "--risk"), {"choices": ["low", "medium", "high"]}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable JSON"}),
        ),
    ),
    CommandSpec(
        "new",
        "Create a draft spec from the native split lifecycle surface",
        "scafld.commands.lifecycle:cmd_new",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID (kebab-case)"}),
            ArgumentSpec(("-t", "--title"), {"help": "Task title"}),
            ArgumentSpec(("-s", "--size"), {"choices": ["micro", "small", "medium", "large"]}),
            ArgumentSpec(("-r", "--risk"), {"choices": ["low", "medium", "high"]}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable JSON"}),
        ),
        public=False,
        help_visible=False,
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
        "build",
        "Start approved work and run validation until blocked or complete",
        "scafld.commands.workflow:cmd_build",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable JSON"}),
        ),
    ),
    CommandSpec(
        "start",
        "Move an approved spec into active execution",
        "scafld.commands.lifecycle:cmd_start",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable start JSON"}),
        ),
        public=False,
        help_visible=False,
    ),
    CommandSpec(
        "exec",
        "Run acceptance criteria from the native split lifecycle surface",
        "scafld.commands.execution:cmd_exec",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--phase",), {"help": "Restrict execution to one phase id"}),
            ArgumentSpec(("--resume",), {"action": "store_true", "help": "Skip already-passed criteria"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable execution JSON"}),
        ),
    ),
    CommandSpec(
        "review",
        "Run the adversarial review gate and execute or emit the challenger handoff",
        "scafld.commands.review:cmd_review",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--runner",), {"choices": REVIEW_RUNNER_VALUES, "help": "Review runner mode override"}),
            ArgumentSpec(("--provider",), {"choices": REVIEW_PROVIDER_VALUES, "help": "External review provider override"}),
            ArgumentSpec(("--model",), {"help": "External review model override"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable review handoff JSON"}),
        ),
    ),
    CommandSpec(
        "complete",
        "Archive a reviewed task",
        "scafld.commands.review:cmd_complete",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--human-reviewed",), {"action": "store_true", "help": "Allow a human-confirmed override when the review gate is blocked"}),
            ArgumentSpec(("--reason",), {"help": "Required with --human-reviewed; records why the override was applied"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable completion JSON"}),
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
        "report",
        "Aggregate stats across all specs",
        "scafld.commands.reporting:cmd_report",
        (
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable report JSON"}),
            ArgumentSpec(("--runtime-only",), {"action": "store_true", "help": "Restrict the report to specs with session runtime data"}),
        ),
    ),
    CommandSpec(
        "handoff",
        "Render the current handoff for a task without moving the lifecycle",
        "scafld.commands.handoff:cmd_handoff",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--phase",), {"help": "Render a phase handoff for this phase id"}),
            ArgumentSpec(("--recovery",), {"help": "Render a recovery handoff for this failed acceptance criterion"}),
            ArgumentSpec(("--review",), {"action": "store_true", "help": "Render the current review handoff"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable handoff JSON"}),
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
        public=False,
    ),
    CommandSpec(
        "sync",
        "Compare a spec's recorded origin binding with live git state",
        "scafld.commands.lifecycle:cmd_sync",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable sync JSON"}),
        ),
        public=False,
    ),
    CommandSpec(
        "summary",
        "Render a concise markdown or JSON summary from spec state",
        "scafld.commands.projections:cmd_summary",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable summary JSON"}),
        ),
        public=False,
    ),
    CommandSpec(
        "checks",
        "Render CI-friendly check state from spec state",
        "scafld.commands.projections:cmd_checks",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable check JSON"}),
        ),
        public=False,
    ),
    CommandSpec(
        "pr-body",
        "Render a deterministic PR body from spec state",
        "scafld.commands.projections:cmd_pr_body",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable PR-body JSON"}),
        ),
        public=False,
    ),
    CommandSpec(
        "fail",
        "Mark spec as failed",
        "scafld.commands.lifecycle:cmd_fail",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable failure-archive JSON"}),
        ),
        public=False,
    ),
    CommandSpec(
        "cancel",
        "Cancel a spec",
        "scafld.commands.lifecycle:cmd_cancel",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--reason",), {"help": "Cancellation reason"}),
            ArgumentSpec(("--superseded-by",), {"dest": "superseded_by", "help": "Replacement task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable cancellation JSON"}),
        ),
        public=False,
    ),
    CommandSpec(
        "validate",
        "Validate spec against schema",
        "scafld.commands.lifecycle:cmd_validate",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("--json",), {"action": "store_true", "help": "Emit machine-readable validation JSON"}),
        ),
        public=False,
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
        public=False,
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
        public=False,
    ),
    CommandSpec(
        "diff",
        "Show git history and changes for a spec",
        "scafld.commands.audit:cmd_diff",
        (
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
        ),
        public=False,
    ),
    CommandSpec(
        "adapter",
        "Run a provider against the current scafld handoff",
        "scafld.commands.adapter:cmd_adapter",
        (
            ArgumentSpec(("provider",), {"choices": ["codex", "claude"]}),
            ArgumentSpec(("mode",), {"choices": ["build", "review"]}),
            ArgumentSpec(("task_id",), {"help": "Task ID"}),
            ArgumentSpec(("provider_args",), {"nargs": argparse.REMAINDER}),
        ),
        public=False,
        help_visible=False,
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


def command_spec(name):
    return COMMAND_SPECS_BY_NAME.get(name)


def build_parser(*, include_advanced=False, active_command=None):
    parser = argparse.ArgumentParser(
        prog="scafld",
        description=(
            "Build long-running AI coding work under adversarial review.\n"
            "Use --advanced with --help to show the full operator command surface."
        ),
    )
    parser.add_argument("--version", action="store_true", help="Show version")
    parser.add_argument(
        "--advanced",
        action="store_true",
        help=argparse.SUPPRESS if not include_advanced else "Show advanced and legacy commands in help",
    )
    sub = parser.add_subparsers(dest="command")

    for spec in COMMAND_SPECS:
        if not spec.help_visible and spec.name != active_command:
            continue
        if not include_advanced and not spec.public:
            continue
        command_parser = sub.add_parser(spec.name, help=spec.help, description=spec.help)
        for argument in spec.args:
            command_parser.add_argument(*argument.flags, **argument.kwargs)

    return parser
