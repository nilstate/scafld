import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from scafld.runtime_contracts import session_path
from scafld.session_store import default_session
from scafld.spec_markdown import render_spec_markdown
from tests.spec_fixture import basic_spec


REPO_ROOT = Path(__file__).resolve().parents[1]
CLI_ROOT = REPO_ROOT / "cli"


class SpecCommandTests(unittest.TestCase):
    def make_workspace(self):
        temp = tempfile.TemporaryDirectory()
        root = Path(temp.name)
        (root / ".scafld" / "core").mkdir(parents=True)
        (root / ".scafld" / "core" / "manifest.json").write_text("{}", encoding="utf-8")
        spec_path = root / ".scafld" / "specs" / "active" / "reconcile-task.md"
        spec_path.parent.mkdir(parents=True)
        spec_path.write_text(
            render_spec_markdown(
                basic_spec(
                    "reconcile-task",
                    status="in_progress",
                    title="Reconcile Task",
                    file_path="app.txt",
                    command="true",
                )
            ),
            encoding="utf-8",
        )
        session = default_session("reconcile-task", model_profile="default")
        session["entries"].append({"type": "attempt", "recorded_at": "2026-04-30T00:00:00Z"})
        session["criterion_states"]["ac1_1"] = {"status": "pass", "phase_id": "phase1"}
        session["phase_blocks"] = {"phase1": {"status": "completed"}}
        path = session_path(root, "reconcile-task", spec_path=spec_path)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(session), encoding="utf-8")
        return temp, root, spec_path

    def run_scafld(self, root, *args):
        env = os.environ.copy()
        env["PATH"] = f"{CLI_ROOT}:{env.get('PATH', '')}"
        env["PYTHONPATH"] = str(REPO_ROOT)
        return subprocess.run(
            ["scafld", *args],
            cwd=root,
            env=env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )

    def test_reconcile_check_and_repair_preserve_human_prose(self):
        temp, root, spec_path = self.make_workspace()
        self.addCleanup(temp.cleanup)
        spec_path.write_text(
            spec_path.read_text(encoding="utf-8").replace("Fixture summary.", "Human prose."),
            encoding="utf-8",
        )

        check = self.run_scafld(root, "reconcile", "reconcile-task", "--json")
        self.assertEqual(check.returncode, 0, check.stderr or check.stdout)
        payload = json.loads(check.stdout)
        self.assertTrue(payload["result"]["spec_file"].endswith("reconcile-task.md"))
        self.assertTrue(payload["result"]["drift"])
        self.assertFalse(payload["result"]["repaired"])

        repair = self.run_scafld(root, "reconcile", "reconcile-task", "--repair", "--json")
        self.assertEqual(repair.returncode, 0, repair.stderr or repair.stdout)
        payload = json.loads(repair.stdout)
        self.assertTrue(payload["result"]["drift"])
        self.assertTrue(payload["result"]["repaired"])

        repaired_text = spec_path.read_text(encoding="utf-8")
        preserved_prose = "Human prose."
        self.assertIn(preserved_prose, repaired_text)
        self.assertIn("Status: completed", repaired_text)
        self.assertIn("- [x] `ac1_1`", repaired_text)


if __name__ == "__main__":
    unittest.main()
