## Context

The Go core already implements the ground server. A Python server is running independently on port 8444.

## Goals / Non-Goals

**Goals:** Provide a persistent least-privilege systemd service, private credential setup, isolated listener, and operational verification.

**Non-Goals:** Altering the existing Python server, changing the protocol, or asserting public network security.

## Decisions

Use a separate systemd unit with an unprivileged dynamic user and a root-only EnvironmentFile. This avoids putting the PSK in command-line arguments and does not require modifying the concurrently developed Go executable. Bind loopback by default; operators must explicitly evaluate and configure any external exposure. Use port 18444 to avoid the existing listener. Enable restart on failure and boot via systemd.

## Risks / Trade-offs

- The compatible protocol does not authenticate data frames and traffic is not fully encrypted; do not describe it as a secure tunnel. Restrict network access and use a protected transport if needed.
- Environment variables can be inspected by privileged processes and the service user; protect the host and never log or share the key.
- The shared core is under active development; deployment requires an immutable tested build and independent compatibility validation.

## Migration Plan

Install under distinct paths, generate a fresh private key, test locally, and enable the new unit independently. Roll back only the new unit and its files; leave Python and its port untouched.
