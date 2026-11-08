package routertraffic

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/jaxxstorm/starlight/satmux"
)

func localListener(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func TestListenerWithCore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ground := localListener(t)
	go func() { _ = satmux.Serve(ctx, ground, []byte("test")) }()
	core, err := satmux.NewClient(ctx, ground.Addr().String(), []byte("test"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	origin := localListener(t)
	go func() {
		c, err := origin.Accept()
		if err == nil {
			defer c.Close()
			_, _ = io.Copy(c, c)
		}
	}()
	listener := localListener(t)
	s, err := NewListener(ListenerConfig{MaxConnections: 2, OpenTimeout: 5 * time.Second, MaxLifetime: 10 * time.Second,
		Destination: func(net.Conn) (*net.TCPAddr, error) { return origin.Addr().(*net.TCPAddr), nil },
		Open:        CoreOpener(core), Direct: (&net.Dialer{}).DialContext})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	c, err := net.Dial("tcp4", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = c.SetDeadline(time.Now().Add(8 * time.Second))
	if _, err := c.Write([]byte("through-core")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len("through-core"))
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "through-core" {
		t.Fatalf("response %q: %v", buf, err)
	}
	_ = c.Close()
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := s.Counters(); got.Accepted != 1 || got.Direct != 0 || got.Active != 0 {
		t.Fatalf("unexpected counters: %+v", got)
	}
}

func TestListenerRejectsOverCapacityAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener := localListener(t)
	entered := make(chan struct{})
	s, err := NewListener(ListenerConfig{MaxConnections: 1, OpenTimeout: time.Minute, MaxLifetime: time.Minute,
		Destination: func(net.Conn) (*net.TCPAddr, error) { return &net.TCPAddr{IP: net.IPv4(203, 0, 113, 1), Port: 80}, nil },
		Open: func(ctx context.Context, _ string, _ int) (Stream, error) {
			close(entered)
			<-ctx.Done()
			return nil, ctx.Err()
		},
		Direct: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("unexpected direct dial")
		}})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	first, err := net.Dial("tcp4", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	<-entered
	second, err := net.Dial("tcp4", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = second.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = second.Read(make([]byte, 1))
	_ = second.Close()
	if err == nil {
		t.Fatal("excess connection was not closed")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := s.Counters(); got.Accepted != 1 || got.Rejected != 1 || got.Active != 0 {
		t.Fatalf("unexpected counters: %+v", got)
	}
}

func TestListenerFallsBackToDirect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	origin := localListener(t)
	go func() {
		conn, err := origin.Accept()
		if err == nil {
			defer conn.Close()
			_, _ = io.Copy(conn, conn)
		}
	}()
	listener := localListener(t)
	s, err := NewListener(ListenerConfig{MaxConnections: 1, OpenTimeout: time.Second, MaxLifetime: 5 * time.Second,
		Destination: func(net.Conn) (*net.TCPAddr, error) { return origin.Addr().(*net.TCPAddr), nil },
		Open:        func(context.Context, string, int) (Stream, error) { return nil, errors.New("offline") },
		Direct:      (&net.Dialer{}).DialContext})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	conn, err := net.Dial("tcp4", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = conn.Write([]byte("direct"))
	buf := make([]byte, 6)
	if _, err := io.ReadFull(conn, buf); err != nil || string(buf) != "direct" {
		t.Fatalf("response %q: %v", buf, err)
	}
	_ = conn.Close()
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := s.Counters(); got.Direct != 1 || got.Active != 0 {
		t.Fatalf("unexpected counters: %+v", got)
	}
}
