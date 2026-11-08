## ADDED Requirements

### Requirement: Explicit and scoped client traffic integration
The deployment SHALL offer traffic integration only on a verified router and only when the core provides a supported interception interface. It SHALL be disabled by default and require explicit operator opt-in and selection of eligible connected-client ingress and traffic. Documentation SHALL identify supported protocols, exclusions, and direct-routing privacy implications; unsupported traffic SHALL NOT be claimed as proxied.

#### Scenario: Default installation
- **WHEN** installation completes without traffic integration opt-in
- **THEN** no Starlight interception rules are active and existing routing remains unchanged

#### Scenario: Eligible client traffic
- **WHEN** an operator enables integration and a connected client sends traffic within the documented supported scope
- **THEN** the router directs that traffic to the ready core without requiring client-side Starlight installation

#### Scenario: Unsupported core capability
- **WHEN** the core lacks a verified interception interface for a traffic type
- **THEN** the deployment does not offer that traffic type as supported

### Requirement: Avoid loops and preserve management
Interception SHALL exclude the core's own upstream connections, router management and other traffic required to maintain router operation, and SHALL modify only identifiable Starlight-owned firewall resources. It SHALL use the selected firmware's supported firewall mechanism and coexist with unrelated rules.

#### Scenario: Core upstream and management traffic
- **WHEN** integration is active and the core connects to its ground server or an administrator accesses the router
- **THEN** those connections are not redirected back into Starlight and management remains accessible

### Requirement: Reconcile interception with service readiness
The deployment SHALL activate interception only after the core is ready and SHALL remove or bypass it when the service is stopped, disabled, unavailable, or removed. It SHALL reconcile owned rules after reboot, unexpected termination, and firewall reload without duplicates or stale redirection; if the target cannot provide this behavior it SHALL NOT be supported for integration. When interception is unavailable, existing normal routing SHALL be restored and the operator SHALL be able to observe the unavailable state.

#### Scenario: Core fails after activation
- **WHEN** the core terminates unexpectedly while interception is active
- **THEN** interception is removed or bypassed, normal routing is restored, and the failure is observable

#### Scenario: Firewall reload or reboot
- **WHEN** the firewall reloads or the router reboots with integration enabled
- **THEN** owned rules are reconciled without duplicates and redirect traffic only after the core is ready

#### Scenario: Integration disabled or uninstalled
- **WHEN** the operator disables integration or removes Starlight
- **THEN** all Starlight-owned interception hooks and rules are removed without removing unrelated firewall rules
