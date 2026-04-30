import shutil
import subprocess
from pathlib import Path

from scafld.command_runtime import find_root, find_workspaces
from scafld.config import detect_init_config, render_init_local_config
from scafld.errors import ScafldError
from scafld.git_state import source_git_metadata
from scafld.output import emit_command_json
from scafld.runtime_bundle import (
    ACTIVE_DIR,
    APPROVED_DIR,
    ARCHIVE_DIR,
    CORE_DIR,
    CORE_MANIFEST_PATH,
    CONFIG_LOCAL_PATH,
    DRAFTS_DIR,
    REVIEWS_DIR,
    RUNS_DIR,
    SCAFLD_DIR,
    SPECS_DIR,
    VERSION,
    scafld_source_root,
    sync_framework_bundle,
)
from scafld.terminal import C_BOLD, C_CYAN, C_DIM, C_GREEN, C_YELLOW, c


SCAFLD_GITIGNORE_MARKER = "# scafld"
SCAFLD_GITIGNORE_ENTRIES = (
    f"{RUNS_DIR}/",
    f"{REVIEWS_DIR}/",
    f"{CORE_DIR}/",
)


def ensure_scafld_gitignore(project_root):
    """Ensure scafld runtime paths are present in the workspace .gitignore.

    Returns (status, missing) where status is "created", "updated", or "skip"
    and missing is the list of entries that were appended.
    """
    gitignore = project_root / ".gitignore"
    if not gitignore.exists():
        body = SCAFLD_GITIGNORE_MARKER + "\n" + "\n".join(SCAFLD_GITIGNORE_ENTRIES) + "\n"
        gitignore.write_text(body)
        return "created", list(SCAFLD_GITIGNORE_ENTRIES)
    existing = gitignore.read_text()
    existing_entries = {line.strip() for line in existing.splitlines()}
    missing = [entry for entry in SCAFLD_GITIGNORE_ENTRIES if entry not in existing_entries]
    if not missing:
        return "skip", []
    appendix_parts = []
    if not existing.endswith("\n"):
        appendix_parts.append("\n")
    if SCAFLD_GITIGNORE_MARKER not in existing:
        appendix_parts.append(SCAFLD_GITIGNORE_MARKER + "\n")
    appendix_parts.append("\n".join(missing) + "\n")
    with open(gitignore, "a", encoding="utf-8") as handle:
        handle.write("".join(appendix_parts))
    return "updated", missing


