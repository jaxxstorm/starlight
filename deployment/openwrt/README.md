# Experimental OpenWrt client package

This is an **offline prototype**, not a published or supported package. Do not install it on the active router: target-specific installation, procd behavior, upgrades, rollback, removal, package trust and GL.iNet Manage Sources have not been validated. In particular, this package does not yet meet the change's verified-installation and failed-upgrade rollback requirements. It has no maintainer scripts, so it does not enable or start a service by itself. It creates no feed, firewall rules or transparent traffic integration.

Build locally with a compatible Go toolchain and the current core:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /path/to/starlight ./cmd/starlight
python3 deployment/openwrt/build_ipk.py --binary /path/to/starlight --version 0.1.0-1 --output /path/to/starlight-client_0.1.0-1_aarch64_cortex-a53.ipk
python3 -m unittest discover -s deployment/openwrt -p 'test_*.py' -v
```

The builder checks basic ELF structure, class, byte order, machine, loadable segments and absence of a dynamic interpreter. It does not prove static linking, kernel ABI, Go build provenance, binary integrity or runtime functionality. `--output` must not already exist. The IPK architecture reflects the inspected router's opkg architecture; this does not qualify the device. Index creation, feed signing, trust provisioning and publication are deliberately not implemented here.

The package contains `/usr/sbin/starlight`, `/etc/init.d/starlight`, and the opkg-preserved `/etc/starlight/client.conf` (mode 0600). Before future service activation, an administrator must fill in `host`, numeric `port`, and an even-length hexadecimal `psk`, with a matching reachable ground server and PSK. Configuration must remain root-owned, mode 0600 or 0400. Values are parsed as data, not sourced as shell; duplicate or unknown keys are rejected. The PSK is passed to procd in the `SATMUX_PSK` environment rather than a command-line argument; privileged users and the supervisor can access process environment/configuration, so this is not a secret isolation boundary. Do not put credentials in a published package or feed.

Optional settings `carriers`, `socks_port`, `http_port`, and `fast_port` default to 12, 1080, 8080, and 8081. The experimental wrapper accepts 1–64 carriers and distinct listener ports 1–65535, written as decimal without leading zeroes. This limit is a deployment guard, not a measured target resource budget. Operators must coordinate listener ports with other local services; validation cannot reserve ports or prevent races. The wrapper does not change network or firewall configuration.

The current core exposes local loopback SOCKS, HTTP proxy and fast-fetch listeners, not router-wide interception. It has no documented standalone validation or readiness command; the service performs limited syntax checks only. A reachable, compatible, separately configured ground server is required for actual forwarding. Use an isolated test device to verify configuration, service enabling/start/stop, reboot, restart limits, opkg behavior and security before deciding on production integration. Do not infer package-manager rollback, uninstall retention, or authenticated-feed behavior from these local tests.

The optional `transparent_port` setting enables an experimental IPv4 listener on all interfaces. It defaults to `0` (disabled); a nonzero value must not collide with the proxy ports. It does not create firewall rules, health-gate capture, or restrict access, and is not ready for production use. Do not enable it on the router until the firewall and recovery work is validated.

For the optional native, offline core smoke test, build a native binary in a temporary directory and run `STARLIGHT_TEST_BINARY=/path/to/native/starlight python3 -m unittest discover -s deployment/openwrt -p 'test_runtime.py' -v`. The test uses loopback only, without a ground server. It demonstrates that a local listener and running process are not proof that a ground connection is ready; it does not validate successful forwarding or router service behavior.

## Core coordination needed

- Agree on a supported, side-effect-free configuration validation contract, including server address, key handling and resource limits, so installation can validate before activation without duplicating core rules.
- Agree on an authenticated ground-carrier readiness and failure signal suitable for startup verification and upgrade rollback. Today `NewClient` returns before background connections and handshakes succeed, so process or listener status is insufficient.
- Confirm a stable client invocation, bounded carrier/resource configuration, graceful shutdown, credential delivery and loopback listener contract. A real ground server address and matching PSK must be provided independently; none are packaged here.
- Coordinate any future transparent interception and traffic failure behavior separately; this prototype does not implement it.

## Lifecycle safety gap

`routerinstall.StageVerified` can locally stage and verify bytes without replacing existing files, and tests cover cleanup and preservation of pre-existing test files on failed staging. It is not wired into opkg or the procd service and provides no activation, persistent transaction journal, restart, readiness verification or rollback. The IPK has no verified mechanism to prevent opkg from replacing the previous executable during an upgrade. Do not install or upgrade it on an active router until the package-manager transaction and recovery path have been designed and tested on an isolated device.
