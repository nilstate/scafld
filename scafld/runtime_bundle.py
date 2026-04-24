import hashlib
import json
import os
import sysconfig
from pathlib import Path

from scafld._version import __version__ as VERSION
from scafld.config import load_config as load_workspace_config
from scafld.git_state import source_git_metadata


AI_DIR = ".ai"
FRAMEWORK_DIR = f"{AI_DIR}/scafld"
SPECS_DIR = f"{AI_DIR}/specs"
DRAFTS_DIR = f"{SPECS_DIR}/drafts"
APPROVED_DIR = f"{SPECS_DIR}/approved"
ACTIVE_DIR = f"{SPECS_DIR}/active"
ARCHIVE_DIR = f"{SPECS_DIR}/archive"
REVIEWS_DIR = f"{AI_DIR}/reviews"
RUNS_DIR = f"{AI_DIR}/runs"
SCHEMA_PATH = f"{AI_DIR}/schemas/spec.json"
CONFIG_PATH = f"{AI_DIR}/config.yaml"
CONFIG_LOCAL_PATH = f"{AI_DIR}/config.local.yaml"
FRAMEWORK_CONFIG_PATH = f"{FRAMEWORK_DIR}/config.yaml"
FRAMEWORK_SCHEMA_PATH = f"{FRAMEWORK_DIR}/schemas/spec.json"
FRAMEWORK_MANIFEST_PATH = f"{FRAMEWORK_DIR}/manifest.json"
FRAMEWORK_MANIFEST_SCHEMA_VERSION = 1
FRAMEWORK_BUNDLE_FILES = [
    (f"{AI_DIR}/config.yaml", FRAMEWORK_CONFIG_PATH),
    (f"{AI_DIR}/schemas/spec.json", FRAMEWORK_SCHEMA_PATH),
    (f"{AI_DIR}/prompts/plan.md", f"{FRAMEWORK_DIR}/prompts/plan.md"),
    (f"{AI_DIR}/prompts/harden.md", f"{FRAMEWORK_DIR}/prompts/harden.md"),
    (f"{AI_DIR}/prompts/exec.md", f"{FRAMEWORK_DIR}/prompts/exec.md"),
    (f"{AI_DIR}/prompts/review.md", f"{FRAMEWORK_DIR}/prompts/review.md"),
    (f"{AI_DIR}/prompts/recovery.md", f"{FRAMEWORK_DIR}/prompts/recovery.md"),
    (f"{AI_DIR}/README.md", f"{FRAMEWORK_DIR}/README.md"),
    (f"{AI_DIR}/OPERATORS.md", f"{FRAMEWORK_DIR}/OPERATORS.md"),
    (f"{SPECS_DIR}/README.md", f"{FRAMEWORK_DIR}/specs/README.md"),
    (f"{SPECS_DIR}/examples/add-error-codes.yaml", f"{FRAMEWORK_DIR}/specs/examples/add-error-codes.yaml"),
    ("scripts/scafld-provider-adapter.sh", f"{FRAMEWORK_DIR}/scripts/scafld-provider-adapter.sh"),
    ("scripts/scafld-codex-build.sh", f"{FRAMEWORK_DIR}/scripts/scafld-codex-build.sh"),
    ("scripts/scafld-codex-review.sh", f"{FRAMEWORK_DIR}/scripts/scafld-codex-review.sh"),
    ("scripts/scafld-claude-build.sh", f"{FRAMEWORK_DIR}/scripts/scafld-claude-build.sh"),
    ("scripts/scafld-claude-review.sh", f"{FRAMEWORK_DIR}/scripts/scafld-claude-review.sh"),
]


def scafld_source_root():
    """Directory that contains the canonical scafld source tree."""
    override = os.environ.get("SCAFLD_SOURCE_ROOT")
    if override:
        return Path(override).expanduser().resolve()

    candidates = [
        Path(__file__).resolve().parents[1],
        Path(sysconfig.get_path("data")) / "share" / "scafld",
    ]
    for candidate in candidates:
        if (candidate / "cli" / "scafld").exists():
            return candidate
    return candidates[0]


def sha256_bytes(payload):
    return hashlib.sha256(payload).hexdigest()


def write_bytes_if_changed(path, payload):
    """Write bytes only when the content changes. Returns created/updated/unchanged."""
    if path.exists():
        if path.read_bytes() == payload:
            return "unchanged"
        path.write_bytes(payload)
        return "updated"

    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(payload)
    return "created"


def resolve_framework_asset(root, legacy_rel, managed_rel):
    """Prefer the managed framework bundle when it exists, else fall back to legacy paths."""
    managed = root / FRAMEWORK_DIR / managed_rel
    if managed.exists():
        return managed
    return root / legacy_rel


def resolve_schema_path(root):
    return resolve_framework_asset(root, SCHEMA_PATH, "schemas/spec.json")


def resolve_prompt_path(root, name):
    """Resolve prompt templates from the workspace first, then the managed reset copy."""
    workspace_prompt = root / AI_DIR / "prompts" / name
    if workspace_prompt.exists():
        return workspace_prompt
    return root / FRAMEWORK_DIR / "prompts" / name


def build_framework_manifest(workspace_root, source_root, managed_assets):
    """Build deterministic metadata that proves which scafld bundle a workspace uses."""
    git_metadata = source_git_metadata(source_root)
    return {
        "schema_version": FRAMEWORK_MANIFEST_SCHEMA_VERSION,
        "scafld_version": VERSION,
        "source_commit": git_metadata["commit"],
        "source_dirty": git_metadata["dirty"],
        "workspace_config_mode": "legacy_overlay" if (workspace_root / CONFIG_PATH).exists() else "managed_only",
        "managed_assets": managed_assets,
    }


def sync_framework_bundle(workspace_root, source_root=None):
    """Install/update the framework-managed runtime bundle inside a workspace."""
    source_root = source_root or scafld_source_root()
    created = []
    updated = []
    unchanged = []
    managed_assets = {}

    for src_rel, dest_rel in FRAMEWORK_BUNDLE_FILES:
        src = source_root / src_rel
        if not src.exists():
            continue

        payload = src.read_bytes()
        managed_assets[dest_rel] = {
            "source": src_rel,
            "sha256": sha256_bytes(payload),
        }

        dest = workspace_root / dest_rel
        status = write_bytes_if_changed(dest, payload)
        if status == "created":
            created.append(dest_rel)
        elif status == "updated":
            updated.append(dest_rel)
        else:
            unchanged.append(dest_rel)

    manifest_payload = build_framework_manifest(workspace_root, source_root, managed_assets)
    manifest_text = (json.dumps(manifest_payload, indent=2, sort_keys=True) + "\n").encode()
    manifest_status = write_bytes_if_changed(workspace_root / FRAMEWORK_MANIFEST_PATH, manifest_text)

    return {
        "workspace": workspace_root,
        "created": created,
        "updated": updated,
        "unchanged": unchanged,
        "manifest_status": manifest_status,
        "manifest_path": workspace_root / FRAMEWORK_MANIFEST_PATH,
        "manifest": manifest_payload,
    }


def load_runtime_config(root):
    """Load workspace config with local overlay support."""
    return load_workspace_config(root, FRAMEWORK_CONFIG_PATH, CONFIG_PATH, CONFIG_LOCAL_PATH)
