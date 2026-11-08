## 1. Coordinate integration

- [ ] 1.1 Confirm the core module, Go version, single command package, both role entrypoints, CGO-free dependencies, and offline smoke-test commands with core maintainers; record integration decisions.
- [ ] 1.2 Agree on the version/revision injection and reporting interface and identify any core work that must land before release integration.
- [x] 1.3 Confirm the initial target matrix against intended devices and record unsupported architectures and qualification gaps; resolve any contract changes before implementation.

## 2. Build and artifact generation

- [ ] 2.1 Add a documented local build entrypoint using the core command, pinned Go version, CGO disabled, and development version metadata.
- [ ] 2.2 Add fail-fast matrix builds for linux/amd64, linux/arm64, and linux/arm GOARM=7 with the specified artifact names and tagged version/revision metadata.
- [x] 2.3 Generate the exact SHA-256 manifest for the three binaries and add automated checks for expected filenames, manifest entries, and checksum verification.

## 3. CI and release

- [ ] 3.1 Add pull-request checks for native tests, offline version/role smoke tests, and all cross-builds without publication.
- [ ] 3.2 Add tag validation and tagged-commit build/test/manifest gates for release automation, with minimal token permissions.
- [ ] 3.3 Implement draft release creation, asset upload and exact asset verification before publishing; fail safely on errors or an existing published release.

## 4. Documentation and validation

- [ ] 4.1 Document prerequisites, local and matrix build commands, version metadata, target selection, architecture/kernel limitations, and executable permissions.
- [ ] 4.2 Document release authorization, tag procedure, checksum verification and its integrity-only limitation, and safe draft cleanup/retry.
- [ ] 4.3 Run native tests and offline role/version smoke tests, build all targets, inspect binary architecture where available, and record results and runtime qualification gaps.
- [ ] 4.4 Test manifest verification including a deliberately modified binary, a target-build failure, invalid tag, upload failure, and published-release collision.
- [ ] 4.5 Exercise an end-to-end tagged release in an authorized non-production repository or equivalent controlled test; verify tagged revision, published asset set, downloaded checksums, and no publication on failure.

## Current blockers

- 1.1–1.2: `cmd/starlight` supplies client/server subcommands and offline help; core maintainer confirmation and the version interface remain pending. Read-only inspection of the GL-E5800 NA Mudi 7 reported aarch64 / aarch64_cortex-a53, GL.iNet 4.10.0, OpenWrt 23.05.4 and kernel 5.15.170-perf. The arm64 matrix target matches its architecture, but runtime qualification is pending; no binary was run on the router.
- 2.1–2.2: The development build driver targets `cmd/starlight`, but no version/revision interface is available; it does not satisfy metadata requirements.
- 3.1–3.3: Offline role help, ELF checks and a standalone semantic-version tag validator are available, but no workflow is enabled until version smoke checks and release gates can be implemented without publishing incomplete binaries.
- 4.1–4.5: Preliminary tooling instructions and isolated tests exist, but full operator guidance and end-to-end/runtime validation require the core, release authorization, and target devices or emulation.
