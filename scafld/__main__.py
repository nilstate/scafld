import os
import sysconfig
from pathlib import Path


def runtime_root():
    override = os.environ.get("SCAFLD_SOURCE_ROOT")
    if override:
        return Path(override).expanduser().resolve()

    candidates = [
        Path(sysconfig.get_path("data")) / "share" / "scafld",
        Path(__file__).resolve().parents[1],
    ]
    for candidate in candidates:
        if (candidate / "cli" / "scafld").exists():
            return candidate

    raise RuntimeError("scafld runtime files not found")


def main():
    root = runtime_root()
    os.environ.setdefault("SCAFLD_SOURCE_ROOT", str(root))
    from scafld.commands.app import main as run_main

    run_main()


if __name__ == "__main__":
    main()
