package routerinstall

import (
	"errors"
	"math"
)

// StorageBudget computes the additional free bytes required to stage an
// artifact while reserving space for rollback and installation assets. Callers
// must determine the sizes from their actual installation plan, and recheck
// free space on the destination filesystem before changing it. Pass zero for
// rollbackBytes only when no rollback copy is needed.
func StorageBudget(artifactBytes, rollbackBytes, assetsBytes uint64) (uint64, error) {
	if artifactBytes == 0 {
		return 0, errors.New("artifact size is unknown or zero")
	}
	if rollbackBytes > math.MaxUint64-artifactBytes {
		return 0, errors.New("storage budget overflows")
	}
	total := artifactBytes + rollbackBytes
	if assetsBytes > math.MaxUint64-total {
		return 0, errors.New("storage budget overflows")
	}
	return total + assetsBytes, nil
}
