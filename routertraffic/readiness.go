package routertraffic

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// CaptureHealth checks the actual client pool and whether the relay's listening
// socket accepts local TCP connections. It does not prove firewall routing,
// destination forwarding, or health after this point-in-time check.
// The caller must supply a deadline and arrange independent lease expiry.
func CaptureHealth(ctx context.Context, listenerAddress string, probe func(context.Context) error) error {
	if _, ok := ctx.Deadline(); !ok || listenerAddress == "" || probe == nil {
		return errors.New("routertraffic: capture health requires deadline, listener address and probe")
	}
	host, _, err := net.SplitHostPort(listenerAddress)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("routertraffic: health listener address must be numeric loopback")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp4", listenerAddress)
	if err != nil {
		return fmt.Errorf("routertraffic: relay listener: %w", err)
	}
	_ = conn.Close()
	if err := probe(ctx); err != nil {
		return fmt.Errorf("routertraffic: client pool: %w", err)
	}
	return ctx.Err()
}
