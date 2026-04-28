import tempfile
import unittest
from pathlib import Path

from scafld.acceptance import (
    EXPECTED_KINDS,
    _legacy_expected_to_kind,
    check_kind,
    evaluate_acceptance_criterion,
    resolve_kind,
)


class ResolveKindLegacyMappingTest(unittest.TestCase):
    def test_legacy_exit_code_zero_maps(self):
        self.assertEqual(_legacy_expected_to_kind("exit code 0"), ("exit_code_zero", {}))
        self.assertEqual(_legacy_expected_to_kind("EXIT CODE 0"), ("exit_code_zero", {}))
        self.assertEqual(_legacy_expected_to_kind("  exit  code  0  "), ("exit_code_zero", {}))

    def test_legacy_exit_code_nonzero_maps(self):
        kind, fields = _legacy_expected_to_kind("exit code 1")
        self.assertEqual(kind, "exit_code_nonzero")
        self.assertEqual(fields, {"expected_exit_code": 1})

    def test_legacy_no_matches_maps(self):
        self.assertEqual(_legacy_expected_to_kind("no matches"), ("no_matches", {}))

    def test_legacy_unmapped_falls_back_to_substring(self):
        kind, fields = _legacy_expected_to_kind("all tests pass")
        self.assertEqual(kind, "legacy_substring")
        self.assertEqual(fields, {"expected_substring": "all tests pass"})

    def test_legacy_empty_maps_to_unset(self):
        self.assertEqual(_legacy_expected_to_kind(""), ("unset", {}))
        self.assertEqual(_legacy_expected_to_kind("   "), ("unset", {}))


class ResolveKindExplicitTest(unittest.TestCase):
    def test_explicit_kind_wins_over_legacy_string(self):
        criterion = {"expected_kind": "no_matches", "expected": "exit code 0"}
        kind, _ = resolve_kind(criterion)
        self.assertEqual(kind, "no_matches")

    def test_unknown_explicit_kind_returns_invalid_kind(self):
        criterion = {"expected_kind": "made_up_kind"}
        kind, fields = resolve_kind(criterion)
        self.assertEqual(kind, "invalid_kind")
        self.assertEqual(fields, {"declared_kind": "made_up_kind"})

    def test_explicit_kind_in_enum(self):
        for kind in EXPECTED_KINDS:
            with self.subTest(kind=kind):
                resolved, _ = resolve_kind({"expected_kind": kind})
                self.assertEqual(resolved, kind)


class CheckKindExitCodeZeroTest(unittest.TestCase):
    def test_pass_on_exit_zero(self):
        passed, reason = check_kind(0, "anything", {"expected_kind": "exit_code_zero"})
        self.assertTrue(passed)
        self.assertEqual(reason, "")

    def test_fail_on_nonzero(self):
        passed, reason = check_kind(1, "anything", {"expected_kind": "exit_code_zero"})
        self.assertFalse(passed)
        self.assertIn("expected exit code 0", reason)


class CheckKindExitCodeNonzeroTest(unittest.TestCase):
    def test_pass_on_any_nonzero_when_no_pin(self):
        passed, _ = check_kind(7, "", {"expected_kind": "exit_code_nonzero"})
        self.assertTrue(passed)

    def test_fail_on_zero(self):
        passed, reason = check_kind(0, "", {"expected_kind": "exit_code_nonzero"})
        self.assertFalse(passed)
        self.assertIn("expected non-zero exit code", reason)

    def test_pin_to_specific_code_pass(self):
        passed, _ = check_kind(2, "", {"expected_kind": "exit_code_nonzero", "expected_exit_code": 2})
        self.assertTrue(passed)

    def test_pin_to_specific_code_fail(self):
        passed, reason = check_kind(7, "", {"expected_kind": "exit_code_nonzero", "expected_exit_code": 2})
        self.assertFalse(passed)
        self.assertIn("expected exit code 2, got 7", reason)


class CheckKindNoMatchesTest(unittest.TestCase):
    def test_pass_on_empty_output(self):
        passed, _ = check_kind(1, "", {"expected_kind": "no_matches"})
        self.assertTrue(passed)

    def test_pass_on_whitespace_only(self):
        passed, _ = check_kind(1, "   \n", {"expected_kind": "no_matches"})
        self.assertTrue(passed)

    def test_pass_on_nonzero_exit_with_output(self):
        # Mirrors legacy `expected: "no matches"` semantics: rg-style
        # "no match found" returns non-zero with empty stdout AND
        # any non-zero exit is accepted as evidence of no matches.
        passed, _ = check_kind(1, "irrelevant", {"expected_kind": "no_matches"})
        self.assertTrue(passed)

    def test_fail_on_zero_exit_with_output(self):
        passed, reason = check_kind(0, "match found", {"expected_kind": "no_matches"})
        self.assertFalse(passed)
        self.assertIn("expected no matches", reason)


