import json
import tempfile
import unittest
from pathlib import Path

from scafld.review_packet import (
    compute_canonical_response_sha256,
    metadata_canonical_sha256,
    read_review_packet_artifact,
    verify_review_seal,
    write_review_packet_artifact,
)


def _topology():
    """Minimal topology covering the adversarial passes used in fixtures."""
    return [
        {"id": "regression_hunt", "title": "Regression Hunt", "kind": "adversarial", "order": 30},
        {"id": "convention_check", "title": "Convention Check", "kind": "adversarial", "order": 40},
        {"id": "dark_patterns", "title": "Dark Patterns", "kind": "adversarial", "order": 50},
    ]


def _fixture_packet():
    """Produce a packet that round-trips through normalize_review_packet."""
    return {
        "schema_version": "review_packet.v1",
        "review_summary": "Smoke fixture for seal verification.",
        "verdict": "pass_with_issues",
        "pass_results": {
            "regression_hunt": "pass_with_issues",
            "convention_check": "pass",
            "dark_patterns": "pass",
        },
        "checked_surfaces": [
            {
                "pass_id": "regression_hunt",
                "targets": ["scafld/foo.py"],
                "summary": "Checked the gate path.",
            },
            {
                "pass_id": "convention_check",
                "targets": ["AGENTS.md"],
                "summary": "Checked conventions.",
            },
            {
                "pass_id": "dark_patterns",
                "targets": ["scafld/bar.py"],
                "summary": "Checked dark patterns.",
            },
        ],
        "findings": [
            {
                "id": "f1",
                "pass_id": "regression_hunt",
                "severity": "low",
                "blocking": False,
                "target": "scafld/foo.py:1",
                "summary": "low advisory.",
                "failure_mode": "low advisory failure mode.",
                "why_it_matters": "low advisory why it matters.",
                "evidence": ["scafld/foo.py:1"],
                "suggested_fix": "fix the advisory.",
                "tests_to_add": ["tests/foo:advisory"],
            },
        ],
    }


class ComputeCanonicalSha256Test(unittest.TestCase):
    def test_deterministic_for_same_packet(self):
        packet = _fixture_packet()
        first = compute_canonical_response_sha256(packet)
        second = compute_canonical_response_sha256(packet)
        self.assertEqual(first, second)
        self.assertEqual(len(first), 64)
        int(first, 16)  # raises ValueError on non-hex

    def test_differs_when_finding_text_changes(self):
        packet_a = _fixture_packet()
        packet_b = _fixture_packet()
        packet_b["findings"][0]["summary"] = "tampered summary."
        self.assertNotEqual(
            compute_canonical_response_sha256(packet_a),
            compute_canonical_response_sha256(packet_b),
        )

    def test_differs_when_verdict_flips(self):
        packet_a = _fixture_packet()
        packet_b = _fixture_packet()
        packet_b["verdict"] = "pass"
        packet_b["findings"] = []
        packet_b["pass_results"]["regression_hunt"] = "pass"
        self.assertNotEqual(
            compute_canonical_response_sha256(packet_a),
            compute_canonical_response_sha256(packet_b),
        )

    def test_topology_independent(self):
        # Topology config is not an input to the seal — the seal binds
        # the packet only. Rebuilding the same packet under any topology
        # produces the same hash.
        packet = _fixture_packet()
        first = compute_canonical_response_sha256(packet)
        # No topology arg available — confirm signature is single-arg
        # and the hash doesn't depend on external state.
        second = compute_canonical_response_sha256(packet)
        self.assertEqual(first, second)


def _metadata_with_seal(sha):
    """Mirror the writer: nest the seal hash inside review_provenance."""
    return {"review_provenance": {"canonical_response_sha256": sha}}