def cmd_init(args):
    """Bootstrap scafld workspace in the current directory."""
    project_root = Path.cwd()
    json_mode = bool(getattr(args, "json", False))
    scafld_dir = scafld_source_root()
    result = {
        "project": str(project_root),
        "templates": [],
        "directories": [],
        "bundle": {},
        "framework_files": [],
        "examples": None,
        "config": {},
        "gitignore": {},
        "next_steps": [
            "Edit AGENTS.md and add project invariants",
            "Edit CONVENTIONS.md and add stack-specific rules",
            "Edit CLAUDE.md and add project overview and commands",
            "Edit .scafld/config.local.yaml and review build/test/lint commands",
            "Use the handoff-first wrapper scripts so the agent reads the current executor or challenger brief first",
            "Run scafld plan my-feature -t \"My feature\" -s small -r low",
        ],
    }

    if not json_mode:
        print(f"{c(C_BOLD, 'scafld init')}")
        print(f"  project: {project_root}")
        print()

    if not json_mode:
        print(f"{c(C_BOLD, 'Templates:')}")
    for fname in ["AGENTS.md", "CONVENTIONS.md", "CLAUDE.md"]:
        dest = project_root / fname
        src = scafld_dir / fname
        if dest.exists():
            action = "skip"
            note = "exists"
            if not json_mode:
                print(f"  {c(C_DIM, 'skip')}: {fname} (exists)")
        elif src.exists():
            shutil.copy2(str(src), str(dest))
            action = "created"
            note = None
            if not json_mode:
                print(f"  {c(C_GREEN, 'created')}: {fname}")
        else:
            action = "missing"
            note = "not in scafld source"
            if not json_mode:
                print(f"  {c(C_YELLOW, 'missing')}: {fname} (not in scafld source)")
        result["templates"].append({"path": fname, "action": action, "note": note})
    if not json_mode:
        print()

    if not json_mode:
        print(f"{c(C_BOLD, 'Directories:')}")
    for directory in [DRAFTS_DIR, APPROVED_DIR, ACTIVE_DIR, ARCHIVE_DIR, RUNS_DIR, f"{SCAFLD_DIR}/prompts", REVIEWS_DIR]:
        path = project_root / directory
        path.mkdir(parents=True, exist_ok=True)
        result["directories"].append({"path": f"{directory}/", "action": "ok"})
        if not json_mode:
            print(f"  {c(C_GREEN, 'ok')}: {directory}/")
    if not json_mode:
        print()

    if not json_mode:
        print(f"{c(C_BOLD, 'Framework files:')}")
    copy_files = [
        (f"{SCAFLD_DIR}/config.yaml", f"{SCAFLD_DIR}/config.yaml"),
        (f"{SCAFLD_DIR}/core/prompts/plan.md", f"{SCAFLD_DIR}/prompts/plan.md"),
        (f"{SCAFLD_DIR}/core/prompts/harden.md", f"{SCAFLD_DIR}/prompts/harden.md"),
        (f"{SCAFLD_DIR}/core/prompts/exec.md", f"{SCAFLD_DIR}/prompts/exec.md"),
        (f"{SCAFLD_DIR}/core/prompts/review.md", f"{SCAFLD_DIR}/prompts/review.md"),
        (f"{SCAFLD_DIR}/core/prompts/recovery.md", f"{SCAFLD_DIR}/prompts/recovery.md"),
        (f"{SCAFLD_DIR}/core/README.md", f"{SCAFLD_DIR}/core/README.md"),
        (f"{SCAFLD_DIR}/core/OPERATORS.md", f"{SCAFLD_DIR}/core/OPERATORS.md"),
        (f"{SPECS_DIR}/README.md", f"{SPECS_DIR}/README.md"),
        (f"{SCAFLD_DIR}/core/scripts/scafld-provider-adapter.sh", f"{SCAFLD_DIR}/core/scripts/scafld-provider-adapter.sh"),
        (f"{SCAFLD_DIR}/core/scripts/scafld-codex-build.sh", f"{SCAFLD_DIR}/core/scripts/scafld-codex-build.sh"),
        (f"{SCAFLD_DIR}/core/scripts/scafld-codex-review.sh", f"{SCAFLD_DIR}/core/scripts/scafld-codex-review.sh"),
        (f"{SCAFLD_DIR}/core/scripts/scafld-claude-build.sh", f"{SCAFLD_DIR}/core/scripts/scafld-claude-build.sh"),
        (f"{SCAFLD_DIR}/core/scripts/scafld-claude-review.sh", f"{SCAFLD_DIR}/core/scripts/scafld-claude-review.sh"),
    ]
    for src_rel, dest_rel in copy_files:
        src = scafld_dir / src_rel
        dest = project_root / dest_rel
        if not src.exists():
            continue
        if dest.exists():
            action = "skip"
            note = "exists"
            if not json_mode:
                print(f"  {c(C_DIM, 'skip')}: {dest_rel} (exists)")
        else:
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(str(src), str(dest))
            action = "created"
            note = None
            if not json_mode:
                print(f"  {c(C_GREEN, 'created')}: {dest_rel}")
        result["framework_files"].append({"path": dest_rel, "action": action, "note": note})
    if not json_mode:
        print()

    if not json_mode:
        print(f"{c(C_BOLD, 'Managed bundle:')}")
    bundle = sync_framework_bundle(project_root, scafld_dir)
    result["bundle"] = {
        "created": list(bundle["created"]),
        "updated": list(bundle["updated"]),
        "unchanged": list(bundle["unchanged"]),
        "manifest_status": bundle["manifest_status"],
        "manifest_path": str(bundle["manifest_path"].relative_to(project_root)),
        "manifest": bundle["manifest"],
    }
    if bundle["created"]:
        bundle_label = c(C_GREEN, "created")
    elif bundle["updated"]:
        bundle_label = c(C_YELLOW, "updated")
    else:
        bundle_label = c(C_DIM, "skip")
    manifest_color = C_GREEN if bundle["manifest_status"] == "created" else C_YELLOW if bundle["manifest_status"] == "updated" else C_DIM
    if not json_mode:
        print(f"  {bundle_label}: {CORE_DIR}/ ({len(bundle['created'])} files created, {len(bundle['updated'])} updated)")
        print(f"  {c(manifest_color, bundle['manifest_status'])}: {CORE_MANIFEST_PATH}")
        print()

    examples_src = scafld_dir / SPECS_DIR / "examples"
    examples_dest = project_root / SPECS_DIR / "examples"
    if examples_src.is_dir() and not examples_dest.exists():
        shutil.copytree(str(examples_src), str(examples_dest))
        result["examples"] = {"path": f"{SPECS_DIR}/examples/", "action": "created"}
        if not json_mode:
            print(f"  {c(C_GREEN, 'created')}: {SPECS_DIR}/examples/")
    else:
        result["examples"] = {
            "path": f"{SPECS_DIR}/examples/",
            "action": "skip" if examples_dest.exists() else "missing",
        }
    if not json_mode:
        print()

    if not json_mode:
        print(f"{c(C_BOLD, 'Config:')}")
    init_detection = detect_init_config(project_root)
    local_config = project_root / CONFIG_LOCAL_PATH
    if local_config.exists():
        result["config"] = {
            "path": CONFIG_LOCAL_PATH,
            "action": "skip",
            "summary": init_detection["summary"],
        }
        if not json_mode:
            print(f"  {c(C_DIM, 'skip')}: {CONFIG_LOCAL_PATH} (exists)")
    else:
        local_config.write_text(render_init_local_config(init_detection))
        result["config"] = {
            "path": CONFIG_LOCAL_PATH,
            "action": "created",
            "summary": init_detection["summary"],
        }
        if not json_mode:
            print(f"  {c(C_CYAN, 'detected')}: {init_detection['summary']}")
            print(f"  {c(C_GREEN, 'created')}: {CONFIG_LOCAL_PATH}")
    if not json_mode:
        print()

    if not json_mode:
        print(f"{c(C_BOLD, 'Gitignore:')}")
    status, missing = ensure_scafld_gitignore(project_root)
    result["gitignore"] = {"path": ".gitignore", "action": status, "added": missing}
    if not json_mode:
        if status == "created":
            print(f"  {c(C_GREEN, 'created')}: .gitignore")
        elif status == "updated":
            print(f"  {c(C_GREEN, 'updated')}: .gitignore (added {', '.join(missing)})")
        else:
            print(f"  {c(C_DIM, 'skip')}: .gitignore (entries up to date)")
    if not json_mode:
        print()

    if json_mode:
        emit_command_json("init", state={"workspace": str(project_root)}, result=result)
        return

    plan_command = 'scafld plan my-feature -t "My feature" -s small -r low'
    agent_hint = "\"Let's plan [feature].\""
    print(f"""{c(C_BOLD, 'Done!')} Next steps:

  1. Edit {c(C_CYAN, 'AGENTS.md')} - add your project's invariants
  2. Edit {c(C_CYAN, 'CONVENTIONS.md')} - add your tech stack and patterns
  3. Edit {c(C_CYAN, 'CLAUDE.md')} - add project overview and commands
  4. Edit {c(C_CYAN, '.scafld/config.local.yaml')} - set your build/test/lint commands

  Then: {c(C_BOLD, plan_command)} or tell your agent {c(C_BOLD, agent_hint)}
  Handoff-first wrappers:
    {c(C_BOLD, '.scafld/core/scripts/scafld-codex-build.sh <task-id>')} / {c(C_BOLD, '.scafld/core/scripts/scafld-claude-build.sh <task-id>')}
    {c(C_BOLD, '.scafld/core/scripts/scafld-codex-review.sh <task-id>')} / {c(C_BOLD, '.scafld/core/scripts/scafld-claude-review.sh <task-id>')}
""")


