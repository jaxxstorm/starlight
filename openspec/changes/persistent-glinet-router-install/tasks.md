## 1. Resolve integration and target dependencies

- [ ] 1.1 Verify the selected GL.iNet GL-E5800 NA Mudi 7 5G candidate and record firmware, CPU/ABI, persistent storage, init system, firewall backend and available resource budget; see `router-evidence.md` for read-only observations and outstanding qualification.
- [ ] 1.2 Coordinate and record the core's actual build, invocation, config validation, credentials, readiness, shutdown, transparent interception and supported traffic contracts; flag missing capabilities as blockers rather than inventing them.
- [ ] 1.3 Verify target-specific firewall reload, crash cleanup and boot ordering mechanisms; do not enable integration on a target where safe reconciliation cannot be demonstrated.
- [ ] 1.4 Establish a trusted release integrity verification method and document the trust anchor and artifact distribution process.
- [ ] 1.5 Reconcile the separate router-wide traffic change with the user's fail-open decision before implementing traffic integration.
- [ ] 1.6 Verify GL.iNet Manage Sources input, UI discovery/install and opkg signature enforcement/key provisioning on the selected firmware without disabling security checks; define an authenticated feed trust and rotation process.

## 2. Build and persistent lifecycle

- [ ] 2.1 Add a reproducible cross-build and distribution assets for the verified CPU/ABI, using the core's actual executable interface.
- [ ] 2.2 Implement compatibility, integrity, prerequisites and storage preflight checks before router changes, with actionable failures.
- [ ] 2.3 Install executable and service assets on persistent storage, with separate administrator-only configuration and credentials and supported configuration validation.
- [ ] 2.4 Implement native supervisor integration for enable, disable, start, stop, status, bounded restart and boot persistence.
- [ ] 2.5 Implement staged upgrade with configuration preservation, startup verification, and restoration of previous executable and enabled state on failure.
- [ ] 2.6 Implement removal of owned artifacts and service, default configuration retention and explicit credential purge.
- [ ] 2.7 Build a versioned `aarch64_cortex-a53` IPK and authenticated HTTPS feed index for the qualified target, with package metadata, config preservation, safe service lifecycle and dependencies; coordinate binary and publication with the release owner.
- [ ] 2.8 Verify that opkg and GL.iNet GUI upgrades, failures, removal and purge meet rollback, service and configuration requirements; do not publish if package-manager semantics cannot satisfy them.
- [x] 2.9 Build an explicitly experimental local client IPK and procd service prototype with private configuration and no automatic activation or firewall changes.

## 3. Opt-in traffic integration

- [ ] 3.1 Implement explicit opt-in and selection of supported client ingress and traffic, defaulting to no interception.
- [ ] 3.2 Implement target-native, identifiable firewall hooks scoped to supported traffic and exclusions for core upstream, management and router control traffic.
- [ ] 3.3 Implement readiness-gated activation and idempotent cleanup/reconciliation on stop, crash, disable, removal, reboot and firewall reload, restoring normal routing on failure.
- [ ] 3.4 Expose integration availability and failure state without leaking secrets; make direct-routing failure behavior visible before opt-in.

## 4. Verification and documentation

- [ ] 4.1 Add automated tests for preflight failures, config permissions and redaction, service controls, restart bounds, upgrade rollback, removal and purge.
- [ ] 4.2 Add automated tests for opt-in scope, upstream and management exclusions, readiness, repeated reconciliation, firewall reload, failure cleanup and preservation of unrelated rules.
- [ ] 4.3 On the selected physical router, verify install, reboot, restart, deliberate crash, failed upgrade and rollback, removal and independent management recovery; record firmware and test results.
- [ ] 4.4 On the selected router, verify eligible connected-client traffic reaches a real core and ground server, unsupported traffic behavior, loop avoidance, firewall reload and fail-open routing after failure.
- [ ] 4.5 Measure artifact/storage footprint, idle and loaded RAM/CPU, restart rate and logging behavior on target; document results and constraints.
- [ ] 4.6 Document target matrix, integrity verification, configuration, credentials, lifecycle, traffic coverage and direct-routing privacy implications, diagnostics, upgrade, recovery, removal and firmware upgrade limitations.
- [ ] 4.7 Test feed index and package integrity, signatures and trust failure, wrong architecture, package installation and upgrade via Manage Sources, and package removal on the qualified target; document source URL and trust bootstrap.
- [x] 4.8 Locally test prototype archive structure, reproducibility, architecture rejection, service syntax and mocked configuration validation; record remaining device and server dependencies.
