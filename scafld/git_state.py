import hashlib
import subprocess
from pathlib import Path


def sha256_bytes(payload):
    return hashlib.sha256(payload).hexdigest()


def source_git_metadata(source_root):
    """Return source revision metadata when scafld is running from a git checkout."""
    metadata = {
        "commit": None,
        "dirty": None,
    }
    if not (source_root / ".git").exists():
        return metadata

    try:
        commit = subprocess.run(
            ["git", "-C", str(source_root), "rev-parse", "HEAD"],
            capture_output=True,
            text=True,
            timeout=5,
            check=False,
        )
        if commit.returncode == 0:
            metadata["commit"] = commit.stdout.strip() or None

        dirty = subprocess.run(
            ["git", "-C", str(source_root), "status", "--porcelain"],
            capture_output=True,
            text=True,
            timeout=5,
            check=False,
        )
        if dirty.returncode == 0:
            metadata["dirty"] = bool(dirty.stdout.strip())
    except Exception:
        pass

    return metadata


def run_git_capture(root, args, timeout=10):
    """Run git in the workspace root and return stdout bytes plus a decoded error string."""
    try:
        proc = subprocess.run(
            ["git", "-C", str(root), *args],
            capture_output=True,
            timeout=timeout,
            check=False,
        )
    except Exception as exc:
        return None, str(exc)

    if proc.returncode != 0:
        stderr = proc.stderr.decode("utf-8", errors="replace").strip()
        stdout = proc.stdout.decode("utf-8", errors="replace").strip()
        detail = stderr or stdout or f"git {' '.join(args)} exited with code {proc.returncode}"
        return None, detail

    return proc.stdout, None


def review_git_pathspec(excluded_rel=None):
    """Return a root-scoped pathspec that optionally excludes one relative path."""
    pathspec = ["--", "."]
    if excluded_rel:
        excluded = Path(excluded_rel).as_posix()
        pathspec.append(f":(exclude){excluded}")
    return pathspec


def capture_review_git_state(root, excluded_rel=None):
    """Capture the reviewed git state, excluding the review artifact itself."""
    empty = {
        "reviewed_head": None,
        "reviewed_dirty": None,
        "reviewed_diff": None,
    }

    inside_work_tree, error = run_git_capture(root, ["rev-parse", "--is-inside-work-tree"])
    if inside_work_tree is None:
        return empty, None
    if inside_work_tree.decode("utf-8", errors="replace").strip() != "true":
        return empty, None

    head_bytes, error = run_git_capture(root, ["rev-parse", "HEAD"])
    if head_bytes is None:
        return empty, error

    pathspec = review_git_pathspec(excluded_rel)

    status_bytes, error = run_git_capture(root, ["status", "--porcelain=v1", "-z", "--untracked-files=all", *pathspec])
    if status_bytes is None:
        return empty, error

    unstaged_bytes, error = run_git_capture(root, ["diff", "--no-ext-diff", "--binary", *pathspec])
    if unstaged_bytes is None:
        return empty, error

    staged_bytes, error = run_git_capture(root, ["diff", "--cached", "--no-ext-diff", "--binary", *pathspec])
    if staged_bytes is None:
        return empty, error

    untracked_bytes, error = run_git_capture(root, ["ls-files", "--others", "--exclude-standard", "-z", *pathspec])
    if untracked_bytes is None:
        return empty, error

    fingerprint = bytearray()
    fingerprint.extend(b"unstaged\0")
    fingerprint.extend(unstaged_bytes)
    fingerprint.extend(b"\0staged\0")
    fingerprint.extend(staged_bytes)
    fingerprint.extend(b"\0untracked\0")

    for raw_path in sorted(path for path in untracked_bytes.split(b"\0") if path):
        rel_path = raw_path.decode("utf-8", errors="surrogateescape")
        candidate = root / rel_path
        fingerprint.extend(raw_path)
        fingerprint.extend(b"\0")
        if candidate.exists() and candidate.is_file():
            try:
                fingerprint.extend(candidate.read_bytes())
            except OSError as exc:
                return empty, f"could not read untracked file {rel_path}: {exc}"
        else:
            fingerprint.extend(b"<missing-or-non-file>")
        fingerprint.extend(b"\0")

    return {
        "reviewed_head": head_bytes.decode("utf-8", errors="replace").strip() or None,
        "reviewed_dirty": bool(status_bytes.rstrip(b"\0")),
        "reviewed_diff": sha256_bytes(bytes(fingerprint)),
    }, None
