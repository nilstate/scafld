import os
import subprocess
import tempfile
import unittest
from pathlib import Path

from scafld.audit_scope import git_sync_excluded_paths
from scafld.git_state import capture_review_git_state
from scafld.review_workflow import review_binding_excluded_rels


class ReviewBindingExcludedRelsTest(unittest.TestCase):
    def test_includes_per_task_review_artifact(self):
        excluded = review_binding_excluded_rels("my-task", ".scafld/reviews/my-task.md")
        self.assertIn(".scafld/reviews/my-task.md", excluded)
        self.assertIn(".scafld/runs/my-task", excluded)

    def test_includes_scafld_control_plane_prefixes(self):
        excluded = review_binding_excluded_rels("my-task", ".scafld/reviews/my-task.md")
        for prefix in git_sync_excluded_paths():
            self.assertIn(prefix, excluded, f"control-plane prefix {prefix!r} must be excluded from review binding")

    def test_no_duplicates(self):
        excluded = review_binding_excluded_rels("my-task", ".scafld/reviews/my-task.md")
        self.assertEqual(len(excluded), len(set(excluded)))

    def test_handles_empty_review_file_rel(self):
        excluded = review_binding_excluded_rels("my-task", "")
        self.assertNotIn("", excluded)
        self.assertIn(".scafld/runs/my-task", excluded)
        for prefix in git_sync_excluded_paths():
            self.assertIn(prefix, excluded)


def _git_init(root):
    env = {**os.environ, "GIT_AUTHOR_NAME": "test", "GIT_AUTHOR_EMAIL": "test@test", "GIT_COMMITTER_NAME": "test", "GIT_COMMITTER_EMAIL": "test@test"}
    subprocess.run(["git", "init", "-q", str(root)], check=True, env=env)
    subprocess.run(["git", "-C", str(root), "config", "user.name", "test"], check=True)
    subprocess.run(["git", "-C", str(root), "config", "user.email", "test@test"], check=True)
    (root / "README.md").write_text("hello\n")
    subprocess.run(["git", "-C", str(root), "add", "."], check=True, env=env)
    subprocess.run(["git", "-C", str(root), "commit", "-q", "-m", "init"], check=True, env=env)


class ReviewAnchorEngineeringDiffOnlyTest(unittest.TestCase):
    def test_archiving_one_spec_does_not_change_anchor_for_another(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _git_init(root)

            (root / ".scafld" / "specs" / "active").mkdir(parents=True)
            (root / ".scafld" / "specs" / "archive" / "2026-04").mkdir(parents=True)
            (root / ".scafld" / "runs" / "spec-a").mkdir(parents=True)
            (root / ".scafld" / "specs" / "active" / "spec-a.md").write_text("active: true\n")
            (root / ".scafld" / "runs" / "spec-a" / "session.json").write_text("{}\n")

            excluded = review_binding_excluded_rels("spec-b", ".scafld/reviews/spec-b.md")
            before, _ = capture_review_git_state(root, excluded)
            self.assertIsNotNone(before)

            archive_dest = root / ".scafld" / "specs" / "archive" / "2026-04" / "spec-a.md"
            (root / ".scafld" / "specs" / "active" / "spec-a.md").rename(archive_dest)

            after, _ = capture_review_git_state(root, excluded)
            self.assertEqual(before["reviewed_diff"], after["reviewed_diff"])

    def test_engineering_change_outside_dotai_invalidates_anchor(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _git_init(root)

            excluded = review_binding_excluded_rels("spec-b", ".scafld/reviews/spec-b.md")
            before, _ = capture_review_git_state(root, excluded)
            self.assertIsNotNone(before)

            (root / "README.md").write_text("hello\nadded line\n")

            after, _ = capture_review_git_state(root, excluded)
            self.assertNotEqual(before["reviewed_diff"], after["reviewed_diff"])


if __name__ == "__main__":
    unittest.main()
