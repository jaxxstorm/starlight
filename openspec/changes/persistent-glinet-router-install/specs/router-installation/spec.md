## ADDED Requirements

### Requirement: GL.iNet package feed installation
The deployment SHALL provide an opkg-compatible IPK and HTTPS package feed for the verified target that an operator can add using GL.iNet Plug-ins > Manage Sources. The feed SHALL include discoverable versioned package metadata, architecture, package filename, size and cryptographic digest, and SHALL provide authenticated metadata using a documented trust anchor and provisioning procedure. Feed and package operations SHALL preserve the installation, upgrade, rollback, configuration and removal requirements in this specification; they SHALL NOT require disabling signature, certificate, or checksum verification.

#### Scenario: Install from configured source
- **WHEN** an operator adds the documented authenticated feed to a verified router and refreshes packages in the GL.iNet interface
- **THEN** the compatible Starlight package is discoverable and installable through the interface

#### Scenario: Invalid or untrusted feed
- **WHEN** feed metadata or package verification fails or the feed trust anchor is unavailable
- **THEN** installation or upgrade is rejected without disabling verification or activating the package

#### Scenario: Upgrade through package manager
- **WHEN** an operator upgrades the package through the GL.iNet interface or opkg
- **THEN** the same configuration preservation, startup verification and failed-upgrade recovery requirements apply as for any other installation method

### Requirement: Verified target and installation preflight
The deployment SHALL document at least one verified GL.iNet model and firmware version, architecture/ABI, dependencies, storage and memory requirements, and supported traffic scope. The installer SHALL check detectable compatibility, available storage, and artifact integrity before changing the router, and SHALL refuse unsupported or unverifiable configurations with an actionable error.

#### Scenario: Unsupported router
- **WHEN** installation preflight detects an unsupported architecture or firmware
- **THEN** installation stops before modifying service, configuration, or firewall state and explains the incompatibility

#### Scenario: Insufficient space or invalid artifact
- **WHEN** available storage is insufficient for staging and rollback or artifact integrity verification fails
- **THEN** installation stops before activating changes and reports the cause

### Requirement: Persistent supervised service
The installation SHALL place the compatible Go executable and service assets in persistent storage and use the verified firmware's native service supervisor to enable, disable, start, stop, and inspect the service. An enabled service SHALL start after reboot and recover from unexpected termination with bounded restart behavior.

#### Scenario: Reboot with service enabled
- **WHEN** the router reboots after a successful installation and service enablement
- **THEN** the supervisor starts the configured service without manual intervention

#### Scenario: Repeated process failures
- **WHEN** the service repeatedly exits unexpectedly
- **THEN** the supervisor limits restart attempts or frequency and exposes the failure to the operator

### Requirement: Secure configuration and validation
The deployment SHALL keep configuration and credentials separate from the executable, restrict access to authorized administrators, validate configuration using the core's supported mechanism before enabling or activating it, and avoid exposing secrets in diagnostics or logs.

#### Scenario: Invalid configuration
- **WHEN** an operator attempts to enable or start with invalid configuration
- **THEN** activation fails with an actionable diagnostic without disclosing credentials

### Requirement: Reversible upgrade and removal
An upgrade SHALL preserve configuration and previous working executable and enabled state until the new service is verified; a failed activation SHALL restore the previous executable and enabled state and report the failure. Removal SHALL disable the service and remove only Starlight-owned assets, retain configuration by default, and offer an explicit credential purge.

#### Scenario: Failed upgrade
- **WHEN** a staged upgrade fails configuration or startup verification
- **THEN** the prior executable and enabled state are restored and the operator is informed

#### Scenario: Ordinary removal and explicit purge
- **WHEN** an operator removes the installation without requesting purge
- **THEN** the service and owned runtime assets are removed and configuration is retained
- **WHEN** the operator explicitly requests purge
- **THEN** retained Starlight configuration and credentials are also removed

### Requirement: Operational guidance and diagnostics
The deployment SHALL document installation, configuration, status, logs, enablement, upgrade, recovery, removal, expected resource usage, and firmware upgrade limitations. Status SHALL distinguish running, stopped, and failed states without exposing secrets.

#### Scenario: Diagnose stopped service
- **WHEN** an operator checks a stopped or failed service
- **THEN** the status and documentation identify its state and how to inspect the relevant diagnostics
