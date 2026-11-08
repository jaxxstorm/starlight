# Preliminary build and artifact tooling

This tooling is **not release-ready**. The current command has `client` and
`server` role entrypoints, but no version/revision injection or reporting
interface. Do not publish binaries produced by this provisional build driver.
It does not embed development metadata. No automated release workflow is enabled.

## Development builds

From the repository root, use Python 3.8 or newer and a Go toolchain compatible
with the module's `go.mod` (currently Go 1.23.0). The tool does not install or
pin the toolchain. Build for the host or the provisional Linux matrix:

```sh
python3 tools/release/build.py native --command ./cmd/starlight
python3 tools/release/build.py matrix --command ./cmd/starlight
```

`--command` accepts the Go main package path and defaults to `./cmd/starlight`.
`--output` selects the output directory (default
`dist`). Existing output files are never intentionally overwritten. The matrix
sets `CGO_ENABLED=0` for linux/amd64 (GOAMD64=v1), linux/arm64
(GOARM64=v8.0), and linux/arm with GOARM=7;
native builds also disable CGO. These are compilation targets, not confirmed
device support. Check the router CPU, instruction set, kernel and firmware;
MIPS, ARMv6 and other architectures are not covered. Hardware or emulated
runtime qualification remains necessary.

The initial device of interest is the GL.iNet GL-E5800 NA Mudi 7. A read-only
inspection of one device reported model `GL.iNet E5800`, `uname -m` as
`aarch64`, OpenWrt architecture `aarch64_cortex-a53`, OpenWrt 23.05.4
(`sdx75/generic`), GL.iNet firmware 4.10.0 and kernel `5.15.170-perf`. The
linux/arm64 artifact matches the reported architecture. No executable has been
run on that router; this observation does not establish runtime compatibility,
nor does it apply automatically to other firmware or devices. Check each
device's architecture and firmware independently before selecting an artifact.

## Checksums

After independently obtaining the three correctly named release binaries in a
directory, create and validate their manifest (the version must match their
filenames, e.g. `starlight_v1.2.3_linux_amd64`):

```sh
python3 tools/release/artifacts.py create path/to/artifacts v1.2.3
python3 tools/release/artifacts.py verify path/to/artifacts v1.2.3
```

The creator refuses to overwrite an existing manifest. Verification requires
exactly the three expected manifest entries and checks their SHA-256 digests.
On Linux, downloaded files can also be checked with `sha256sum -c checksums.txt`;
use `chmod +x` on a downloaded executable. Checksums detect corruption but do
not authenticate artifacts if an attacker can replace the manifest too.

Run the standalone tooling tests with:

```sh
python3 -m unittest discover -s tools/release -p 'test_*.py'
```

To test the current core, run `python3 tools/release/validate.py`. It runs Go
tests and tooling tests, builds a native executable, checks both role help
paths without credentials or services, cross-builds the matrix, checks ELF
machine identifiers and Go build settings, and verifies the development checksum manifest. Help
output is a dispatch smoke test, not a functional client/server test. This does
not test version reporting or runtime execution on the target architectures.

Tag-driven publication, authorization, draft recovery, and release validation
remain blocked on core integration and an authorized test repository. No
release should be created using these preliminary tools.
The standalone `python3 tools/release/tag.py v1.2.3` command validates a
v-prefixed Semantic Version 2.0.0 tag, but does not publish or verify Git state.