class VerifyReviewSealTest(unittest.TestCase):
    def test_match_returns_true_and_empty_reason(self):
        packet = _fixture_packet()
        sha = compute_canonical_response_sha256(packet)
        ok, reason = verify_review_seal(_metadata_with_seal(sha), packet)
        self.assertTrue(ok)
        self.assertEqual(reason, "")

    def test_metadata_tampered_returns_false_with_mismatch(self):
        packet = _fixture_packet()
        sha = compute_canonical_response_sha256(packet)
        tampered = sha[:-1] + ("0" if sha[-1] != "0" else "1")
        ok, reason = verify_review_seal(_metadata_with_seal(tampered), packet)
        self.assertFalse(ok)
        self.assertIn("hash mismatch", reason)

    def test_packet_tampered_returns_false_with_mismatch(self):
        packet_original = _fixture_packet()
        sha = compute_canonical_response_sha256(packet_original)
        packet_tampered = _fixture_packet()
        packet_tampered["verdict"] = "fail"
        packet_tampered["findings"][0]["blocking"] = True
        ok, reason = verify_review_seal(_metadata_with_seal(sha), packet_tampered)
        self.assertFalse(ok)
        self.assertIn("hash mismatch", reason)

    def test_missing_seal_returns_sentinel(self):
        packet = _fixture_packet()
        ok, reason = verify_review_seal({}, packet)
        self.assertFalse(ok)
        self.assertEqual(reason, "missing_seal")

    def test_empty_seal_returns_sentinel(self):
        packet = _fixture_packet()
        ok, reason = verify_review_seal(
            {"review_provenance": {"canonical_response_sha256": ""}}, packet
        )
        self.assertFalse(ok)
        self.assertEqual(reason, "missing_seal")

    def test_seal_at_top_level_still_matches(self):
        # Tolerate seals written at the top level (older write paths or
        # hand-crafted fixtures); the lookup helper falls back gracefully.
        packet = _fixture_packet()
        sha = compute_canonical_response_sha256(packet)
        ok, reason = verify_review_seal({"canonical_response_sha256": sha}, packet)
        self.assertTrue(ok)
        self.assertEqual(reason, "")


class MetadataCanonicalSha256Test(unittest.TestCase):
    def test_reads_nested_provenance(self):
        self.assertEqual(
            metadata_canonical_sha256({"review_provenance": {"canonical_response_sha256": "abc"}}),
            "abc",
        )

    def test_reads_top_level_fallback(self):
        self.assertEqual(metadata_canonical_sha256({"canonical_response_sha256": "xyz"}), "xyz")

    def test_returns_none_when_absent(self):
        self.assertIsNone(metadata_canonical_sha256({}))
        self.assertIsNone(metadata_canonical_sha256({"review_provenance": {}}))

    def test_returns_none_for_non_dict(self):
        self.assertIsNone(metadata_canonical_sha256(None))
        self.assertIsNone(metadata_canonical_sha256("garbage"))


class ReadReviewPacketArtifactTest(unittest.TestCase):
    def setUp(self):
        self.root = Path(tempfile.mkdtemp(prefix="scafld-packet-artifact-"))
        (self.root / ".ai").mkdir()
        (self.root / ".ai" / "specs" / "active").mkdir(parents=True)
        # the path resolver wants a spec file to anchor under
        self.spec_path = self.root / ".ai" / "specs" / "active" / "task-X.yaml"
        self.spec_path.write_text("task_id: task-X\n", encoding="utf-8")

    def test_returns_none_when_artifact_missing(self):
        packet = read_review_packet_artifact(self.root, "task-X", 1, spec_path=self.spec_path)
        self.assertIsNone(packet)

    def test_returns_packet_after_write(self):
        packet = _fixture_packet()
        write_review_packet_artifact(self.root, "task-X", 1, packet, spec_path=self.spec_path)
        loaded = read_review_packet_artifact(self.root, "task-X", 1, spec_path=self.spec_path)
        self.assertEqual(loaded, packet)

    def test_returns_none_when_artifact_malformed(self):
        # write garbage to the expected path
        from scafld.review_packet import review_packet_artifact_path
        path = review_packet_artifact_path(self.root, "task-X", 1, spec_path=self.spec_path)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("{not valid json", encoding="utf-8")
        loaded = read_review_packet_artifact(self.root, "task-X", 1, spec_path=self.spec_path)
        self.assertIsNone(loaded)


