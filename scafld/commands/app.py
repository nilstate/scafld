#!/usr/bin/env python3
"""Packaged scafld command entrypoint."""

import sys

from scafld.errors import ScafldError
from scafld.output import emit_cli_error, emit_command_json, error_payload

from scafld.terminal import C_RED, c
from .surface import build_parser, command_spec, resolve_command

__all__ = ["main"]


def main(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    command_name = next((arg for arg in argv if not arg.startswith("-")), None)
    spec = command_spec(command_name) if command_name else None
    include_advanced = "--advanced" in argv or (spec is not None and not spec.public)
    parser = build_parser(include_advanced=include_advanced)
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
