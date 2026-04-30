import unittest

from scafld.spec_markdown import parse_spec_markdown, render_spec_markdown
from scafld.audit_scope import (
    AUDIT_IGNORED_PREFIXES,
    collect_declared_change_map,
    filter_audit_paths,
    normalize_change_path,
)
from tests.spec_fixture import basic_spec


def spec_text_with_changes(changes):
    data = basic_spec("x", title="Fixture", file_path="placeholder.py")
    data["phases"][0]["changes"] = changes
    return render_spec_markdown(data)


class NormalizeChangePathTest(unittest.TestCase):
    def test_passthrough_for_simple_path(self):
        self.assertEqual(normalize_change_path("scafld/foo.py"), "scafld/foo.py")

    def test_dot_prefix_collapsed(self):
        self.assertEqual(normalize_change_path("./scafld/foo.py"), "scafld/foo.py")

    def test_double_slash_collapsed(self):
        self.assertEqual(normalize_change_path("scafld//foo.py"), "scafld/foo.py")

    def test_trailing_slash_collapsed(self):
        self.assertEqual(normalize_change_path("scafld/"), "scafld")

    def test_backslash_coerced_to_posix(self):
        self.assertEqual(normalize_change_path("scafld\\foo.py"), "scafld/foo.py")

    def test_parent_prefix_preserved_for_sibling_repos(self):
        # ../cloud/foo.py is the multi-repo dogfood case; we must not
        # resolve it to an absolute or repo-rooted path because the
        # spec author meant a sibling-repo path.
        self.assertEqual(
            normalize_change_path("../cloud/foo.py"),
            "../cloud/foo.py",
        )

    def test_empty_returns_empty_string(self):
        self.assertEqual(normalize_change_path(""), "")
        self.assertEqual(normalize_change_path("   "), "")
        self.assertEqual(normalize_change_path("."), "")

    def test_non_string_returns_empty_string(self):
        self.assertEqual(normalize_change_path(None), "")
        self.assertEqual(normalize_change_path(42), "")

    def test_idempotent(self):
        first = normalize_change_path("./scafld//foo.py")
        second = normalize_change_path(first)
        self.assertEqual(first, second)


class CollectDeclaredChangeMapTest(unittest.TestCase):
    def test_canonicalizes_declared_paths(self):
        text = spec_text_with_changes([
            {"file": "./scafld//foo.py", "action": "update", "content_spec": "Fixture"},
            {"file": "../cloud/bar.py", "action": "update", "content_spec": "Fixture"},
            {"file": "scafld/baz.py", "action": "update", "content_spec": "Fixture"},
        ])
        declared = collect_declared_change_map(text)
        self.assertIn("scafld/foo.py", declared)
        self.assertIn("../cloud/bar.py", declared)
        self.assertIn("scafld/baz.py", declared)

    def test_drops_empty_or_dot_paths(self):
        text = spec_text_with_changes([
            {"file": ".", "action": "update", "content_spec": "Fixture"},
            {"file": "", "action": "update", "content_spec": "Fixture"},
            {"file": "scafld/foo.py", "action": "update", "content_spec": "Fixture"},
        ])
        declared = collect_declared_change_map(text)
        self.assertEqual(list(declared), ["scafld/foo.py"])


class FilterAuditPathsTest(unittest.TestCase):
    def test_normalizes_actual_paths(self):
        # git porcelain paths are already canonical, but defensive
        # against a caller passing slashes differently. Declared and
        # actual sides must end up in the same canonical form.
        paths = {"./scafld/foo.py", "scafld/bar.py", "scafld//baz.py"}
        out = filter_audit_paths(paths)
        self.assertEqual(out, {"scafld/foo.py", "scafld/bar.py", "scafld/baz.py"})

    def test_drops_audit_ignored_prefixes(self):
        for prefix in AUDIT_IGNORED_PREFIXES:
            with self.subTest(prefix=prefix):
                paths = {f"{prefix}thing.txt", "scafld/foo.py"}
                self.assertEqual(filter_audit_paths(paths), {"scafld/foo.py"})

    def test_preserves_sibling_repo_paths(self):
        paths = {"../cloud/foo.py", "scafld/foo.py"}
        out = filter_audit_paths(paths)
        self.assertEqual(out, {"../cloud/foo.py", "scafld/foo.py"})

    def test_drops_empty_strings(self):
        paths = {"", "  ", "scafld/foo.py"}
        self.assertEqual(filter_audit_paths(paths), {"scafld/foo.py"})


class DeclaredAndActualSymmetryTest(unittest.TestCase):
    """The whole point of normalization: declared and actual sides
    classify as `matched` when an operator wrote a non-canonical form
    in the spec but git emits the canonical form (or vice versa)."""

    def test_declared_dot_prefix_matches_canonical_actual(self):
        text = spec_text_with_changes([{"file": "./scafld/foo.py", "action": "update", "content_spec": "Fixture"}])
        declared = collect_declared_change_map(text)
        actual = filter_audit_paths({"scafld/foo.py"})
        # Both sides agree on canonical form.
        self.assertEqual(set(declared.keys()) & actual, {"scafld/foo.py"})

    def test_declared_double_slash_matches_canonical_actual(self):
        text = spec_text_with_changes([{"file": "scafld//foo.py", "action": "update", "content_spec": "Fixture"}])
        declared = collect_declared_change_map(text)
        actual = filter_audit_paths({"scafld/foo.py"})
        self.assertEqual(set(declared.keys()) & actual, {"scafld/foo.py"})

    def test_sibling_repo_declared_matches_sibling_actual(self):
        text = spec_text_with_changes([{"file": "../cloud/foo.py", "action": "update", "content_spec": "Fixture"}])
        declared = collect_declared_change_map(text)
        # Simulate git emitting the same path (e.g. via worktree or
        # parent-dir invocation).
        actual = filter_audit_paths({"../cloud/foo.py"})
        self.assertEqual(set(declared.keys()) & actual, {"../cloud/foo.py"})


