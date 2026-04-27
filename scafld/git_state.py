import hashlib
import subprocess
from pathlib import Path

from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.projections import build_origin_binding, stored_sync_snapshot
from scafld.spec_store import prune_empty


def sha256_bytes(payload):
    return hashlib.sha256(payload).hexdigest()


def decode_text(payload):
    """Decode git stdout bytes into a trimmed text string."""
    return payload.decode("utf-8", errors="replace").strip()


def decode_path_list(payload):
    """Decode a NUL-delimited git path list into sorted relative paths."""
    return sorted(
        raw_path.decode("utf-8", errors="surrogateescape")
        for raw_path in payload.split(b"\0")
        if raw_path
    )


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


def run_git_text(root, args, timeout=10):
    """Run git and decode stdout into text."""
    payload, error = run_git_capture(root, args, timeout=timeout)
    if payload is None:
        return None, error
    return decode_text(payload) or None, None


def run_git_text_optional(root, args, timeout=10):
    """Run git for optional metadata; return None on lookup failure."""
    payload, error = run_git_capture(root, args, timeout=timeout)
    if payload is None:
        return None
    return decode_text(payload) or None


def git_pathspec(excluded_rels=None):
    """Return a root-scoped pathspec that can exclude one or more relative paths."""
    pathspec = ["--", "."]
    for excluded_rel in excluded_rels or []:
        if excluded_rel:
            excluded = Path(excluded_rel).as_posix()
            pathspec.append(f":(exclude){excluded}")
    return pathspec


def review_git_pathspec(excluded_rel=None):
    """Return a root-scoped pathspec that excludes one or more review paths."""
    if excluded_rel is None:
        return git_pathspec()
    if isinstance(excluded_rel, (str, Path)):
        return git_pathspec([excluded_rel])
    return git_pathspec(excluded_rel)


def _is_git_work_tree(root):
    payload, _error = run_git_capture(root, ["rev-parse", "--is-inside-work-tree"])
    return payload is not None and payload.decode("utf-8", errors="replace").strip() == "true"


def _path_is_excluded(path, excluded_rels=None):
    normalized = Path(path).as_posix().rstrip("/")
    if not normalized:
        return False
    for excluded_rel in excluded_rels or []:
        excluded = Path(excluded_rel).as_posix().rstrip("/")
        if not excluded:
            continue
        if normalized == excluded or normalized.startswith(f"{excluded}/") or excluded.startswith(f"{normalized}/"):
            return True
    return False


def list_submodule_paths(root):
    """Return configured submodule paths for this work tree."""
    gitmodules = Path(root) / ".gitmodules"
    if not gitmodules.exists():
        return [], None

    payload, error = run_git_capture(
        root,
        ["config", "--file", ".gitmodules", "--get-regexp", r"^submodule\..*\.path$"],
    )
    if payload is None:
        if error and "exited with code 1" not in error:
            return None, error
        return [], None

    paths = []
    for line in decode_text(payload).splitlines():
        parts = line.split(None, 1)
        if len(parts) != 2:
            continue
        raw_path = parts[1].strip()
        submodule_path = Path(raw_path)
        if submodule_path.is_absolute() or ".." in submodule_path.parts:
            continue
        paths.append(submodule_path.as_posix())
    return sorted(set(paths)), None


def ref_exists(root, ref_name):
    """Return True when a commit-ish ref exists locally."""
    payload, _error = run_git_capture(root, ["rev-parse", "--verify", "--quiet", f"{ref_name}^{{commit}}"])
    return payload is not None


def branch_exists(root, branch_name):
    """Return True when a local branch exists."""
    return ref_exists(root, f"refs/heads/{branch_name}")


def list_remotes(root):
    """Return configured git remotes for the repo."""
    output, error = run_git_text(root, ["remote"])
    if error:
        return [], error
    if not output:
        return [], None
    return [line.strip() for line in output.splitlines() if line.strip()], None


def remote_from_upstream(upstream):
    """Return the remote name extracted from an upstream ref."""
    if upstream and "/" in upstream:
        return upstream.split("/", 1)[0]
    return None


def default_remote(root):
    """Prefer origin, otherwise return the first configured remote."""
    remotes, error = list_remotes(root)
    if error:
        return None, error
    if "origin" in remotes:
        return "origin", None
    return (remotes[0], None) if remotes else (None, None)


def current_branch(root):
    """Return the current branch name, or None when detached."""
    branch_name, error = run_git_text(root, ["rev-parse", "--abbrev-ref", "HEAD"])
    if error:
        return None, error
    if branch_name == "HEAD":
        return None, None
    return branch_name, None


