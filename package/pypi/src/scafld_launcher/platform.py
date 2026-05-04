import os
import platform
from dataclasses import dataclass


@dataclass(frozen=True)
class Target:
    goos: str
    goarch: str
    ext: str = ""


def target() -> Target:
    system = platform.system().lower()
    machine = platform.machine().lower()

    goos = {
        "darwin": "darwin",
        "linux": "linux",
        "windows": "windows",
    }.get(system)
    goarch = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "aarch64": "arm64",
        "arm64": "arm64",
    }.get(machine)

    if not goos or not goarch:
        raise RuntimeError(f"unsupported platform: {system}/{machine}")
    return Target(goos=goos, goarch=goarch, ext=".exe" if goos == "windows" else "")


def cache_root() -> str:
    if override := os.environ.get("SCAFLD_INSTALL_DIR"):
        return override
    if platform.system().lower() == "windows":
        base = os.environ.get("LOCALAPPDATA") or os.path.expanduser("~\\AppData\\Local")
        return os.path.join(base, "scafld", "bin")
    return os.path.join(os.path.expanduser("~"), ".cache", "scafld", "bin")
