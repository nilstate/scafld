from scafld.reviewing import parse_review_file
from scafld.review_workflow import load_review_topology
from scafld.runtime_bundle import REVIEWS_DIR


def review_signal_payload(review_data):
    sections = review_data.get("adversarial_sections") or {}
    completed_round = review_data.get("round_status") == "completed"
    grounded_findings = len(review_data.get("blocking", [])) + len(review_data.get("non_blocking", []))
    no_issue_sections = sum(1 for section in sections.values() if section.get("state") == "no_issues")
    checked_sections = sum(
        1
        for section in sections.values()
        if section.get("state") == "no_issues" and section.get("checked") and section.get("evidence", True)
    )
    finding_sections = sum(1 for section in sections.values() if section.get("state") == "findings")
    format_compliant_clean_review = bool(
        completed_round
        and review_data.get("verdict") == "pass"
        and sections
        and all(
            section.get("state") == "no_issues" and section.get("checked")
            and section.get("evidence", True)
            for section in sections.values()
        )
        and not review_data.get("errors")
    )
    return {
        "exists": bool(review_data.get("exists")),
        "completed_round": completed_round,
        "verdict": review_data.get("verdict"),
        "blocking_findings": len(review_data.get("blocking", [])),
        "non_blocking_findings": len(review_data.get("non_blocking", [])),
        "grounded_findings": grounded_findings,
        "finding_sections": finding_sections,
        "no_issue_sections": no_issue_sections,
        "checked_no_issue_sections": checked_sections,
        "format_compliant_clean_review": format_compliant_clean_review,
    }


def load_review_signal(root, task_id, topology=None):
    topology = topology or load_review_topology(root)
    review_path = root / REVIEWS_DIR / f"{task_id}.md"
    review_data = parse_review_file(review_path, topology)
    return review_signal_payload(review_data)
