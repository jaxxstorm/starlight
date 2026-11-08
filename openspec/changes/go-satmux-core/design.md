## Context

The repository starts without an implementation. Upstream `satmux.py` defines the v3 protocol and behavior.

## Goals / Non-Goals

**Goals:** Compatible client/server protocol and forwarding with explicit tests.

**Non-Goals:** Router setup, installation and release tooling.

## Decisions

Keep protocol primitives separate from forwarding and local proxy runtime, so interoperability can be tested independently. Preserve the exact upstream handshake and wire encoding rather than invent a replacement. Use bounded decoding and synchronized writes for safe concurrent use.

Expose the implementation as `satmux` package APIs and a `cmd/starlight` executable. Proxy listeners default to loopback. The Unix TLS bridge is opt-in in the executable; its socket must be created in a directory protected from untrusted users, and existing paths are not removed.

## Risks / Trade-offs

- Upstream encrypts fetch payloads but does not authenticate frames; wire compatibility preserves this limitation rather than claiming authenticated encryption.
- Full forwarding and proxy behavior require integration tests with actual upstream peers; unit tests alone do not demonstrate compatibility.
- Ground fetch uses Go's HTTP transport, so edge cases including redirects and response headers require further comparison with upstream. The implementation limits decompressed data and uses bounded per-stream buffering rather than upstream's unbounded pending-open list.
- Linux BBR, MSS, buffer sizing and keepalive options are best-effort; unsupported kernel options do not cause connection failure. Cross-compilation does not prove runtime BBR availability.
- The intended router is ARM64 GL.iNet 4.10.0 / OpenWrt 23.05.4. A static Linux ARM64 executable builds locally; execution, certificate-store discovery and kernel feature availability remain unverified on that device.
- A passive carrier count is not proof of path health. Fail-open consumers should periodically use a deadline-bounded matching PING/PONG probe over the authenticated carrier pool and apply their own freshness and hysteresis policy. The wire protocol does not authenticate individual frames; a probe does not validate arbitrary origin connectivity or local routing.
