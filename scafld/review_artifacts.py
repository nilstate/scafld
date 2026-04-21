import json
import re

from scafld.reviewing import build_review_topology, render_review_pass_results, review_passes_by_kind
from scafld.runtime_bundle import load_runtime_config
from scafld.spec_parsing import now_iso
from scafld.spec_store import yaml_read_nested

from .audit_scope import collect_changed_files


def load_review_topology(root):
    """Load the configured review topology for built-in review passes."""
    config = load_runtime_config(root)
    review_config = config.get("review")
    if not isinstance(review_config, dict):
        raise ValueError("config.review must be a mapping")
    return build_review_topology(review_config)


def ensure_review_file_header(review_file, task_id, spec_text):
    """Create the shared review file header if it does not exist yet."""
    if review_file.exists():
        return

    task_title = yaml_read_nested(spec_text, "task", "title") or task_id
    task_summary = yaml_read_nested(spec_text, "task", "summary") or ""
    changed_files = collect_changed_files(spec_text)
    files_section = "\n".join(f"- {path}" for path in changed_files) if changed_files else "- (see git diff)"

    review_file.parent.mkdir(parents=True, exist_ok=True)
    review_file.write_text(f"""# Review: {task_id}

## Spec
{task_title}
{task_summary}

## Files Changed
{files_section}
""")


def append_review_round(
    review_file,
    task_id,
    spec_text,
    topology,
    metadata,
    verdict="",
    blocking=None,
    non_blocking=None,
    section_bodies=None,
):
    """Append a review round using Review Artifact v3."""
    ensure_review_file_header(review_file, task_id, spec_text)

    existing_text = review_file.read_text()
    review_count = len(re.findall(r"^## Review \d+\s+—", existing_text, re.MULTILINE)) + 1
    metadata_json = json.dumps(metadata, indent=2)
    blocking_body = "\n".join(blocking or [])
    non_blocking_body = "\n".join(non_blocking or [])
    verdict_body = verdict or ""
    section_bodies = section_bodies or {}
    adversarial_sections = "\n\n".join(
        f"### {definition['title']}\n{section_bodies.get(definition['id'], '')}"
        for definition in review_passes_by_kind(topology, "adversarial")
    )

    round_text = f"""## Review {review_count} — {now_iso()}

### Metadata
```json
{metadata_json}
```

### Pass Results
{render_review_pass_results(topology, metadata.get("pass_results"))}

{adversarial_sections}

### Blocking
{blocking_body}

### Non-blocking
{non_blocking_body}

### Verdict
{verdict_body}
"""

    if existing_text.strip():
        review_file.write_text(existing_text.rstrip() + "\n\n---\n\n" + round_text)
    else:
        review_file.write_text(round_text)
    return review_count


def upsert_review_block(text, review_block):
    """Replace the top-level review block or insert it before trailing metadata."""
    lines = text.splitlines(True)
    result = []
    i = 0

    while i < len(lines):
        if re.match(r"^review:\s*$", lines[i]):
            i += 1
            while i < len(lines):
                line = lines[i]
                if line.strip() and not line[0].isspace():
                    break
                i += 1
            continue
        result.append(lines[i])
        i += 1

    block_text = review_block.strip() + "\n"
    insert_idx = None
    for idx, line in enumerate(result):
        if re.match(r"^(self_eval|deviations|metadata):", line):
            insert_idx = idx
            break

    if insert_idx is None:
        if result and not result[-1].endswith("\n"):
            result[-1] += "\n"
        if result and result[-1].strip():
            result.append("\n")
        result.append(block_text)
    else:
        if insert_idx > 0 and result[insert_idx - 1].strip():
            result.insert(insert_idx, "\n")
            insert_idx += 1
        result.insert(insert_idx, block_text)

    return "".join(result)
