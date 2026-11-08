#!/usr/bin/env python3
"""Validate preliminary builds against the current Starlight command."""

import os
import struct
import subprocess
import sys
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
TOOLS = Path(__file__).resolve().parent


def run(*args, **kwargs):
    print("+", " ".join(map(str, args)), flush=True)
    return subprocess.run(args, cwd=ROOT, check=True, **kwargs)


def check_role(binary, role):
    env = os.environ.copy()
    env.pop("SATMUX_PSK", None)
    env.pop("ORB_MUX_PSK", None)
    result = subprocess.run([str(binary), role, "-h"], cwd=ROOT, env=env,
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                            text=True, timeout=10)
    # Go's flag package currently returns flag.ErrHelp, which the command exits
    # with status 1. Accept a future conventional successful help exit as well.
    if result.returncode not in (0, 1) or f"Usage of {role}:" not in result.stdout:
        raise ValueError(f"{role} help smoke test failed: {result.stdout}")
    print(f"{role} offline help: OK")


def check_elf(path, elf_class, machine):
    with path.open("rb") as stream:
        header = stream.read(20)
    if (len(header) < 20 or header[:4] != b"\x7fELF" or
            header[4] != elf_class or header[5] != 1 or
            struct.unpack_from("<H", header, 18)[0] != machine):
        raise ValueError(f"unexpected ELF architecture: {path}")
    print(f"{path.name}: ELF class {elf_class}, machine {machine}")


def check_build_settings(path, *settings):
    result = run("go", "version", "-m", str(path), stdout=subprocess.PIPE, text=True)
    recorded = {line.strip()[len("build\t"):] for line in result.stdout.splitlines()
                if line.strip().startswith("build\t")}
    missing = set(settings) - recorded
    if missing:
        raise ValueError(f"{path.name} missing build settings: {', '.join(sorted(missing))}")
    print(f"{path.name}: required Go build settings verified")


def main():
    try:
        run("go", "test", "./...")
        run(sys.executable, "-B", "-m", "unittest", "discover", "-s", str(TOOLS), "-p", "test_*.py")
        with tempfile.TemporaryDirectory(prefix="starlight-validation-") as tmp:
            root = Path(tmp)
            run(sys.executable, str(TOOLS / "build.py"), "native", "--output", str(root / "native"))
            for role in ("client", "server"):
                check_role(root / "native" / "starlight_dev", role)
            directory = root / "matrix"
            run(sys.executable, str(TOOLS / "build.py"), "matrix", "--output", str(directory))
            for label, elf_class, machine, arch, baseline in (
                    ("amd64", 2, 62, "amd64", "GOAMD64=v1"),
                    ("arm64", 2, 183, "arm64", "GOARM64=v8.0"),
                    ("armv7", 1, 40, "arm", "GOARM=7")):
                binary = directory / f"starlight_dev_linux_{label}"
                check_elf(binary, elf_class, machine)
                check_build_settings(binary, "CGO_ENABLED=0", "GOOS=linux", f"GOARCH={arch}", baseline)
            run(sys.executable, str(TOOLS / "artifacts.py"), "create", str(directory), "dev")
            run(sys.executable, str(TOOLS / "artifacts.py"), "verify", str(directory), "dev")
        print("Compile and offline help checks passed; version metadata and device runtime remain unverified.")
    except (OSError, subprocess.SubprocessError, ValueError) as exc:
        print(f"validation failed: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
