## Why

Clients connected to a travel router cannot benefit from satmux-style multiplexing without individually configuring a proxy. A router-wide integration is needed, but it must coordinate with the concurrently developed client protocol rather than inventing another wire format.

## What Changes

- Transparently capture eligible LAN-originated TCP connections and relay them through the existing Starlight client, preserving their original destinations.
- Define explicit DNS, UDP/IPv6, local-network, captive-portal, bypass, failure, and recovery behavior rather than claiming all traffic is accelerated.
- Add opt-in, reversible router traffic integration with least-privilege boundaries, loop prevention, status, and bounded resource usage.
- Fail open to normal routing when the service or ground server is unavailable; disclose that traffic may bypass Starlight.
- Coordinate the integration contract and release/deployment ownership before implementation; do not modify satmux wire semantics or weaken security.

## Capabilities

### New Capabilities

- `router-traffic`: Router-wide transparent TCP interception, destination preservation, traffic policy, safety, and operational lifecycle.

### Modified Capabilities

None.

## Impact

The initial target is GL.iNet GL-E5800 NA Mudi 7 5G, with firmware unspecified and no hardware currently available. The Go client exposes an optimistic destination-aware stream interface, subject to core coordination. Router firewall, routing, DNS, privileges, and service lifecycle are affected; exact supported firmware and backend require validation. Ground-server and wire protocol remain owned by core; packaging and autostart coordinate with deployment/release work.