def cmd_update(args):
    """Refresh the managed scafld bundle in one workspace or across a tree."""
    source_root = scafld_source_root()
    git_metadata = source_git_metadata(source_root)

    if args.self:
        if not (source_root / ".git").exists():
            raise ScafldError("self-update requires scafld to run from a git checkout")

        print(f"{c(C_BOLD, 'Self update:')}")
        print(f"  source: {source_root}")
        result = subprocess.run(
            ["git", "-C", str(source_root), "pull", "--ff-only"],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            output = (result.stdout + result.stderr).strip()
            details = ["git pull --ff-only failed"]
            if output:
                details.extend(output.splitlines()[-5:])
            raise ScafldError("failed to refresh source checkout", details)
        print(f"  {c(C_GREEN, 'ok')}: source checkout refreshed")
        git_metadata = source_git_metadata(source_root)
        print()

    if args.scan_root:
        workspaces = find_workspaces(args.scan_root)
        mode_label = f"scan_root={Path(args.scan_root).expanduser().resolve()}"
    else:
        root = find_root()
        if root is None:
            if args.self:
                return
            raise ScafldError(
                "not in a scafld project (no .scafld/ directory found)",
                ["run 'scafld init' first or pass --scan-root /path"],
            )
        workspaces = [root]
        mode_label = f"workspace={root}"

    if not workspaces:
        print(f"{c(C_YELLOW, 'warn')}: no scafld workspaces found under {args.scan_root}")
        return

    print(f"{c(C_BOLD, 'scafld update')}")
    print(f"  source: {source_root}")
    print(f"  version: {VERSION}")
    if git_metadata["commit"]:
        dirty_suffix = " dirty" if git_metadata["dirty"] else ""
        print(f"  commit: {git_metadata['commit'][:12]}{dirty_suffix}")
    print(f"  target: {mode_label}")
    print()

    total_created = 0
    total_updated = 0
    total_unchanged = 0

    for workspace in workspaces:
        summary = sync_framework_bundle(workspace, source_root)
        total_created += len(summary["created"])
        total_updated += len(summary["updated"])
        total_unchanged += len(summary["unchanged"])

        gitignore_status, gitignore_missing = ensure_scafld_gitignore(workspace)
        gitignore_color = C_DIM if gitignore_status == "skip" else C_GREEN
        gitignore_label = "up to date" if gitignore_status == "skip" else gitignore_status
        gitignore_suffix = f" (added {', '.join(gitignore_missing)})" if gitignore_missing else ""

        print(f"{c(C_BOLD, str(workspace))}")
        print(f"  bundle: {len(summary['created'])} created, {len(summary['updated'])} updated, {len(summary['unchanged'])} unchanged")
        print(f"  manifest: {summary['manifest_status']} {summary['manifest_path'].relative_to(workspace)}")
        print(f"  gitignore: {c(gitignore_color, gitignore_label)}{gitignore_suffix}")
        if args.verbose:
            for rel in summary["created"]:
                print(f"    {c(C_GREEN, 'created')} {rel}")
            for rel in summary["updated"]:
                print(f"    {c(C_YELLOW, 'updated')} {rel}")
        print()

    print(f"{c(C_BOLD, 'Summary:')}")
    print(f"  workspaces: {len(workspaces)}")
    print(f"  bundle files: {total_created} created, {total_updated} updated, {total_unchanged} unchanged")


def cmd_version(args):
    print(f"scafld {VERSION}")
