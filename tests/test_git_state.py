import subprocess
import tempfile
import unittest
from pathlib import Path

from scafld.git_state import (
    bind_task_branch,
    build_origin_sync_payload,
    capture_review_git_state,
    capture_workspace_git_state,
    list_changed_files_against_ref,
    list_working_tree_changed_files,
    refresh_origin_sync,
)


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
    git(root, "init", "-b", "main")
    git(root, "config", "user.name", "Scafld Tests")
    git(root, "config", "user.email", "tests@example.com")
    (root / "tracked.txt").write_text("seed\n", encoding="utf-8")
    git(root, "add", "tracked.txt")
    git(root, "commit", "-m", "chore: seed repo")


def init_superproject_with_submodule(root):
    source = root / "source"
    superproject = root / "super"
    source.mkdir()
    superproject.mkdir()
    init_repo(source)
    init_repo(superproject)
    git(
        superproject,
        "-c",
        "protocol.file.allow=always",
        "submodule",
        "add",
        str(source),
        "modules/source",
    )
    git(superproject, "add", ".gitmodules", "modules/source")
    git(superproject, "commit", "-m", "chore: add submodule")
    return superproject, superproject / "modules" / "source"


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

    def test_list_working_tree_changed_files_expands_dirty_submodules(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            superproject, submodule = init_superproject_with_submodule(root)

            (submodule / "tracked.txt").write_text("updated inside submodule\n", encoding="utf-8")

            changed, error = list_working_tree_changed_files(superproject)

            self.assertIsNone(error)
            self.assertEqual(set(changed), {"modules/source/tracked.txt"})

    def test_capture_review_git_state_fingerprints_dirty_submodule_contents(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            superproject, submodule = init_superproject_with_submodule(root)

            (submodule / "tracked.txt").write_text("dirty one\n", encoding="utf-8")
            before, error = capture_review_git_state(superproject)
            self.assertIsNone(error)

            (submodule / "tracked.txt").write_text("dirty two\n", encoding="utf-8")
            after, error = capture_review_git_state(superproject)

            self.assertIsNone(error)
            self.assertTrue(before["reviewed_dirty"])
            self.assertTrue(after["reviewed_dirty"])
            self.assertNotEqual(before["reviewed_diff"], after["reviewed_diff"])

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

    def test_capture_workspace_git_state_reports_branch_and_default_base(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            init_repo(root)

            state, error = capture_workspace_git_state(root)

            self.assertIsNone(error)
            self.assertEqual(state["branch"], "main")
            self.assertEqual(state["default_base_ref"], "main")
            self.assertFalse(state["dirty"])
            self.assertFalse(state["detached"])

    def test_build_origin_sync_payload_ignores_scafld_control_plane_files(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            init_repo(root)

            spec_file = root / ".ai" / "specs" / "active" / "fixture.yaml"
            spec_file.parent.mkdir(parents=True, exist_ok=True)
            spec_file.write_text("status: in_progress\n", encoding="utf-8")

            sync = build_origin_sync_payload(
                root,
                {"git": {"branch": "main"}},
                excluded_rels=[".ai/specs/", ".ai/reviews/", ".ai/runs/", ".ai/config.local.yaml"],
            )

            self.assertEqual(sync["status"], "in_sync")
            self.assertEqual(sync["reasons"], [])

    def test_build_origin_sync_payload_detects_branch_mismatch_and_dirty_workspace(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            init_repo(root)
            (root / "tracked.txt").write_text("updated\n", encoding="utf-8")

            sync = build_origin_sync_payload(root, {"git": {"branch": "feature-task"}})

            self.assertEqual(sync["status"], "drift")
            self.assertIn("workspace has uncommitted changes", sync["reasons"])
            self.assertTrue(any("expected feature-task" in reason for reason in sync["reasons"]))

    def test_refresh_origin_sync_persists_checked_snapshot(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            init_repo(root)
            (root / "tracked.txt").write_text("updated\n", encoding="utf-8")

            origin, sync = refresh_origin_sync(
                root,
                {"git": {"branch": "main"}},
                checked_at="2026-04-21T12:00:00Z",
            )

            self.assertEqual(sync["status"], "drift")
            self.assertEqual(origin["sync"]["status"], "drift")
            self.assertEqual(origin["sync"]["last_checked_at"], "2026-04-21T12:00:00Z")
            self.assertIn("workspace has uncommitted changes", origin["sync"]["reasons"])

    def test_bind_task_branch_creates_branch_and_records_origin(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            init_repo(root)

            result = bind_task_branch(
                root,
                "feature-task",
                {},
                bound_at="2026-04-21T12:00:00Z",
            )

            self.assertEqual(result["action"], "created_branch")
            self.assertEqual(result["branch"], "feature-task")
            self.assertEqual(result["origin"]["git"]["branch"], "feature-task")
            self.assertEqual(result["origin"]["git"]["mode"], "created_branch")
            self.assertEqual(result["origin"]["sync"]["status"], "in_sync")
            self.assertEqual(result["sync"]["status"], "in_sync")
            self.assertEqual(git(root, "branch", "--show-current").stdout.strip(), "feature-task")


if __name__ == "__main__":
    unittest.main()
