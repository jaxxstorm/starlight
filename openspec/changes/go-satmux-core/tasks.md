## 1. Wire protocol

- [x] 1.1 Implement v3 handshake and directional key derivation.
- [x] 1.2 Implement framing, compression and encrypted payloads.
- [x] 1.3 Test against upstream-compatible vectors and malformed inputs.

## 2. Forwarding runtime

- [x] 2.1 Implement multiplexed ground server and client carrier pool.
- [x] 2.2 Implement SOCKS5, HTTP, fast fetch and Unix TLS bridge.
- [x] 2.3 Test basic TCP and ground-fetch interoperability with upstream client and server.
- [x] 2.4 Expand compatibility coverage for TLS, redirects, failures, shutdown and all proxy modes; fix observed redirect differences.
- [x] 2.5 Apply best-effort upstream TCP socket tuning, including Linux BBR and MSS, and validate supported builds.
- [ ] 2.6 Validate the static ARM64 executable, CA bundle discovery and available socket options on the target OpenWrt environment when authorized and available.
- [ ] 2.7 Expand upstream compatibility and stress coverage for slow consumers, concurrent load, malformed traffic and edge-case HTTP semantics.

## 3. Ground readiness and configuration

- [x] 3.1 Validate client destination, key and carrier configuration before starting carrier goroutines.
- [x] 3.2 Expose a bounded active authenticated-carrier probe and a separate passive snapshot for fail-open consumers.
- [x] 3.3 Test readiness transitions, timeouts, wrong replies, recovery and invalid configuration.
