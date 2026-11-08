package routertraffic

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jaxxstorm/starlight/satmux"
)

// CoreOpener adapts the current optimistic core stream API. It does not prove
// ground or destination health and must not be used as a firewall lease signal.
func CoreOpener(client *satmux.Client) OpenStream {
	return func(ctx context.Context, host string, port int) (Stream, error) {
		if client == nil {
			return nil, errors.New("routertraffic: nil core client")
		}
		stream, err := client.Open(ctx, host, port)
		if err != nil {
			return nil, err
		}
		return stream, nil
	}
}

// Destination returns the original IPv4 destination of an accepted connection.
type Destination func(net.Conn) (*net.TCPAddr, error)

// ListenerConfig defines finite per-listener resource and time limits.
type ListenerConfig struct {
	MaxConnections int
	OpenTimeout    time.Duration
	MaxLifetime    time.Duration
	Destination    Destination
	Open           OpenStream
	Direct         DialDirect
}

// Counters is a point-in-time snapshot, not a synchronized transaction.
type Counters struct {
	Accepted, Rejected, DestinationErrors, Direct, RelayErrors uint64
	Active                                                     int64
}

// Listener relays connections accepted from a caller-supplied listener. It
// never configures interception and is not a crash-safe firewall controller.
type Listener struct {
	config                                                     ListenerConfig
	accepted, rejected, destinationErrors, direct, relayErrors atomic.Uint64
	active                                                     atomic.Int64
}

func NewListener(c ListenerConfig) (*Listener, error) {
	if c.MaxConnections <= 0 || c.OpenTimeout <= 0 || c.MaxLifetime <= 0 || c.Destination == nil || c.Open == nil || c.Direct == nil {
		return nil, errors.New("routertraffic: invalid listener configuration")
	}
	return &Listener{config: c}, nil
}

func (s *Listener) Counters() Counters {
	return Counters{s.accepted.Load(), s.rejected.Load(), s.destinationErrors.Load(), s.direct.Load(), s.relayErrors.Load(), s.active.Load()}
}

// Serve owns listener and closes active connections on context cancellation or
// listener failure. A Listener must not be served concurrently more than once.
func (s *Listener) Serve(ctx context.Context, listener net.Listener) error {
	if listener == nil {
		return errors.New("routertraffic: nil listener")
	}
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	var wg sync.WaitGroup
	defer func() { cancel(); stop(); _ = listener.Close(); wg.Wait() }()
	slots := make(chan struct{}, s.config.MaxConnections)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		select {
		case slots <- struct{}{}:
		default:
			s.rejected.Add(1)
			_ = conn.Close()
			continue
		}
		s.accepted.Add(1)
		s.active.Add(1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots; s.active.Add(-1); _ = conn.Close() }()
			flowCtx, cancel := context.WithTimeout(ctx, s.config.MaxLifetime)
			defer cancel()
			target, err := s.config.Destination(conn)
			if err != nil {
				s.destinationErrors.Add(1)
				return
			}
			usedDirect, err := Relay(flowCtx, conn, target, s.config.Open, s.config.Direct, s.config.OpenTimeout)
			if usedDirect {
				s.direct.Add(1)
			}
			if err != nil {
				s.relayErrors.Add(1)
			}
		}()
	}
}
