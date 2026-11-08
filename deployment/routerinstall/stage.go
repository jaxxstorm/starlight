package routerinstall

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

// StageVerified writes an artifact to a new private file in dir and returns its
// path only after verifying the exact staged bytes. dir must be an existing,
// administrator-controlled directory on the intended persistent filesystem.
// The caller owns the returned file and must remove it when no longer needed.
// This does not activate the file, validate configuration, or authenticate the
// supplied digest. Failed stages are removed on a best-effort basis.
func (p Preflight) StageVerified(dir string, artifact io.Reader) (path string, err error) {
	if err := p.checkFacts(); err != nil {
		return "", err
	}
	if artifact == nil {
		return "", errors.New("artifact is missing")
	}
	if p.RequiredBytes > math.MaxInt64 {
		return "", errors.New("storage budget exceeds supported staging size")
	}
	f, err := os.CreateTemp(dir, ".starlight-stage-*")
	if err != nil {
		return "", fmt.Errorf("create staged artifact: %w", err)
	}
	name := f.Name()
	defer func() {
		if err != nil {
			_ = f.Close()
			if removeErr := os.Remove(name); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove incomplete staged artifact: %w", removeErr))
			}
		}
	}()

	// Never write more than the already checked budget, including on bad input.
	limited := &io.LimitedReader{R: artifact, N: int64(p.RequiredBytes)}
	if err = p.Check(io.TeeReader(limited, f)); err != nil {
		return "", err
	}
	if limited.N == 0 {
		var extra [1]byte
		n, readErr := io.ReadFull(artifact, extra[:])
		if n != 0 {
			return "", errors.New("artifact exceeds staging budget")
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return "", fmt.Errorf("read artifact: %w", readErr)
		}
	}
	if err = f.Sync(); err != nil {
		return "", fmt.Errorf("sync staged artifact: %w", err)
	}
	// Verify what was actually written, not just the bytes read from the source.
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind staged artifact: %w", err)
	}
	if err = p.Check(f); err != nil {
		return "", fmt.Errorf("verify staged artifact: %w", err)
	}
	if err = f.Close(); err != nil {
		return "", fmt.Errorf("close staged artifact: %w", err)
	}
	return name, nil
}
