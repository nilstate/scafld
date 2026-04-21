import subprocess
import tempfile
import unittest
from pathlib import Path

from scafld.git_state import list_changed_files_against_ref, list_working_tree_changed_files


def git(root, *args):
    result = subprocess.run(
        ["git", "-C", str(root), *args],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        raise AssertionError(result.stderr.strip() or result.stdout.strip() or f"git {' '.join(args)} failed")
    return result


def init_repo(root):
    git(root, "init")
    git(root, "config", "user.name", "Scafld Tests")
    git(root, "config", "user.email", "tests@example.com")
    (root / "tracked.txt").write_text("seed\n", encoding="utf-8")
    git(root, "add", "tracked.txt")
    git(root, "commit", "-m", "chore: seed repo")


class GitStateTest(unittest.TestCase):
    def test_list_working_tree_changed_files_includes_unstaged_staged_and_untracked(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            init_repo(root)

            (root / "tracked.txt").write_text("updated\n", encoding="utf-8")
            (root / "staged.txt").write_text("staged\n", encoding="utf-8")
            git(root, "add", "staged.txt")
            (root / "untracked.txt").write_text("untracked\n", encoding="utf-8")

            changed, error = list_working_tree_changed_files(root)

            self.assertIsNone(error)
            self.assertEqual(set(changed), {"tracked.txt", "staged.txt", "untracked.txt"})

    def test_list_changed_files_against_ref_keeps_untracked_files_visible(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            init_repo(root)

            (root / "tracked.txt").write_text("updated\n", encoding="utf-8")
            (root / "untracked.txt").write_text("untracked\n", encoding="utf-8")

            changed, error = list_changed_files_against_ref(root, "HEAD")

            self.assertIsNone(error)
            self.assertEqual(set(changed), {"tracked.txt", "untracked.txt"})

    def test_list_working_tree_changed_files_honors_excluded_paths(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            init_repo(root)

            ignored = root / ".ai" / "reviews" / "fixture.md"
            ignored.parent.mkdir(parents=True, exist_ok=True)
            ignored.write_text("review\n", encoding="utf-8")
            (root / "tracked.txt").write_text("updated\n", encoding="utf-8")

            changed, error = list_working_tree_changed_files(root, excluded_rels=[".ai/reviews/fixture.md"])

            self.assertIsNone(error)
            self.assertEqual(set(changed), {"tracked.txt"})


if __name__ == "__main__":
    unittest.main()
