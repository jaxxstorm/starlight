#!/usr/bin/env python3
"""Provisional CGO-free development builds; not a release publisher."""

import argparse
import os
import subprocess
import sys
import tempfile
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("native", "matrix"))
    parser.add_argument("--command", default="./cmd/starlight", help="Go main package")
    parser.add_argument("--output", type=Path, default=Path("dist"))
    args = parser.parse_args()
    targets = [("amd64", "amd64", None), ("arm64", "arm64", None), ("armv7", "arm", "7")]
    if args.mode == "native":
        targets = [("native", None, None)]
    args.output.mkdir(parents=True, exist_ok=True)
    try:
        # Stage everything before copying any artifacts, so a failed build is not
        # mistaken for a completed matrix. Never overwrite existing output.
        with tempfile.TemporaryDirectory(prefix="starlight-build-") as tmp:
            staged = []
            for label, arch, arm in targets:
                name = "starlight_dev" if arch is None else f"starlight_dev_linux_{label}"
                dest = args.output / name
                if dest.exists() or dest.is_symlink():
                    raise FileExistsError(f"refusing to overwrite {dest}")
                env = os.environ.copy()
                env["CGO_ENABLED"] = "0"
                if arch is not None:
                    env.update(GOOS="linux", GOARCH=arch)
                    # Do not inherit a developer's higher CPU baseline.
                    for key in ("GOARM", "GOARM64", "GOAMD64"):
                        env.pop(key, None)
                    if arm:
                        env["GOARM"] = arm
                    elif arch == "arm64":
                        env["GOARM64"] = "v8.0"
                    elif arch == "amd64":
                        env["GOAMD64"] = "v1"
                else:
                    for key in ("GOOS", "GOARCH", "GOARM"):
                        env.pop(key, None)
                path = Path(tmp) / name
                subprocess.run(["go", "build", "-o", str(path), args.command], env=env, check=True)
                staged.append((path, dest))
            for source, dest in staged:
                # Exclusive creation also guards against a concurrent output writer.
                with source.open("rb") as src, dest.open("xb") as out:
                    while chunk := src.read(1024 * 1024):
                        out.write(chunk)
                dest.chmod(0o755)
                print(dest)
    except (OSError, subprocess.CalledProcessError) as exc:
        print(f"build failed: {exc}", file=sys.stderr)
        return 1
    print("Development build only: version metadata and role validation are not yet integrated.", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
