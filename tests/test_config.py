import tempfile
import unittest
from pathlib import Path

from scafld.config import load_config
from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError


class LoadConfigTest(unittest.TestCase):
    def test_invalid_yaml_raises_control_plane_error(self):
        root = Path(tempfile.mkdtemp(prefix="scafld-config-test-"))
        (root / ".ai").mkdir()
        (root / ".ai" / "config.yaml").write_text("review:\n  external: [broken\n", encoding="utf-8")

        with self.assertRaises(ScafldError) as ctx:
            load_config(root, ".ai/scafld/config.yaml", ".ai/config.yaml", ".ai/config.local.yaml")

        self.assertEqual(ctx.exception.code, EC.INVALID_ARGUMENTS)
        self.assertIn("invalid scafld config YAML", ctx.exception.message)
        self.assertIn(".ai/config.yaml", "\n".join(ctx.exception.details))

    def test_legacy_timeout_overlay_overrides_managed_timeout_keys(self):
        root = Path(tempfile.mkdtemp(prefix="scafld-config-test-"))
        (root / ".ai" / "scafld").mkdir(parents=True)
        (root / ".ai" / "scafld" / "config.yaml").write_text(
            """
review:
  external:
    idle_timeout_seconds: 180
    absolute_max_seconds: 1800
""",
            encoding="utf-8",
        )
        (root / ".ai" / "config.local.yaml").write_text(
            """
review:
  external:
    timeout_seconds: 1
""",
            encoding="utf-8",
        )

        config = load_config(root, ".ai/scafld/config.yaml", ".ai/config.yaml", ".ai/config.local.yaml")

        external = config["review"]["external"]
        self.assertEqual(external["timeout_seconds"], 1)
        self.assertEqual(external["idle_timeout_seconds"], 1)
        self.assertEqual(external["absolute_max_seconds"], 1)
