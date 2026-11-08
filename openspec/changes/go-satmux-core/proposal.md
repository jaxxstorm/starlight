## Why

Starlight needs a Go implementation of satmux's carrier protocol and client and server behavior so it can interoperate with the upstream implementation on routers and ground servers.

## What Changes

- Implement the authenticated v3 handshake, framing, compression and encrypted fetch encoding.
- Implement multiplexed TCP forwarding, warm carriers, local proxies and ground fetch compatible with upstream.
- Validate protocol interoperability and failure handling.

## Capabilities

### New Capabilities
- `satmux-core`: Compatible carrier protocol and client/server forwarding facilities.

### Modified Capabilities

None.

## Impact

Go core packages and tests; upstream satmux peers. Router integration and packaging are outside this change.
