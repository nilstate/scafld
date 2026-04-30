import tempfile
import unittest
from pathlib import Path

from scafld.config import load_config
from scafld.error_codes import ErrorCode as EC
from scafld.errors import ScafldError


class LoadConfigTest(unittest.TestCase):
    def test_invalid_markdown_raises_control_plane_error(self):
        root = Path(tempfile.mkdtemp(prefix="scafld-config-test-"))
        (root / ".scafld").mkdir()
        (root / ".scafld" / "config.yaml").write_text("review:\n  external: [broken\n", encoding="utf-8")

        with self.assertRaises(ScafldError) as ctx:
            load_config(root, ".scafld/core/config.yaml", ".scafld/config.yaml", ".scafld/config.local.yaml")

        self.assertEqual(ctx.exception.code, EC.INVALID_ARGUMENTS)
        self.assertIn("invalid scafld config YAML", ctx.exception.message)
        self.assertIn(".scafld/config.yaml", "\n".join(ctx.exception.details))

    def test_local_timeout_overlay_uses_v2_timeout_keys_only(self):
        root = Path(tempfile.mkdtemp(prefix="scafld-config-test-"))
        (root / ".scafld" / "core").mkdir(parents=True)
        (root / ".scafld" / "core" / "config.yaml").write_text(
            """
review:
  external:
    idle_timeout_seconds: 180
    absolute_max_seconds: 1800
""",
            encoding="utf-8",
        )
        (root / ".scafld" / "config.local.yaml").write_text(
            """
review:
  external:
    idle_timeout_seconds: 60
    absolute_max_seconds: 600
""",
            encoding="utf-8",
        )

        config = load_config(root, ".scafld/core/config.yaml", ".scafld/config.yaml", ".scafld/config.local.yaml")

        external = config["review"]["external"]
        self.assertNotIn("timeout_seconds", external)
        self.assertEqual(external["idle_timeout_seconds"], 60)
        self.assertEqual(external["absolute_max_seconds"], 600)
