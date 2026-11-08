package satmux

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// ServeGHTLS serves HTTP requests on a caller-created Unix listener, forwarding
// them through the carrier with locally verified TLS. The caller is responsible
// for securely creating the socket and restricting its filesystem permissions.
func ServeGHTLS(ctx context.Context, listener net.Listener, client *Client) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { <-ctx.Done(); listener.Close() }()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go func() {
			defer conn.Close()
			stop := context.AfterFunc(ctx, func() { conn.Close() })
			defer stop()
			_ = handleGHTLS(ctx, conn, client)
		}()
	}
}

func handleGHTLS(ctx context.Context, conn net.Conn, client *Client) error {
	return handleGHTLSWithConfig(ctx, conn, client, nil)
}

func handleGHTLSWithConfig(ctx context.Context, conn net.Conn, client *Client, config *tls.Config) error {
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return err
	}
	defer req.Body.Close()
	address := req.Host
	if address == "" {
		address = "api.github.com"
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = strings.Trim(address, "[]")
		address = net.JoinHostPort(host, "443")
	}
	openCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	s, err := client.OpenAddress(openCtx, address)
	if err != nil {
		return err
	}
	defer s.Close()
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()
	stop := context.AfterFunc(ctx, func() { local.Close(); remote.Close(); s.Close() })
	defer stop()
	go func() { _, _ = io.Copy(s, remote); _ = s.CloseWrite() }()
	go func() { _, _ = io.Copy(remote, s); remote.Close() }()
	if config == nil {
		config = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		config = config.Clone()
	}
	config.ServerName = host
	secure := tls.Client(local, config)
	defer secure.Close()
	handshakeCtx, stopHandshake := context.WithTimeout(ctx, 15*time.Second)
	defer stopHandshake()
	if err = secure.HandshakeContext(handshakeCtx); err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Time{})
	req.RequestURI = ""
	req.Close = true
	req.Header.Del("Connection")
	if err = req.Write(secure); err != nil {
		return err
	}
	_, err = io.Copy(conn, secure)
	return err
}
