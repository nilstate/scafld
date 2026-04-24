import sys
from pathlib import Path

from scafld.audit_scope import git_sync_excluded_paths
from scafld.command_runtime import find_root, require_root
from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.git_state import bind_task_branch, refresh_origin_sync
from scafld.lifecycle_runtime import (
    approve_spec_snapshot,
    new_spec_snapshot,
    start_spec_snapshot,
    status_snapshot,
    validation_snapshot,
)
from scafld.output import emit_command_json, error_payload
from scafld.projections import humanize_binding_mode, origin_payload, summarize_origin_source
from scafld.runtime_bundle import (
    ACTIVE_DIR,
    APPROVED_DIR,
    ARCHIVE_DIR,
    DRAFTS_DIR,
    REVIEWS_DIR,
    sync_framework_bundle,
)
from scafld.runtime_contracts import archive_run_artifacts
from scafld.spec_parsing import count_phases, now_iso
from scafld.spec_store import (
    find_all_specs,
    load_spec_document,
    move_spec,
    require_spec,
    write_spec_document,
    yaml_read_nested,
)
from scafld.terminal import C_BOLD, C_CYAN, C_DIM, C_GREEN, C_RED, C_YELLOW, STATUS_COLORS, c


def print_move_result(root, move_result):
    transition = move_result_payload(root, move_result)
    print(f"{c(C_GREEN, '  moved')}: {transition['from']} -> {transition['to']}")
    print(f" {c(C_DIM, 'status')}: {c(STATUS_COLORS.get(transition['status'], ''), transition['status'])}")


def move_result_payload(root, move_result):
    return {
        "from": str(move_result.source.relative_to(root)),
        "to": str(move_result.dest.relative_to(root)),
        "status": move_result.new_status,
    }


def filter_specs(specs, filter_text):
    if not filter_text:
        return specs

    flt = filter_text.lower()
    if flt in ("draft", "drafts"):
        return [(spec_path, label) for spec_path, label in specs if label == "drafts"]
    if flt in ("approved",):
        return [(spec_path, label) for spec_path, label in specs if label == "approved"]
    if flt in ("active", "in_progress"):
        return [(spec_path, label) for spec_path, label in specs if label == "active"]
    if flt in ("archive", "archived", "completed", "done"):
        return [(spec_path, label) for spec_path, label in specs if label.startswith("archive")]
    return [(spec_path, label) for spec_path, label in specs if flt in spec_path.stem.lower()]


def listing_groups(specs):
    groups = {}
    for spec_path, label in specs:
        groups.setdefault(label, []).append(spec_path)
    return groups


def branch_binding_snapshot(root, spec, task_id, *, name=None, base=None, bind_current=False):
    rel = spec.relative_to(root)
    data = load_spec_document(spec)
    status = data.get("status")

    if status in ("completed", "failed", "cancelled"):
        raise ScafldError(
            f"cannot bind a branch for a {status} spec",
            [f"spec is archived at {rel}"],
            code=EC.INVALID_SPEC_STATUS,
        )

    timestamp = now_iso()
    binding = bind_task_branch(
        root,
        task_id,
        origin_payload(data),
        name=name,
        base=base,
        bind_current=bind_current,
        bound_at=timestamp,
        excluded_rels=git_sync_excluded_paths(),
    )
    data["origin"] = binding["origin"]
    data["updated"] = timestamp
    write_spec_document(spec, data)

    return {
        "state": {
            "file": str(rel),
            "status": status,
            "branch": binding["branch"],
        },
        "result": {
            "action": binding["action"],
            "origin": origin_payload(data),
            "sync": binding["sync"],
        },
        "binding": binding,
    }


