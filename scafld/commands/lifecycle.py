import json
import re
import sys
from pathlib import Path

from scafld.audit_scope import git_sync_excluded_paths
from scafld.command_runtime import find_root, require_root
from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError
from scafld.git_state import bind_task_branch, build_origin_sync_payload, refresh_origin_sync
from scafld.output import emit_command_json, error_payload
from scafld.projections import humanize_binding_mode, origin_payload, phase_counts, summarize_origin_source
from scafld.review_artifacts import load_review_topology
from scafld.reviewing import load_review_state
from scafld.runtime_bundle import (
    ACTIVE_DIR,
    APPROVED_DIR,
    ARCHIVE_DIR,
    CONFIG_LOCAL_PATH,
    CONFIG_PATH,
    DRAFTS_DIR,
    FRAMEWORK_CONFIG_PATH,
    LOGS_DIR,
    REVIEWS_DIR,
    resolve_schema_path,
    sync_framework_bundle,
)
from scafld.spec_lifecycle import move_result_payload, validate_spec
from scafld.spec_parsing import count_phases, now_iso, parse_phase_status_entries
from scafld.spec_store import (
    find_all_specs,
    find_spec,
    load_spec_document,
    move_spec,
    require_spec,
    write_spec_document,
    yaml_read_field,
    yaml_read_nested,
)
from scafld.spec_templates import build_new_spec_scaffold
from scafld.terminal import C_BOLD, C_CYAN, C_DIM, C_GREEN, C_RED, C_YELLOW, STATUS_COLORS, c


def print_move_result(root, move_result):
    transition = move_result_payload(root, move_result)
    print(f"{c(C_GREEN, '  moved')}: {transition['from']} -> {transition['to']}")
    print(f" {c(C_DIM, 'status')}: {c(STATUS_COLORS.get(transition['status'], ''), transition['status'])}")


def cmd_new(args):
    """Create a new spec from template."""
    json_mode = bool(getattr(args, "json", False))
    root = find_root()
    auto_initialized = False
    if root is None:
        root = Path.cwd()
        for rel in (DRAFTS_DIR, APPROVED_DIR, ACTIVE_DIR, ARCHIVE_DIR, LOGS_DIR, REVIEWS_DIR):
            (root / rel).mkdir(parents=True, exist_ok=True)
        sync_framework_bundle(root)
        auto_initialized = True
    task_id = args.task_id

    if not re.match(r'^[a-z0-9][a-z0-9-]*[a-z0-9]$', task_id) and not re.match(r'^[a-z0-9]$', task_id):
        raise ScafldError("task-id must be kebab-case (a-z, 0-9, hyphens)", code=EC.INVALID_TASK_ID)

    existing = find_spec(root, task_id)
    if existing:
        rel = existing.relative_to(root)
        raise ScafldError(f"spec already exists: {rel}", code=EC.SPEC_EXISTS)

    dest_dir = root / DRAFTS_DIR
    dest_dir.mkdir(parents=True, exist_ok=True)
    dest = dest_dir / f"{task_id}.yaml"

    ts = now_iso()
    scaffold = build_new_spec_scaffold(
        root,
        task_id,
        timestamp=ts,
        title=args.title,
        size=args.size,
        risk=args.risk,
        framework_config_path=FRAMEWORK_CONFIG_PATH,
        config_path=CONFIG_PATH,
        config_local_path=CONFIG_LOCAL_PATH,
    )
    title = scaffold["title"]
    size = scaffold["size"]
    risk = scaffold["risk"]
    repo_context = scaffold["repo_context"]
    dest.write_text(scaffold["text"])
    rel = dest.relative_to(root)
    if json_mode:
        emit_command_json(
            "new",
            task_id=task_id,
            state={"status": "draft", "file": str(rel)},
            result={
                "title": title,
                "size": size,
                "risk": risk,
                "auto_initialized": auto_initialized,
                "repo_context": repo_context,
                "next_commands": [
                    f"scafld harden {task_id}",
                    f"scafld approve {task_id}",
                ],
            },
        )
        return
    print(f"{c(C_GREEN, 'created')}: {rel}")
    print(f"  title: {args.title or task_id.replace('-', ' ').title()}")
    print(f"   size: {size}  risk: {risk}")
    print()
    print(f"  Edit the spec, then optionally: {c(C_BOLD, f'scafld harden {task_id}')}")
    print(f"  When ready: {c(C_BOLD, f'scafld approve {task_id}')}")


