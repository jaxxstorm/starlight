# GL.iNet router installation and usage

The experimental integration runs the Go client under OpenWrt `procd` and provides a native **Applications → Starlight** page in GL.iNet's portal. It does not use LuCI.

Live testing used a GL-E5800 NA Mudi 7 on GL.iNet 4.10.0 / OpenWrt 23.05.4. IPv4 TCP forwarding, a ground-server outage/recovery, capture disable/enable, and one physical reboot were exercised. Firmware reloads, upgrades, uninstall cleanup, and sustained operation are not fully validated.

## Build and install

From the repository root, with Go and Python 3 installed:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o starlight-linux-arm64 ./cmd/starlight
python3 deployment/openwrt/build_ipk.py \
  --binary starlight-linux-arm64 --version 0.1.0-1 \
  --output starlight-client_0.1.0-1_aarch64_cortex-a53.ipk
scp -O starlight-client_0.1.0-1_aarch64_cortex-a53.ipk root@192.168.8.1:/tmp/
```

The builder refuses to overwrite an existing output file. The package architecture is currently fixed to `aarch64_cortex-a53`; it includes GL.iNet-specific portal files and is not a generic stock-OpenWrt package.

On the router:

```sh
opkg install /tmp/starlight-client_0.1.0-1_aarch64_cortex-a53.ipk
```

The package does not automatically start or enable the service. The previously tested router was also updated manually outside opkg; that deployment is not evidence of a verified fresh-install or upgrade transaction. Back up configuration before replacing an existing installation. A signed feed for **Manage Sources** and automatic upgrade rollback are not implemented yet.

## Configure the client

Edit `/etc/starlight/client.conf` on the router:

```ini
host=GROUND_SERVER_IPV4
port=18444
psk=YOUR_HEXADECIMAL_PSK
carriers=12
socks_port=31080
http_port=31081
fast_port=31082
transparent_port=0
router_wide=0
```

Replace the placeholders; the key must match the ground server. The example avoids port 8080, used by the web interface on the tested device. Listener ports must be distinct. Keep the configuration root-owned and private:

```sh
chmod 700 /etc/starlight
chmod 600 /etc/starlight/client.conf
/etc/init.d/starlight enable
/etc/init.d/starlight start
```

Refresh the portal. Loading newly installed RPC code may require `nginx -s reload` on this firmware. This reload does not install firewall rules.

## Portal controls

In **Applications → Starlight**:

- **Starlight client:** start/enable or stop/disable the client service.
- **Router-wide traffic:** persist the optional capture setting and apply it by restarting a running client. The client must be running for capture to become active.
- **Ground server:** edit address and port. Router-wide mode currently requires an IPv4 address.
- **PSK:** leave blank to keep the saved key, or enter a replacement hex key.
- **Generate and download PSK:** generate a new key without immediately applying it. Configure the ground server with the downloaded key, then save it on the router.
- **Test connectivity:** authenticate to the ground server using saved settings.
- **Run benchmark:** compare direct router requests and ground fetch; see [benchmark details](usage.md#benchmark).

Normal status responses do not return the saved PSK. If the portal is accessed over HTTP, credentials entered or downloaded are not encrypted on the local connection.

## Router-wide routing

The current service captures eligible **IPv4 TCP** connections arriving on `br-lan`. It excludes router/local destinations, the discovered LAN prefix, the configured ground address, and TCP port 53. Router-originated traffic is not captured. DNS, UDP/QUIC, and IPv6 continue using existing routing.

The client owns the `inet starlight_router` nftables table. It renews a 15-second kernel-expiring lease while its relay and carrier checks succeed. On failure, new connections fall back to normal routing once capture is revoked or expires. Existing connections and previously redirected connection-tracking entries may fail rather than recover transparently.

This is topology-specific experimental code, not a generic router VPN. It assumes `br-lan`; additional LANs, guest networks, portal bypasses and other local networks need explicit validation. See [platform scope](installing.md).

## Service and recovery

```sh
/etc/init.d/starlight stop
/etc/init.d/starlight start
/etc/init.d/starlight disable
/etc/init.d/starlight enable
logread -e starlight
```

For a coordinated restart of the running installation:

```sh
/usr/libexec/starlight-restart
```

This waits for the previous service and capture lease before starting the replacement. For captive-portal login, disable router-wide traffic, complete login, then re-enable it.

Stop and disable the service before removing or upgrading the experimental package. Confirm that the Starlight table is inactive or removed; no maintainer scripts currently guarantee cleanup on package removal. Files copied manually outside opkg are not removed by uninstalling the package. Firmware-upgrade retention has not been validated.
