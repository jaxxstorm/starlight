package routertraffic

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

type testStream struct{ net.Conn }

func (s testStream) CloseWrite() error { return nil }

func TestFallbackBeforeForwarding(t *testing.T) {
	client, inbound := net.Pipe()
	defer client.Close()
	remote, outbound := net.Pipe()
	defer remote.Close()
	target := &net.TCPAddr{IP: net.IPv4(203, 0, 113, 4), Port: 443}
	done := make(chan bool, 1)
	go func() {
		used, _ := Relay(context.Background(), inbound, target, func(context.Context, string, int) (Stream, error) {
			return nil, errors.New("unavailable")
		}, func(_ context.Context, network, address string) (net.Conn, error) {
			if network != "tcp4" || address != target.String() {
				t.Errorf("unexpected direct target %s %s", network, address)
			}
			return outbound, nil
		}, time.Second)
		done <- used
	}()
	go func() { _, _ = client.Write([]byte("request")) }()
	buf := make([]byte, 7)
	if _, err := io.ReadFull(remote, buf); err != nil || string(buf) != "request" {
		t.Fatalf("fallback payload %q: %v", buf, err)
	}
	remote.Close()
	if !<-done {
		t.Fatal("direct fallback not reported")
	}
}

func TestNoFallbackAfterOptimisticOpen(t *testing.T) {
	client, inbound := net.Pipe()
	defer client.Close()
	remote, outbound := net.Pipe()
	called := false
	go remote.Close()
	used, _ := Relay(context.Background(), inbound, &net.TCPAddr{IP: net.IPv4(203, 0, 113, 5), Port: 80}, func(context.Context, string, int) (Stream, error) {
		return testStream{outbound}, nil
	}, func(context.Context, string, string) (net.Conn, error) {
		called = true
		return nil, errors.New("unexpected")
	}, time.Second)
	if used || called {
		t.Fatal("replayed an optimistically opened stream")
	}
}

func TestCancelledContextDoesNotFallback(t *testing.T) {
	client, inbound := net.Pipe()
	defer client.Close()
	defer inbound.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	_, err := Relay(ctx, inbound, &net.TCPAddr{IP: net.IPv4(203, 0, 113, 5), Port: 80}, func(ctx context.Context, _ string, _ int) (Stream, error) {
		return nil, ctx.Err()
	}, func(context.Context, string, string) (net.Conn, error) {
		called = true
		return nil, errors.New("unexpected")
	}, time.Second)
	if !errors.Is(err, context.Canceled) || called {
		t.Fatalf("error %v, direct called %v", err, called)
	}
}

func TestNilDirectConnectionReturnsError(t *testing.T) {
	client, inbound := net.Pipe()
	defer client.Close()
	defer inbound.Close()
	used, err := Relay(context.Background(), inbound, &net.TCPAddr{IP: net.IPv4(203, 0, 113, 5), Port: 80}, func(context.Context, string, int) (Stream, error) {
		return nil, errors.New("unavailable")
	}, func(context.Context, string, string) (net.Conn, error) {
		return nil, nil
	}, time.Second)
	if used || err == nil {
		t.Fatalf("used %v, error %v; expected error", used, err)
	}
}

func TestCancellationDuringOpenClosesStream(t *testing.T) {
	client, inbound := net.Pipe()
	defer client.Close()
	defer inbound.Close()
	remote, outbound := net.Pipe()
	defer remote.Close()
	ctx, cancel := context.WithCancel(context.Background())
	used, err := Relay(ctx, inbound, &net.TCPAddr{IP: net.IPv4(203, 0, 113, 5), Port: 80}, func(context.Context, string, int) (Stream, error) {
		cancel()
		return testStream{outbound}, nil
	}, func(context.Context, string, string) (net.Conn, error) {
		t.Error("unexpected fallback")
		return nil, errors.New("unexpected")
	}, time.Second)
	if used || !errors.Is(err, context.Canceled) {
		t.Fatalf("used %v, error %v", used, err)
	}
	if _, err := remote.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("stream not closed: %v", err)
	}
}
