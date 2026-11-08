# Independent Go ground service

This unit runs the existing Go `starlight server` separately from any Python satmux instance. It binds **127.0.0.1:18444**, not 8444. Loopback is not reachable directly from a router: use an independently secured tunnel, or explicitly review and configure external exposure. Satmux v3 authenticates the handshake but does not authenticate data frames or encrypt all traffic. It is not a privacy guarantee. An authenticated client can proxy outbound connections, including to addresses reachable from the server; restrict access and trust clients accordingly.

## Install on a systemd Linux host

Build a tested binary for the host architecture, e.g. `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o starlight-ground ./cmd/starlight`. Transfer the binary and unit over a trusted channel, then as root:

```sh
install -d -m 0700 /etc/starlight-ground
install -d -m 0755 /usr/local/libexec
install -m 0755 starlight-ground /usr/local/libexec/starlight-ground
install -m 0644 starlight-ground.service /etc/systemd/system/starlight-ground.service
```

Before generating a key, check `/etc/starlight-ground/secret.env` does not already exist; do not overwrite a deployed key. Generate a fresh 32-byte PSK locally on the host without printing it (requires OpenSSL):

```sh
umask 077
set -C
printf 'SATMUX_PSK=%s\n' "$(openssl rand -hex 32)" > /etc/starlight-ground/secret.env
set +C
chmod 0600 /etc/starlight-ground/secret.env
systemctl daemon-reload
systemctl enable --now starlight-ground.service
```

The environment file is read by systemd and must be root-owned; the key is passed in the service environment, not its command line. Privileged users and the service itself can read that environment. Provision the same key to trusted clients over a secure channel without printing it to logs, shell history, or tickets. Do not reuse the Python service key.

Check `systemctl status starlight-ground.service`, `journalctl -u starlight-ground.service`, and `ss -ltn` for the loopback listener. Test an authenticated client through the intended protected transport and verify both forwarding and authentication failure. Verify the existing Python listener independently. An active systemd unit alone does not establish interoperability or public reachability.

## Rollback

Stop and disable only `starlight-ground.service`; remove its unit, binary, and secret only after confirming they belong to this deployment and retaining the secret if rollback is temporary. Run `systemctl daemon-reload`. Never stop or remove the Python service or its files. Restarting the Go service drops its active streams.