def sync_snapshot(root, spec, task_id):
    rel = spec.relative_to(root)
    data = load_spec_document(spec)
    status = data.get("status")
    origin = origin_payload(data)
    git_binding = origin.get("git") if isinstance(origin.get("git"), dict) else {}

    if not git_binding.get("branch"):
        raise ScafldError(
            "spec has no bound branch or origin metadata yet",
            [f"bind one first: scafld branch {task_id}"],
            code=EC.ORIGIN_UNBOUND,
            next_action=f"scafld branch {task_id}",
        )

    timestamp = now_iso()
    data["origin"], sync = refresh_origin_sync(
        root,
        data.get("origin"),
        checked_at=timestamp,
        excluded_rels=git_sync_excluded_paths(),
    )
    data["updated"] = timestamp
    write_spec_document(spec, data)

    sync_status = sync.get("status")
    ok = sync_status == "in_sync"
    error = None
    if sync_status == "drift":
        error = error_payload(
            "git drift detected",
            code=EC.GIT_DRIFT,
            details=list(sync.get("reasons") or []),
            exit_code=1,
        )
    elif sync_status == "unavailable":
        error = error_payload(
            "live git state is unavailable",
            code=EC.GIT_STATE_UNAVAILABLE,
            details=list(sync.get("reasons") or []),
            exit_code=1,
        )

    return {
        "ok": ok,
        "error": error,
        "state": {
            "file": str(rel),
            "status": status,
            "sync_status": sync_status,
        },
        "result": {
            "origin": origin_payload(data),
            "sync": sync,
        },
        "git_binding": git_binding,
    }


def cmd_new(args):
    """Create a new spec from template."""
    json_mode = bool(getattr(args, "json", False))
    root = find_root()
    auto_initialized = False
    if root is None:
        root = Path.cwd()
        for rel in (DRAFTS_DIR, APPROVED_DIR, ACTIVE_DIR, ARCHIVE_DIR, REVIEWS_DIR):
            (root / rel).mkdir(parents=True, exist_ok=True)
        sync_framework_bundle(root)
        auto_initialized = True
    snapshot = new_spec_snapshot(
        root,
        args.task_id,
        title=getattr(args, "title", None),
        size=getattr(args, "size", None),
        risk=getattr(args, "risk", None),
        auto_initialized=auto_initialized,
    )
    rel = Path(snapshot["state"]["file"])
    result = snapshot["result"]
    if json_mode:
        emit_command_json(
            "new",
            task_id=args.task_id,
            state=snapshot["state"],
            result=result,
        )
        return
    print(f"{c(C_GREEN, 'created')}: {rel}")
    print(f"  title: {result['title']}")
    print(f"   size: {result['size']}  risk: {result['risk']}")
    print()
    print(f"  Edit the spec, then optionally: {c(C_BOLD, f'scafld harden {args.task_id}')}")
    print(f"  When ready: {c(C_BOLD, f'scafld approve {args.task_id}')}")


