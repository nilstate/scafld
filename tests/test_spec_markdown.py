import unittest
from pathlib import Path

from scafld.errors import ScafldError
from scafld.spec_markdown import (
    parse_spec_markdown,
    render_spec_markdown,
    update_spec_markdown,
)
from tests.spec_fixture import basic_spec


class SpecMarkdownTests(unittest.TestCase):
    def test_example_parses(self):
        path = Path(".scafld/core/specs/examples/markdown-2.0-skeleton.md")
        data = parse_spec_markdown(path.read_text(encoding="utf-8"), path=path)

        self.assertEqual(data["spec_version"], "2.0")
        self.assertEqual(data["task_id"], "spec-skeleton")
        self.assertEqual(data["phases"][0]["id"], "phase1")

    def test_core_examples_are_byte_stable_golden_round_trips(self):
        paths = [
            Path(".scafld/core/specs/examples/markdown-2.0-skeleton.md"),
            Path(".scafld/core/specs/examples/add-error-codes.md"),
        ]

        for path in paths:
            with self.subTest(path=str(path)):
                text = path.read_text(encoding="utf-8")
                rendered = render_spec_markdown(parse_spec_markdown(text, path=path))

                self.assertEqual(rendered, text)

    def test_unclosed_code_fence_is_rejected(self):
        text = render_spec_markdown(basic_spec("bad-fence"))
        text = text.replace("## Summary\n\nFixture summary.", "## Summary\n\n```text\nbad: true")

        with self.assertRaises(ScafldError) as ctx:
            parse_spec_markdown(text)

        self.assertIn("unclosed Markdown code fence", " ".join(ctx.exception.details))

    def test_phase_identity_uses_heading_number_for_id_and_text_for_name(self):
        text = render_spec_markdown(basic_spec("phase-identity"))
        text = text.replace("## Phase 1: Fixture phase", "## Phase 1: Human renamed phase")

        data = parse_spec_markdown(text)

        self.assertEqual(data["phases"][0]["id"], "phase1")
        self.assertEqual(data["phases"][0]["name"], "Human renamed phase")

    def test_duplicate_phase_heading_is_rejected(self):
        text = render_spec_markdown(basic_spec("duplicate-phase"))
        text = text.replace(
            "## Rollback",
            "## Phase 1: Duplicate phase\n\n"
            "Goal:\n\n"
            "Duplicate phase.\n\n"
            "Status: pending\n"
            "Dependencies: none\n\n"
            "Changes:\n"
            "- None.\n\n"
            "Acceptance:\n"
            "- None.\n\n"
            "## Rollback",
        )

        with self.assertRaises(ScafldError) as ctx:
            parse_spec_markdown(text)

        self.assertIn("duplicate phase selector", " ".join(ctx.exception.details))

    def test_no_loss_round_trip_for_core_fields(self):
        data = basic_spec(
            "no-loss",
            status="in_progress",
            title="No Loss",
            file_path="app.txt",
            command="true",
            criterion_result="pass",
            phase_status="completed",
            dod_status="done",
        )
        data["review"] = {"status": "passed", "findings": []}
        text = render_spec_markdown(data)
        parsed = parse_spec_markdown(text)

        self.assertEqual(parsed["task_id"], "no-loss")
        self.assertEqual(parsed["status"], "in_progress")
        self.assertEqual(parsed["task"]["title"], "No Loss")
        self.assertEqual(parsed["phases"][0]["status"], "completed")
        self.assertEqual(parsed["phases"][0]["acceptance_criteria"][0]["result"], "pass")
        self.assertEqual(parsed["review"]["status"], "passed")

    def test_update_spec_markdown_keeps_human_prose(self):
        text = render_spec_markdown(basic_spec("golden-update"))
        text = text.replace("Fixture summary.", "Human edited prose.")
        data = parse_spec_markdown(text)
        data["status"] = "in_progress"

        updated = update_spec_markdown(text, data)

        self.assertIn("Human edited prose.", updated)
        self.assertIn("status: in_progress", updated)

    def test_update_spec_markdown_ignores_heading_like_text_inside_code_fences(self):
        text = render_spec_markdown(basic_spec("fenced-heading"))
        text = text.replace(
            "Fixture summary.",
            "Human prose.\n\n```markdown\n## Acceptance\nnot a section\n```",
        )
        data = parse_spec_markdown(text)
        data["current_state"] = {"next": "scafld start fenced-heading"}

        updated = update_spec_markdown(text, data)

        self.assertIn("```markdown\n## Acceptance\nnot a section\n```", updated)
        self.assertIn("Next: scafld start fenced-heading", updated)

    def test_update_spec_markdown_rejects_phase_shape_mismatch(self):
        text = render_spec_markdown(basic_spec("phase-mismatch"))
        data = parse_spec_markdown(text)
        data["phases"].append(
            {
                "id": "phase2",
                "name": "Missing on disk",
                "objective": "This phase has no heading in the document.",
                "changes": [],
                "acceptance_criteria": [],
                "status": "pending",
            }
        )

        with self.assertRaises(ScafldError) as ctx:
            update_spec_markdown(text, data)

        self.assertIn("phase headings do not match model phases", " ".join(ctx.exception.details))

    def test_rendered_runner_sections_use_readable_markdown(self):
        data = basic_spec("readable-runner-sections")
        data["metadata"] = {
            "estimated_effort_hours": 1.5,
            "actual_effort_hours": 2,
            "ai_model": "fixture-model",
            "react_cycles": 3,
            "tags": ["release", "grammar"],
        }
        data["self_eval"] = {
            "status": "passed",
            "completeness": 9,
            "architecture_fidelity": 8,
            "spec_alignment": 10,
            "validation_depth": 8,
            "total": 35,
            "second_pass_performed": True,
            "notes": "The fixture is readable.",
            "improvements": ["Tighten one edge case."],
        }
        data["deviations"] = [{"rule": "scope", "reason": "Fixture", "mitigation": "Covered"}]
        data["harden_rounds"] = [
            {
                "round": 1,
                "started_at": "2026-04-30T00:00:00Z",
                "ended_at": "2026-04-30T00:01:00Z",
                "outcome": "passed",
                "questions": [
                    {
                        "question": "What proves the shape?",
                        "grounded_in": "docs/spec-schema.md",
                        "recommended_answer": "The golden render.",
                    }
                ],
            }
        ]

        text = render_spec_markdown(data)
        parsed = parse_spec_markdown(text)

        self.assertIn("Strategy: per_phase", text)
        self.assertIn("Commands:\n- `phase1`: `true`", text)
        self.assertIn("Tags:\n- release\n- grammar", text)
        self.assertIn("- Round 1", text)
        self.assertNotIn("strategy: per_phase", text)
        self.assertNotIn("harden_rounds:", text)
        self.assertEqual(parsed["metadata"]["estimated_effort_hours"], 1.5)
        self.assertEqual(parsed["metadata"]["tags"], ["release", "grammar"])
        self.assertEqual(parsed["self_eval"]["second_pass_performed"], True)
        self.assertEqual(parsed["deviations"][0]["rule"], "scope")
        self.assertEqual(parsed["harden_rounds"][0]["questions"][0]["grounded_in"], "docs/spec-schema.md")

    def test_update_spec_markdown_preserves_unknown_sections(self):
        text = render_spec_markdown(basic_spec("unknown-section"))
        text = text.replace("## Rollback", "## Human Notes\n\nDo not lose this paragraph.\n\n## Rollback")
        data = parse_spec_markdown(text)
        data["current_state"] = {"next": "scafld start unknown-section"}

        updated = update_spec_markdown(text, data)

        self.assertIn("## Human Notes\n\nDo not lose this paragraph.", updated)
        self.assertIn("Next: scafld start unknown-section", updated)


if __name__ == "__main__":
    unittest.main()
