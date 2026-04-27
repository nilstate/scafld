import tempfile
import unittest
from pathlib import Path

from scafld.command_runtime import CommandContext, find_root, require_root
from scafld.errors import ScafldError


class CommandRuntimeTest(unittest.TestCase):
    def test_find_root_returns_nearest_workspace(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            workspace = Path(temp_dir) / "repo"
            nested = workspace / "src" / "deep"
            (workspace / ".ai").mkdir(parents=True)
            (workspace / ".ai" / "config.yaml").write_text("modes: {}\n", encoding="utf-8")
            nested.mkdir(parents=True)

            self.assertEqual(find_root(nested), workspace.resolve())

    def test_require_root_raises_structured_error_when_missing(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            with self.assertRaises(ScafldError) as context:
                require_root(Path(temp_dir))

            self.assertIn("not in a scafld project", context.exception.message)
            self.assertIn("scafld init", "\n".join(context.exception.details))

    def test_command_context_uses_discovered_root(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            workspace = Path(temp_dir) / "repo"
            nested = workspace / "tools"
            (workspace / ".ai" / "scafld").mkdir(parents=True)
            (workspace / ".ai" / "scafld" / "manifest.json").write_text("{}", encoding="utf-8")
            nested.mkdir(parents=True)

            context = CommandContext.from_cwd(nested)
            self.assertEqual(context.root, workspace.resolve())


if __name__ == "__main__":
    unittest.main()
