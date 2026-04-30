import re
import tempfile
import unittest
from pathlib import Path

from scafld.lifecycle_runtime import validate_spec, new_spec_snapshot
from scafld.spec_markdown import parse_spec_markdown
from scafld.spec_templates import build_slim_spec_scaffold
from tests.spec_fixture import write_basic_spec


def _workspace():
    tmp = Path(tempfile.mkdtemp(prefix="scafld-clean-plan-"))
    (tmp / ".scafld").mkdir()
    (tmp / ".scafld" / "core" / "schemas").mkdir(parents=True)
    # validate_spec needs the schema to exist; copy from repo.
    repo_root = Path(__file__).resolve().parent.parent
    (tmp / ".scafld" / "core" / "schemas" / "spec.json").write_text(
        (repo_root / ".scafld" / "core" / "schemas" / "spec.json").read_text(encoding="utf-8"),
        encoding="utf-8",
    )
    (tmp / ".scafld" / "specs").mkdir()
    (tmp / ".scafld" / "specs" / "drafts").mkdir()
    (tmp / ".scafld" / "specs" / "active").mkdir()
    (tmp / ".scafld" / "specs" / "archive").mkdir()
    (tmp / ".scafld" / "config.yaml").write_text("version: '1.0'\n", encoding="utf-8")
    return tmp


class SlimScaffoldShapeTest(unittest.TestCase):
    def test_slim_still_uses_complete_markdown_grammar(self):
        root = _workspace()
        scaffold = build_slim_spec_scaffold(
            root,
            "slim-line-budget",
            timestamp="2026-04-28T00:00:00Z",
            title="Slim line budget check",
            command="pytest tests/foo.py",
            files=["tests/foo.py", "scafld/foo.py"],
        )
        line_count = sum(1 for _ in scaffold["text"].splitlines())
        self.assertGreater(line_count, 100)
        self.assertIn("## Current State", scaffold["text"])
        self.assertIn("## Phase 1: Slim line budget check", scaffold["text"])
        self.assertIn("Acceptance:\n- [ ] `ac1_1`", scaffold["text"])

    def test_no_todo_markers(self):
        # validate_spec rejects TODO markers; the slim scaffold must
        # produce a spec that passes validation immediately.
        root = _workspace()
        scaffold = build_slim_spec_scaffold(
            root,
            "slim-no-todo",
            timestamp="2026-04-28T00:00:00Z",
            title="Slim no TODO",
            command="true",
            files=["app.py"],
        )
        text = scaffold["text"]
        self.assertNotRegex(text, r'^\s+(?:command|content_spec|description|file):\s*"?TODO')
        self.assertNotRegex(text, r'^\s*-\s+"?TODO')

    def test_explicit_expected_kind(self):
        root = _workspace()
        scaffold = build_slim_spec_scaffold(
            root,
            "slim-kind",
            timestamp="2026-04-28T00:00:00Z",
            title="Slim kind",
            command="true",
            files=["app.py"],
        )
        self.assertIn("Expected kind: `exit_code_zero`", scaffold["text"])

    def test_command_inserted_verbatim(self):
        root = _workspace()
        scaffold = build_slim_spec_scaffold(
            root,
            "slim-command",
            timestamp="2026-04-28T00:00:00Z",
            title="Slim command",
            command="grep -q 'baseline' app.txt",
            files=["app.txt"],
        )
        self.assertIn("grep -q 'baseline' app.txt", scaffold["text"])

    def test_one_change_per_file(self):
        root = _workspace()
        scaffold = build_slim_spec_scaffold(
            root,
            "slim-multi-file",
            timestamp="2026-04-28T00:00:00Z",
            title="Slim multi-file",
            command="true",
            files=["a.py", "b.py", "c.py"],
        )
        text = scaffold["text"]
        self.assertEqual(text.count(") - See task summary."), 6)
        for path in ("a.py", "b.py", "c.py"):
            self.assertIn(f"`{path}`", text)

    def test_default_size_and_risk(self):
        root = _workspace()
        scaffold = build_slim_spec_scaffold(
            root,
            "slim-defaults",
            timestamp="2026-04-28T00:00:00Z",
            title="Defaults",
            command="true",
            files=["app.py"],
        )
        self.assertEqual(scaffold["size"], "small")
        self.assertEqual(scaffold["risk"], "low")
        self.assertIn("size: small", scaffold["text"])
        self.assertIn("risk_level: low", scaffold["text"])

    def test_requires_command(self):
        root = _workspace()
        with self.assertRaises(ValueError):
            build_slim_spec_scaffold(
                root,
                "slim-no-cmd",
                timestamp="2026-04-28T00:00:00Z",
                title="Slim",
                command="",
                files=["app.py"],
            )

    def test_requires_files(self):
        root = _workspace()
        with self.assertRaises(ValueError):
            build_slim_spec_scaffold(
                root,
                "slim-no-files",
                timestamp="2026-04-28T00:00:00Z",
                title="Slim",
                command="true",
                files=[],
            )