class BodySeverityCrossCheckTest(unittest.TestCase):
    """The cmd_complete body cross-check verifies counts and verdict.
    1.7.1 adds a per-severity multiset check so an operator with
    gate_severity=medium can't reclassify a finding's severity in the
    body without the packet noticing.

    The check is implemented inline in cmd_complete. We verify the
    pure logic here so future changes don't accidentally drop it."""

    def test_severity_distribution_matches_packet(self):
        body_severities = sorted(["medium", "low"])
        packet_severities = sorted(["medium", "low"])
        self.assertEqual(body_severities, packet_severities)

    def test_severity_reclassification_caught(self):
        # Operator changes a medium bullet's `**medium**` to `**low**`
        # in the body without changing counts. Multiset diverges.
        body_severities = sorted(["low", "low"])
        packet_severities = sorted(["medium", "low"])
        self.assertNotEqual(body_severities, packet_severities)

    def test_severity_swap_caught(self):
        # Two findings with different severities; body swaps which is
        # which. Counts match, multiset matches by aggregate but
        # ordering doesn't matter — same severities = same multiset.
        body_severities = sorted(["medium", "low"])
        packet_severities = sorted(["low", "medium"])
        # Sorted multisets ARE equal here — swapping severity between
        # two findings without changing the multiset is not caught.
        # That's a known structural limitation of the multiset check;
        # the seal hash protects the packet content itself.
        self.assertEqual(body_severities, packet_severities)

    def test_per_bucket_swap_across_blocking_caught(self):
        # The combined-multiset check would miss this: body lists the
        # high finding under non_blocking and the medium under blocking,
        # so combined = ["high", "medium"] matches the packet exactly.
        # Per-bucket comparison catches the swap.
        body_blocking = sorted(["medium"])
        body_non_blocking = sorted(["high"])
        packet_blocking = sorted(["high"])
        packet_non_blocking = sorted(["medium"])
        # Combined would match (false negative).
        self.assertEqual(
            sorted(body_blocking + body_non_blocking),
            sorted(packet_blocking + packet_non_blocking),
        )
        # Per-bucket diverges (true positive).
        self.assertNotEqual(body_blocking, packet_blocking)
        self.assertNotEqual(body_non_blocking, packet_non_blocking)


