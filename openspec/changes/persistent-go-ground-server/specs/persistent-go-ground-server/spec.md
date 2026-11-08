## ADDED Requirements

### Requirement: Independent persistent ground server
The system SHALL provide a separate systemd unit that starts the Go server at boot and restarts it on failure, without changing the existing Python service.

#### Scenario: Independent listener
- **WHEN** the unit starts with its default configuration
- **THEN** it listens on loopback port 18444 without binding port 8444.

### Requirement: Private credentials
The deployment SHALL use a separately generated PSK stored in a root-only configuration file and SHALL NOT put the PSK in command-line arguments or logs.

#### Scenario: Missing credential
- **WHEN** the credential file is absent or invalid
- **THEN** the server fails to start rather than using a default key.

### Requirement: Documented operation
The deployment SHALL document installation, verification, network exposure risks, and independent rollback.

#### Scenario: Controlled removal
- **WHEN** an operator rolls back the Go service
- **THEN** the documented procedure leaves the Python service and its files unchanged.
