import shutil
import subprocess
from pathlib import Path

from scafld.command_runtime import find_root, find_workspaces
from scafld.config import detect_init_config, render_init_local_config
from scafld.errors import ScafldError
from scafld.git_state import source_git_metadata
from scafld.output import emit_command_json

from .shared import (
    AI_DIR,
    ACTIVE_DIR,
    APPROVED_DIR,
    ARCHIVE_DIR,
    CONFIG_LOCAL_PATH,
    DRAFTS_DIR,
    FRAMEWORK_DIR,
    FRAMEWORK_MANIFEST_PATH,
    LOGS_DIR,
    SPECS_DIR,
    VERSION,
    c,
    scafld_source_root,
    sync_framework_bundle,
    C_BOLD,
    C_CYAN,
    C_DIM,
    C_GREEN,
    C_YELLOW,
)


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
            "Edit .ai/config.local.yaml and review build/test/lint commands",
            "Run scafld new my-feature",
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
    for directory in [DRAFTS_DIR, APPROVED_DIR, ACTIVE_DIR, ARCHIVE_DIR, LOGS_DIR, f"{AI_DIR}/schemas", f"{AI_DIR}/prompts", f"{AI_DIR}/reviews"]:
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
        (f"{AI_DIR}/config.yaml", f"{AI_DIR}/config.yaml"),
        (f"{AI_DIR}/schemas/spec.json", f"{AI_DIR}/schemas/spec.json"),
        (f"{AI_DIR}/prompts/plan.md", f"{AI_DIR}/prompts/plan.md"),
        (f"{AI_DIR}/prompts/harden.md", f"{AI_DIR}/prompts/harden.md"),
        (f"{AI_DIR}/prompts/exec.md", f"{AI_DIR}/prompts/exec.md"),
        (f"{AI_DIR}/prompts/review.md", f"{AI_DIR}/prompts/review.md"),
        (f"{AI_DIR}/README.md", f"{AI_DIR}/README.md"),
        (f"{AI_DIR}/OPERATORS.md", f"{AI_DIR}/OPERATORS.md"),
        (f"{SPECS_DIR}/README.md", f"{SPECS_DIR}/README.md"),
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
        print(f"  {bundle_label}: {FRAMEWORK_DIR}/ ({len(bundle['created'])} files created, {len(bundle['updated'])} updated)")
        print(f"  {c(manifest_color, bundle['manifest_status'])}: {FRAMEWORK_MANIFEST_PATH}")
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
    gitignore = project_root / ".gitignore"
    scafld_entries = [
        "# scafld",
        ".ai/logs/",
        ".ai/reviews/",
    ]
    if gitignore.exists():
        existing = gitignore.read_text()
        if "# scafld" in existing:
            result["gitignore"] = {"path": ".gitignore", "action": "skip", "note": "scafld entries exist"}
            if not json_mode:
                print(f"  {c(C_DIM, 'skip')}: .gitignore (scafld entries exist)")
        else:
            with open(gitignore, "a", encoding="utf-8") as handle:
                handle.write("\n" + "\n".join(scafld_entries) + "\n")
            result["gitignore"] = {"path": ".gitignore", "action": "updated", "note": "added scafld entries"}
            if not json_mode:
                print(f"  {c(C_GREEN, 'updated')}: .gitignore (added scafld entries)")
    else:
        gitignore.write_text("\n".join(scafld_entries) + "\n")
        result["gitignore"] = {"path": ".gitignore", "action": "created", "note": None}
        if not json_mode:
            print(f"  {c(C_GREEN, 'created')}: .gitignore")
    if not json_mode:
        print()

    if json_mode:
        emit_command_json("init", state={"workspace": str(project_root)}, result=result)
        return

    agent_hint = "\"Let's plan [feature].\""
    print(f"""{c(C_BOLD, 'Done!')} Next steps:

  1. Edit {c(C_CYAN, 'AGENTS.md')} - add your project's invariants
  2. Edit {c(C_CYAN, 'CONVENTIONS.md')} - add your tech stack and patterns
  3. Edit {c(C_CYAN, 'CLAUDE.md')} - add project overview and commands
  4. Edit {c(C_CYAN, '.ai/config.local.yaml')} - set your build/test/lint commands

  Then: {c(C_BOLD, 'scafld new my-feature')} or tell your agent {c(C_BOLD, agent_hint)}
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
                "not in a scafld project (no .ai/ directory found)",
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

        print(f"{c(C_BOLD, str(workspace))}")
        print(f"  bundle: {len(summary['created'])} created, {len(summary['updated'])} updated, {len(summary['unchanged'])} unchanged")
        print(f"  manifest: {summary['manifest_status']} {summary['manifest_path'].relative_to(workspace)}")
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
