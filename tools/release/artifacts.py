#!/usr/bin/env python3
"""Create and verify the exact Starlight release checksum manifest."""

import argparse
import hashlib
import re
import sys
from pathlib import Path


def names(version):
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._+-]*", version):
        raise ValueError("invalid version for artifact filenames")
    return [f"starlight_{version}_linux_{arch}" for arch in ("amd64", "arm64", "armv7")]


def digest(path):
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"not a regular artifact: {path}")
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def manifest(directory, version):
    return "".join(f"{digest(directory / name)}  {name}\n" for name in names(version))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("create", "verify"))
    parser.add_argument("directory", type=Path)
    parser.add_argument("version")
    args = parser.parse_args()
    try:
        expected = manifest(args.directory, args.version)
        path = args.directory / "checksums.txt"
        if args.action == "create":
            # Exclusive creation prevents accidentally replacing a previous manifest.
            with path.open("x", encoding="ascii", newline="\n") as stream:
                stream.write(expected)
        elif path.is_symlink() or path.read_bytes() != expected.encode("ascii"):
            raise ValueError("checksum manifest does not match the expected artifacts")
    except (OSError, ValueError) as exc:
        print(f"artifact verification failed: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
