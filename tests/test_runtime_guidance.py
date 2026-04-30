import tempfile
import unittest
from pathlib import Path
from unittest import mock

from scafld.runtime_guidance import derive_task_guidance
from tests.spec_fixture import basic_spec


class RuntimeGuidanceTests(unittest.TestCase):
    def test_draft_with_open_harden_round_points_at_harden(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            spec = root / ".scafld" / "specs" / "drafts" / "task.md"
            data = basic_spec("task", status="draft")
            data["harden_status"] = "in_progress"

            guidance = derive_task_guidance(root, "task", spec, data, "draft", {})

        self.assertEqual(guidance["next_action"]["type"], "harden")
        self.assertEqual(guidance["next_action"]["command"], "scafld harden task --mark-passed")

    def test_recovery_pending_points_at_recovery_handoff(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            spec = root / ".scafld" / "specs" / "active" / "task.md"
            data = basic_spec("task", status="in_progress")
            session = {"criterion_states": {"ac1_1": {"status": "recovery_pending"}}}

            guidance = derive_task_guidance(root, "task", spec, data, "in_progress", session)

        self.assertEqual(guidance["next_action"]["type"], "recovery_handoff")
        self.assertEqual(guidance["next_action"]["criterion_id"], "ac1_1")
        self.assertTrue(guidance["current_handoff"]["handoff_file"].endswith("executor-recovery-ac1_1.md"))

    def test_passed_review_points_at_complete(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            spec = root / ".scafld" / "specs" / "active" / "task.md"
            data = basic_spec("task", status="in_progress", criterion_result="pass", phase_status="completed")
            review_state = {"exists": True, "verdict": "pass"}
            review_gate = {"gate_reason": None}

            guidance = derive_task_guidance(
                root,
                "task",
                spec,
                data,
                "in_progress",
                {},
                review_state=review_state,
                review_gate=review_gate,
            )

        self.assertEqual(guidance["next_action"]["type"], "complete")
        self.assertEqual(guidance["next_action"]["command"], "scafld complete task")

    def test_active_running_review_reports_stale_process(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            spec = root / ".scafld" / "specs" / "active" / "task.md"
            data = basic_spec("task", status="in_progress", criterion_result="pass", phase_status="completed")
            session = {
                "entries": [
                    {
                        "type": "provider_invocation",
                        "role": "challenger",
                        "gate": "review",
                        "status": "running",
                        "provider": "codex",
                        "pid": 999999,
                    }
                ]
            }
            with mock.patch("scafld.runtime_guidance.provider_invocation_process_alive", return_value=False):
                guidance = derive_task_guidance(root, "task", spec, data, "in_progress", session)

        self.assertEqual(guidance["next_action"]["type"], "review_stale")
        self.assertTrue(guidance["next_action"]["blocked"])
        self.assertEqual(guidance["block_reason"], "external review process not alive")


if __name__ == "__main__":
    unittest.main()
