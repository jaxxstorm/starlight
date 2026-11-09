# Starlight

Starlight is a Go rewrite of [satmux](https://github.com/Th0rgal/satmux), the satellite Wi-Fi accelerator created by **Th0rgal**. Credit for the original design and protocol belongs to that project.

Starlight multiplexes connections over persistent TCP connections to a ground server to improve browsing on high-latency networks, particularly inflight Wi-Fi. A single Go binary runs either the client or the server. An experimental GL.iNet/OpenWrt integration adds persistent operation, router-wide IPv4 TCP routing, and controls in the router's portal.

## Quickstart

Build with Go 1.23 or newer:

```sh
git clone https://github.com/jaxxstorm/starlight.git
cd starlight
go build -o starlight ./cmd/starlight
```

Generate a shared key once and securely copy it to both machines:

```sh
umask 077
openssl rand -hex 32 > starlight.psk
```

On a reachable ground server, allow inbound TCP port 18444 and run:

```sh
SATMUX_PSK="$(cat starlight.psk)" ./starlight server --host 0.0.0.0 --port 18444
```

On your client, replace `GROUND_SERVER` with that server's address:

```sh
SATMUX_PSK="$(cat starlight.psk)" ./starlight client --host GROUND_SERVER --port 18444
```

In another terminal, try the local HTTP proxy:

```sh
curl --proxy http://127.0.0.1:8080 https://example.com
```

## Documentation

The detailed guides are viewable with Docsify. See [local preview instructions](Docs/development.md#preview-the-documentation).

- [Platforms and building](Docs/installing.md)
- [Client usage and benchmarking](Docs/usage.md)
- [Persistent ground server](Docs/ground-server.md)
- [GL.iNet router installation and portal](Docs/router.md)
- [Development and release tooling](Docs/development.md)

The core client/server are not GL.iNet-specific. The current router package and portal are. Router-wide routing currently covers IPv4 TCP; DNS, UDP/QUIC, and IPv6 follow normal routing. Starlight is experimental and is not a general-purpose VPN; see the [protocol and traffic notes](Docs/usage.md#traffic-and-protocol-notes).
