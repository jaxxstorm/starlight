#!/usr/bin/env python3
"""Validate a v-prefixed Semantic Version 2.0.0 release tag."""

import re
import sys


SEMVER = re.compile(
    r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?"
    r"(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?"
)


def valid_tag(tag):
    match = SEMVER.fullmatch(tag)
    if not match:
        return False
    prerelease = match.group(4)
    return not prerelease or all(
        not (part.isdigit() and len(part) > 1 and part.startswith("0"))
        for part in prerelease.split(".")
    )


def main():
    if len(sys.argv) != 2 or not valid_tag(sys.argv[1]):
        print("expected a v-prefixed Semantic Version 2.0.0 tag", file=sys.stderr)
        return 1
    print(sys.argv[1])
    return 0


if __name__ == "__main__":
    sys.exit(main())