class NewSpecSnapshotRoutingTest(unittest.TestCase):
    def test_command_routes_to_slim(self):
        root = _workspace()
        result = new_spec_snapshot(
            root,
            "slim-route",
            title="Slim route",
            command="pytest",
            files="a.py,b.py",
        )
        spec_path = root / result["state"]["file"]
        text = spec_path.read_text(encoding="utf-8")
        self.assertIn("scafld plan --command", text)  # planning_log marker
        self.assertIn("Expected kind: `exit_code_zero`", text)

    def test_files_string_splits_on_comma(self):
        root = _workspace()
        result = new_spec_snapshot(
            root,
            "slim-comma",
            title="Slim comma",
            command="pytest",
            files="a.py, b.py , c.py",
        )
        spec_path = root / result["state"]["file"]
        text = spec_path.read_text(encoding="utf-8")
        for path in ("a.py", "b.py", "c.py"):
            self.assertIn(f"`{path}`", text)

    def test_no_command_falls_back_to_verbose(self):
        root = _workspace()
        result = new_spec_snapshot(
            root,
            "verbose-route",
            title="Verbose",
        )
        spec_path = root / result["state"]["file"]
        text = spec_path.read_text(encoding="utf-8")
        # Verbose path emits TODO scaffolding; slim path doesn't.
        self.assertIn("TODO", text)


try:
    import yaml as _yaml  # noqa: F401
    _HAS_PYYAML = True
except ImportError:
    _HAS_PYYAML = False


@unittest.skipUnless(_HAS_PYYAML, "validate_spec requires PyYAML to load the spec document")
class ValidateSlimSpecTest(unittest.TestCase):
    def test_slim_spec_passes_validate_spec(self):
        root = _workspace()
        result = new_spec_snapshot(
            root,
            "slim-validates",
            title="Slim validates",
            command="pytest",
            files="app.py",
        )
        spec_path = root / result["state"]["file"]
        errors = validate_spec(root, spec_path)
        self.assertEqual(errors, [], f"validate_spec rejected the slim scaffold: {errors}")


def _write_active_spec_with_change(root, task_id, file_path, ownership=None):
    """Drop a minimal active spec that declares one file change.

    Used by plan-time conflict tests: the new spec under test will
    overlap this file. ownership None means exclusive (default).
    """
    write_basic_spec(
        root / ".scafld" / "specs" / "active" / f"{task_id}.md",
        task_id,
        status="in_progress",
        title="Other active spec",
        file_path=file_path,
        ownership=ownership,
    )


@unittest.skipUnless(_HAS_PYYAML, "active_declared_changes loads Markdown spec front matter; requires PyYAML")
class PlanTimeConflictTest(unittest.TestCase):
    def test_plan_auto_tags_shared_when_other_active_overlaps(self):
        from scafld.errors import ScafldError
        root = _workspace()
        _write_active_spec_with_change(root, "other-shared", "tests/foo.py", ownership="shared")
        result = new_spec_snapshot(
            root,
            "slim-shared-overlap",
            title="Slim shared",
            command="pytest tests/foo.py",
            files="tests/foo.py",
        )
        spec_path = root / result["state"]["file"]
        text = spec_path.read_text(encoding="utf-8")
        # The slim spec's change entry for tests/foo.py should now
        # carry shared ownership (auto-tagged).
        self.assertIn("`tests/foo.py` (all, shared)", text)

    def test_plan_refuses_on_exclusive_conflict(self):
        from scafld.errors import ScafldError
        root = _workspace()
        _write_active_spec_with_change(root, "other-exclusive", "scafld/foo.py")
        with self.assertRaises(ScafldError) as ctx:
            new_spec_snapshot(
                root,
                "slim-exclusive-overlap",
                title="Slim exclusive",
                command="pytest",
                files="scafld/foo.py",
            )
        self.assertIn("exclusively owned", str(ctx.exception))
        # Refused plan must NOT leave a half-baked spec on disk.
        leaked = (root / ".scafld" / "specs" / "drafts" / "slim-exclusive-overlap.md").exists()
        self.assertFalse(leaked, "plan that refuses on conflict must not write the spec file")

    def test_plan_passes_when_no_other_active_overlaps(self):
        root = _workspace()
        # Other active spec touches a different file — no overlap.
        _write_active_spec_with_change(root, "other-different", "scafld/bar.py")
        result = new_spec_snapshot(
            root,
            "slim-no-overlap",
            title="Slim no overlap",
            command="pytest",
            files="scafld/foo.py",
        )
        spec_path = root / result["state"]["file"]
        self.assertTrue(spec_path.exists())


class ApplySharedOwnershipTest(unittest.TestCase):
    def test_rewrites_matching_file_entry(self):
        from scafld.audit_scope import apply_shared_ownership
        root = _workspace()
        path = root / ".scafld" / "specs" / "active" / "x.md"
        write_basic_spec(path, "x", status="in_progress", file_path="a.py")
        text = path.read_text(encoding="utf-8")
        result = apply_shared_ownership(text, {"a.py"})
        data = parse_spec_markdown(result)
        self.assertEqual(data["phases"][0]["changes"][0]["ownership"], "shared")

    def test_idempotent_when_already_shared(self):
        from scafld.audit_scope import apply_shared_ownership
        root = _workspace()
        path = root / ".scafld" / "specs" / "active" / "x.md"
        write_basic_spec(path, "x", status="in_progress", file_path="a.py", ownership="shared")
        text = path.read_text(encoding="utf-8")
        result = apply_shared_ownership(text, {"a.py"})
        self.assertEqual(result, text)

    def test_skips_non_target_files(self):
        from scafld.audit_scope import apply_shared_ownership
        root = _workspace()
        path = root / ".scafld" / "specs" / "active" / "x.md"
        write_basic_spec(path, "x", status="in_progress", file_path="b.py")
        text = path.read_text(encoding="utf-8")
        result = apply_shared_ownership(text, {"a.py"})
        self.assertEqual(result, text)


if __name__ == "__main__":
    unittest.main()
