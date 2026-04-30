CANONICAL_HARDEN_RUBRIC = (
    {
        "id": "product_goal",
        "question": "What is the real product goal, not just the requested implementation?",
        "field": "task.summary",
        "severity": "high",
    },
    {
        "id": "authority",
        "question": "What is authoritative when two artifacts contain the same fact?",
        "field": "authority",
        "severity": "high",
    },
    {
        "id": "ownership",
        "question": "What are the ownership boundaries?",
        "field": "task.context.files_impacted",
        "severity": "high",
    },
    {
        "id": "recovery",
        "question": "What fails halfway, and how is it repaired?",
        "field": "rollback",
        "severity": "medium",
    },
    {
        "id": "invariants",
        "question": "What invariants must be testable?",
        "field": "task.acceptance.definition_of_done",
        "severity": "high",
    },
    {
        "id": "cutovers",
        "question": "What hidden cutovers are bundled?",
        "field": "task.scope",
        "severity": "medium",
    },
    {
        "id": "fixtures",
        "question": "What examples or golden fixtures prove the shape?",
        "field": "examples",
        "severity": "medium",
    },
    {
        "id": "operations",
        "question": "What operational command lets a human recover?",
        "field": "operations",
        "severity": "medium",
    },
    {
        "id": "dogfood",
        "question": "Can we dogfood this?",
        "field": "validation",
        "severity": "medium",
    },
    {
        "id": "complexity_containment",
        "question": "What complexity is being accepted, and why is it worth it?",
        "field": "risks",
        "severity": "medium",
    },
)


def _present(value):
    if value is None:
        return False
    if isinstance(value, str):
        return bool(value.strip())
    if isinstance(value, (list, tuple, set, dict)):
        return bool(value)
    return True


def _path_value(data, dotted):
    current = data
    for part in dotted.split("."):
        if not isinstance(current, dict):
            return None
        current = current.get(part)
    return current


def harden_findings(spec_data):
    """Return deterministic spec-quality findings for the canonical rubric."""
    spec_data = spec_data if isinstance(spec_data, dict) else {}
    findings = []
    for item in CANONICAL_HARDEN_RUBRIC:
        field = item["field"]
        if field in {"authority", "examples", "operations", "validation", "risks"}:
            # These are judgment surfaces; they may live in prose, acceptance,
            # or phase text. The harden prompt asks the agent to inspect them
            # rather than pretending a single schema field proves the answer.
            continue
        value = _path_value(spec_data, field)
        if not _present(value):
            findings.append(
                {
                    "id": item["id"],
                    "severity": item["severity"],
                    "field": field,
                    "question": item["question"],
                    "blocking": item["severity"] in {"critical", "high"},
                    "suggested_fix": f"Answer: {item['question']}",
                }
            )
    return findings


def unresolved_blockers(findings):
    return [finding for finding in findings or [] if finding.get("blocking")]


def harden_can_pass(spec_data):
    return not unresolved_blockers(harden_findings(spec_data))
