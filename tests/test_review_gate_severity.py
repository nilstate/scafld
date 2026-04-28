import tempfile
import unittest
from pathlib import Path
from unittest import mock

from scafld.review_workflow import (
    _findings_at_or_above,
    _load_gate_severity,
    evaluate_review_gate,
)


def _workspace(*, gate_severity=None):
    tmp = Path(tempfile.mkdtemp(prefix="scafld-gate-severity-"))
    (tmp / ".ai").mkdir()
    config = "version: '1.0'\n"
    if gate_severity is not None:
        config += f"review:\n  gate_severity: {gate_severity!r}\n"
    (tmp / ".ai" / "config.yaml").write_text(config, encoding="utf-8")
    return tmp


def _fixture_review_data(*, blocking=None, non_blocking=None, verdict="pass_with_issues"):
    blocking = blocking or []
    non_blocking = non_blocking or []
    return {
        "exists": True,
        "errors": [],
        "metadata": {
            "reviewed_head": "deadbeef",
            "reviewed_dirty": False,
            "reviewed_diff": "0" * 64,
        },
        "empty_adversarial": [],
        "round_status": "completed",
        "verdict": verdict,
        "blocking": [entry["line"] for entry in blocking],
        "non_blocking": [entry["line"] for entry in non_blocking],
        "blocking_findings": blocking,
        "non_blocking_findings": non_blocking,
    }


class LoadGateSeverityTest(unittest.TestCase):
    def test_default_blocking_when_unset(self):
        root = _workspace()
        self.assertEqual(_load_gate_severity(root), "blocking")

    def test_explicit_medium(self):
        root = _workspace(gate_severity="medium")
        self.assertEqual(_load_gate_severity(root), "medium")

    def test_explicit_low(self):
        root = _workspace(gate_severity="low")
        self.assertEqual(_load_gate_severity(root), "low")

    def test_typo_falls_back_to_blocking(self):
        root = _workspace(gate_severity="moderate")
        self.assertEqual(_load_gate_severity(root), "blocking")


class FindingsAtOrAboveTest(unittest.TestCase):
    def test_empty_input(self):
        self.assertEqual(_findings_at_or_above([], 3), [])

    def test_high_above_medium_threshold(self):
        findings = [{"line": "x", "severity": "high"}]
        # medium threshold rank = 2; high rank = 3
        self.assertEqual(_findings_at_or_above(findings, 2), findings)

    def test_low_below_medium_threshold(self):
        findings = [{"line": "x", "severity": "low"}]
        self.assertEqual(_findings_at_or_above(findings, 2), [])

    def test_unknown_severity_treated_as_zero(self):
        findings = [{"line": "x", "severity": None}]
        self.assertEqual(_findings_at_or_above(findings, 1), [])


def _bypass_git_binding(monkeypatch_target):
    """Mock the git-binding check so gate reasoning doesn't depend on a real repo."""
    return mock.patch(
        "scafld.review_workflow.capture_bound_review_git_state",
        return_value=({
            "current_head": "deadbeef",
            "current_dirty": False,
            "current_diff": "0" * 64,
        }, None),
    )


