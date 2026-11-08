package routertraffic

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestExperimentalNFTLeaseTransactions(t *testing.T) {
	var scripts []string
	lease, err := newExperimentalNFTLease("br-lan", func(_ context.Context, script string) error {
		scripts = append(scripts, script)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.RenewExperimental(context.Background(), time.Second+time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	if err := lease.RevokeExperimental(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 2 || scripts[0] != "flush set inet starlight_router capture_lease\nadd element inet starlight_router capture_lease { \"br-lan\" timeout 1000ms }\n" || scripts[1] != "flush set inet starlight_router capture_lease\n" {
		t.Fatalf("unexpected transactions: %q", scripts)
	}
}

func TestExperimentalNFTLeaseValidationAndErrors(t *testing.T) {
	for _, name := range []string{"", "br-lan\nflush ruleset", "1234567890123456", "a\"b"} {
		if _, err := newExperimentalNFTLease(name, func(context.Context, string) error { return nil }); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	want := errors.New("nft failed")
	lease, err := newExperimentalNFTLease("br-lan", func(context.Context, string) error { return want })
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.RenewExperimental(context.Background(), time.Nanosecond); err == nil || !strings.Contains(err.Error(), "TTL") {
		t.Fatalf("invalid ttl: %v", err)
	}
	if err := lease.RenewExperimental(context.Background(), time.Second); !errors.Is(err, want) {
		t.Fatalf("lost backend error: %v", err)
	}
}
