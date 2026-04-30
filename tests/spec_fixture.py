import argparse
import os
from pathlib import Path

from scafld.spec_markdown import render_spec_markdown


DEFAULT_TS = "2026-04-28T00:00:00Z"


def basic_spec(
    task_id,
    *,
    status="draft",
    title="Fixture",
    file_path="README.md",
    command="true",
    expected_kind="exit_code_zero",
    phase_status="pending",
    ownership=None,
    created=DEFAULT_TS,
    updated=None,
    phase_id="phase1",
    dod_status="pending",
    criterion_result=None,
    review_status="not_started",
):
    updated = updated or created
    change = {"file": file_path, "action": "update", "content_spec": "Fixture change."}
    if ownership:
        change["ownership"] = ownership
    criterion = {
        "id": "ac1_1",
        "type": "test",
        "description": "Run the fixture command.",
        "command": command,
        "expected_kind": expected_kind,
    }
    if criterion_result:
        criterion["result"] = criterion_result
        criterion["status"] = criterion_result
        criterion["checked_at"] = updated
        criterion["evidence"] = "fixture evidence"
    return {
        "spec_version": "2.0",
        "task_id": task_id,
        "created": created,
        "updated": updated,
        "status": status,
        "harden_status": "not_run",
        "task": {
            "title": title,
            "summary": "Fixture summary.",
            "size": "small",
            "risk_level": "low",
            "context": {
                "cwd": ".",
                "packages": ["tests"],
                "files_impacted": [
                    {
                        "path": file_path,
                        "reason": "Fixture change.",
                        **({"ownership": ownership} if ownership else {}),
                    }
                ],
                "invariants": ["fixture"],
                "related_docs": [],
            },
            "objectives": ["Exercise scafld behavior."],
            "touchpoints": [{"area": "tests", "description": "Fixture surface."}],
            "acceptance": {
                "validation_profile": "strict",
                "definition_of_done": [
                    {
                        "id": "dod1",
                        "description": "Fixture done.",
                        "status": dod_status,
                    }
                ],
                "validation": [],
            },
        },
        "planning_log": [{"timestamp": created, "actor": "test", "summary": "Fixture created."}],
        "phases": [
            {
                "id": phase_id,
                "name": "Fixture phase",
                "objective": "Exercise scafld behavior.",
                "changes": [change],
                "acceptance_criteria": [criterion],
                "status": phase_status,
            }
        ],
        "rollback": {"strategy": "per_phase", "commands": {phase_id: "true"}},
        "review": {"status": review_status, "findings": []},
    }


def write_basic_spec(path, task_id, **kwargs):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(render_spec_markdown(basic_spec(task_id, **kwargs)), encoding="utf-8")
    return path


def main():
    parser = argparse.ArgumentParser(description="write a v2 Markdown scafld fixture")
    parser.add_argument("path")
    parser.add_argument("task_id")
    parser.add_argument("status", nargs="?", default="draft")
    parser.add_argument("title", nargs="?", default="Fixture")
    parser.add_argument("file_path", nargs="?", default="README.md")
    parser.add_argument("command", nargs="?", default="true")
    args = parser.parse_args()
    write_basic_spec(
        args.path,
        args.task_id,
        status=args.status,
        title=args.title,
        file_path=args.file_path,
        command=args.command,
        expected_kind=os.environ.get("SCAFLD_SPEC_EXPECTED_KIND", "exit_code_zero"),
        phase_status=os.environ.get("SCAFLD_SPEC_PHASE_STATUS", "pending"),
        ownership=os.environ.get("SCAFLD_SPEC_OWNERSHIP") or None,
        created=os.environ.get("SCAFLD_SPEC_CREATED", DEFAULT_TS),
        updated=os.environ.get("SCAFLD_SPEC_UPDATED") or None,
        phase_id=os.environ.get("SCAFLD_SPEC_PHASE_ID", "phase1"),
        dod_status=os.environ.get("SCAFLD_SPEC_DOD_STATUS", "pending"),
        criterion_result=os.environ.get("SCAFLD_SPEC_CRITERION_RESULT") or None,
        review_status=os.environ.get("SCAFLD_SPEC_REVIEW_STATUS", "not_started"),
    )


if __name__ == "__main__":
    main()