def current_upstream(root):
    """Return the symbolic upstream ref for the current branch, if configured."""
    return run_git_text_optional(root, ["rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"])


def remote_url(root, remote_name):
    """Return the configured URL for a remote, if available."""
    if not remote_name:
        return None
    return run_git_text_optional(root, ["remote", "get-url", remote_name])


def working_tree_dirty(root, excluded_rels=None):
    """Return whether the workspace has changes outside excluded paths."""
    changed, error = list_working_tree_changed_files(root, excluded_rels=excluded_rels)
    if changed is None:
        return None, error
    return bool(changed), None


def resolve_default_base_ref(root, remote_name=None):
    """Return the best local default base ref for branch creation."""
    candidate_remotes = []
    if remote_name:
        candidate_remotes.append(remote_name)
    remotes, error = list_remotes(root)
    if error:
        return None, error
    for remote in remotes:
        if remote not in candidate_remotes:
            candidate_remotes.append(remote)

    for remote in candidate_remotes:
        symbolic = run_git_text_optional(root, ["symbolic-ref", f"refs/remotes/{remote}/HEAD"])
        if symbolic and symbolic.startswith("refs/remotes/"):
            return symbolic.removeprefix("refs/remotes/"), None

    for local_name in ("main", "master", "trunk", "develop"):
        if branch_exists(root, local_name):
            return local_name, None

    branch_name, error = current_branch(root)
    if error:
        return None, error
    return branch_name, None


def capture_workspace_git_state(root, excluded_rels=None):
    """Capture the live git facts needed for origin binding and sync checks."""
    repo_root, error = run_git_text(root, ["rev-parse", "--show-toplevel"])
    if error:
        return None, error

    head_sha, error = run_git_text(root, ["rev-parse", "HEAD"])
    if error:
        return None, error

    branch_name, error = current_branch(root)
    if error:
        return None, error

    dirty, error = working_tree_dirty(root, excluded_rels=excluded_rels)
    if error:
        return None, error

    upstream = current_upstream(root)
    remote_name = remote_from_upstream(upstream)
    if not remote_name:
        remote_name, error = default_remote(root)
        if error:
            return None, error

    default_base_ref, error = resolve_default_base_ref(root, remote_name=remote_name)
    if error:
        return None, error

    return {
        "repo_root": repo_root,
        "branch": branch_name,
        "head_sha": head_sha,
        "upstream": upstream,
        "remote": remote_name,
        "remote_url": remote_url(root, remote_name),
        "default_base_ref": default_base_ref,
        "dirty": bool(dirty),
        "detached": branch_name is None,
    }, None


def refresh_origin_sync(root, origin=None, *, checked_at, excluded_rels=None):
    """Refresh one stored origin block with the current sync snapshot."""
    sync = build_origin_sync_payload(root, origin, excluded_rels=excluded_rels)
    stored_origin = origin if isinstance(origin, dict) else {}
    refreshed_origin = dict(stored_origin)
    refreshed_origin["sync"] = stored_sync_snapshot(sync, checked_at=checked_at)
    return prune_empty(refreshed_origin), sync


def run_git_mutation(root, args, timeout=30):
    """Run a mutating git command and return an optional error string."""
    _payload, error = run_git_capture(root, args, timeout=timeout)
    return error


def checkout_branch(root, branch_name):
    """Checkout an existing local branch."""
    return run_git_mutation(root, ["checkout", branch_name])


def create_branch(root, branch_name, base_ref):
    """Create and checkout a new local branch from a base ref."""
    return run_git_mutation(root, ["checkout", "-b", branch_name, base_ref])


