# Persistent ground server

Use a Linux host reachable from your client, ideally near the network's terrestrial egress. The same Starlight binary provides the server role.

## Install a systemd service

Build for the server architecture using [the build guide](installing.md). Copy the binary and `deployment/ground-server/starlight-ground.service` to the host, then run as root from the directory containing them:

```sh
install -d -m 0700 /etc/starlight-ground
install -d -m 0755 /usr/local/libexec
install -m 0755 starlight-linux-amd64 /usr/local/libexec/starlight-ground
install -m 0644 starlight-ground.service /etc/systemd/system/starlight-ground.service
```

Create a key only if none is already configured:

```sh
umask 077
set -C
printf 'SATMUX_PSK=%s\n' "$(openssl rand -hex 32)" > /etc/starlight-ground/secret.env
set +C
```

Alternatively, put your existing hex key in that file as `SATMUX_PSK=<hex-key>` and set its mode to `0600`. A PSK downloaded from the router is a plain hex-key file, not an environment file; add the `SATMUX_PSK=` prefix when provisioning this service.

## Listen address

The supplied unit defaults to **127.0.0.1:18444**, suitable for a local or protected transport. To accept remote client connections, add a systemd override:

```sh
install -d -m 0755 /etc/systemd/system/starlight-ground.service.d
cat > /etc/systemd/system/starlight-ground.service.d/listen.conf <<'EOF'
[Service]
ExecStart=
ExecStart=/usr/local/libexec/starlight-ground server --host 0.0.0.0 --port 18444
EOF
systemctl daemon-reload
systemctl enable --now starlight-ground.service
```

If the service was already running, restart it to apply the override. Allow inbound TCP 18444 in the host and provider firewalls as appropriate. Use a different port if another service already owns it. See the [protocol notes](usage.md#traffic-and-protocol-notes) before choosing public exposure.

## Operate and verify

```sh
systemctl status starlight-ground.service
journalctl -u starlight-ground.service
systemctl restart starlight-ground.service
```

Provision the same key on the client, run `starlight probe`, then make a request through a client proxy. An active systemd unit alone does not prove external reachability or forwarding.

To stop persistent operation:

```sh
systemctl disable --now starlight-ground.service
```

The service uses `/usr/local/libexec/starlight-ground` and `/etc/starlight-ground/secret.env`. Changing the key requires coordinating clients and restarting the service. Restarts drop active streams.
