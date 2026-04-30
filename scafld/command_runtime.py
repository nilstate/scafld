import os
from dataclasses import dataclass
from pathlib import Path

from scafld.error_codes import ErrorCode
from scafld.errors import ScafldError

SCAFLD_DIR = ".scafld"
SPECS_DIR = f"{SCAFLD_DIR}/specs"
CONFIG_PATH = f"{SCAFLD_DIR}/config.yaml"
CORE_MANIFEST_PATH = f"{SCAFLD_DIR}/core/manifest.json"
SCAN_PRUNE_DIRS = {
    ".git",
    ".hg",
    ".svn",
    ".venv",
    "__pycache__",
    "node_modules",
    "dist",
    "build",
    "coverage",
    ".next",
    ".turbo",
}


def is_scafld_workspace(root):
    """Detect a scafld workspace root."""
    scafld_root = root / SCAFLD_DIR
    return scafld_root.is_dir() and (
        (root / SPECS_DIR).is_dir()
        or (root / CONFIG_PATH).exists()
        or (root / CORE_MANIFEST_PATH).exists()
    )


def find_root(start=None):
    """Walk up from a path to find the nearest scafld workspace root."""
    current = Path(start or Path.cwd()).expanduser().resolve()
    while current != current.parent:
        if is_scafld_workspace(current):
            return current
        current = current.parent
    return current if is_scafld_workspace(current) else None


def require_root(start=None):
    """Return the nearest workspace root or raise a structured command error."""
    root = find_root(start)
    if root is None:
        raise ScafldError(
            "not in a scafld project (no .scafld/ directory found)",
            ["run 'scafld init' to set up a workspace"],
            code=ErrorCode.WORKSPACE_NOT_FOUND,
        )
    return root


def find_workspaces(scan_root):
    """Recursively discover scafld workspaces under a root path."""
    scan_root = Path(scan_root).expanduser().resolve()
    workspaces = []
    seen = set()

    for current, dirnames, _ in os.walk(scan_root):
        current_path = Path(current)
        next_dirs = []

        for dirname in dirnames:
            if dirname in SCAN_PRUNE_DIRS:
                continue
            if dirname == SCAFLD_DIR and is_scafld_workspace(current_path):
                if current_path not in seen:
                    workspaces.append(current_path)
                    seen.add(current_path)
                continue
            next_dirs.append(dirname)

        dirnames[:] = next_dirs

    workspaces.sort()
    return workspaces


@dataclass(frozen=True)
class CommandContext:
    """Shared command context rooted in one discovered workspace."""

    root: Path

    @classmethod
    def from_cwd(cls, start=None):
        return cls(root=require_root(start))
