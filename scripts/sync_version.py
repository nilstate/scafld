#!/usr/bin/env python3
"""Sync derived package metadata from the canonical scafld version."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
VERSION_FILE = ROOT / "scafld" / "_version.py"
PACKAGE_JSON = ROOT / "package.json"
VERSION_RE = re.compile(r'^__version__\s*=\s*"([^"]+)"\s*$', re.MULTILINE)
SEMVER_RE = re.compile(r"^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$")


def read_version_file(path: Path) -> str:
    text = path.read_text(encoding="utf-8")
    match = VERSION_RE.search(text)
    if not match:
        raise SystemExit(f"could not read __version__ from {path}")
    return match.group(1)


def write_version_file(path: Path, version: str) -> None:
    path.write_text(f'__version__ = "{version}"\n', encoding="utf-8")


def validate_release_version(version: str) -> str:
    if not SEMVER_RE.fullmatch(version):
        raise SystemExit(f"invalid release version: {version}")
    return version


def canonical_version() -> str:
    return read_version_file(VERSION_FILE)


def package_json() -> dict:
    return json.loads(PACKAGE_JSON.read_text(encoding="utf-8"))


def write_package_json(data: dict) -> None:
    PACKAGE_JSON.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")


def check_sync(expected_version: str) -> list[str]:
    errors: list[str] = []
    package = package_json()

    actual_version = package.get("version")
    if actual_version != expected_version:
        errors.append(
            f"package.json version mismatch: expected {expected_version}, found {actual_version}"
        )

    return errors


def sync_package_json(expected_version: str) -> bool:
    package = package_json()
    changed = False
    if package.get("version") != expected_version:
        package["version"] = expected_version
        changed = True
    if changed:
        write_package_json(package)
    return changed


def main() -> int:
    parser = argparse.ArgumentParser(description="Sync derived package metadata from scafld/_version.py")
    parser.add_argument("--check", action="store_true", help="Fail if derived files drift from the canonical version")
    parser.add_argument("--write", action="store_true", help="Rewrite derived files to match the canonical version")
    parser.add_argument("--tag", help="Optional release tag/version that must match the canonical version")
    parser.add_argument("--print", dest="print_version", action="store_true", help="Print the canonical version")
    parser.add_argument("--quiet", action="store_true", help="Suppress success output")
    args = parser.parse_args()

    version = canonical_version()

    if args.print_version:
        print(version)

    if args.tag and args.tag != version:
        print(f"tag mismatch: expected {version}, got {args.tag}", file=sys.stderr)
        return 1

    if args.write:
        changed = sync_package_json(version)
        if not args.quiet:
            print("updated package.json" if changed else "package.json already up to date")

    if args.check:
        errors = check_sync(version)
        if errors:
            for error in errors:
                print(error, file=sys.stderr)
            return 1
        if not args.quiet:
            print(f"version sync OK: {version}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
