import datetime
import tempfile
import unittest
from pathlib import Path

from scafld.errors import ScafldError
from scafld.spec_markdown import parse_spec_markdown
from scafld.spec_store import ARCHIVE_DIR, APPROVED_DIR, DRAFTS_DIR, find_all_specs, find_specs, move_spec, require_spec
from tests.spec_fixture import write_basic_spec


def write_spec(path, task_id, status="draft"):
    write_basic_spec(path, task_id, status=status, title="Fixture")


class SpecStoreTest(unittest.TestCase):
    def test_find_specs_lists_live_before_archive(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            write_spec(root / DRAFTS_DIR / "fixture.md", "fixture")
            write_spec(root / ARCHIVE_DIR / "2026-03" / "fixture.md", "fixture", status="completed")

            specs = find_specs(root, "fixture")
            self.assertEqual([path.relative_to(root).as_posix() for path in specs], [
                ".scafld/specs/drafts/fixture.md",
                ".scafld/specs/archive/2026-03/fixture.md",
            ])

    def test_require_spec_rejects_duplicates(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            write_spec(root / DRAFTS_DIR / "fixture.md", "fixture")
            write_spec(root / APPROVED_DIR / "fixture.md", "fixture", status="approved")

            with self.assertRaises(ScafldError) as context:
                require_spec(root, "fixture")

            self.assertIn("ambiguous task-id", context.exception.message)
            self.assertIn(".scafld/specs/drafts/fixture.md", "\n".join(context.exception.details))

    def test_move_spec_updates_metadata_and_destination(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            spec_path = root / DRAFTS_DIR / "fixture.md"
            write_spec(spec_path, "fixture")

            result = move_spec(root, spec_path, "approved")

            self.assertFalse(spec_path.exists())
            self.assertEqual(result.dest.relative_to(root).as_posix(), ".scafld/specs/approved/fixture.md")
            text = result.dest.read_text(encoding="utf-8")
            data = parse_spec_markdown(text)
            self.assertEqual(data["status"], "approved")
            self.assertEqual(data["planning_log"][-1]["summary"], "Spec approved")

    def test_find_all_specs_includes_archive_labels(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            write_spec(root / DRAFTS_DIR / "draft.md", "draft")
            month = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m")
            write_spec(root / ARCHIVE_DIR / month / "done.md", "done", status="completed")

            specs = [(path.relative_to(root).as_posix(), label) for path, label in find_all_specs(root)]
            self.assertIn((".scafld/specs/drafts/draft.md", "drafts"), specs)
            self.assertIn((f".scafld/specs/archive/{month}/done.md", f"archive/{month}"), specs)

    def test_markdown_parser_ignores_control_syntax_inside_fenced_code(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            spec_path = root / DRAFTS_DIR / "fixture.md"
            write_spec(spec_path, "fixture")
            text = spec_path.read_text(encoding="utf-8").replace(
                "## Summary\n\n",
                "## Summary\n\n```md\n## Phase 99: Fake phase\nAcceptance:\n- [ ] `fake` test\n```\n\n",
                1,
            )

            data = parse_spec_markdown(text)

            self.assertEqual([phase["id"] for phase in data["phases"]], ["phase1"])

if __name__ == "__main__":
    unittest.main()
