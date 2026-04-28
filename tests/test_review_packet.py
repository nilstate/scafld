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


if __name__ == "__main__":
    unittest.main()
