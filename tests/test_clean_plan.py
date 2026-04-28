import re
import tempfile
import unittest
from pathlib import Path

from scafld.lifecycle_runtime import validate_spec, new_spec_snapshot
from scafld.spec_templates import build_slim_spec_scaffold


def _workspace():
    tmp = Path(tempfile.mkdtemp(prefix="scafld-clean-plan-"))
    (tmp / ".ai").mkdir()
    (tmp / ".ai" / "schemas").mkdir()
    # validate_spec needs the schema to exist; copy from repo.
    repo_root = Path(__file__).resolve().parent.parent
    (tmp / ".ai" / "schemas" / "spec.json").write_text(
        (repo_root / ".ai" / "schemas" / "spec.json").read_text(encoding="utf-8"),
        encoding="utf-8",
    )
    (tmp / ".ai" / "specs").mkdir()
    (tmp / ".ai" / "specs" / "drafts").mkdir()
    (tmp / ".ai" / "specs" / "active").mkdir()
    (tmp / ".ai" / "specs" / "archive").mkdir()
    (tmp / ".ai" / "config.yaml").write_text("version: '1.0'\n", encoding="utf-8")
    return tmp


class SlimScaffoldShapeTest(unittest.TestCase):
    def test_under_40_lines(self):
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
        self.assertLess(line_count, 40, f"slim scaffold is {line_count} lines, expected < 40")

    def test_no_todo_sentinels(self):
        # validate_spec rejects TODO sentinels; the slim scaffold must
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
        self.assertIn('expected_kind: "exit_code_zero"', scaffold["text"])

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
        self.assertEqual(text.count("- file:"), 3)
        for path in ("a.py", "b.py", "c.py"):
            self.assertIn(f'"{path}"', text)

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
        self.assertIn('size: "small"', scaffold["text"])
        self.assertIn('risk_level: "low"', scaffold["text"])

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
        self.assertIn('expected_kind: "exit_code_zero"', text)

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
            self.assertIn(f'"{path}"', text)

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
    ownership_line = f'        ownership: "{ownership}"\n' if ownership else ""
    text = (
        f'spec_version: "1.1"\n'
        f'task_id: "{task_id}"\n'
        f'created: "2026-04-28T00:00:00Z"\n'
        f'updated: "2026-04-28T00:00:00Z"\n'
        f'status: "in_progress"\n'
        f'task:\n'
        f'  title: "Other active spec"\n'
        f'  summary: "Other"\n'
        f'  size: "small"\n'
        f'  risk_level: "low"\n'
        f'planning_log:\n'
        f'  - timestamp: "2026-04-28T00:00:00Z"\n'
        f'    actor: "user"\n'
        f'    summary: "fixture"\n'
        f'phases:\n'
        f'  - id: "phase1"\n'
        f'    name: "Phase 1"\n'
        f'    objective: "Phase 1"\n'
        f'    changes:\n'
        f'      - file: "{file_path}"\n'
        f'        action: "update"\n'
        f'{ownership_line}'
        f'        content_spec: "noop"\n'
        f'    acceptance_criteria:\n'
        f'      - id: "ac1_1"\n'
        f'        type: "test"\n'
        f'        command: "true"\n'
        f'        expected_kind: "exit_code_zero"\n'
        f'    status: "pending"\n'
    )
    (root / ".ai" / "specs" / "active" / f"{task_id}.yaml").write_text(text, encoding="utf-8")


@unittest.skipUnless(_HAS_PYYAML, "active_declared_changes loads spec yaml; requires PyYAML")
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
        # carry ownership: "shared" (auto-tagged).
        self.assertIn('ownership: shared', text)

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
        leaked = (root / ".ai" / "specs" / "drafts" / "slim-exclusive-overlap.yaml").exists()
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
    """Pure-text rewriter; doesn't need PyYAML."""

    def test_rewrites_matching_file_entry(self):
        from scafld.audit_scope import apply_shared_ownership
        text = (
            "phases:\n"
            "  - id: phase1\n"
            "    changes:\n"
            "      - file: \"a.py\"\n"
            "        action: \"update\"\n"
            "        content_spec: \"noop\"\n"
        )
        result = apply_shared_ownership(text, {"a.py"})
        self.assertIn('ownership: shared', result)
        # Sibling fields preserved.
        self.assertIn("action: update", result)
        self.assertIn("content_spec: noop", result)

    def test_idempotent_when_already_shared(self):
        from scafld.audit_scope import apply_shared_ownership
        text = (
            "phases:\n"
            "  - id: phase1\n"
            "    changes:\n"
            "      - file: \"a.py\"\n"
            "        action: \"update\"\n"
            "        ownership: \"shared\"\n"
            "        content_spec: \"noop\"\n"
        )
        result = apply_shared_ownership(text, {"a.py"})
        # Should NOT inject a second ownership line. Count any form
        # ('ownership: "shared"' or unquoted) — the early-return path
        # preserves the input formatting; the round-trip path emits
        # the unquoted form.
        ownership_lines = result.count("ownership:")
        self.assertEqual(ownership_lines, 1)

    def test_skips_non_target_files(self):
        from scafld.audit_scope import apply_shared_ownership
        text = (
            "phases:\n"
            "  - id: phase1\n"
            "    changes:\n"
            "      - file: \"a.py\"\n"
            "        action: \"update\"\n"
            "        content_spec: \"noop\"\n"
            "      - file: \"b.py\"\n"
            "        action: \"update\"\n"
            "        content_spec: \"noop\"\n"
        )
        result = apply_shared_ownership(text, {"a.py"})
        # ownership added once, on a.py only
        self.assertEqual(result.count('ownership: shared'), 1)


if __name__ == "__main__":
    unittest.main()
