package main

import (
	"context"
	"strings"
	"testing"
)

func TestRejectsNonLoopbackListener(t *testing.T) {
	err := run(context.Background(), []string{"-listen", "0.0.0.0:1234", "-ground", "127.0.0.1:8443", "-target", "127.0.0.1:80"})
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected loopback restriction, got %v", err)
	}
}

func TestRequiresKey(t *testing.T) {
	t.Setenv("SATMUX_PSK", "")
	err := run(context.Background(), []string{"-ground", "127.0.0.1:8443", "-target", "127.0.0.1:80"})
	if err == nil || !strings.Contains(err.Error(), "SATMUX_PSK") {
		t.Fatalf("expected key error, got %v", err)
	}
}