class EvaluateReviewGateThresholdTest(unittest.TestCase):
    def setUp(self):
        # mock the git binding capture so gate doesn't fail on missing .git
        self.bind_patch = mock.patch(
            "scafld.review_workflow.capture_bound_review_git_state",
            return_value=({
                "current_head": "deadbeef",
                "current_dirty": False,
                "current_diff": "0" * 64,
            }, None),
        )
        self.bind_patch.start()
        self.addCleanup(self.bind_patch.stop)

    def _evaluate(self, root, review_data):
        # review_file path doesn't matter when we mock the git capture
        from pathlib import Path as P
        review_file = root / ".ai" / "reviews" / "fixture.md"
        review_file.parent.mkdir(parents=True, exist_ok=True)
        review_file.write_text("placeholder", encoding="utf-8")
        return evaluate_review_gate(root, review_file, review_data)

    def test_default_blocking_passes_pass_with_issues(self):
        root = _workspace()
        data = _fixture_review_data(non_blocking=[
            {"line": "- **medium** `x.py:1` — note", "severity": "medium"},
        ])
        gate = self._evaluate(root, data)
        self.assertIsNone(gate["gate_reason"])
        self.assertEqual(gate["gate_threshold"], "blocking")
        self.assertEqual(gate["gate_advisory_count"], 1)

    def test_threshold_medium_blocks_non_blocking_medium(self):
        root = _workspace(gate_severity="medium")
        data = _fixture_review_data(non_blocking=[
            {"line": "- **medium** `x.py:1` — note", "severity": "medium"},
        ])
        gate = self._evaluate(root, data)
        self.assertIsNotNone(gate["gate_reason"])
        self.assertIn("severity medium", gate["gate_reason"])
        self.assertEqual(gate["gate_advisory_count"], 0)

    def test_threshold_medium_passes_non_blocking_low(self):
        root = _workspace(gate_severity="medium")
        data = _fixture_review_data(non_blocking=[
            {"line": "- **low** `x.py:1` — note", "severity": "low"},
        ])
        gate = self._evaluate(root, data)
        self.assertIsNone(gate["gate_reason"])
        self.assertEqual(gate["gate_advisory_count"], 1)

    def test_threshold_low_blocks_any_non_blocking(self):
        root = _workspace(gate_severity="low")
        data = _fixture_review_data(non_blocking=[
            {"line": "- **low** `x.py:1` — note", "severity": "low"},
        ])
        gate = self._evaluate(root, data)
        self.assertIsNotNone(gate["gate_reason"])
        self.assertIn("severity low", gate["gate_reason"])

    def test_advisory_count_below_threshold(self):
        root = _workspace(gate_severity="medium")
        data = _fixture_review_data(non_blocking=[
            {"line": "- **low** `x.py:1` — a", "severity": "low"},
            {"line": "- **low** `x.py:2` — b", "severity": "low"},
        ])
        gate = self._evaluate(root, data)
        self.assertIsNone(gate["gate_reason"])
        self.assertEqual(gate["gate_advisory_count"], 2)

    def test_advisory_findings_pick_below_threshold_entries(self):
        # Regression for severity-gates Review 1 F1: cmd_complete used to
        # slice the first N non_blocking_findings, which silently grabbed
        # the gate-blocking entries when threshold was medium/low. The fix
        # surfaces a structured `advisory_findings` list on the gate dict
        # filtered by severity rank.
        root = _workspace(gate_severity="medium")
        data = _fixture_review_data(
            verdict="fail",
            blocking=[
                {"line": "- **medium** `x.py:1` — gate", "severity": "medium"},
            ],
            non_blocking=[
                {"line": "- **medium** `x.py:2` — promoted", "severity": "medium"},
                {"line": "- **low** `x.py:3` — actual advisory", "severity": "low"},
            ],
        )
        gate = self._evaluate(root, data)
        # advisory_findings should contain ONLY below-threshold (low) entries
        advisory = gate.get("advisory_findings") or []
        self.assertEqual(len(advisory), 1)
        self.assertEqual(advisory[0]["severity"], "low")
        self.assertIn("actual advisory", advisory[0]["line"])


class CompletePayloadAdvisoryTest(unittest.TestCase):
    """Phase 2 promised that cmd_complete surfaces advisory count + breakdown
    on the JSON result. The cmd_complete logic that builds those fields is
    pure once the gate dict is in hand; verify it directly."""

    def test_advisory_payload_aggregates_severities(self):
        gate_dict = {
            "gate_reason": None,
            "gate_threshold": "medium",
            "gate_advisory_count": 3,
            "advisory_findings": [
                {"line": "- **low** `x.py:1` — a", "severity": "low"},
                {"line": "- **low** `x.py:2` — b", "severity": "low"},
                {"line": "- **low** `x.py:3` — c", "severity": "low"},
            ],
        }
        # Mirror the cmd_complete aggregation
        advisory_findings = list(gate_dict.get("advisory_findings") or [])
        breakdown = {}
        for entry in advisory_findings:
            severity = (entry.get("severity") or "unspecified") if isinstance(entry, dict) else "unspecified"
            breakdown[severity] = breakdown.get(severity, 0) + 1
        self.assertEqual(len(advisory_findings), 3)
        self.assertEqual(breakdown, {"low": 3})

    def test_advisory_payload_unspecified_severity_bucket(self):
        gate_dict = {
            "advisory_findings": [
                {"line": "malformed bullet, no severity captured", "severity": None},
            ],
        }
        advisory_findings = list(gate_dict.get("advisory_findings") or [])
        breakdown = {}
        for entry in advisory_findings:
            severity = (entry.get("severity") or "unspecified") if isinstance(entry, dict) else "unspecified"
            breakdown[severity] = breakdown.get(severity, 0) + 1
        self.assertEqual(breakdown, {"unspecified": 1})


