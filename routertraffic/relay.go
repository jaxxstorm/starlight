// Package routertraffic provides building blocks for transparent TCP routing.
// It does not install firewall rules or enable interception.
package routertraffic

import (
	"context"
	"errors"
	"io"
	"net"
	"time"
)

// Stream is the subset of the core stream interface required by the relay.
type Stream interface {
	io.ReadWriteCloser
	CloseWrite() error
}

// OpenStream opens an optimistic multiplexed stream to an IP and port. A
// successful return does not imply that the ground server reached the target.
type OpenStream func(context.Context, string, int) (Stream, error)

// DialDirect opens a normal router-originated TCP connection to the target.
type DialDirect func(context.Context, string, string) (net.Conn, error)

// Relay opens a stream and relays a single intercepted connection. If opening
// the stream fails before forwarding data, it tries direct routing instead.
// It never retries or replays a stream after opening it. The returned boolean
// reports whether a direct connection was used. The caller owns inbound.
// This function is not a substitute for an independent firewall fail-open
// mechanism when the process or listener is unavailable.
func Relay(ctx context.Context, inbound net.Conn, target *net.TCPAddr, open OpenStream, direct DialDirect, timeout time.Duration) (bool, error) {
	if inbound == nil || target == nil || target.IP.To4() == nil || target.Port < 1 || target.Port > 65535 || open == nil || direct == nil || timeout <= 0 {
		return false, errors.New("routertraffic: invalid relay configuration")
	}
	openCtx, cancel := context.WithTimeout(ctx, timeout)
	stream, err := open(openCtx, target.IP.String(), target.Port)
	cancel()
	if ctx.Err() != nil {
		if stream != nil {
			_ = stream.Close()
		}
		return false, ctx.Err()
	}
	if err != nil {
		if stream != nil {
			_ = stream.Close()
		}
		dialCtx, cancel := context.WithTimeout(ctx, timeout)
		conn, dialErr := direct(dialCtx, "tcp4", target.String())
		cancel()
		if dialErr != nil {
			if conn != nil {
				_ = conn.Close()
			}
			return false, dialErr
		}
		if conn == nil {
			return false, errors.New("routertraffic: dialer returned a nil connection")
		}
		defer conn.Close()
		return true, copyBoth(ctx, inbound, conn)
	}
	if stream == nil {
		return false, errors.New("routertraffic: opener returned a nil stream")
	}
	defer stream.Close()
	return false, copyBoth(ctx, inbound, stream)
}

func copyBoth(ctx context.Context, inbound net.Conn, outbound io.ReadWriteCloser) error {
	stop := context.AfterFunc(ctx, func() { _ = inbound.Close(); _ = outbound.Close() })
	defer stop()
	type result struct {
		err       error
		halfClose bool
	}
	done := make(chan result, 2)
	go func() {
		_, err := io.Copy(outbound, inbound)
		halfClose := false
		if w, ok := outbound.(interface{ CloseWrite() error }); ok {
			halfClose = w.CloseWrite() == nil
		}
		done <- result{err, halfClose}
	}()
	go func() {
		_, err := io.Copy(inbound, outbound)
		halfClose := false
		if w, ok := inbound.(interface{ CloseWrite() error }); ok {
			halfClose = w.CloseWrite() == nil
		}
		done <- result{err, halfClose}
	}()
	first := <-done
	if first.err != nil || !first.halfClose {
		_ = inbound.Close()
		_ = outbound.Close()
	}
	second := <-done
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if first.err != nil {
		return first.err
	}
	return second.err
}
