package routertraffic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeLease struct {
	mu       sync.Mutex
	events   chan string
	renewErr error
}

func (f *fakeLease) Renew(context.Context, time.Duration) error {
	f.events <- "renew"
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.renewErr
}
func (f *fakeLease) Revoke(context.Context) error { f.events <- "revoke"; return nil }

func leaseEvent(t *testing.T, events <-chan string, want string) {
	t.Helper()
	select {
	case got := <-events:
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %q", want)
	}
}

func TestLeaseHealthTransitionsAndShutdown(t *testing.T) {
	backend := &fakeLease{events: make(chan string, 16)}
	var mu sync.Mutex
	healthy := true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- RunLease(ctx, LeaseConfig{Backend: backend, Interval: 20 * time.Millisecond, Timeout: 5 * time.Millisecond, TTL: 100 * time.Millisecond, Health: func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			if !healthy {
				return errors.New("offline")
			}
			return nil
		}})
	}()
	leaseEvent(t, backend.events, "renew")
	mu.Lock()
	healthy = false
	mu.Unlock()
	leaseEvent(t, backend.events, "revoke")
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	leaseEvent(t, backend.events, "revoke")
}

func TestLeaseRenewFailureRevokesAndReturns(t *testing.T) {
	backend := &fakeLease{events: make(chan string, 4), renewErr: errors.New("backend failed")}
	err := RunLease(context.Background(), LeaseConfig{Backend: backend, Interval: 20 * time.Millisecond, Timeout: 5 * time.Millisecond, TTL: 100 * time.Millisecond, Health: func(context.Context) error { return nil }})
	if err == nil {
		t.Fatal("expected error")
	}
	leaseEvent(t, backend.events, "renew")
	leaseEvent(t, backend.events, "revoke")
}

func TestLeaseRejectsUnsafeTiming(t *testing.T) {
	err := RunLease(context.Background(), LeaseConfig{Backend: &fakeLease{events: make(chan string, 1)}, Interval: time.Second, Timeout: time.Second, TTL: time.Second, Health: func(context.Context) error { return nil }})
	if err == nil {
		t.Fatal("expected invalid timing error")
	}
}

func TestLeaseHealthTimeoutRevokes(t *testing.T) {
	backend := &fakeLease{events: make(chan string, 4)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunLease(ctx, LeaseConfig{Backend: backend, Interval: 20 * time.Millisecond, Timeout: 5 * time.Millisecond, TTL: 100 * time.Millisecond, Health: func(ctx context.Context) error {
			<-ctx.Done()
			return nil // A late success after the deadline is not healthy.
		}})
	}()
	leaseEvent(t, backend.events, "revoke")
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	leaseEvent(t, backend.events, "revoke")
}
