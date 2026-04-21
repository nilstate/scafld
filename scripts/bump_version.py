#!/usr/bin/env python3
"""Bump the canonical scafld version and sync derived package metadata."""

from __future__ import annotations

import argparse

from sync_version import (
    VERSION_FILE,
    canonical_version,
    check_sync,
    sync_package_json,
    validate_release_version,
    write_version_file,
)


def main() -> int:
    parser = argparse.ArgumentParser(description="Set the canonical scafld release version")
    parser.add_argument("version", help="New release version, for example 1.4.3")
    args = parser.parse_args()

    next_version = validate_release_version(args.version)
    current_version = canonical_version()
    changed = False

    if current_version != next_version:
        write_version_file(VERSION_FILE, next_version)
        changed = True

    if sync_package_json(next_version):
        changed = True

    errors = check_sync(next_version)
    if errors:
        for error in errors:
            print(error)
        return 1

    if changed:
        print(f"bumped scafld from {current_version} to {next_version}")
    else:
        print(f"scafld already at {next_version}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
