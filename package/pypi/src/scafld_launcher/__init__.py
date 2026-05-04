"""Python launcher for the native scafld binary."""

from importlib.metadata import PackageNotFoundError, version

try:
    __version__ = version("scafld")
except PackageNotFoundError:  # pragma: no cover - editable source tree
    __version__ = "0.0.0"
