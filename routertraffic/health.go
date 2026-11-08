package routertraffic

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/jaxxstorm/starlight/satmux"
)

// ProbeGround authenticates a fresh carrier to the ground server and verifies
// a matching protocol ping/pong. The context must have a deadline. This probe
// does not prove that the existing client pool, local listener, firewall,
// destination reachability, or the router's forwarding path is healthy; all
// relevant checks must succeed before a capture lease is renewed.
func ProbeGround(ctx context.Context, address string, psk []byte) error {
	deadline, ok := ctx.Deadline()
	if !ok || address == "" || len(psk) == 0 {
		return errors.New("routertraffic: ground probe requires deadline, address, and PSK")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("routertraffic: ground probe dial: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("routertraffic: ground probe deadline: %w", err)
	}
	var session [16]byte
	if _, err := io.ReadFull(rand.Reader, session[:]); err != nil {
		return fmt.Errorf("routertraffic: ground probe random session: %w", err)
	}
	if _, err := satmux.ClientHandshake(conn, psk, session[:]); err != nil {
		return fmt.Errorf("routertraffic: ground probe authentication: %w", err)
	}
	var payload [8]byte
	if _, err := io.ReadFull(rand.Reader, payload[:]); err != nil {
		return fmt.Errorf("routertraffic: ground probe random payload: %w", err)
	}
	if err := satmux.WriteFrame(conn, satmux.Frame{Type: satmux.Ping, Payload: payload[:]}); err != nil {
		return fmt.Errorf("routertraffic: ground probe ping: %w", err)
	}
	frame, err := satmux.ReadFrame(conn)
	if err != nil {
		return fmt.Errorf("routertraffic: ground probe pong: %w", err)
	}
	if frame.Type != satmux.Pong || frame.StreamID != 0 || !bytes.Equal(frame.Payload, payload[:]) {
		return errors.New("routertraffic: ground probe invalid pong")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
