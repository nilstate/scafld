import unittest

from scafld.spec_markdown import render_spec_markdown
from scafld.spec_reconcile import projection_matches, rebuild_spec_from_session
from tests.spec_fixture import basic_spec


class SpecReconcileTests(unittest.TestCase):
    def test_rebuild_projects_session_state_without_losing_prose(self):
        spec_text = render_spec_markdown(
            basic_spec(
                "reconcile-task",
                status="in_progress",
                title="Reconcile Task",
                file_path="app.txt",
                command="true",
            )
        )
        spec_text = spec_text.replace("Fixture summary.", "Human-owned prose survives.")
        session = {
            "entries": [{"type": "attempt", "recorded_at": "2026-04-30T00:00:00Z"}],
            "criterion_states": {
                "ac1_1": {
                    "status": "pass",
                    "phase_id": "phase1",
                    "reason": "session-backed pass",
                }
            },
            "phase_blocks": {"phase1": {"status": "completed"}},
        }

        rebuilt = rebuild_spec_from_session(spec_text, session)

        self.assertIn("Human-owned prose survives.", rebuilt)
        self.assertIn("Latest runner update: 2026-04-30T00:00:00Z", rebuilt)
        self.assertIn("Status: completed", rebuilt)
        self.assertIn("- [x] `ac1_1`", rebuilt)
        self.assertIn("Result: pass", rebuilt)
        self.assertFalse(projection_matches(spec_text, session))
        self.assertTrue(projection_matches(rebuilt, session))

    def test_golden_reconcile_output_is_stable(self):
        spec_text = render_spec_markdown(
            basic_spec(
                "golden-reconcile",
                status="in_progress",
                title="Golden Reconcile",
                file_path="app.txt",
                command="true",
            )
        )
        session = {
            "entries": [{"type": "attempt", "recorded_at": "2026-04-30T00:00:00Z"}],
            "criterion_states": {"ac1_1": {"status": "pass", "phase_id": "phase1"}},
            "phase_blocks": {"phase1": {"status": "completed"}},
        }

        rebuilt = rebuild_spec_from_session(spec_text, session)

        self.assertIn("Status: completed", rebuilt)
        self.assertIn("- [x] `ac1_1` test - Run the fixture command.", rebuilt)
        self.assertIn("Latest runner update: 2026-04-30T00:00:00Z", rebuilt)


if __name__ == "__main__":
    unittest.main()
