package satmux

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientConfigValidation(t *testing.T) {
	valid := ClientConfig{ServerAddress: "example.com:8443", PSK: []byte("key"), Carriers: 1}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, addr := range []string{"", "example.com", ":443", "0.0.0.0:443", "[::]:443", "host:0", "host:65536", "host:http", "bad host:443", "host:+2", "bad/path:443", "[invalid::ip]:443"} {
		cfg := valid
		cfg.ServerAddress = addr
		if err := cfg.Validate(); err == nil {
			t.Errorf("accepted %q", addr)
		}
	}
	for _, addr := range []string{"[::1]:8443", "127.0.0.1:1", "example.com:65535"} {
		cfg := valid
		cfg.ServerAddress = addr
		if err := cfg.Validate(); err != nil {
			t.Errorf("rejected %q: %v", addr, err)
		}
	}
	for _, cfg := range []ClientConfig{{ServerAddress: valid.ServerAddress, Carriers: 1}, {ServerAddress: valid.ServerAddress, PSK: valid.PSK}} {
		if err := cfg.Validate(); err == nil {
			t.Error("invalid config accepted")
		}
	}
	if _, err := NewClient(context.Background(), "host:0", []byte("key"), 1); err == nil {
		t.Fatal("NewClient accepted invalid config")
	}
	key, err := ParsePSKHex("01 23\nff")
	if err != nil || string(key) != "\x01\x23\xff" {
		t.Fatalf("key %x: %v", key, err)
	}
	for _, input := range []string{"", "a b", "secret", "aa0"} {
		if _, err := ParsePSKHex(input); err == nil || strings.Contains(err.Error(), input) && input != "" {
			t.Fatalf("bad key error for %q: %v", input, err)
		}
	}
}

func waitReady(t *testing.T, c *Client) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for !c.Readiness().Ready {
		select {
		case <-deadline.C:
			t.Fatal("carrier did not authenticate")
		case <-ticker.C:
		}
	}
}

func TestProbeMatchingPongAndRecovery(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	var good atomic.Bool
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err := ServerHandshake(conn, []byte("test")); err != nil {
			return
		}
		for {
			f, err := ReadFrame(conn)
			if err != nil {
				return
			}
			if f.Type != Ping {
				continue
			}
			if !good.Load() {
				f.Payload[0] ^= 0xff
			}
			if WriteFrame(conn, Frame{Type: Pong, Payload: f.Payload}) != nil {
				return
			}
		}
	}()
	client, err := NewClient(context.Background(), l.Addr().String(), []byte("test"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	waitReady(t, client)
	if got := client.Readiness(); got.ActiveCarriers != 1 || got.ConfiguredCarriers != 1 {
		t.Fatalf("snapshot: %+v", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = client.Probe(ctx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wrong PONG probe: %v", err)
	}
	if !client.Readiness().Ready {
		t.Fatal("wrong PONG should not silently redefine passive snapshot")
	}
	good.Store(true)
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		err = client.Probe(ctx)
		if err == nil {
			break
		}
		if !errors.Is(err, ErrProbeBusy) {
			t.Fatalf("recovery probe: %v", err)
		}
		select {
		case <-ctx.Done():
			t.Fatal("probe never released")
		case <-time.After(time.Millisecond):
		}
	}
	client.Close()
	if got := client.Readiness(); got.Ready || got.ActiveCarriers != 0 {
		t.Fatalf("closed snapshot: %+v", got)
	}
	if !errors.Is(client.Probe(context.Background()), ErrNotReady) {
		t.Fatal("closed client reported healthy")
	}
}

func TestProbeHealthyServerAndNoCarrier(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- Serve(ctx, l, []byte("test")) }()
	c, err := NewClient(ctx, l.Addr().String(), []byte("test"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	waitReady(t, c)
	probeCtx, stop := context.WithTimeout(ctx, time.Second)
	defer stop()
	if err := c.Probe(probeCtx); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
	if err := c.Probe(context.Background()); !errors.Is(err, ErrNotReady) {
		t.Fatalf("after shutdown: %v", err)
	}
}

func TestProbeDeadlineWhileWriterBlocked(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	keys := Keys{C2S: make([]byte, 32), S2C: make([]byte, 32)}
	cc := newCarrier(a, keys, false)
	defer cc.close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &Client{ctx: ctx, carriers: []*carrier{cc}}
	cc.writeMu.Lock()
	probeCtx, stop := context.WithTimeout(ctx, 50*time.Millisecond)
	err := c.Probe(probeCtx)
	stop()
	cc.writeMu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked probe: %v", err)
	}
}

func TestProbeWriteCancellation(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	keys := Keys{C2S: make([]byte, 32), S2C: make([]byte, 32)}
	cc := newCarrier(a, keys, false)
	defer cc.close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &Client{ctx: ctx, carriers: []*carrier{cc}}
	probeCtx, stop := context.WithTimeout(ctx, 50*time.Millisecond)
	defer stop()
	if err := c.Probe(probeCtx); !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, ErrNotReady) {
		t.Fatalf("stalled write: %v", err)
	}
	select {
	case <-cc.done:
	case <-time.After(time.Second):
		t.Fatal("timed-out frame write did not close carrier")
	}
	if _, err := b.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("peer not closed: %v", err)
	}
}

func TestProbeAnyCarrierAndBusy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	keys := Keys{C2S: make([]byte, 32), S2C: make([]byte, 32)}
	a, peerA := net.Pipe()
	b, peerB := net.Pipe()
	defer peerA.Close()
	defer peerB.Close()
	first, second := newCarrier(a, keys, false), newCarrier(b, keys, false)
	defer first.close()
	defer second.close()
	go first.run(ctx)
	go second.run(ctx)
	go func() {
		for {
			if _, err := ReadFrame(peerA); err != nil {
				return
			}
		}
	}()
	go func() {
		for {
			f, err := ReadFrame(peerB)
			if err != nil {
				return
			}
			if f.Type == Ping {
				if WriteFrame(peerB, Frame{Type: Pong, Payload: f.Payload}) != nil {
					return
				}
			}
		}
	}()
	c := &Client{ctx: ctx, carriers: []*carrier{first, second}}
	probeCtx, stop := context.WithTimeout(ctx, time.Second)
	defer stop()
	if err := c.Probe(probeCtx); err != nil {
		t.Fatalf("responsive second carrier: %v", err)
	}
	// Both gates are occupied: a concurrent check must not return a stale
	// success or wait indefinitely.
	first.probeGate <- struct{}{}
	second.probeGate <- struct{}{}
	if err := c.Probe(probeCtx); !errors.Is(err, ErrProbeBusy) {
		t.Fatalf("busy probe: %v", err)
	}
	<-first.probeGate
	<-second.probeGate
}
