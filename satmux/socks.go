package satmux

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"time"
)

// ServeSOCKS serves unauthenticated SOCKS5 CONNECT. Exposing this listener to
// untrusted networks allows them to use the ground server as a proxy.
func ServeSOCKS(ctx context.Context, listener net.Listener, client *Client) error {
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
			_ = handleSOCKS(ctx, conn, client)
		}()
	}
}

func handleSOCKS(ctx context.Context, conn net.Conn, client *Client) error {
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	var greeting [2]byte
	if _, err := io.ReadFull(conn, greeting[:]); err != nil {
		return err
	}
	if greeting[0] != 5 {
		return errors.New("satmux: invalid SOCKS version")
	}
	methods := make([]byte, int(greeting[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}
	ok := false
	for _, method := range methods {
		if method == 0 {
			ok = true
		}
	}
	if !ok {
		_ = writeAll(conn, []byte{5, 255})
		return errors.New("satmux: no supported authentication")
	}
	if err := writeAll(conn, []byte{5, 0}); err != nil {
		return err
	}
	var req [4]byte
	if _, err := io.ReadFull(conn, req[:]); err != nil {
		return err
	}
	if req[0] != 5 || req[1] != 1 || req[2] != 0 {
		_ = socksReply(conn, 7)
		return errors.New("satmux: unsupported SOCKS request")
	}
	var host string
	switch req[3] {
	case 1, 4:
		n := 4
		if req[3] == 4 {
			n = 16
		}
		ip := make([]byte, n)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return err
		}
		host = net.IP(ip).String()
	case 3:
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			return err
		}
		b := make([]byte, int(n[0]))
		if _, err := io.ReadFull(conn, b); err != nil {
			return err
		}
		host = string(b)
	default:
		_ = socksReply(conn, 8)
		return errors.New("satmux: unsupported address type")
	}
	var b [2]byte
	if _, err := io.ReadFull(conn, b[:]); err != nil {
		return err
	}
	openCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	s, err := client.Open(openCtx, host, int(binary.BigEndian.Uint16(b[:])))
	if err != nil {
		_ = socksReply(conn, 4)
		return err
	}
	defer s.Close()
	stopStream := context.AfterFunc(ctx, func() { s.Close() })
	defer stopStream()
	if err := socksReply(conn, 0); err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Time{})
	go func() { _, _ = io.Copy(s, conn); _ = s.CloseWrite() }()
	_, err = io.Copy(conn, s)
	return err
}

func socksReply(w io.Writer, code byte) error {
	return writeAll(w, []byte{5, code, 0, 1, 0, 0, 0, 0, 0, 0})
}

// OpenAddress is a convenience wrapper for a host:port target.
func (c *Client) OpenAddress(ctx context.Context, address string) (*Stream, error) {
	host, p, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return nil, err
	}
	return c.Open(ctx, host, port)
}
