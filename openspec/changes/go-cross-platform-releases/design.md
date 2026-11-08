## Context

The repository now contains a Go module and a `cmd/starlight` entrypoint with `client` and `server` roles; core development continues concurrently. Build integration depends on its finalized command and version interface. This change defines the distribution contract without prescribing application internals.

## Goals / Non-Goals

**Goals:** Provide a repeatable cross-build and tag-driven release path for Linux router and server architectures, with one executable per target, verifiable artifacts, and clear operator instructions.

**Non-Goals:** Implement client/server functionality, protocol changes, router service installation, package managers, automatic updates, cross-platform runtime emulation, artifact signing, or byte-for-byte reproducible builds. Checksums detect corruption but are not signatures.

## Decisions

1. **Initial matrix:** linux/amd64 (GOAMD64=v1), linux/arm64 (GOARM64=v8.0), and linux/arm with GOARM=7. The first two cover common server and 64-bit router CPUs; ARMv7 is a pragmatic 32-bit baseline. Explicit baselines prevent inherited build environment settings from requiring newer CPU instructions. A read-only inspection of the intended GL-E5800 reported `aarch64` and `aarch64_cortex-a53`, matching the arm64 architecture; runtime compatibility remains unverified. Document that model, CPU, kernel and instruction-set compatibility must be checked and that MIPS, ARMv6 and other targets are not yet covered. Alternative: promise every router architecture immediately; rejected without a device inventory or validation.
2. **Executable builds:** use the core's agreed single command package with `CGO_ENABLED=0`, explicit GOOS/GOARCH/GOARM, and the Go version pinned consistently with the module. Fail the matrix if any target or role is unavailable rather than publish partial releases. Go's standard cross compiler avoids target C toolchains; if core dependencies require CGO, coordinate a core change or revise this contract explicitly, not a silent fallback.
3. **Distribution:** publish raw executables named `starlight_<version>_linux_amd64`, `starlight_<version>_linux_arm64`, and `starlight_<version>_linux_armv7`, plus `checksums.txt` containing SHA-256 sums for exactly those files. Document `chmod +x` after downloading. Raw binaries avoid platform-dependent archive extraction; no configs or external shared libraries are bundled. Version and revision are embedded through the core's agreed interface; local untagged builds identify themselves as development builds. The exact version command/flags are agreed with core rather than invented here.
4. **Automation:** use repository-hosted CI and a GitHub Releases workflow, given the GitHub repository location, with least-privilege release write permissions only in the publishing job. A push of a `v`-prefixed semantic version tag triggers a build from that tagged commit. Verify tag syntax and run tests, matrix builds, and integrity checks before creating a draft; upload and verify the complete asset set before publishing. On failure, fail the job and leave any created release unpublished; document cleanup/retry instead of overwriting a published release. CI on pull requests runs tests and cross-builds without publishing. A pure shell/Go-based approach is preferred over a new release framework unless a compelling repository standard emerges.
5. **Validation:** run native tests and a native executable smoke test for version and both role entrypoints using the actual core CLI, with no network service or credentials required; cross-built binaries are compile-checked, with ELF architecture inspected where tooling is available. Runtime checks on representative ARM hardware or emulation are a separate release qualification step and must not be claimed as completed by compilation alone.

## Risks / Trade-offs

- [Unknown core entrypoint, version interface, or role smoke-test semantics] → Coordinate before implementing; block rather than assume flags or introduce a second executable.
- [CGO-free builds or a target fail due to core dependencies] → Gate publication on every target; resolve upstream or update the contract through review.
- [Router architecture and kernel diversity] → Publish the narrow matrix and explicit limitations; qualify actual devices separately.
- [Release upload fails partway] → Keep release draft unpublished; document cleanup and safe retry, including collision with existing published releases.
- [Checksums can be replaced alongside artifacts] → Explain integrity-only scope and defer authenticated signing.

## Migration Plan

Once core integration points are available, add the build path and PR checks, test a release in a non-production repository or draft, then enable tag-driven publication with appropriate token permissions. Existing installations are unaffected. Disable the workflow to stop future releases; delete an unpublished draft after confirming ownership. Do not automatically delete or overwrite published releases.

## Open Questions

- Which actual GL.iNet models and firmware/kernel versions must be qualified, and are additional MIPS or ARMv6 targets needed?
- The inspected GL-E5800 NA Mudi 7 reports aarch64, GL.iNet 4.10.0, OpenWrt 23.05.4 and kernel 5.15.170-perf; when and how will its arm64 binary be runtime-qualified with explicit authorization?
- What entrypoint, role smoke-test commands, version variables, and minimum Go version will the core expose?
- Who owns release authorization and provides a safe repository for end-to-end publication testing?