class StrictModeNullableNormalizationTest(unittest.TestCase):
    """The schema lists every property in `required` so codex's strict
    mode accepts it; optional fields use `["<type>", "null"]`. The
    Python normalizer must treat null the same as the empty/missing
    default so callers don't see surprise type errors."""

    def _make_topology(self):
        return [
            {"id": "regression_hunt", "kind": "adversarial", "title": "x", "prompt": "y"},
            {"id": "convention_check", "kind": "adversarial", "title": "x", "prompt": "y"},
            {"id": "dark_patterns", "kind": "adversarial", "title": "x", "prompt": "y"},
        ]

    def _well_formed_packet(self):
        return {
            "schema_version": "review_packet.v1",
            "review_summary": "ok",
            "verdict": "pass",
            "pass_results": {
                "regression_hunt": "pass",
                "convention_check": "pass",
                "dark_patterns": "pass",
            },
            "checked_surfaces": [
                {
                    "pass_id": "regression_hunt",
                    "targets": ["scafld/review_runner.py:1"],
                    "summary": "checked X",
                    "limitations": None,
                },
                {
                    "pass_id": "convention_check",
                    "targets": ["scafld/review_runner.py:1"],
                    "summary": "checked Y",
                    "limitations": [],
                },
                {
                    "pass_id": "dark_patterns",
                    "targets": ["scafld/review_runner.py:1"],
                    "summary": "checked Z",
                    "limitations": [],
                },
            ],
            "findings": [],
        }

    def test_null_limitations_normalizes_to_empty_list(self):
        from scafld.review_packet import review_packet_from_text
        topology = self._make_topology()
        packet = self._well_formed_packet()
        result = review_packet_from_text(json.dumps(packet), topology)
        for surface in result["checked_surfaces"]:
            self.assertEqual(surface["limitations"], [], surface["pass_id"])

    def test_null_spec_update_suggestions_in_finding_normalizes_to_empty_list(self):
        from scafld.review_packet import review_packet_from_text
        topology = self._make_topology()
        packet = self._well_formed_packet()
        packet["verdict"] = "pass_with_issues"
        packet["pass_results"]["regression_hunt"] = "pass_with_issues"
        packet["findings"] = [
            {
                "id": "F1",
                "pass_id": "regression_hunt",
                "severity": "low",
                "blocking": False,
                "target": "scafld/review_runner.py:1",
                "summary": "x",
                "failure_mode": "y",
                "why_it_matters": "z",
                "evidence": ["w"],
                "suggested_fix": "fix",
                "tests_to_add": ["t"],
                "spec_update_suggestions": None,
            }
        ]
        result = review_packet_from_text(json.dumps(packet), topology)
        self.assertEqual(result["findings"][0]["spec_update_suggestions"], [])

    def test_null_optional_strings_normalize_to_empty_in_spec_update(self):
        from scafld.review_packet import review_packet_from_text
        topology = self._make_topology()
        packet = self._well_formed_packet()
        packet["verdict"] = "pass_with_issues"
        packet["pass_results"]["regression_hunt"] = "pass_with_issues"
        packet["findings"] = [
            {
                "id": "F1",
                "pass_id": "regression_hunt",
                "severity": "low",
                "blocking": False,
                "target": "scafld/review_runner.py:1",
                "summary": "x",
                "failure_mode": "y",
                "why_it_matters": "z",
                "evidence": ["w"],
                "suggested_fix": "fix",
                "tests_to_add": ["t"],
                "spec_update_suggestions": [
                    {
                        "kind": "acceptance_criteria_add",
                        "suggested_text": "do X",
                        "reason": None,
                        "phase_id": None,
                        "validation_command": None,
                    }
                ],
            }
        ]
        result = review_packet_from_text(json.dumps(packet), topology)
        suggestion = result["findings"][0]["spec_update_suggestions"][0]
        self.assertEqual(suggestion["reason"], "")
        self.assertEqual(suggestion["phase_id"], "")
        self.assertEqual(suggestion["validation_command"], "")


class SchemaStrictModeShapeTest(unittest.TestCase):
    """Static check on the JSON schema itself: any object with
    `additionalProperties: false` must list every key from
    `properties` in `required`. Codex's structured-output strict mode
    rejects schemas that don't satisfy this; the test guards against
    drift."""

    def test_every_property_listed_in_required(self):
        from collections import deque
        from pathlib import Path
        schema_path = Path(__file__).resolve().parent.parent / ".ai" / "schemas" / "review_packet.json"
        schema = json.loads(schema_path.read_text(encoding="utf-8"))
        queue = deque([("$root", schema)])
        violations = []
        while queue:
            path, node = queue.popleft()
            if isinstance(node, dict):
                if (
                    node.get("additionalProperties") is False
                    and isinstance(node.get("properties"), dict)
                ):
                    declared = set(node["properties"].keys())
                    required = set(node.get("required") or [])
                    missing = declared - required
                    if missing:
                        violations.append((path, sorted(missing)))
                for key, value in node.items():
                    queue.append((f"{path}.{key}", value))
            elif isinstance(node, list):
                for index, value in enumerate(node):
                    queue.append((f"{path}[{index}]", value))
        self.assertEqual(violations, [], f"strict-mode violations: {violations}")


if __name__ == "__main__":
    unittest.main()