def bind_task_branch(
    root,
    task_id,
    existing_origin=None,
    *,
    name=None,
    base=None,
    bind_current=False,
    bound_at,
    excluded_rels=None,
):
    """Create or bind a task branch and return updated origin metadata."""
    live_state, error = capture_workspace_git_state(root, excluded_rels=excluded_rels)
    if live_state is None:
        raise ScafldError(
            "branch requires a git work tree",
            [error] if error else [],
            code=EC.GIT_REPOSITORY_REQUIRED,
        )

    existing_origin = existing_origin if isinstance(existing_origin, dict) else {}
    existing_git = existing_origin.get("git") if isinstance(existing_origin.get("git"), dict) else {}

    if bind_current:
        if live_state.get("detached"):
            raise ScafldError(
                "cannot bind the current branch from a detached HEAD",
                ["checkout a branch first or rerun without --bind-current"],
                code=EC.DETACHED_HEAD,
            )
        current_branch_name = live_state.get("branch")
        if name and name != current_branch_name:
            raise ScafldError(
                "--name must match the current branch when used with --bind-current",
                [f"current branch: {current_branch_name}"],
                code=EC.INVALID_ARGUMENTS,
            )
        desired_branch = current_branch_name
        base_ref = base or existing_git.get("base_ref") or live_state.get("default_base_ref")
        action = "bound_current"
    else:
        desired_branch = name or existing_git.get("branch") or task_id
        base_ref = base or existing_git.get("base_ref") or live_state.get("default_base_ref")

        if live_state.get("detached"):
            raise ScafldError(
                "cannot switch branches from a detached HEAD",
                ["checkout a branch first, then rerun scafld branch"],
                code=EC.DETACHED_HEAD,
            )

        if live_state.get("branch") == desired_branch:
            action = "bound_current"
        else:
            if live_state.get("dirty"):
                raise ScafldError(
                    "refusing to switch branches with uncommitted changes",
                    ["commit or stash changes, or use --bind-current to record the current branch as-is"],
                    code=EC.DIRTY_WORKTREE,
                )
            if branch_exists(root, desired_branch):
                error = checkout_branch(root, desired_branch)
                if error:
                    raise ScafldError(
                        f"could not checkout branch {desired_branch}",
                        [error],
                        code=EC.GIT_CHECKOUT_FAILED,
                    )
                action = "checked_out_existing"
            else:
                if not base_ref:
                    raise ScafldError(
                        "could not determine a base ref for branch creation",
                        ["provide --base explicitly or create a main/master branch first"],
                        code=EC.BASE_REF_REQUIRED,
                    )
                if not ref_exists(root, base_ref):
                    raise ScafldError(
                        f"base ref does not exist locally: {base_ref}",
                        ["fetch or create the base ref, then retry"],
                        code=EC.BASE_REF_NOT_FOUND,
                    )
                error = create_branch(root, desired_branch, base_ref)
                if error:
                    raise ScafldError(
                        f"could not create branch {desired_branch} from {base_ref}",
                        [error],
                        code=EC.GIT_BRANCH_CREATE_FAILED,
                    )
                action = "created_branch"

    live_state, error = capture_workspace_git_state(root, excluded_rels=excluded_rels)
    if live_state is None:
        raise ScafldError(
            "could not capture git state after binding the branch",
            [error] if error else [],
            code=EC.GIT_STATE_UNAVAILABLE,
        )

    origin = build_origin_binding(
        existing_origin,
        live_state,
        desired_branch,
        base_ref,
        action,
        bound_at=bound_at,
    )
    origin, sync = refresh_origin_sync(root, origin, checked_at=bound_at, excluded_rels=excluded_rels)
    return {
        "action": action,
        "branch": desired_branch,
        "base_ref": base_ref,
        "origin": origin,
        "sync": sync,
    }


def build_origin_sync_payload(root, origin=None, excluded_rels=None):
    """Compare stored origin metadata with live git state."""
    origin = origin or {}
    repo_meta = origin.get("repo") if isinstance(origin.get("repo"), dict) else {}
    git_meta = origin.get("git") if isinstance(origin.get("git"), dict) else {}
    expected = {
        "remote": repo_meta.get("remote"),
        "remote_url": repo_meta.get("remote_url"),
        "branch": git_meta.get("branch"),
        "base_ref": git_meta.get("base_ref"),
        "upstream": git_meta.get("upstream"),
    }

    actual, error = capture_workspace_git_state(root, excluded_rels=excluded_rels)
    if actual is None:
        return {
            "bound": bool(expected["branch"]),
            "status": "unavailable",
            "reasons": [error] if error else [],
            "expected": expected,
            "actual": {},
        }

    reasons = []
    if expected["branch"]:
        if actual["detached"]:
            reasons.append("workspace is on a detached HEAD")
        if actual["dirty"]:
            reasons.append("workspace has uncommitted changes")
        if actual["branch"] != expected["branch"]:
            current = actual["branch"] or "HEAD"
            reasons.append(f"current branch is {current}, expected {expected['branch']}")
        if expected["upstream"] and actual["upstream"] != expected["upstream"]:
            current = actual["upstream"] or "none"
            reasons.append(f"current upstream is {current}, expected {expected['upstream']}")
        if expected["remote"] and actual["remote"] != expected["remote"]:
            current = actual["remote"] or "none"
            reasons.append(f"current remote is {current}, expected {expected['remote']}")
        if expected["remote_url"] and actual["remote_url"] != expected["remote_url"]:
            current = actual["remote_url"] or "none"
            reasons.append(f"current remote URL is {current}, expected {expected['remote_url']}")
        if expected["base_ref"] and not ref_exists(root, expected["base_ref"]):
            reasons.append(f"expected base ref {expected['base_ref']} is not available locally")

    status = "unbound"
    if expected["branch"]:
        status = "drift" if reasons else "in_sync"

    return {
        "bound": bool(expected["branch"]),
        "status": status,
        "reasons": reasons,
        "expected": expected,
        "actual": actual,
    }