def cmd_status(args):
    """Show status of a spec."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    text = spec.read_text()
    data = load_spec_document(spec)
    rel = spec.relative_to(root)

    status = yaml_read_field(text, "status") or "unknown"
    title = yaml_read_nested(text, "task", "title") or ""
    size = yaml_read_nested(text, "task", "size") or ""
    risk = yaml_read_nested(text, "task", "risk_level") or ""
    updated = yaml_read_field(text, "updated") or ""
    phase_totals = phase_counts(*count_phases(text))

    state = {
        "file": str(rel),
        "status": status,
        "size": size,
        "risk": risk,
        "updated_at": updated,
    }
    result = {
        "title": title,
        "phase_statuses": parse_phase_status_entries(text),
        "phase_counts": phase_totals,
    }
    origin = origin_payload(data)
    sync = build_origin_sync_payload(root, origin, excluded_rels=git_sync_excluded_paths())
    result["origin"] = origin
    result["sync"] = sync

    if getattr(args, "json", False):
        try:
            topology = load_review_topology(root)
        except Exception as exc:
            review_state = {"exists": False, "errors": [str(exc)]}
        else:
            review_state = load_review_state(root / REVIEWS_DIR / f"{args.task_id}.md", topology)
        result["review_state"] = review_state
        emit_command_json("status", task_id=args.task_id, state=state, result=result)
        return

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


def cmd_list(args):
    """List all specs."""
    root = require_root()
    specs = find_all_specs(root)

    if not specs:
        print(f"{c(C_DIM, 'No specs found.')}")
        print(f"  Create one: {c(C_BOLD, 'scafld new my-feature')}")
        return

    if args.filter:
        flt = args.filter.lower()
        if flt in ("draft", "drafts"):
            specs = [(s, l) for s, l in specs if l == "drafts"]
        elif flt in ("approved",):
            specs = [(s, l) for s, l in specs if l == "approved"]
        elif flt in ("active", "in_progress"):
            specs = [(s, l) for s, l in specs if l == "active"]
        elif flt in ("archive", "archived", "completed", "done"):
            specs = [(s, l) for s, l in specs if l.startswith("archive")]
        else:
            specs = [(s, l) for s, l in specs if flt in s.stem.lower()]

    if not specs:
        print(f"{c(C_DIM, 'No matching specs.')}")
        return

    groups = {}
    for spec_path, label in specs:
        groups.setdefault(label, []).append(spec_path)

    for label in groups:
        print(f"{c(C_BOLD, label)}/")
        for spec_path in groups[label]:
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
    spec = require_spec(root, args.task_id)

    errors = validate_spec(root, spec)
    if errors:
        raise ScafldError(
            "spec has validation errors, cannot approve",
            [f"- {error}" for error in errors] + [f"fix the spec, then retry: scafld approve {args.task_id}"],
            code=EC.VALIDATION_FAILED,
            next_action=f"scafld validate {args.task_id}",
        )

    move_result = move_spec(root, spec, "approved")
    if getattr(args, "json", False):
        emit_command_json(
            "approve",
            task_id=args.task_id,
            state={"status": move_result.new_status},
            result={"transition": move_result_payload(root, move_result)},
        )
        return
    print_move_result(root, move_result)


def cmd_start(args):
    """Move spec from approved to active."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    move_result = move_spec(root, spec, "in_progress")
    if getattr(args, "json", False):
        emit_command_json(
            "start",
            task_id=args.task_id,
            state={"status": move_result.new_status},
            result={"transition": move_result_payload(root, move_result)},
        )
        return
    print_move_result(root, move_result)


def cmd_branch(args):
    """Create or bind a task branch and record the origin metadata in the spec."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    rel = spec.relative_to(root)
    data = load_spec_document(spec)
    status = data.get("status")

    if status in ("completed", "failed", "cancelled"):
        raise ScafldError(
            f"cannot bind a branch for a {status} spec",
            [f"spec is archived at {rel}"],
            code=EC.INVALID_SPEC_STATUS,
        )

    existing_origin = origin_payload(data)
    timestamp = now_iso()
    binding = bind_task_branch(
        root,
        args.task_id,
        existing_origin,
        name=args.name,
        base=args.base,
        bind_current=args.bind_current,
        bound_at=timestamp,
        excluded_rels=git_sync_excluded_paths(),
    )
    data["origin"] = binding["origin"]
    data["updated"] = timestamp
    write_spec_document(spec, data)

    result = {
        "action": binding["action"],
        "origin": origin_payload(data),
        "sync": binding["sync"],
    }
    state = {
        "file": str(rel),
        "status": status,
        "branch": binding["branch"],
    }

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
    rel = spec.relative_to(root)
    data = load_spec_document(spec)
    status = data.get("status")
    origin = origin_payload(data)
    git_binding = origin.get("git") if isinstance(origin.get("git"), dict) else {}

    if not git_binding.get("branch"):
        raise ScafldError(
            "spec has no bound branch or origin metadata yet",
            [f"bind one first: scafld branch {args.task_id}"],
            code=EC.ORIGIN_UNBOUND,
            next_action=f"scafld branch {args.task_id}",
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
    state = {
        "file": str(rel),
        "status": status,
        "sync_status": sync_status,
    }
    result = {
        "origin": origin_payload(data),
        "sync": sync,
    }

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
    rel = spec.relative_to(root)
    text = spec.read_text()

    errors = validate_spec(root, spec)
    spec_version = yaml_read_field(text, "spec_version")
    task_id = yaml_read_field(text, "task_id")
    status = yaml_read_field(text, "status")
    total, completed, failed, in_prog = count_phases(text)

    state = {
        "file": str(rel),
        "status": status,
        "schema_version": spec_version,
    }
    result = {
        "valid": not bool(errors),
        "phase_count": total,
        "phase_counts": {
            "total": total,
            "completed": completed,
            "failed": failed,
            "in_progress": in_prog,
            "pending": total - completed - failed - in_prog,
        },
        "errors": errors,
    }

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
    print(f"  spec_version: {spec_version}")
    print(f"  task_id: {task_id}")
    print(f"  status: {status}")
    print(f"  phases: {total}")


def cmd_fail(args):
    """Move spec from active to archive (failed)."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    move_result = move_spec(root, spec, "failed")
    if getattr(args, "json", False):
        emit_command_json(
            "fail",
            task_id=args.task_id,
            state={"status": move_result.new_status},
            result={"transition": move_result_payload(root, move_result)},
        )
        return
    print_move_result(root, move_result)


def cmd_cancel(args):
    """Move spec to archive (cancelled)."""
    root = require_root()
    spec = require_spec(root, args.task_id)
    move_result = move_spec(root, spec, "cancelled")
    if getattr(args, "json", False):
        emit_command_json(
            "cancel",
            task_id=args.task_id,
            state={"status": move_result.new_status},
            result={"transition": move_result_payload(root, move_result)},
        )
        return
    print_move_result(root, move_result)
