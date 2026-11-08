package routertraffic

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/jaxxstorm/starlight/satmux"
)

func TestProbeGroundAuthenticatedPing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ground := localListener(t)
	go func() { _ = satmux.Serve(ctx, ground, []byte("probe-key")) }()
	probeCtx, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	if err := ProbeGround(probeCtx, ground.Addr().String(), []byte("probe-key")); err != nil {
		t.Fatal(err)
	}
	if err := ProbeGround(probeCtx, ground.Addr().String(), []byte("wrong-key")); err == nil {
		t.Fatal("expected authentication failure")
	}
}

func TestProbeGroundStalledHandshake(t *testing.T) {
	listener := localListener(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err == nil {
			defer conn.Close()
			var b [1]byte
			_, _ = conn.Read(b[:])
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := ProbeGround(ctx, listener.Addr().String(), []byte("key")); err == nil {
		t.Fatal("expected timeout")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("probe did not close connection")
	}
}

func TestProbeGroundRequiresDeadline(t *testing.T) {
	if err := ProbeGround(context.Background(), net.JoinHostPort("127.0.0.1", "1"), []byte("key")); err == nil {
		t.Fatal("expected deadline error")
	}
}

func TestProbeGroundRejectsWrongPong(t *testing.T) {
	listener := localListener(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err := satmux.ServerHandshake(conn, []byte("key")); err != nil {
			return
		}
		if _, err := satmux.ReadFrame(conn); err != nil {
			return
		}
		_ = satmux.WriteFrame(conn, satmux.Frame{Type: satmux.Pong, Payload: []byte("wrong")})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := ProbeGround(ctx, listener.Addr().String(), []byte("key")); err == nil {
		t.Fatal("accepted invalid pong")
	}
	<-done
}