def _submodule_working_tree_changed_map(root, excluded_rels=None):
    submodule_paths, error = list_submodule_paths(root)
    if submodule_paths is None:
        return None, error

    changed = {}
    for submodule_path in submodule_paths:
        if _path_is_excluded(submodule_path, excluded_rels):
            continue
        submodule_root = Path(root) / submodule_path
        if not _is_git_work_tree(submodule_root):
            continue
        nested_changed, error = list_working_tree_changed_files(submodule_root)
        if nested_changed is None:
            return None, f"submodule {submodule_path}: {error}"
        if nested_changed:
            changed[submodule_path] = [
                f"{submodule_path}/{path}"
                for path in nested_changed
            ]
    return changed, None


def list_working_tree_changed_files(root, excluded_rels=None):
    """Return tracked and untracked file paths changed in the current workspace."""
    pathspec = git_pathspec(excluded_rels)
    changed = set()

    commands = (
        ["diff", "--name-only", "-z", "--no-ext-diff", *pathspec],
        ["diff", "--cached", "--name-only", "-z", "--no-ext-diff", *pathspec],
        ["ls-files", "--others", "--exclude-standard", "-z", *pathspec],
    )
    for args in commands:
        payload, error = run_git_capture(root, args)
        if payload is None:
            return None, error
        changed.update(decode_path_list(payload))

    submodule_changed, error = _submodule_working_tree_changed_map(root, excluded_rels=excluded_rels)
    if submodule_changed is None:
        return None, error
    for submodule_path, nested_paths in submodule_changed.items():
        changed.discard(submodule_path)
        changed.update(nested_paths)

    return sorted(changed), None


def list_changed_files_against_ref(root, base_ref, excluded_rels=None):
    """Return file paths changed relative to a git base ref plus current untracked files."""
    pathspec = git_pathspec(excluded_rels)
    diff_bytes, error = run_git_capture(
        root,
        ["diff", "--name-only", "-z", "--no-ext-diff", base_ref, *pathspec],
    )
    if diff_bytes is None:
        return None, error

    untracked_bytes, error = run_git_capture(
        root,
        ["ls-files", "--others", "--exclude-standard", "-z", *pathspec],
    )
    if untracked_bytes is None:
        return None, error

    changed = set(decode_path_list(diff_bytes))
    changed.update(decode_path_list(untracked_bytes))

    submodule_changed, error = _submodule_working_tree_changed_map(root, excluded_rels=excluded_rels)
    if submodule_changed is None:
        return None, error
    for submodule_path, nested_paths in submodule_changed.items():
        changed.discard(submodule_path)
        changed.update(nested_paths)

    return sorted(changed), None


def _submodule_review_state(root, excluded_rel=None):
    submodule_paths, error = list_submodule_paths(root)
    if submodule_paths is None:
        return None, False, error

    fingerprint = bytearray()
    dirty = False
    for submodule_path in submodule_paths:
        if _path_is_excluded(submodule_path, excluded_rel):
            continue
        submodule_root = Path(root) / submodule_path
        if not _is_git_work_tree(submodule_root):
            continue
        state, error = capture_review_git_state(submodule_root)
        if error:
            return None, False, f"submodule {submodule_path}: {error}"
        fingerprint.extend(submodule_path.encode("utf-8", errors="surrogateescape"))
        fingerprint.extend(b"\0")
        for key in ("reviewed_head", "reviewed_dirty", "reviewed_diff"):
            fingerprint.extend(str(state.get(key)).encode("utf-8", errors="surrogateescape"))
            fingerprint.extend(b"\0")
        dirty = dirty or bool(state.get("reviewed_dirty"))
    return bytes(fingerprint), dirty, None


def capture_review_git_state(root, excluded_rel=None):
    """Capture the reviewed git state, excluding review control-plane paths."""
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

    submodule_fingerprint, submodule_dirty, error = _submodule_review_state(root, excluded_rel)
    if submodule_fingerprint is None:
        return empty, error
    fingerprint.extend(b"\0submodules\0")
    fingerprint.extend(submodule_fingerprint)

    return {
        "reviewed_head": head_bytes.decode("utf-8", errors="replace").strip() or None,
        "reviewed_dirty": bool(status_bytes.rstrip(b"\0")) or submodule_dirty,
        "reviewed_diff": sha256_bytes(bytes(fingerprint)),
    }, None
