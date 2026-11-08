## 1. Coordination and feasibility gates

- [ ] 1.1 Agree and record with core owners the stream-open interface, IP destination handling, readiness, error/close behavior, security and upstream interoperability; do not implement the adapter until agreed.
- [ ] 1.2 Identify IPv6 topology and demonstrate original-destination recovery, timed lease expiry and fail-open reload/reboot/crash behavior on the GL-E5800 NA Mudi 7 5G. Read-only inspection has identified GL.iNet 4.10.0, OpenWrt 23.05.4 and fw4/nftables; no capture has been tested.
- [ ] 1.3 Agree service, packaging, privilege, startup-order and firewall reload responsibilities with deployment/release owners; establish measured resource budgets.

## 2. Traffic adapter and limits

- [ ] 2.1 Implement and unit-test the transparent TCP listener and original IPv4 destination recovery against the agreed core interface.
- [ ] 2.2 Implement bidirectional relay, cancellation, stream failure and cleanup without TLS termination, with direct fallback before bytes flow and no replay; test success and error paths.
- [ ] 2.3 Implement and test finite configurable connection/buffer limits and timeouts, including overload rejection and counters.

## 3. Router lifecycle and policy

- [ ] 3.1 Implement prerequisite and configuration validation, opt-in enable/disable and owned firewall rule management for the validated backend.
- [ ] 3.2 Implement LAN-forward-only capture and exclusions for local/router destinations, configured bypasses and ground endpoints; verify ordering and loop prevention.
- [ ] 3.3 Implement restart, reboot, firewall reload and interrupted-update reconciliation with fail-open behavior and independently verified cleanup after crash, plus safe removal of owned state on disable/uninstall.
- [ ] 3.4 Integrate readiness, status, counters and least-privilege service startup with the agreed deployment lifecycle.

## 4. Verification and documentation

- [ ] 4.1 Add integration tests with multiple unconfigured LAN clients and a compatible core/ground server for original destination, bidirectional TCP, TLS, concurrent flows and upstream interoperability.
- [ ] 4.2 Test router/local/carrier and explicit bypass exclusions, portal access, UDP/QUIC, IPv6 and DNS behavior, and ensure no misleading privacy claims.
- [ ] 4.3 Inject listener/core failure, activation failure, firewall reload, process crash, reboot and interrupted rule changes; verify observable direct fallback for new flows, no residual owned rules after disable, and preservation of unrelated firewall rules.
- [ ] 4.6 Measure the lease expiry and health-detection window, NAT conntrack behavior, DNS chain priority and flow-offload interactions; test whether previously redirected tuples require client retry and document the resulting outage bounds.
- [ ] 4.4 Run resource and sustained-load tests on each declared router target, validating limits, timeouts, reconnect behavior and a documented memory/CPU envelope.
- [ ] 4.5 Document supported firmware/backend, setup, enable/disable, reboot behavior, scope, DNS/IPv6/UDP limitations, portal recovery, troubleshooting and rollback; obtain core and deployment owner review.
