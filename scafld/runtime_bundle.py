import hashlib
import json
import os
import sysconfig
from pathlib import Path

from scafld._version import __version__ as VERSION
from scafld.config import load_config as load_workspace_config
from scafld.git_state import source_git_metadata


SCAFLD_DIR = ".scafld"
CORE_DIR = f"{SCAFLD_DIR}/core"
SPECS_DIR = f"{SCAFLD_DIR}/specs"
DRAFTS_DIR = f"{SPECS_DIR}/drafts"
APPROVED_DIR = f"{SPECS_DIR}/approved"
ACTIVE_DIR = f"{SPECS_DIR}/active"
ARCHIVE_DIR = f"{SPECS_DIR}/archive"
REVIEWS_DIR = f"{SCAFLD_DIR}/reviews"
RUNS_DIR = f"{SCAFLD_DIR}/runs"
CONFIG_PATH = f"{SCAFLD_DIR}/config.yaml"
CONFIG_LOCAL_PATH = f"{SCAFLD_DIR}/config.local.yaml"
CORE_CONFIG_PATH = f"{CORE_DIR}/config.yaml"
CORE_SCHEMA_PATH = f"{CORE_DIR}/schemas/spec.json"
CORE_REVIEW_PACKET_SCHEMA_PATH = f"{CORE_DIR}/schemas/review_packet.json"
CORE_MANIFEST_PATH = f"{CORE_DIR}/manifest.json"
CORE_MANIFEST_SCHEMA_VERSION = 1
CORE_BUNDLE_FILES = [
    (f"{SCAFLD_DIR}/core/config.yaml", CORE_CONFIG_PATH),
    (f"{SCAFLD_DIR}/core/schemas/spec.json", CORE_SCHEMA_PATH),
    (f"{SCAFLD_DIR}/core/schemas/review_packet.json", CORE_REVIEW_PACKET_SCHEMA_PATH),
    (f"{SCAFLD_DIR}/core/prompts/plan.md", f"{CORE_DIR}/prompts/plan.md"),
    (f"{SCAFLD_DIR}/core/prompts/harden.md", f"{CORE_DIR}/prompts/harden.md"),
    (f"{SCAFLD_DIR}/core/prompts/exec.md", f"{CORE_DIR}/prompts/exec.md"),
    (f"{SCAFLD_DIR}/core/prompts/review.md", f"{CORE_DIR}/prompts/review.md"),
    (f"{SCAFLD_DIR}/core/prompts/recovery.md", f"{CORE_DIR}/prompts/recovery.md"),
    (f"{SCAFLD_DIR}/core/README.md", f"{CORE_DIR}/README.md"),
    (f"{SCAFLD_DIR}/core/OPERATORS.md", f"{CORE_DIR}/OPERATORS.md"),
    (f"{SCAFLD_DIR}/specs/README.md", f"{CORE_DIR}/specs/README.md"),
    (f"{SCAFLD_DIR}/core/specs/examples/add-error-codes.md", f"{CORE_DIR}/specs/examples/add-error-codes.md"),
    (f"{SCAFLD_DIR}/core/specs/examples/markdown-2.0-skeleton.md", f"{CORE_DIR}/specs/examples/markdown-2.0-skeleton.md"),
    (f"{SCAFLD_DIR}/core/scripts/scafld-provider-adapter.sh", f"{CORE_DIR}/scripts/scafld-provider-adapter.sh"),
    (f"{SCAFLD_DIR}/core/scripts/scafld-codex-build.sh", f"{CORE_DIR}/scripts/scafld-codex-build.sh"),
    (f"{SCAFLD_DIR}/core/scripts/scafld-codex-review.sh", f"{CORE_DIR}/scripts/scafld-codex-review.sh"),
    (f"{SCAFLD_DIR}/core/scripts/scafld-claude-build.sh", f"{CORE_DIR}/scripts/scafld-claude-build.sh"),
    (f"{SCAFLD_DIR}/core/scripts/scafld-claude-review.sh", f"{CORE_DIR}/scripts/scafld-claude-review.sh"),
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


def resolve_schema_path(root):
    return root / CORE_SCHEMA_PATH


def resolve_review_packet_schema_path(root):
    return root / CORE_REVIEW_PACKET_SCHEMA_PATH


def resolve_prompt_path(root, name):
    """Resolve prompt templates from the workspace first, then the managed reset copy."""
    workspace_prompt = root / SCAFLD_DIR / "prompts" / name
    if workspace_prompt.exists():
        return workspace_prompt
    return root / CORE_DIR / "prompts" / name


def build_framework_manifest(workspace_root, source_root, managed_assets):
    """Build deterministic metadata that proves which scafld bundle a workspace uses."""
    git_metadata = source_git_metadata(source_root)
    return {
        "schema_version": CORE_MANIFEST_SCHEMA_VERSION,
        "scafld_version": VERSION,
        "source_commit": git_metadata["commit"],
        "source_dirty": git_metadata["dirty"],
        "workspace_config_mode": "project_overlay" if (workspace_root / CONFIG_PATH).exists() else "managed_only",
        "managed_assets": managed_assets,
    }


def sync_framework_bundle(workspace_root, source_root=None):
    """Install/update the framework-managed runtime bundle inside a workspace."""
    source_root = source_root or scafld_source_root()
    created = []
    updated = []
    unchanged = []
    managed_assets = {}

    for src_rel, dest_rel in CORE_BUNDLE_FILES:
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
    manifest_status = write_bytes_if_changed(workspace_root / CORE_MANIFEST_PATH, manifest_text)

    return {
        "workspace": workspace_root,
        "created": created,
        "updated": updated,
        "unchanged": unchanged,
        "manifest_status": manifest_status,
        "manifest_path": workspace_root / CORE_MANIFEST_PATH,
        "manifest": manifest_payload,
    }


def load_runtime_config(root):
    """Load workspace config with local overlay support."""
    return load_workspace_config(root, CORE_CONFIG_PATH, CONFIG_PATH, CONFIG_LOCAL_PATH)