class EvaluateCriterionStrictModeTest(unittest.TestCase):
    """Integration tests for the strict-mode reject-before-run path on
    `evaluate_acceptance_criterion`. 1.7.0 ships strict-only; criteria
    without `expected_kind` declared fail loudly without running the
    command."""

    def _workspace(self):
        tmp = tempfile.mkdtemp(prefix="scafld-acceptance-")
        root = Path(tmp)
        (root / ".ai").mkdir()
        (root / ".ai" / "config.yaml").write_text("version: '1.0'\n", encoding="utf-8")
        return root

    def test_unset_kind_rejected_without_running_command(self):
        root = self._workspace()
        sentinel = root / "side_effect.txt"
        criterion = {
            "id": "ac1_1",
            "command": f"touch '{sentinel}'",
        }
        result = evaluate_acceptance_criterion(root, criterion)
        self.assertEqual(result["status"], "fail")
        self.assertIn("expected_kind", result["fail_reason"])
        self.assertIsNone(result["exit_code"])
        self.assertFalse(sentinel.exists(), "rejection must short-circuit BEFORE subprocess.run")

    def test_legacy_substring_rejected(self):
        root = self._workspace()
        criterion = {
            "id": "ac1_2",
            "command": "echo all tests pass",
            "expected": "all tests pass",
        }
        result = evaluate_acceptance_criterion(root, criterion)
        self.assertEqual(result["status"], "fail")
        self.assertIn("expected_kind", result["fail_reason"])
        self.assertIn("ac1_2", result["fail_reason"])

    def test_invalid_kind_rejected_without_running_command(self):
        # A typo'd expected_kind (e.g. "exit_cod_zero") must be rejected
        # BEFORE subprocess.run so commands with side effects don't run
        # against an unevaluable contract. Mirrors the unset/legacy_substring
        # short-circuit; closes the gap that allowed Review 6 F1.
        root = self._workspace()
        sentinel = root / "side_effect.txt"
        criterion = {
            "id": "ac1_invalid",
            "command": f"touch '{sentinel}'",
            "expected_kind": "exit_cod_zero",
        }
        result = evaluate_acceptance_criterion(root, criterion)
        self.assertEqual(result["status"], "fail")
        self.assertIsNone(result["exit_code"])
        self.assertIn("unknown expected_kind", result["fail_reason"])
        self.assertIn("exit_cod_zero", result["fail_reason"])
        self.assertFalse(sentinel.exists(), "invalid_kind must short-circuit BEFORE subprocess.run")

    def test_explicit_kind_accepted(self):
        root = self._workspace()
        criterion = {
            "id": "ac1_3",
            "command": "true",
            "expected_kind": "exit_code_zero",
        }
        result = evaluate_acceptance_criterion(root, criterion)
        self.assertEqual(result["status"], "pass")

    def test_legacy_exit_code_zero_string_auto_resolves(self):
        root = self._workspace()
        criterion = {
            "id": "ac1_4",
            "command": "true",
            "expected": "exit code 0",  # legacy string that maps cleanly
        }
        result = evaluate_acceptance_criterion(root, criterion)
        self.assertEqual(result["status"], "pass")


class EvaluateCriterionEvidenceTest(unittest.TestCase):
    """`evidence_required: true` rejects criteria whose command exits 0
    with empty stdout. Stops the 'compile + unittest pass with no real
    work' phase-advance pattern."""

    def _workspace(self):
        tmp = tempfile.mkdtemp(prefix="scafld-evidence-")
        root = Path(tmp)
        (root / ".ai").mkdir()
        (root / ".ai" / "config.yaml").write_text("version: '1.0'\n", encoding="utf-8")
        return root

    def test_evidence_required_passes_with_stdout(self):
        root = self._workspace()
        criterion = {
            "id": "ac1_1",
            "command": "echo something",
            "expected_kind": "exit_code_zero",
            "evidence_required": True,
        }
        result = evaluate_acceptance_criterion(root, criterion)
        self.assertEqual(result["status"], "pass")

    def test_evidence_required_fails_with_empty_stdout(self):
        root = self._workspace()
        criterion = {
            "id": "ac1_2",
            "command": "true",
            "expected_kind": "exit_code_zero",
            "evidence_required": True,
        }
        result = evaluate_acceptance_criterion(root, criterion)
        self.assertEqual(result["status"], "fail")
        self.assertIn("evidence_required", result["fail_reason"])

    def test_evidence_not_required_passes_with_empty_stdout(self):
        root = self._workspace()
        criterion = {
            "id": "ac1_3",
            "command": "true",
            "expected_kind": "exit_code_zero",
        }
        result = evaluate_acceptance_criterion(root, criterion)
        self.assertEqual(result["status"], "pass")


if __name__ == "__main__":
    unittest.main()
