package satmux

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// ServeHTTPProxy serves HTTP proxy requests and CONNECT tunnels. The listener
// must not be exposed to untrusted clients without an external access policy.
func ServeHTTPProxy(ctx context.Context, listener net.Listener, client *Client) error {
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
			_ = handleHTTPProxy(ctx, conn, client)
		}()
	}
}

func handleHTTPProxy(ctx context.Context, conn net.Conn, client *Client) error {
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	defer req.Body.Close()
	address := req.Host
	if req.URL.Host != "" {
		address = req.URL.Host
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		port := "80"
		if req.Method == http.MethodConnect {
			port = "443"
		}
		address = net.JoinHostPort(strings.Trim(address, "[]"), port)
	}
	openCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	s, err := client.OpenAddress(openCtx, address)
	if err != nil {
		_, _ = io.WriteString(conn, "HTTP/1.1 502 Bad Gateway\r\nConnection: close\r\n\r\n")
		return err
	}
	defer s.Close()
	stop := context.AfterFunc(ctx, func() { s.Close() })
	defer stop()
	_ = conn.SetReadDeadline(time.Time{})
	if req.Method == http.MethodConnect {
		if _, err = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			return err
		}
	} else {
		if req.URL.IsAbs() && req.URL.Scheme != "http" {
			return fmt.Errorf("satmux: unsupported proxy scheme")
		}
		req.RequestURI = ""
		for k := range req.Header {
			if strings.HasPrefix(strings.ToLower(k), "proxy-") {
				req.Header.Del(k)
			}
		}
		if err = req.Write(s); err != nil {
			return err
		}
	}
	go func() { _, _ = io.Copy(s, reader); _ = s.CloseWrite() }()
	_, err = io.Copy(conn, s)
	return err
}
