#!/usr/bin/env python3
"""Build an experimental offline IPK; this does not create a trusted feed."""
import argparse
import gzip
import io
import pathlib
import re
import struct
import tarfile

HERE = pathlib.Path(__file__).resolve().parent


def archive(entries):
    result = io.BytesIO()
    with gzip.GzipFile(fileobj=result, mode="wb", mtime=0, filename="") as zipped:
        with tarfile.open(fileobj=zipped, mode="w", format=tarfile.GNU_FORMAT) as tar:
            for name, data, mode in entries:
                info = tarfile.TarInfo(name)
                info.size, info.mode, info.mtime = len(data) if data is not None else 0, mode, 0
                info.uid = info.gid = 0
                info.uname = info.gname = "root"
                if data is None:
                    info.type = tarfile.DIRTYPE
                    tar.addfile(info)
                else:
                    tar.addfile(info, io.BytesIO(data))
    return result.getvalue()


def validate_binary(binary):
    """Reject malformed AArch64 ELF inputs and those needing an interpreter.

    This checks the ELF structure needed for packaging, not provenance, the
    instruction set used by the executable, or actual router compatibility.
    """
    error = "binary must be a structurally valid AArch64 ELF64 executable"
    if len(binary) < 64 or binary[:7] != b"\x7fELF\x02\x01\x01":
        raise ValueError(error)
    if binary[7] not in (0, 3):  # ELF System V or Linux OS ABI
        raise ValueError(error)
    kind, machine, version = struct.unpack_from("<HHI", binary, 16)
    phoff = struct.unpack_from("<Q", binary, 32)[0]
    ehsize, phentsize, phnum = struct.unpack_from("<HHH", binary, 52)
    if (kind not in (2, 3) or machine != 183 or version != 1 or ehsize != 64
            or phentsize < 56 or phnum == 0 or phoff < ehsize
            or phoff + phentsize * phnum > len(binary)):
        raise ValueError(error)
    load_found = False
    for index in range(phnum):
        offset = phoff + index * phentsize
        ph_type, _, file_offset, _, _, file_size, memory_size, _ = struct.unpack_from("<IIQQQQQQ", binary, offset)
        if ph_type == 3:  # PT_INTERP requires an external dynamic loader.
            raise ValueError("binary must not require a dynamic interpreter")
        if ph_type == 1:  # PT_LOAD
            if file_size > memory_size or file_offset + file_size > len(binary):
                raise ValueError(error)
            load_found = True
    if not load_found:
        raise ValueError(error)


def build(binary, version):
    if not re.fullmatch(r"[0-9][A-Za-z0-9.+~_-]*", version):
        raise ValueError("invalid package version")
    validate_binary(binary)
    control = (f"Package: starlight-client\nVersion: {version}\n"
               "Architecture: aarch64_cortex-a53\n"
               "Maintainer: Starlight\nSection: net\n"
               "Description: Experimental Starlight router client (manual configuration required)\n").encode()
    control_tar = archive([("./control", control, 0o644),
                           ("./conffiles", b"/etc/starlight/client.conf\n", 0o644)])
    data_tar = archive([("./usr/sbin/starlight", binary, 0o755),
                        ("./usr/libexec/starlight-probe", (HERE / "starlight-probe").read_bytes(), 0o755),
                        ("./usr/libexec/starlight-restart", (HERE / "starlight-restart").read_bytes(), 0o755),
                        ("./etc/init.d/starlight", (HERE / "starlight.init").read_bytes(), 0o755),
                        ("./etc/starlight", None, 0o700),
                        ("./etc/starlight/client.conf", (HERE / "client.conf").read_bytes(), 0o600),
                        ("./usr/share/oui/menu.d/starlight.json", (HERE / "oui-starlight.json").read_bytes(), 0o644),
                        ("./www/views/gl-sdk4-ui-starlight.common.js", (HERE / "oui-starlight.js").read_bytes(), 0o644),
                        ("./usr/lib/oui-httpd/rpc/starlight", (HERE / "oui-starlight-rpc.lua").read_bytes(), 0o644)])
    return archive([("./debian-binary", b"2.0\n", 0o644),
                    ("./control.tar.gz", control_tar, 0o644),
                    ("./data.tar.gz", data_tar, 0o644)])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=pathlib.Path)
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", required=True, type=pathlib.Path)
    args = parser.parse_args()
    try:
        package = build(args.binary.read_bytes(), args.version)
        # Avoid accidentally overwriting a release or existing artifact.
        with args.output.open("xb") as output:
            output.write(package)
    except (OSError, ValueError) as exc:
        parser.exit(1, f"build_ipk: {exc}\n")


if __name__ == "__main__":
    main()