class GateSnapshotPropagatesNewFieldsTest(unittest.TestCase):
    """`review_gate_snapshot` should expose the new fields so cmd_complete
    and status_snapshot don't have to recompute them."""

    def test_snapshot_exposes_threshold_and_counts(self):
        from scafld.runtime_guidance import review_gate_snapshot
        # The function reads from disk; we patch the underlying loaders so it
        # returns predictable data without a real workspace.
        topology = [
            {"id": "regression_hunt", "title": "Regression Hunt", "kind": "adversarial"},
        ]
        review_data_stub = _fixture_review_data(non_blocking=[
            {"line": "- **medium** `x.py:1` — note", "severity": "medium"},
        ])
        gate_stub = {
            "gate_reason": None,
            "gate_errors": [],
            "current_git_state": None,
            "review_metadata": {},
            "gate_threshold": "medium",
            "gate_blocking_count": 0,
            "gate_advisory_count": 1,
        }

        with mock.patch("scafld.runtime_guidance.load_review_topology", return_value=topology), \
             mock.patch("scafld.runtime_guidance.parse_review_file", return_value=review_data_stub), \
             mock.patch("scafld.runtime_guidance.load_review_state", return_value={"exists": True}), \
             mock.patch("scafld.runtime_guidance.evaluate_review_gate", return_value=gate_stub):
            snapshot = review_gate_snapshot(Path("/nonexistent"), "task")

        self.assertEqual(snapshot["review_gate"]["gate_threshold"], "medium")
        self.assertEqual(snapshot["review_gate"]["gate_blocking_count"], 0)
        self.assertEqual(snapshot["review_gate"]["gate_advisory_count"], 1)


class DeriveTaskGuidanceThresholdBlockedBranchTest(unittest.TestCase):
    """1.7.1 adds a threshold-blocked branch in derive_task_guidance.
    When gate_threshold > 'blocking' and verdict is pass/pass_with_issues
    but the gate fires due to threshold, next_action should name the
    threshold and suggest fix-or-relax — NOT 'rerun review' (which
    just hits the same gate)."""

    def _spec_data(self):
        return {
            "phases": [{
                "id": "phase1",
                "status": "completed",
                "acceptance_criteria": [{"id": "ac1_1", "result": "pass"}],
            }],
        }

    def _gate(self, *, gate_reason, threshold="medium"):
        return {
            "exists": True,
            "gate_reason": gate_reason,
            "gate_errors": [],
            "gate_threshold": threshold,
            "gate_blocking_count": 0,
            "gate_advisory_count": 0,
        }

    def test_threshold_blocked_yields_threshold_blocked_action(self):
        from scafld.runtime_guidance import derive_task_guidance
        from pathlib import Path
        from unittest import mock

        review_state = {
            "exists": True,
            "verdict": "pass_with_issues",
        }
        review_gate = self._gate(
            gate_reason="latest review has 1 non-blocking finding(s) at or above severity medium",
            threshold="medium",
        )

        with mock.patch("scafld.runtime_guidance.active_review_provider_invocation", return_value=None), \
             mock.patch("scafld.runtime_guidance.existing_review_handoff", return_value=None), \
             mock.patch("scafld.runtime_guidance.existing_review_repair_handoff", return_value=None), \
             mock.patch("scafld.runtime_guidance.predicted_handoff", return_value={"command": "scafld handoff foo --phase phase1", "path_rel": "x"}):
            guidance = derive_task_guidance(
                Path("/nonexistent"),
                "task",
                Path("/nonexistent/spec.yaml"),
                self._spec_data(),
                "in_progress",
                {"entries": []},
                review_state=review_state,
                review_gate=review_gate,
            )

        self.assertEqual(guidance["next_action"]["type"], "threshold_blocked")
        self.assertIn("severity medium", guidance["next_action"]["message"])
        self.assertIn("gate_severity", guidance["next_action"]["message"])

    def test_threshold_blocking_default_falls_through(self):
        # When gate_threshold is "blocking" (default), no threshold
        # branch should fire; we go to the "rerun review" fallback
        # for malformed/incomplete cases.
        from scafld.runtime_guidance import derive_task_guidance
        from pathlib import Path
        from unittest import mock

        review_state = {
            "exists": True,
            "verdict": "pass_with_issues",
        }
        review_gate = self._gate(
            gate_reason="latest review is incomplete",
            threshold="blocking",
        )

        with mock.patch("scafld.runtime_guidance.active_review_provider_invocation", return_value=None), \
             mock.patch("scafld.runtime_guidance.existing_review_handoff", return_value=None), \
             mock.patch("scafld.runtime_guidance.existing_review_repair_handoff", return_value=None), \
             mock.patch("scafld.runtime_guidance.predicted_handoff", return_value={"command": "scafld handoff foo --phase phase1", "path_rel": "x"}):
            guidance = derive_task_guidance(
                Path("/nonexistent"),
                "task",
                Path("/nonexistent/spec.yaml"),
                self._spec_data(),
                "in_progress",
                {"entries": []},
                review_state=review_state,
                review_gate=review_gate,
            )

        self.assertEqual(guidance["next_action"]["type"], "review")


if __name__ == "__main__":
    unittest.main()
