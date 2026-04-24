import sys

from scafld.adapter_runtime import run_provider
from scafld.command_runtime import require_root
from scafld.errors import ScafldError


def cmd_adapter(args):
    root = require_root()
    try:
        exit_code = run_provider(
            root,
            args.provider,
            args.mode,
            args.task_id,
            list(getattr(args, "provider_args", []) or []),
        )
    except ValueError as exc:
        raise ScafldError(str(exc))
    sys.exit(exit_code)
