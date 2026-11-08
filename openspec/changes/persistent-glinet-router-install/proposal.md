## Why

Router users need Starlight to survive reboots and serve connected devices without installing software on each device. A controlled installation and lifecycle are needed on resource-constrained GL.iNet routers without changing the core protocol or weakening its security.

## What Changes

- Define a supported, explicitly verified GL.iNet/OpenWrt deployment target and deliver a router-compatible single-binary installation with configuration and service management.
- Distribute an OpenWrt package through a package feed that users can add in GL.iNet Plug-ins > Manage Sources, once the UI, trust and lifecycle behavior are verified.
- Provide opt-in network-wide traffic integration using the core's supported interception interface, with safe rollback and no persistent firewall changes until validated.
- Document installation, upgrades, diagnostics, recovery, and removal, including resource and compatibility limits.
- Verify lifecycle and traffic behavior on a representative target; coordinate with the concurrently developed core rather than prescribing an unverified interface.
- Use the user-selected fail-open policy when traffic integration becomes available; reconcile the separate traffic change before implementing that policy.

## Capabilities

### New Capabilities

- `router-installation`: Persistent installation, configuration, service lifecycle, upgrade, removal, and operational diagnostics on supported GL.iNet routers.
- `router-traffic-integration`: Opt-in routing of eligible connected-client traffic through the core, with explicit scope, safety, and recovery requirements.

### Modified Capabilities

None.

## Impact

Adds router deployment assets, documentation, automated tests, and a release build for a verified router architecture. The initial candidate is GL.iNet GL-E5800 NA Mudi 7 5G; its firmware and hardware capabilities are not yet verified. Depends on the core's actual executable, configuration, traffic interception, security, and readiness interfaces, and on a compatible GL.iNet firmware's init and firewall facilities. No changes to the upstream-compatible wire protocol are intended.