def cmd_status(args):
    """Show status of a spec."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    snapshot = status_snapshot(root, spec, args.task_id)
    state = snapshot["state"]
    result = snapshot["result"]
    rel = spec.relative_to(root)

    if getattr(args, "json", False):
        emit_command_json("status", task_id=args.task_id, state=state, result=result)
        return

    status = state["status"]
    title = result["title"]
    size = state["size"]
    risk = state["risk"]
    updated = state["updated_at"]
    phase_totals = result["phase_counts"]
    origin = result["origin"]
    sync = result["sync"]
    next_action = result.get("next_action") or {}
    current_handoff = result.get("current_handoff") or {}
    block_reason = result.get("block_reason")
    color = STATUS_COLORS.get(status, "")
    print(f"{c(C_BOLD, title)}")
    print(f"     id: {args.task_id}")
    print(f"   file: {c(C_DIM, str(rel))}")
    print(f" status: {c(color, status)}")
    if size or risk:
        print(f"   size: {size}  risk: {risk}")
    total = phase_totals["total"]
    completed = phase_totals["completed"]
    failed = phase_totals["failed"]
    in_prog = phase_totals["in_progress"]
    if total > 0:
        phase_parts = []
        if completed:
            phase_parts.append(c(C_GREEN, f"{completed} done"))
        if in_prog:
            phase_parts.append(c(C_CYAN, f"{in_prog} active"))
        if failed:
            phase_parts.append(c(C_RED, f"{failed} failed"))
        pending = phase_totals["pending"]
        if pending > 0:
            phase_parts.append(c(C_DIM, f"{pending} pending"))
        print(f" phases: {' / '.join(phase_parts)}  ({total} total)")
    source_summary = summarize_origin_source(origin)
    source = origin.get("source") if isinstance(origin.get("source"), dict) else {}
    repo_binding = origin.get("repo") if isinstance(origin.get("repo"), dict) else {}
    git_binding = origin.get("git") if isinstance(origin.get("git"), dict) else {}
    if source_summary:
        print(f" source: {source_summary}")
    source_url = source.get("url") if isinstance(source.get("url"), str) else None
    if source_url and source_url != source_summary:
        print(f"    url: {c(C_DIM, source_url)}")
    if git_binding.get("branch"):
        base_display = git_binding.get("base_ref") or "unknown"
        print(f" branch: {git_binding['branch']}  base: {base_display}")
    if git_binding.get("upstream"):
        print(f"upstream: {git_binding['upstream']}")
    if git_binding.get("mode"):
        print(f"binding: {humanize_binding_mode(git_binding['mode'])}")
    if repo_binding.get("remote"):
        print(f" remote: {repo_binding['remote']}")
    if sync.get("status") != "unbound":
        sync_color = {
            "in_sync": C_GREEN,
            "drift": C_YELLOW,
            "unavailable": C_RED,
        }.get(sync.get("status"), C_DIM)
        sync_line = sync.get("status") or "unknown"
        reasons = list(sync.get("reasons") or [])
        if reasons:
            sync_line = f"{sync_line} - {reasons[0]}"
        print(f"   sync: {c(sync_color, sync_line)}")
    if updated:
        print(f"updated: {c(C_DIM, updated)}")
    if block_reason:
        print(f"  block: {c(C_YELLOW, block_reason)}")
    if current_handoff:
        handoff_file = current_handoff.get("handoff_file")
        role = current_handoff.get("role")
        gate = current_handoff.get("gate")
        label = f"{role} x {gate}" if role and gate else "handoff"
        if handoff_file:
            print(f"handoff: {c(C_DIM, handoff_file)}  ({label})")
    if next_action:
        command = next_action.get("command")
        message = next_action.get("message")
        followup = next_action.get("followup_command")
        if command:
            print(f"   next: {c(C_BOLD, command)}")
        elif message:
            print(f"   next: {message}")
        if followup:
            print(f"followup: {c(C_DIM, followup)}")


def cmd_list(args):
    """List all specs."""
    root = require_root()
    specs = filter_specs(find_all_specs(root), args.filter)

    if not specs:
        if args.filter:
            print(f"{c(C_DIM, 'No matching specs.')}")
        else:
            print(f"{c(C_DIM, 'No specs found.')}")
            print(f"  Create one: {c(C_BOLD, 'scafld plan my-feature -t \"My feature\" -s small -r low')}")
        return

    for label, group_specs in listing_groups(specs).items():
        print(f"{c(C_BOLD, label)}/")
        for spec_path in group_specs:
            text = spec_path.read_text()
            status = yaml_read_field(text, "status") or "unknown"
            title = yaml_read_nested(text, "task", "title") or spec_path.stem
            color = STATUS_COLORS.get(status, "")
            total, completed, _, _ = count_phases(text)
            phase_str = f" [{completed}/{total}]" if total > 0 else ""
            max_title = 50
            if len(title) > max_title:
                title = title[:max_title - 1] + "…"
            print(f"  {c(color, f'{status:14s}')} {spec_path.stem:30s} {c(C_DIM, title)}{phase_str}")
        print()


def cmd_approve(args):
    """Move spec from drafts to approved. Validates first."""
    root = require_root()
    snapshot = approve_spec_snapshot(root, args.task_id)
    move_result = snapshot["move_result"]
    if getattr(args, "json", False):
        emit_command_json(
            "approve",
            task_id=args.task_id,
            state=snapshot["state"],
            result=snapshot["result"],
        )
        return
    print_move_result(root, move_result)
    print(f"  session: .ai/runs/{args.task_id}/session.json")


def cmd_start(args):
    """Move spec from approved to active."""
    root = require_root()
    snapshot = start_spec_snapshot(root, args.task_id)
    move_result = snapshot["move_result"]
    result = snapshot["result"]
    if getattr(args, "json", False):
        emit_command_json(
            "start",
            task_id=args.task_id,
            state=snapshot["state"],
            result=result,
        )
        return
    print_move_result(root, move_result)
    print(f"  session: .ai/runs/{args.task_id}/session.json")
    print(f"  handoff: {result['handoff_file']}")


def cmd_branch(args):
    """Create or bind a task branch and record the origin metadata in the spec."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    snapshot = branch_binding_snapshot(
        root,
        spec,
        args.task_id,
        name=args.name,
        base=args.base,
        bind_current=args.bind_current,
    )
    state = snapshot["state"]
    result = snapshot["result"]
    binding = snapshot["binding"]
    rel = spec.relative_to(root)

    if getattr(args, "json", False):
        emit_command_json("branch", task_id=args.task_id, state=state, result=result)
        return

    print(f"{c(C_GREEN, 'bound')}: {rel}")
    print(f" action: {binding['action']}")
    print(f" branch: {binding['branch']}")
    if binding["base_ref"]:
        print(f"   base: {binding['base_ref']}")
    sync_status = binding["sync"].get("status") or "unknown"
    sync_color = C_GREEN if sync_status == "in_sync" else C_YELLOW
    print(f"   sync: {c(sync_color, sync_status)}")


