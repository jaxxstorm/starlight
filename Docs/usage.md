# Using Starlight

## Shared key and connection

Client and server must use the same hexadecimal pre-shared key (PSK). Generate a 32-byte key with `openssl rand -hex 32`, store it privately, and transfer it securely. Prefer `SATMUX_PSK` over `--psk-hex` so the key is not in process arguments. Privileged users can still inspect process environments.

```sh
SATMUX_PSK="$(cat starlight.psk)" ./starlight client --host GROUND_SERVER --port 18444
```

The CLI defaults to ground port **8443**, so specify the port explicitly when using another value. The default carrier pool has 12 connections; change it with `--carriers`.

## Client endpoints

| Endpoint | Default | Option |
| --- | --- | --- |
| SOCKS5 | `127.0.0.1:1080` | `--socks-port` |
| HTTP proxy / CONNECT | `127.0.0.1:8080` | `--http-port` |
| Ground fetch | `127.0.0.1:8081` | `--fast-port` |

For example:

```sh
curl --socks5-hostname 127.0.0.1:1080 https://example.com
curl --proxy http://127.0.0.1:8080 https://example.com
curl http://127.0.0.1:8081/https://example.com
```

The SOCKS and CONNECT paths preserve application TLS to the origin. Ground fetch performs DNS and TLS at the ground server and compresses responses for the return path. It is a different path from transparent TCP forwarding.

These proxies bind to loopback. Starting the client alone does not route other devices automatically. See [router-wide routing](router.md#router-wide-routing).

An optional Unix TLS bridge is available with `--gh-sock /private/directory/starlight.sock`; the directory should be private and the socket path must not already exist. It does not automatically configure GitHub CLI.

## Test connectivity

```sh
SATMUX_PSK="$(cat starlight.psk)" ./starlight probe --host GROUND_SERVER --port 18444
```

`probe` authenticates a fresh connection and verifies a ping/pong within eight seconds. Success exits with status 0; failure prints an error and exits nonzero. It does not change configuration or establish that origin requests or firewall forwarding work.

## Benchmark

With the client running:

```sh
./starlight bench --fast-port 8081
```

The command emits JSON comparing direct requests and ground fetch against the same six public sites used by upstream's `satmux-bench`: GitHub repository/API, Hacker News, MDN, Cloudflare, and PyPI. Each phase runs six requests concurrently, first ground fetch, then direct.

Results include status, total time, time to response headers, bytes, failures, and burst wall time. Each request has a 14-second timeout and 2 MiB response cap. A run can consume several MiB of data. Failures and capped responses are not speedup results.

On the router this measures router-originated requests, which bypass LAN capture. It is not a bandwidth test or a measurement of a connected client's traffic. A stopped client produces ground-fetch failures, not silent direct fallback. The [portal](router.md#portal-controls) displays these results as a table.

## Traffic and protocol notes

- Starlight reduces avoidable overhead on high-latency links; it does not remove satellite propagation delay or bypass captive-portal authentication.
- Router-wide capture currently handles IPv4 TCP only. DNS, UDP (including QUIC), and IPv6 use existing router policies.
- Failure policy is **fail open**: new connections return to ordinary routing when capture is inactive. Existing connections may fail and need retrying.
- The upstream-compatible protocol authenticates the connection handshake, but does not authenticate individual data frames or encrypt all traffic. Starlight is not a general-purpose VPN or privacy boundary.
- Changing the PSK on one side disconnects it until the other side has the same key.

## Troubleshooting

1. Run `probe` to check address, port, key, and ground-server reachability.
2. Try an explicit proxy request to distinguish core forwarding from router capture.
3. Check local port conflicts; GL.iNet uses port 8080 for its web interface on the tested device.
4. For an airline login page, disable router-wide routing, authenticate to the network, then enable routing again.
5. For router service logs and controls, see [router operations](router.md#service-and-recovery).