class ClassifyActiveOverlapTest(unittest.TestCase):
    """Plan-time and review-time overlap classification share one policy.

    `plan_time=False` (default, review-time) keeps the bilateral
    semantics: shared only when BOTH sides declare shared.
    `plan_time=True` is unilateral against other specs: shared when
    every overlapping other spec declares shared (caller auto-tags
    this spec); conflict when any other spec declares exclusive.
    """

    def _other(self, *, owner, ownership):
        return {owner: {"path/foo.py": ownership}}

    def test_review_time_bilateral_shared(self):
        from scafld.audit_scope import classify_active_overlap
        result = classify_active_overlap(
            {"path/foo.py": "shared"},
            self._other(owner="other", ownership="shared"),
        )
        self.assertEqual(result["shared_with_other_active"], {"path/foo.py"})
        self.assertEqual(result["active_overlap"], set())

    def test_review_time_bilateral_conflict_when_we_are_exclusive(self):
        from scafld.audit_scope import classify_active_overlap
        result = classify_active_overlap(
            {"path/foo.py": "exclusive"},
            self._other(owner="other", ownership="shared"),
        )
        self.assertEqual(result["active_overlap"], {"path/foo.py"})

    def test_plan_time_auto_tags_shared_when_other_is_shared(self):
        from scafld.audit_scope import classify_active_overlap
        # Our ownership is "exclusive" (the default for unspecified
        # slim spec changes). Plan-time should still classify the
        # path as shared because the OTHER spec declared shared —
        # the caller will auto-tag this spec to match.
        result = classify_active_overlap(
            {"path/foo.py": "exclusive"},
            self._other(owner="other", ownership="shared"),
            plan_time=True,
        )
        self.assertEqual(result["shared_with_other_active"], {"path/foo.py"})
        self.assertEqual(result["active_overlap"], set())

    def test_plan_time_refuses_when_other_is_exclusive(self):
        from scafld.audit_scope import classify_active_overlap
        result = classify_active_overlap(
            {"path/foo.py": "shared"},
            self._other(owner="other", ownership="exclusive"),
            plan_time=True,
        )
        self.assertEqual(result["active_overlap"], {"path/foo.py"})
        self.assertEqual(result["conflict_details"]["path/foo.py"], ["other"])

    def test_plan_time_no_overlap_yields_empty_sets(self):
        from scafld.audit_scope import classify_active_overlap
        result = classify_active_overlap(
            {"path/foo.py": "exclusive"},
            self._other(owner="other", ownership="exclusive"),
        )
        # path/foo.py is only IN the other spec for this fixture;
        # mismatched key, so no overlap.
        result = classify_active_overlap(
            {"different/path.py": "exclusive"},
            self._other(owner="other", ownership="exclusive"),
            plan_time=True,
        )
        self.assertEqual(result["shared_with_other_active"], set())
        self.assertEqual(result["active_overlap"], set())


class ApplySharedOwnershipMarkdownAwareTest(unittest.TestCase):
    """Shared ownership is applied through Markdown runner sections."""

    def _slim_text(self):
        return spec_text_with_changes([
            {"file": "a.py", "action": "update", "content_spec": "noop"},
            {"file": "b.py", "action": "update", "content_spec": "noop"},
        ])

    def test_marks_only_targeted_paths(self):
        from scafld.audit_scope import apply_shared_ownership
        out = apply_shared_ownership(self._slim_text(), {"a.py"})
        data = parse_spec_markdown(out)
        changes = data["phases"][0]["changes"]
        a_entry = next(c for c in changes if c["file"] == "a.py")
        b_entry = next(c for c in changes if c["file"] == "b.py")
        self.assertEqual(a_entry["ownership"], "shared")
        self.assertNotIn("ownership", b_entry)

    def test_idempotent(self):
        from scafld.audit_scope import apply_shared_ownership
        first = apply_shared_ownership(self._slim_text(), {"a.py"})
        second = apply_shared_ownership(first, {"a.py"})
        self.assertEqual(first, second)

    def test_returns_unchanged_text_when_no_targets(self):
        from scafld.audit_scope import apply_shared_ownership
        text = self._slim_text()
        self.assertEqual(apply_shared_ownership(text, set()), text)
        self.assertEqual(apply_shared_ownership(text, None), text)

    def test_normalizes_target_paths(self):
        from scafld.audit_scope import apply_shared_ownership
        # Caller passes ./a.py — should still match the canonical a.py
        # in the spec.
        out = apply_shared_ownership(self._slim_text(), {"./a.py"})
        data = parse_spec_markdown(out)
        a_entry = next(c for c in data["phases"][0]["changes"] if c["file"] == "a.py")
        self.assertEqual(a_entry["ownership"], "shared")

    def test_preserves_other_phase_fields(self):
        from scafld.audit_scope import apply_shared_ownership
        out = apply_shared_ownership(self._slim_text(), {"a.py"})
        data = parse_spec_markdown(out)
        phase = data["phases"][0]
        self.assertEqual(phase["id"], "phase1")
        self.assertEqual(len(phase["changes"]), 2)


if __name__ == "__main__":
    unittest.main()
