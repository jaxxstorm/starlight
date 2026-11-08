## Why

Starlight needs a repeatable way to deliver its Go client and ground server to router and server operators without requiring a Go toolchain on each device. Defining the build and release contract now lets the concurrent core work expose the minimal integration points without changing its behavior or protocol.

## What Changes

- Define a documented, CGO-free cross-build process producing a single executable containing the client and ground-server functionality for each supported target.
- Establish an explicit initial router/server target matrix and artifact naming, version metadata, checksums, and release publication behavior.
- Add automated build and release validation, including failure handling, and document local builds, release operation, and target limitations.
- Coordinate the entrypoint and build metadata with the concurrent core; do not implement application behavior or router service installation here.

## Capabilities

### New Capabilities
- `cross-platform-releases`: Cross-platform executable builds, release artifacts, integrity checks, and publication.

### Modified Capabilities

None.

## Impact

Build scripts or configuration, CI/release automation, and operator documentation will be added after the Go module and executable entrypoint exist. The core must support a CGO-free build and both roles in one executable, and agree on version injection; otherwise the integration must be resolved rather than silently omitting a role or target. Hosted release publication requires repository permissions and a release provider. No protocol, security, router startup, or installation behavior changes are intended.
