import subprocess
import sys

from scafld.audit_scope import (
    active_declared_changes,
    build_audit_file_payloads,
    classify_active_overlap,
    collect_declared_change_map,
    describe_other_active_specs,
    filter_audit_paths,
)
from scafld.command_runtime import require_root
from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.git_state import list_changed_files_against_ref, list_working_tree_changed_files
from scafld.output import emit_command_json, error_payload
from scafld.session_store import effective_changed_paths, ensure_workspace_baseline, load_session
from scafld.spec_store import require_spec
from scafld.terminal import C_BOLD, C_CYAN, C_DIM, C_GREEN, C_RED, C_YELLOW, c


def cmd_audit(args):
    """Compare spec changes against actual git changes to detect scope creep."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    text = spec.read_text()
    json_mode = bool(getattr(args, "json", False))
    mode_label = "working tree" if not args.base else f"vs {args.base}"

    declared_changes = {
        path: ownership
        for path, ownership in collect_declared_change_map(text).items()
        if path != "TODO"
    }
    declared = set(declared_changes)
    other_active_changes = active_declared_changes(root, exclude_task_id=args.task_id)
    other_active_specs = {
        task_id: sorted(files)
        for task_id, files in other_active_changes.items()
    }
    overlap_state = classify_active_overlap(declared_changes, other_active_changes)
    shared_with_other_active = overlap_state["shared_with_other_active"]
    active_overlap = overlap_state["active_overlap"]
    shared_details = overlap_state["shared_details"]
    conflict_details = overlap_state["conflict_details"]
    other_by_file = overlap_state["other_by_file"]
    other_active_declared = set(other_by_file)

    if not declared:
        warning = "no files declared in spec phases"
        if json_mode:
            emit_command_json(
                "audit",
                task_id=args.task_id,
                state={"mode": mode_label},
                result={
                    "declared": [],
                    "matched": [],
                    "undeclared": [],
                    "missing": [],
                    "covered_by_other_active": [],
                    "covered_by_other_active_details": {},
                    "shared_with_other_active": [],
                    "shared_with_other_active_details": {},
                    "active_overlap": [],
                    "active_overlap_details": {},
                    "other_active_specs": other_active_specs,
                    "files": [],
                    "counts": {
                        "declared": 0,
                        "matched": 0,
                        "undeclared": 0,
                        "missing": 0,
                        "covered_by_other_active": 0,
                        "shared_with_other_active": 0,
                        "active_overlap": 0,
                    },
                },
                warnings=[warning],
            )
            return
        print(f"{c(C_YELLOW, 'warn')}: {warning}")
        return

    if args.base:
        actual, error = list_changed_files_against_ref(root, args.base)
        if actual is None:
            raise ScafldError(f"could not diff against base ref {args.base!r}", [error] if error else [])
    else:
        actual, error = list_working_tree_changed_files(root)
        if actual is None:
            raise ScafldError(
                "could not inspect current git working tree",
                [error] if error else ["scope audit requires a git work tree"],
            )

    actual = filter_audit_paths(actual)
    baseline_info = None
    if not args.base:
        session = load_session(root, args.task_id, spec_path=spec)
        if session is not None:
            baseline = session.get("workspace_baseline")
            if not isinstance(baseline, dict) or not isinstance(baseline.get("paths"), dict):
                session = ensure_workspace_baseline(
                    root,
                    args.task_id,
                    paths=sorted(set(actual) - declared),
                    source="audit_bootstrap",
                    spec_path=spec,
                )
                baseline = session.get("workspace_baseline")
            actual = set(effective_changed_paths(root, actual, session))
            if isinstance(baseline, dict):
                baseline_info = {
                    "source": baseline.get("source"),
                    "captured_at": baseline.get("captured_at"),
                    "tracked_paths": len(baseline.get("paths") or {}),
                }
        else:
            actual = set(actual)
    else:
        actual = set(actual)
    covered_by_other_active = (actual - declared) & other_active_declared
    undeclared = actual - declared - covered_by_other_active
    missing = declared - actual
    matched = declared & actual
    overlap_details = {}
    overlap_details.update(shared_details)
    overlap_details.update(conflict_details)
    covered_by_other_active_details = {
        path: [entry["task_id"] for entry in other_by_file.get(path, [])]
        for path in covered_by_other_active
    }
    files = build_audit_file_payloads(
        declared_changes,
        actual,
        covered_by_other_active,
        shared_with_other_active,
        active_overlap,
        overlap_details,
        other_by_file,
    )
    result = {
        "declared": sorted(declared),
        "matched": sorted(matched),
        "undeclared": sorted(undeclared),
        "missing": sorted(missing),
        "covered_by_other_active": sorted(covered_by_other_active),
        "covered_by_other_active_details": covered_by_other_active_details,
        "shared_with_other_active": sorted(shared_with_other_active),
        "shared_with_other_active_details": shared_details,
        "active_overlap": sorted(active_overlap),
        "active_overlap_details": conflict_details,
        "other_active_specs": other_active_specs,
        "files": files,
        "baseline": baseline_info,
        "counts": {
            "declared": len(declared),
            "matched": len(matched),
            "undeclared": len(undeclared),
            "missing": len(missing),
            "covered_by_other_active": len(covered_by_other_active),
            "shared_with_other_active": len(shared_with_other_active),
            "active_overlap": len(active_overlap),
        },
    }

    if json_mode:
        emit_command_json(
            "audit",
            ok=not bool(undeclared or active_overlap),
            task_id=args.task_id,
            state={"mode": mode_label},
            result=result,
            error=error_payload(
                "scope drift or active-spec overlap detected",
                code=EC.SCOPE_DRIFT,
                details=(
                    [f"{path} changed but is not declared in the spec" for path in sorted(undeclared)]
                    + [
                        f"{path} conflicts with active spec(s): {describe_other_active_specs(path, conflict_details)}"
                        for path in sorted(active_overlap)
                    ]
                ),
                exit_code=1,
            ) if undeclared or active_overlap else None,
        )
        if undeclared or active_overlap:
            sys.exit(1)
        return

    print(f"{c(C_BOLD, f'Audit: {args.task_id}')} ({mode_label})")
    print()
    if baseline_info:
        print(
            f"  {c(C_DIM, 'baseline')}: {baseline_info['source']} "
            f"({baseline_info['tracked_paths']} tracked path(s) at {baseline_info['captured_at']})"
        )
        print()

    if not actual:
        if args.base:
            print(f"  {c(C_DIM, f'No files changed vs {args.base}')}")
        else:
            print(f"  {c(C_DIM, 'No files changed in working tree')}")

    if matched:
        print(f"  {c(C_GREEN, 'Declared & changed')} ({len(matched)}):")
        for path in sorted(matched):
            suffix = f" [{declared_changes[path]}]" if declared_changes.get(path) != "exclusive" else ""
            print(f"    {c(C_GREEN, '✓')} {path}{suffix}")

    if undeclared:
        print(f"\n  {c(C_RED, 'Scope creep - changed but not in spec')} ({len(undeclared)}):")
        for path in sorted(undeclared):
            print(f"    {c(C_RED, '!')} {path}")

    if covered_by_other_active:
        print(f"\n  {c(C_CYAN, 'Covered by other active specs')} ({len(covered_by_other_active)}):")
        for path in sorted(covered_by_other_active):
            owners = describe_other_active_specs(path, covered_by_other_active_details)
            suffix = f" ({owners})" if owners else ""
            print(f"    {c(C_CYAN, '↷')} {path}{suffix}")

    if shared_with_other_active:
        print(f"\n  {c(C_CYAN, 'Shared with other active specs')} ({len(shared_with_other_active)}):")
        for path in sorted(shared_with_other_active):
            owners = describe_other_active_specs(path, shared_details)
            suffix = f" ({owners})" if owners else ""
            print(f"    {c(C_CYAN, '≈')} {path}{suffix}")

    if active_overlap:
        print(f"\n  {c(C_RED, 'Conflict - exclusive overlap with other active specs')} ({len(active_overlap)}):")
        for path in sorted(active_overlap):
            owners = describe_other_active_specs(path, conflict_details)
            suffix = f" ({owners})" if owners else ""
            print(f"    {c(C_RED, '×')} {path}{suffix}")

    if missing:
        print(f"\n  {c(C_YELLOW, 'In spec but not changed')} ({len(missing)}):")
        for path in sorted(missing):
            print(f"    {c(C_YELLOW, '?')} {path}")

    print()
    if undeclared:
        pct = len(undeclared) / len(actual) * 100 if actual else 0
        print(f"  {c(C_RED, f'Scope drift: {pct:.0f}%')} ({len(undeclared)}/{len(actual)} files undeclared)")
        sys.exit(1)
    if active_overlap:
        print(f"  {c(C_RED, 'Conflict')} - {len(active_overlap)} file(s) are claimed by multiple active specs")
        sys.exit(1)
    if not actual:
        print(f"  {c(C_GREEN, 'Clean')} - no scope drift detected")
    else:
        print(f"  {c(C_GREEN, 'Clean')} - all changes match spec")


def cmd_diff(args):
    """Show git diff for a spec file."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    rel = spec.relative_to(root)

    try:
        result = subprocess.run(
            ["git", "log", "--oneline", "-10", "--follow", "--", str(rel)],
            capture_output=True,
            text=True,
            cwd=str(root),
        )
        if result.stdout.strip():
            print(f"{c(C_BOLD, f'History: {rel}')}")
            print(result.stdout)
        else:
            print(f"{c(C_DIM, 'No git history for this spec')}")

        diff_result = subprocess.run(
            ["git", "diff", "--", str(rel)],
            capture_output=True,
            text=True,
            cwd=str(root),
        )
        if diff_result.stdout.strip():
            print(f"{c(C_BOLD, 'Uncommitted changes:')}")
            print(diff_result.stdout)
    except FileNotFoundError:
        print(f"{c(C_RED, 'error')}: git not found")
        sys.exit(1)
