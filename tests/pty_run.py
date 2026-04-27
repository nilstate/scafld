#!/usr/bin/env python3
"""Run a shell command under a pseudo-terminal for smoke tests."""

from __future__ import annotations

import os
import pty
import signal
import sys


def main() -> int:
    command = os.environ.get("SCAFLD_SMOKE_PTY_COMMAND")
    if not command:
        print("SCAFLD_SMOKE_PTY_COMMAND is required", file=sys.stderr)
        return 2

    status = pty.spawn(["/bin/sh", "-lc", command])
    if os.WIFEXITED(status):
        return os.WEXITSTATUS(status)
    if os.WIFSIGNALED(status):
        return 128 + os.WTERMSIG(status)
    if os.WIFSTOPPED(status):
        return 128 + signal.SIGTSTP
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
