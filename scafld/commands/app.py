#!/usr/bin/env python3
"""Packaged scafld command entrypoint."""

import sys

from scafld.errors import ScafldError
from scafld.output import emit_cli_error, emit_command_json, error_payload

from scafld.terminal import C_RED, c
from .surface import build_parser, resolve_command

__all__ = ["main"]


def main(argv=None):
    parser = build_parser()
    args = parser.parse_args(argv)

    if args.version:
        resolve_command("version")(args)
        return

    command = resolve_command(args.command)
    if command is None:
        parser.print_help()
        sys.exit(1)

    try:
        command(args)
    except ScafldError as error:
        if getattr(args, "json", False):
            emit_command_json(
                args.command,
                ok=False,
                task_id=getattr(args, "task_id", None),
                error=error_payload(error),
            )
        else:
            emit_cli_error(error, colorize=c, red_code=C_RED)
        sys.exit(error.exit_code)


if __name__ == "__main__":
    main()
