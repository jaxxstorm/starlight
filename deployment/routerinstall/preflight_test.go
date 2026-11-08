package routerinstall

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func validPreflight() Preflight {
	target := Target{Model: "test-model", Firmware: "test-firmware", Architecture: "test-arch"}
	sum := sha256.Sum256([]byte("test artifact"))
	return Preflight{Target: target, Supported: []Target{target}, FreeBytes: 100, RequiredBytes: 100, SHA256: fmt.Sprintf("%x", sum)}
}

func TestPreflight(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Preflight)
		data string
		want string
	}{
		{name: "valid", data: "test artifact"},
		{name: "incomplete identity", edit: func(p *Preflight) { p.Target.Model = "" }, want: "identity is incomplete"},
		{name: "unsupported firmware", edit: func(p *Preflight) { p.Target.Firmware = "other" }, want: "unsupported router target"},
		{name: "empty allowlist", edit: func(p *Preflight) { p.Supported = nil }, want: "unsupported router target"},
		{name: "unknown space requirement", edit: func(p *Preflight) { p.RequiredBytes = 0 }, want: "storage requirement is unknown"},
		{name: "insufficient space", edit: func(p *Preflight) { p.FreeBytes-- }, want: "insufficient storage"},
		{name: "invalid digest", edit: func(p *Preflight) { p.SHA256 = "bad" }, want: "digest is missing or invalid"},
		{name: "modified artifact", data: "different", want: "does not match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validPreflight()
			if tt.edit != nil {
				tt.edit(&p)
			}
			err := p.Check(strings.NewReader(tt.data))
			if tt.want == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("got %v, want error containing %q", err, tt.want)
			}
		})
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestPreflightReaderErrors(t *testing.T) {
	p := validPreflight()
	if err := p.Check(nil); err == nil {
		t.Fatal("missing artifact accepted")
	}
	if err := p.Check(failingReader{}); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got %v, want unexpected EOF", err)
	}
}
