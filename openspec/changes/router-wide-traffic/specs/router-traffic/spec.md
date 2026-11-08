## ADDED Requirements

### Requirement: Opt-in transparent LAN TCP relay
The system SHALL, when explicitly enabled on a supported router, intercept eligible forwarded LAN IPv4 TCP connections and relay them through the Starlight core client to their original IP address and port without client proxy configuration or application TLS termination. The core client SHALL use its existing compatible protocol and security behavior.

#### Scenario: LAN client connects to an external TCP service
- **WHEN** an eligible LAN client opens a TCP connection to an external IPv4 address
- **THEN** the router opens a core stream to that original address and port and relays bytes bidirectionally.

#### Scenario: Core stream cannot be established
- **WHEN** the core cannot open a captured connection
- **THEN** if no application bytes have been relayed, the router attempts a direct connection to the original destination; it SHALL NOT replay an established stream.

### Requirement: Explicit traffic scope and exclusions
The system SHALL exclude router-originated traffic, router and local-network destinations, configured bypass destinations, and core ground-server endpoints from interception. It SHALL document and expose that UDP including QUIC, IPv6, and DNS are not tunneled by this capability and follow existing router routing; DNS resolution may occur locally or upstream and is not guaranteed private. Operators SHALL be able to bypass portal destinations or temporarily disable capture for captive-portal login.

#### Scenario: Local and carrier connections
- **WHEN** a client connects to a local destination or the router opens a ground-server connection
- **THEN** that traffic is not redirected to the transparent listener.

#### Scenario: Unsupported protocol
- **WHEN** a client sends UDP or IPv6 traffic
- **THEN** it follows the pre-existing router policy without a claim of Starlight tunneling.

#### Scenario: Captive portal login
- **WHEN** an operator explicitly bypasses the portal or disables capture
- **THEN** portal traffic can follow existing router routing for login, subject to the router and portal policies.

### Requirement: Safe activation and fail-open operation
The system SHALL leave interception disabled by default and validate support and core/listener readiness before enabling it. When service or ground-server unavailability is detected, it SHALL restore normal routing for new, not-yet-conntracked connections within a documented, validated failure-detection and cleanup bound, report degraded operation when status is available, and disclose that traffic may bypass Starlight. Connections during that window and previously redirected conntrack tuples may fail; existing streams are not replayed. The operator SHALL be able to disable capture explicitly. Crash recovery SHALL NOT depend solely on cleanup by the failed process.

#### Scenario: Unavailable core at activation
- **WHEN** an operator enables capture while the core client or listener is not ready
- **THEN** activation fails and no capture rules are left active.

#### Scenario: Relay outage after activation
- **WHEN** the relay becomes unavailable while capture remains enabled
- **THEN** capture is removed or expires within the documented bound, subsequent new untracked connections follow normal routing, and available status reports the failure and direct fallback.

### Requirement: Reversible persistent lifecycle
The integration SHALL own and identify its router rules, preserve unrelated firewall and routing configuration, and reconcile its configured enabled or disabled state after service restart, router reboot, or firewall reload. Disabling or uninstalling SHALL remove only its owned interception state and restore prior routing behavior. Interrupted operations SHALL be recoverable without manual removal of unrelated rules.

#### Scenario: Enabled router restarts
- **WHEN** the router reboots with capture explicitly enabled
- **THEN** the service restores capture after readiness checks, or leaves normal routing in place and reports degraded status.

#### Scenario: Operator disables capture
- **WHEN** the operator disables capture
- **THEN** owned interception rules are removed and subsequent connections follow the router's prior routing policy.

#### Scenario: Firewall reload or interrupted update
- **WHEN** the firewall reloads or an integration update is interrupted
- **THEN** reconciliation restores capture only when ready or leaves normal routing in place, reports degraded status, and does not damage unrelated rules.

### Requirement: Bounded resources and operational visibility
The system SHALL provide finite configurable connection and buffer limits and timeouts, reject excess work, and expose enabled state, readiness, routing scope, failures and resource counters without logging application payloads or credentials. It SHALL document supported firmware, privileges, configuration, limitations, recovery and measured resource envelope.

#### Scenario: Connection limit reached
- **WHEN** another eligible connection arrives after the configured limit is reached
- **THEN** it is rejected without unbounded resource growth or direct fallback and the rejection is visible in status or counters.

#### Scenario: Operator inspects status
- **WHEN** the operator queries status during an outage
- **THEN** status distinguishes configured enablement from relay readiness and identifies the applicable non-tunneled traffic scope.
