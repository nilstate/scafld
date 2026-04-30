import unittest

from scafld.harden_protocol import CANONICAL_HARDEN_RUBRIC, harden_can_pass, harden_findings
from tests.spec_fixture import basic_spec


class HardenProtocolTests(unittest.TestCase):
    def test_rubric_contains_pivotal_questions(self):
        questions = [item["question"] for item in CANONICAL_HARDEN_RUBRIC]

        self.assertIn("What is the real product goal, not just the requested implementation?", questions)
        self.assertIn("What is authoritative when two artifacts contain the same fact?", questions)
        self.assertIn("Can we dogfood this?", questions)
        self.assertTrue(any(item["id"] == "complexity_containment" for item in CANONICAL_HARDEN_RUBRIC))

    def test_missing_summary_is_blocking(self):
        data = basic_spec("harden-task")
        data["task"]["summary"] = ""

        findings = harden_findings(data)

        self.assertIn("product_goal", {finding["id"] for finding in findings})
        self.assertFalse(harden_can_pass(data))

    def test_approval_policy_blocks_unresolved_findings(self):
        data = basic_spec("harden-task")
        data["task"]["summary"] = ""

        self.assertFalse(harden_can_pass(data))

    def test_sufficient_fixture_can_pass_policy(self):
        data = basic_spec("harden-task")

        self.assertTrue(harden_can_pass(data))


if __name__ == "__main__":
    unittest.main()
