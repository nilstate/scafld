import os
import subprocess
import sys

from scafld_launcher.install import ensure_binary


def main() -> int:
    try:
        binary = ensure_binary()
    except Exception as exc:
        print(f"scafld: unable to install native binary: {exc}", file=sys.stderr)
        return 127

    argv = [binary, *sys.argv[1:]]
    if os.name == "posix":
        os.execv(binary, argv)
        return 127
    completed = subprocess.run(argv)
    return completed.returncode
