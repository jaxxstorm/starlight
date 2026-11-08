## ADDED Requirements

### Requirement: Compatible carrier handshake
The system SHALL authenticate carrier peers and derive directional keys using the upstream v3 handshake.

#### Scenario: Invalid peer
- **WHEN** a peer sends an invalid handshake MAC
- **THEN** the connection is rejected.

### Requirement: Compatible frame encoding
The system SHALL encode and decode v3 frames, compressed data and encrypted fetch data according to upstream.

#### Scenario: Encrypted payload
- **WHEN** a payload is encoded for encrypted transfer
- **THEN** an upstream peer can decrypt and decompress it.

### Requirement: Multiplexed forwarding
The system SHALL provide upstream-compatible multiplexed TCP forwarding and local proxy and ground-fetch functionality.

#### Scenario: Optimistic open
- **WHEN** a local client opens a stream
- **THEN** the carrier can transmit its data without waiting for a ground connect response.

### Requirement: Best-effort TCP tuning
The system SHALL attempt upstream-compatible TCP socket tuning without rejecting a connection when an option is unavailable.

#### Scenario: BBR unavailable on Linux
- **WHEN** the kernel does not support the BBR congestion algorithm
- **THEN** the connection continues using the kernel's available congestion algorithm.

### Requirement: Client configuration validation
The system SHALL reject invalid client destination addresses, empty keys and nonpositive carrier counts before starting the client.

#### Scenario: Invalid destination
- **WHEN** the client destination has an invalid host or port
- **THEN** client configuration validation fails without connecting.

### Requirement: Ground readiness probe
The client SHALL offer a bounded active probe that succeeds only after a currently authenticated carrier returns a matching PONG.

#### Scenario: Silent carrier
- **WHEN** a carrier remains connected but does not return the probe PONG before the deadline
- **THEN** the probe fails and does not report ground readiness.

#### Scenario: Healthy carrier
- **WHEN** at least one authenticated carrier returns the matching PONG within the deadline
- **THEN** the probe succeeds without requiring every carrier to respond.
