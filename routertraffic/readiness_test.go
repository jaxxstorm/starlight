package routertraffic

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestCaptureHealth(t *testing.T) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	called := false
	probe := func(context.Context) error { called = true; return nil }
	if err := CaptureHealth(ctx, l.Addr().String(), probe); err != nil || !called {
		t.Fatalf("health=%v called=%v", err, called)
	}
	want := errors.New("ground unavailable")
	if err := CaptureHealth(ctx, l.Addr().String(), func(context.Context) error { return want }); !errors.Is(err, want) {
		t.Fatalf("expected probe error, got %v", err)
	}
	if err := CaptureHealth(context.Background(), l.Addr().String(), probe); err == nil {
		t.Fatal("missing deadline accepted")
	}
	if err := CaptureHealth(ctx, "192.0.2.1:1234", probe); err == nil {
		t.Fatal("non-loopback address accepted")
	}
	address := l.Addr().String()
	_ = l.Close()
	called = false
	if err := CaptureHealth(ctx, address, probe); err == nil || called {
		t.Fatalf("closed listener: err=%v called=%v", err, called)
	}
}
