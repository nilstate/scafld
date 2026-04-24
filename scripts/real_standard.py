#!/usr/bin/env python3
"""Summarize scafld execution and review signals for explicit task cohorts."""

import argparse
import json
import subprocess
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parent
if str(REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(REPO_ROOT))

from scafld.command_runtime import find_root
from scafld.runtime_bundle import REVIEWS_DIR, scafld_source_root
from scafld.runtime_guidance import review_gate_snapshot
from scafld.spec_store import find_spec, yaml_read_field, yaml_read_nested


QUESTION_SET = [
    {
        "id": "build_flow",
        "prompt": "Did build feel easier than managing the same task through a raw agent loop?",
    },
    {
        "id": "review_signal",
        "prompt": "Did review surface useful, grounded criticism or clearly demonstrate a real attack pass?",
    },
    {
        "id": "handoff_default",
        "prompt": "Did the runtime consume the current handoff by default instead of relying on manual copy/paste?",
    },
    {
        "id": "ceremony_payoff",
        "prompt": "Did the extra ceremony pay for itself on this task?",
    },
]


def parse_args():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", help="workspace root; defaults to the current scafld repo root")
    parser.add_argument("--task", action="append", dest="tasks", help="task id to include; repeatable")
    parser.add_argument("--json", action="store_true", help="emit JSON instead of markdown")
    return parser.parse_args()


def resolve_root(raw_root):
    if raw_root:
        return Path(raw_root).expanduser().resolve()
    root = find_root()
    if root is None:
        raise SystemExit("not in a scafld workspace; pass --root")
    return root


def load_report(root):
    cli = scafld_source_root() / "cli" / "scafld"
    proc = subprocess.run(
        [sys.executable, str(cli), "report", "--json"],
        cwd=str(root),
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise SystemExit(proc.stdout or proc.stderr or "failed to load scafld report")
    return json.loads(proc.stdout)["result"]


def summarize_task(root, task_id, report):
    spec = find_spec(root, task_id)
    spec_path = str(spec.relative_to(root)) if spec else None
    spec_text = spec.read_text() if spec else ""
    status = yaml_read_field(spec_text, "status") if spec_text else None
    title = yaml_read_nested(spec_text, "task", "title") if spec_text else None
    review_state = review_gate_snapshot(root, task_id)
    runtime = ((report.get("llm_runtime") or {}).get("per_task") or {}).get(task_id, {})
    review_signal = ((report.get("review_signal") or {}).get("per_task") or {}).get(task_id, {})
    return {
        "task_id": task_id,
        "title": title or task_id,
        "status": status,
        "spec_path": spec_path,
        "review_file": str((root / REVIEWS_DIR / f"{task_id}.md").relative_to(root)),
        "review_state": review_state["review_state"],
        "review_gate": review_state["review_gate"],
        "runtime": runtime,
        "review_signal": review_signal,
        "questions": QUESTION_SET,
    }


def render_markdown(root, aggregate, tasks):
    lines = [
        "# Task Cohort Summary",
        "",
        f"- Workspace: `{root}`",
    ]
    llm_runtime = aggregate.get("llm_runtime") or {}
    review_signal = aggregate.get("review_signal") or {}
    first = llm_runtime.get("first_attempt_pass_rate") or {}
    recovery = llm_runtime.get("recovery_convergence_rate") or {}
    challenge = llm_runtime.get("challenge_override_rate") or {}
    lines.extend(
        [
            f"- First-attempt pass: {first.get('passed', 0)}/{first.get('total', 0)}",
            f"- Recovery convergence: {recovery.get('recovered', 0)}/{recovery.get('total', 0)}",
            f"- Challenge override: {challenge.get('overrides', 0)}/{challenge.get('total', 0)}",
            f"- Review rounds: {review_signal.get('completed_rounds', 0)}",
            f"- Grounded findings: {review_signal.get('grounded_findings', 0)}",
            f"- Clean reviews with evidence: {review_signal.get('clean_reviews_with_evidence', 0)}",
            "",
        ]
    )
    for task in tasks:
        runtime = task.get("runtime") or {}
        task_review_signal = task.get("review_signal") or {}
        first = runtime.get("first_attempt_pass_rate") or {}
        recovery = runtime.get("recovery_convergence_rate") or {}
        challenge = runtime.get("challenge_override_rate") or {}
        lines.extend(
            [
                f"## {task['title']}",
                "",
                f"- Task: `{task['task_id']}`",
                f"- Status: `{task.get('status') or 'unknown'}`",
                f"- Spec: `{task.get('spec_path') or 'missing'}`",
                f"- Review verdict: `{(task.get('review_state') or {}).get('verdict') or 'not_started'}`",
                f"- First-attempt pass: {first.get('passed', 0)}/{first.get('total', 0)}",
                f"- Recovery convergence: {recovery.get('recovered', 0)}/{recovery.get('total', 0)}",
                f"- Challenge override: {challenge.get('overrides', 0)}/{challenge.get('total', 0)}",
                f"- Grounded findings: {task_review_signal.get('grounded_findings', 0)}",
                f"- Clean review with evidence: `{bool(task_review_signal.get('clean_review_with_evidence'))}`",
                "",
                "Questions:",
            ]
        )
        for question in QUESTION_SET:
            lines.append(f"- {question['prompt']}")
        lines.append("")
    return "\n".join(lines).strip() + "\n"


def main():
    args = parse_args()
    root = resolve_root(args.root)
    report = load_report(root)
    per_task = ((report.get("llm_runtime") or {}).get("per_task") or {})
    review_per_task = ((report.get("review_signal") or {}).get("per_task") or {})
    task_ids = args.tasks or sorted(set(per_task) | set(review_per_task)) or []
    payload = {
        "workspace": str(root),
        "questions": QUESTION_SET,
        "aggregate": {
            "llm_runtime": report.get("llm_runtime") or {},
            "review_signal": report.get("review_signal") or {},
        },
        "tasks": [summarize_task(root, task_id, report) for task_id in task_ids],
    }

    if args.json:
        json.dump(payload, sys.stdout, indent=2)
        sys.stdout.write("\n")
        return

    sys.stdout.write(render_markdown(root, payload["aggregate"], payload["tasks"]))


if __name__ == "__main__":
    main()
