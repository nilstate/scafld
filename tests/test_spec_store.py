import datetime
import tempfile
import textwrap
import unittest
from pathlib import Path

from scafld.errors import ScafldError
from scafld.spec_store import ARCHIVE_DIR, APPROVED_DIR, DRAFTS_DIR, find_all_specs, find_specs, move_spec, require_spec


def write_spec(path, task_id, status="draft"):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        textwrap.dedent(
            f"""\
            spec_version: "1.1"
            task_id: "{task_id}"
            created: "2026-04-21T00:00:00Z"
            updated: "2026-04-21T00:00:00Z"
            status: "{status}"

            task:
              title: "Fixture"
              summary: >
                Fixture summary with enough text for the schema-like shape.
              size: "small"
              risk_level: "low"
              context:
                packages:
                  - "cli"
                invariants:
                  - "fixture"
              objectives:
                - "Exercise spec-store behavior."
              touchpoints:
                - area: "tests"
                  description: "Exercise spec-store behavior."
              acceptance:
                definition_of_done:
                  - id: "dod1"
                    description: "Fixture"
                    status: "pending"
                validation: []

            planning_log:
              - timestamp: "2026-04-21T00:00:00Z"
                actor: "test"
                summary: "Fixture created."

            phases:
              - id: "phase1"
                name: "Fixture"
                objective: "Exercise spec-store behavior"
                changes:
                  - file: "README.md"
                    action: "update"
                    content_spec: "Fixture"
                acceptance_criteria:
                  - id: "ac1_1"
                    type: "test"
                    description: "Fixture"
                    command: "true"
                    expected: "exit code 0"
                status: "pending"

            rollback:
              strategy: "per_phase"
              commands:
                phase1: "true"
            """
        ),
        encoding="utf-8",
    )


class SpecStoreTest(unittest.TestCase):
    def test_find_specs_lists_live_before_archive(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            write_spec(root / DRAFTS_DIR / "fixture.yaml", "fixture")
            write_spec(root / ARCHIVE_DIR / "2026-03" / "fixture.yaml", "fixture", status="completed")

            specs = find_specs(root, "fixture")
            self.assertEqual([path.relative_to(root).as_posix() for path in specs], [
                ".ai/specs/drafts/fixture.yaml",
                ".ai/specs/archive/2026-03/fixture.yaml",
            ])

    def test_require_spec_rejects_duplicates(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            write_spec(root / DRAFTS_DIR / "fixture.yaml", "fixture")
            write_spec(root / APPROVED_DIR / "fixture.yaml", "fixture", status="approved")

            with self.assertRaises(ScafldError) as context:
                require_spec(root, "fixture")

            self.assertIn("ambiguous task-id", context.exception.message)
            self.assertIn(".ai/specs/drafts/fixture.yaml", "\n".join(context.exception.details))

    def test_move_spec_updates_metadata_and_destination(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            spec_path = root / DRAFTS_DIR / "fixture.yaml"
            write_spec(spec_path, "fixture")

            result = move_spec(root, spec_path, "approved")

            self.assertFalse(spec_path.exists())
            self.assertEqual(result.dest.relative_to(root).as_posix(), ".ai/specs/approved/fixture.yaml")
            text = result.dest.read_text(encoding="utf-8")
            self.assertIn('status: "approved"', text)
            self.assertIn('summary: "Spec approved"', text)

    def test_find_all_specs_includes_archive_labels(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            write_spec(root / DRAFTS_DIR / "draft.yaml", "draft")
            month = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m")
            write_spec(root / ARCHIVE_DIR / month / "done.yaml", "done", status="completed")

            specs = [(path.relative_to(root).as_posix(), label) for path, label in find_all_specs(root)]
            self.assertIn((".ai/specs/drafts/draft.yaml", "drafts"), specs)
            self.assertIn((f".ai/specs/archive/{month}/done.yaml", f"archive/{month}"), specs)


if __name__ == "__main__":
    unittest.main()
