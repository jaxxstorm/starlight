## ADDED Requirements

### Requirement: Cross-platform single-executable builds
The build process SHALL produce a CGO-disabled, self-contained Starlight executable containing both client and ground-server functionality for each initial target: linux/amd64, linux/arm64, and linux/arm with GOARM=7. The build SHALL fail if any requested target cannot be built; it SHALL NOT silently omit a role or target.

#### Scenario: Build the supported matrix
- **WHEN** the documented matrix build runs against a compatible core and Go toolchain
- **THEN** it produces one executable for each of the three targets using the corresponding Go build settings

#### Scenario: Target build fails
- **WHEN** any target or required role cannot be built
- **THEN** the build reports failure and the release is not published

### Requirement: Identifiable and verifiable release artifacts
For a version tag, the release SHALL contain exactly one raw executable named `starlight_<version>_linux_amd64`, `starlight_<version>_linux_arm64`, and `starlight_<version>_linux_armv7`, and a `checksums.txt` containing SHA-256 checksums for those executables. Each executable SHALL expose its version and source revision through the core's agreed version interface. Untagged local builds SHALL identify themselves as development builds.

#### Scenario: Verify release downloads
- **WHEN** an operator downloads the executables and checksum file from a complete release
- **THEN** standard SHA-256 verification succeeds for every listed executable and the embedded version and revision identify the tagged source

#### Scenario: Modified binary
- **WHEN** a downloaded executable is modified after checksum generation
- **THEN** checksum verification fails for that executable

### Requirement: Automated gated publication
Pull-request automation SHALL run native tests and build all supported targets without publishing. A `v`-prefixed semantic version tag SHALL trigger release automation against the tagged commit. Release automation SHALL validate the tag, run tests and the complete matrix build, verify the checksum manifest, upload and verify the exact asset set in a draft GitHub release, and only then publish it. Failures SHALL prevent publication; an existing published release SHALL NOT be overwritten automatically.

#### Scenario: Successful tagged release
- **WHEN** a valid version tag is pushed and all checks and asset uploads succeed
- **THEN** a release for that tagged commit is published with the complete required asset set

#### Scenario: Release validation or upload fails
- **WHEN** a test, build, checksum verification, or upload fails
- **THEN** the workflow fails and no release from that run is published

#### Scenario: Invalid tag or published release collision
- **WHEN** a tag selected by the workflow is not a valid `v`-prefixed semantic version or a published release already exists for it
- **THEN** the workflow fails without overwriting a published release

### Requirement: Build and release operator guidance
Documentation SHALL describe prerequisites, local and matrix builds, target selection and limitations, development and release version identification, release permissions and trigger, checksum verification, executable permissions, failure recovery, and the distinction between integrity checks and authenticated signatures. It SHALL state that compilation alone does not establish device compatibility.

#### Scenario: Operator selects a router binary
- **WHEN** an operator consults the documentation for a router
- **THEN** they can identify the available Linux architectures and ARMv7 baseline, the need to check device and kernel compatibility, and that unsupported architectures such as MIPS and ARMv6 are not included

#### Scenario: Incomplete release recovery
- **WHEN** a release run leaves an unpublished draft
- **THEN** the documented recovery explains how to inspect and clean up the draft and retry without overwriting a published release

### Requirement: Build and release validation
Validation SHALL exercise native tests and native smoke tests of version reporting and both role entrypoints using the core's actual CLI without requiring live services or credentials. It SHALL verify the cross-build matrix and release manifest, including a negative checksum test. Runtime qualification on target hardware or emulation SHALL be recorded separately from cross-compilation.

#### Scenario: CI validation completes
- **WHEN** CI validates a compatible core
- **THEN** native tests and role/version smoke tests pass and all target builds complete

#### Scenario: Cross-compilation without device testing
- **WHEN** all cross-builds succeed but no target runtime tests are performed
- **THEN** validation reports compile coverage only and does not claim device runtime qualification
