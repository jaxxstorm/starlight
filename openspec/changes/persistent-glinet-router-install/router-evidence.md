## Read-only router inspection

The user identified the accessible device as GL.iNet GL-E5800 NA Mudi 7 5G. Read-only SSH inspection yielded the following snapshot; it does not qualify the target for installation or prove behavior across reboots.

| Source | Observation |
| --- | --- |
| `/etc/glversion` | `4.10.0` |
| `/etc/openwrt_release` | OpenWrt `23.05.4`, revision `r24012-d8dd03c46f`, target `sdx75/generic`, architecture `aarch64_cortex-a53`, taints `busybox override` |
| `uname` | machine `aarch64`, kernel `5.15.170-perf` |
| `/tmp/sysinfo/model` | `GL.iNet E5800, Qualcomm Technologies, Inc. SDXPINN IDP MBB` |
| `/tmp/sysinfo/board_name` | `qcom,sdxpinn-idp` (generic identifier; not sufficient alone for model detection) |
| `ps` | PID 1 `/sbin/procd` |
| `command -v` | `/sbin/fw4` and `/usr/sbin/nft` exist; active rules and reload behavior were not inspected |
| `df -k /overlay` | 3,021,608 KiB total and 2,804,004 KiB available at inspection; space is dynamic |
| `/proc/mounts` | `/overlay` is ext4 read-write, used as the root overlay upper layer |
| `/proc/meminfo` | 1,636,624 KiB total and 1,148,140 KiB available at inspection; availability is dynamic |
| `opkg print-architecture` | `all`, `noarch`, and `aarch64_cortex-a53` are accepted; architecture match is necessary, not sufficient, for compatibility |
| package tools | `/bin/opkg` reports version `d038e5b6d155784575f62a66a8bb7e874173e92e` (2022-02-24); `/usr/bin/usign` exists and opkg exposes `--verify-program` |
| opkg configuration | `/etc/opkg.conf` specifies root and ram destinations, `/var/opkg-lists`, and `/overlay`; `/etc/opkg/customfeeds.conf` shows a `src/gz` example and `/etc/opkg/distfeeds.conf` contains three GL.iNet HTTPS `src/gz` sources |
| trust material | `/etc/opkg/keys` contains two existing public-key files; neither authorizes a Starlight feed, and actual signature enforcement and UI key provisioning remain unverified |

No router state was intentionally changed; no service, package, firewall, network, upgrade, restart, or reboot test was performed. Memory and free space snapshots are not application budgets. The architecture string alone does not validate a Go build or core dependencies. Need a reliable model/firmware detector, a trusted artifact source, core interfaces, on-device execution and lifecycle tests, resource measurements and separate firewall coordination before declaring support. SSH host key and management access must be independently trusted by the operator.

## Executable smoke test

A CGO-disabled Linux ARM64 build of `cmd/starlight` was copied temporarily to `/tmp` and invoked with `client --help` on the router. It printed the client usage and exited with status 1 (`flag: help requested`). The temporary binary was removed afterwards. This confirms basic execution, not network operation, installation, persistence, or lifecycle behavior.

A subsequent temporary client run connected to the existing upstream Python satmux ground server on TCP port 8444. A router-originated HTTPS request to example.com through the local HTTP proxy returned HTTP 200. The client and temporary files were removed afterwards; the existing ground server was not stopped. This verifies one proxy path, not router-wide routing or persistence.

An additional test used temporary Go builds for both the ground server (Linux amd64, TCP port 18444) and router client (Linux arm64), with a separate test PSK. A router-originated HTTPS request through the client's local HTTP proxy returned HTTP 200. Both test processes and their remote files were removed; the existing Python server remained running. This does not validate persistent operation or router-wide routing.

## Experimental persistent client

Installed experimental `starlight-client` 0.1.0-4 via opkg and enabled its procd service on the router. The package omitted its configuration directory; it was created manually with mode 0700, and a root-only 0600 client configuration provisioned for the public Go ground service. Router port 8080 is occupied by uhttpd, so the client uses loopback ports 31080, 31081 and 31082. A router-originated HTTPS request through port 31081 returned HTTP 200. The package builder was subsequently fixed locally to include the directory; that fix is not in installed version 0.1.0-4. `transparent_port` remains disabled; no firewall capture or settings UI is installed. Reboot, upgrades and removal remain untested. procd service details expose its environment, including the PSK, to callers authorized to inspect it; do not treat it as isolated from privileged users. A credential observed during diagnosis was rotated on both endpoints.

Subsequently copied experimental LuCI controller/model files and GL.iNet OUI menu/view files onto the router outside opkg. The view is served with HTTP 200 and embeds the authenticated LuCI page; the LuCI route requires login. The OUI menu and browser interactions have not been verified in an authenticated browser. The page has client service controls but explicitly reports router-wide capture unavailable; it does not manage firewall rules. Files copied outside opkg are not owned by the installed package and will not automatically be removed with it.

The LuCI integration was subsequently removed at the user's request and replaced by a native OUI component and an authenticated `starlight` RPC object. On-device RPC tests confirmed unauthenticated access is rejected and locally authorized calls can stop/disable and enable/start the client; the client was left enabled and a proxied HTTPS request returned HTTP 200. An authenticated browser test of the OUI component remains outstanding. Router-wide capture is explicitly unavailable; no firewall rules were changed. OUI files were installed outside opkg and are not owned by the installed 0.1.0-4 package.

The OUI component and RPC were subsequently extended with server address/port configuration, PSK input and generation, and a probe that authenticates and exchanges a ping with the configured ground server. Generation returns a new key for browser download without changing the configured key; saving applies the entered key. On-device tests verified generation length without recording the key, successful authenticated connectivity, saving the unchanged address/port, and restoration of active router-wide capture after restart. The browser download and rendering remain unverified. The page and helpers are still installed outside the package. The router's portal is accessed over HTTP in the observed setup; entered or downloaded keys are not protected from observers on that local connection.