def cmd_sync(args):
    """Compare a spec's recorded origin binding with the live git workspace."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    snapshot = sync_snapshot(root, spec, args.task_id)
    ok = snapshot["ok"]
    error = snapshot["error"]
    state = snapshot["state"]
    result = snapshot["result"]
    git_binding = snapshot["git_binding"]
    sync = result["sync"]
    sync_status = state["sync_status"]
    rel = spec.relative_to(root)

    if getattr(args, "json", False):
        emit_command_json("sync", ok=ok, task_id=args.task_id, state=state, result=result, error=error)
        if not ok:
            sys.exit(1)
        return

    sync_color = {
        "in_sync": C_GREEN,
        "drift": C_YELLOW,
        "unavailable": C_RED,
    }.get(sync_status, C_DIM)
    print(f"{c(C_BOLD, f'Sync: {args.task_id}')}")
    print(f"   file: {c(C_DIM, str(rel))}")
    print(f" branch: {git_binding.get('branch')}")
    if git_binding.get("base_ref"):
        print(f"   base: {git_binding['base_ref']}")
    print(f" status: {c(sync_color, sync_status or 'unknown')}")
    for reason in sync.get("reasons") or []:
        print(f"  - {reason}")
    if not ok:
        sys.exit(1)

def cmd_validate(args):
    """Validate a spec against the JSON schema."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    snapshot = validation_snapshot(root, spec)
    state = snapshot["state"]
    result = snapshot["result"]
    task_id = snapshot["task_id"]
    errors = snapshot["errors"]
    rel = spec.relative_to(root)

    if getattr(args, "json", False):
        emit_command_json(
            "validate",
            ok=not bool(errors),
            task_id=task_id,
            state=state,
            result=result,
            error=error_payload(
                "spec validation failed",
                code=EC.VALIDATION_FAILED,
                details=errors,
                exit_code=1,
            ) if errors else None,
        )
        if errors:
            sys.exit(1)
        return

    if errors:
        print(f"{c(C_RED, 'FAIL')}: {rel}")
        for error in errors:
            print(f"  - {error}")
        sys.exit(1)

    print(f"{c(C_GREEN, 'PASS')}: {rel}")
    print(f"  spec_version: {state['schema_version']}")
    print(f"  task_id: {task_id}")
    print(f"  status: {state['status']}")
    print(f"  phases: {result['phase_count']}")


def cmd_fail(args):
    """Move spec from active to archive (failed)."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    move_result = move_spec(root, spec, "failed")
    archive_month = move_result.dest.parent.name
    archived_run_dir = archive_run_artifacts(root, args.task_id, archive_month)
    if getattr(args, "json", False):
        emit_command_json(
            "fail",
            task_id=args.task_id,
            state={"status": move_result.new_status},
            result={
                "transition": move_result_payload(root, move_result),
                "run_archive_dir": str(archived_run_dir.relative_to(root)) if archived_run_dir else None,
            },
        )
        return
    print_move_result(root, move_result)
    if archived_run_dir:
        print(f"  run archive: {c(C_DIM, str(archived_run_dir.relative_to(root)))}")


def cmd_cancel(args):
    """Move spec to archive (cancelled)."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    move_result = move_spec(root, spec, "cancelled")
    archive_month = move_result.dest.parent.name
    archived_run_dir = archive_run_artifacts(root, args.task_id, archive_month)
    if getattr(args, "json", False):
        emit_command_json(
            "cancel",
            task_id=args.task_id,
            state={"status": move_result.new_status},
            result={
                "transition": move_result_payload(root, move_result),
                "run_archive_dir": str(archived_run_dir.relative_to(root)) if archived_run_dir else None,
            },
        )
        return
    print_move_result(root, move_result)
    if archived_run_dir:
        print(f"  run archive: {c(C_DIM, str(archived_run_dir.relative_to(root)))}")
