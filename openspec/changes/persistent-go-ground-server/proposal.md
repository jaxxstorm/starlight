## Why

The Go ground server needs a persistent, independently managed deployment without disturbing the existing Python service.

## What Changes

- Provide a dedicated systemd unit and operator instructions for the existing Go server mode.
- Keep credentials private and configure a separate listener and lifecycle.
- Validate startup, forwarding, failure and shutdown, and record deployment state.

## Capabilities

### New Capabilities
- `persistent-go-ground-server`: Independent persistent operation of the Go ground server.

### Modified Capabilities

None.

## Impact

Linux systemd service assets and deployment documentation; no wire protocol or existing service changes.
