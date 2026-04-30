from pathlib import Path


def find_archived_spec(root, archive_dir, task_id):
    """Find an archived spec by task id. Returns Path or None."""
    archive = root / archive_dir
    if not archive.is_dir():
        return None

    for month_dir in sorted(archive.iterdir(), reverse=True):
        if not month_dir.is_dir():
            continue
        candidate = month_dir / f"{task_id}.md"
        if candidate.exists():
            return candidate
    return None


def parse_code_grounded_in(value):
    """Parse code:<file>:<line> citations into (path, line_number)."""
    body = value[len("code:"):]
    rel_path, sep, raw_line = body.rpartition(":")
    if not sep or not rel_path:
        return None, None
    try:
        line_number = int(raw_line)
    except ValueError:
        return None, None
    return rel_path, line_number


def verify_harden_round_citations(root, archive_dir, round_data):
    """Return warning strings for unresolvable harden citations."""
    warnings = []
    root_resolved = root.resolve()

    for idx, question in enumerate(round_data.get("questions") or [], start=1):
        grounded_in = question.get("grounded_in")
        if not isinstance(grounded_in, str):
            continue

        if grounded_in.startswith("code:"):
            rel_path, line_number = parse_code_grounded_in(grounded_in)
            if rel_path is None or line_number is None or line_number < 1:
                warnings.append(f"question {idx}: invalid code citation {grounded_in}")
                continue

            candidate = (root / rel_path).resolve()
            try:
                candidate.relative_to(root_resolved)
            except ValueError:
                warnings.append(f"question {idx}: code citation escapes workspace root: {grounded_in}")
                continue

            if not candidate.exists() or not candidate.is_file():
                warnings.append(f"question {idx}: code citation not found: {grounded_in}")
                continue

            try:
                line_count = len(candidate.read_text(encoding="utf-8", errors="replace").splitlines())
            except OSError:
                warnings.append(f"question {idx}: code citation could not be read: {grounded_in}")
                continue

            if line_number > line_count:
                warnings.append(
                    f"question {idx}: code citation line {line_number} exceeds {line_count} lines in {rel_path}"
                )

        elif grounded_in.startswith("archive:"):
            archive_task_id = grounded_in.split(":", 1)[1].strip()
            if not archive_task_id or not find_archived_spec(root, archive_dir, archive_task_id):
                warnings.append(f"question {idx}: archive citation not found: {grounded_in}")

    return warnings
