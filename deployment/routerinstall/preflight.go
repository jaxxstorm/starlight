// Package routerinstall contains deployment checks independent of the core and
// any particular router's service or firewall implementation.
package routerinstall

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Target describes a router identity. Values must come from a trusted platform
// detector, not from an untrusted installation artifact.
type Target struct {
	Model        string
	Firmware     string
	Architecture string
}

// Preflight validates caller-supplied facts before any installation mutation.
// Supported targets and the digest must come from an authenticated source;
// a checksum supplied alongside a downloaded binary does not authenticate it.
// RequiredBytes must include staging, rollback and service asset overhead.
type Preflight struct {
	Target        Target
	Supported     []Target
	FreeBytes     uint64
	RequiredBytes uint64
	SHA256        string
}

// Check verifies the target, space budget and artifact digest. It consumes the
// artifact stream but does not alter files or router state. A successful result
// does not eliminate TOCTOU risks: the caller must stage and verify the exact
// bytes it later activates and must recheck space as needed.
func (p Preflight) Check(artifact io.Reader) error {
	if err := p.checkFacts(); err != nil {
		return err
	}
	if artifact == nil {
		return errors.New("artifact is missing")
	}
	expected, _ := hex.DecodeString(p.SHA256)
	h := sha256.New()
	if _, err := io.Copy(h, artifact); err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}
	if subtle.ConstantTimeCompare(expected, h.Sum(nil)) != 1 {
		return errors.New("artifact SHA-256 digest does not match trusted digest")
	}
	return nil
}

func (p Preflight) checkFacts() error {
	if p.Target.Model == "" || p.Target.Firmware == "" || p.Target.Architecture == "" {
		return errors.New("router identity is incomplete; cannot verify compatibility")
	}
	supported := false
	for _, target := range p.Supported {
		if target == p.Target {
			supported = true
			break
		}
	}
	if !supported {
		return fmt.Errorf("unsupported router target: model %q, firmware %q, architecture %q", p.Target.Model, p.Target.Firmware, p.Target.Architecture)
	}
	if p.RequiredBytes == 0 {
		return errors.New("storage requirement is unknown; cannot verify available space")
	}
	if p.FreeBytes < p.RequiredBytes {
		return fmt.Errorf("insufficient storage: need %d bytes, have %d bytes", p.RequiredBytes, p.FreeBytes)
	}
	expected, err := hex.DecodeString(p.SHA256)
	if err != nil || len(expected) != sha256.Size {
		return errors.New("trusted SHA-256 digest is missing or invalid")
	}
	return nil
}
