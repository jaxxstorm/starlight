# Platforms and building

## What runs where?

| Component | Platform scope |
| --- | --- |
| Go client and server | Not tied to GL.iNet; use the binary on a compatible OS/architecture. Development and tests have run on macOS, with live deployment on Linux amd64 and arm64. |
| BBR/MSS socket tuning | Linux-specific, best effort; depends on kernel support. |
| Transparent router capture | Linux original-destination lookup and nftables. Other Linux routers need topology and firewall validation. |
| Persistent router package | OpenWrt `opkg` and `procd`, currently built for `aarch64_cortex-a53`. |
| Portal page | GL.iNet OUI components and RPC; not a generic OpenWrt/LuCI application. |

The tested router is the **GL.iNet GL-E5800 NA Mudi 7**, running GL.iNet 4.10.0 / OpenWrt 23.05.4. Other GL.iNet models and stock OpenWrt devices are not automatically qualified. The Go core can be used independently of the router integration; Windows has not been validated.

## Build from source

Install Go 1.23 or newer, then from the repository root:

```sh
go build -o starlight ./cmd/starlight
```

For a Linux amd64 server:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o starlight-linux-amd64 ./cmd/starlight
```

For an ARM64 Linux router:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o starlight-linux-arm64 ./cmd/starlight
```

Build for the machine that will execute the binary, not necessarily your development machine. Cross-compilation alone does not prove device compatibility.

The binary contains `client`, `server`, `probe`, and `bench`. There is no separate client/server executable or Python runtime requirement. Published releases and a signed router package feed are not yet established; use source builds for now.

Next: [client usage](usage.md), [persistent server](ground-server.md), or [router installation](router.md).
