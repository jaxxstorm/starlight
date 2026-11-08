package routerinstall

import (
	"math"
	"testing"
)

func TestStorageBudget(t *testing.T) {
	tests := []struct {
		name                       string
		artifact, rollback, assets uint64
		want                       uint64
		fail                       bool
	}{
		{"fresh", 10, 0, 3, 13, false},
		{"upgrade", 10, 20, 3, 33, false},
		{"zero artifact", 0, 1, 1, 0, true},
		{"rollback overflow", math.MaxUint64, 1, 0, 0, true},
		{"assets overflow", math.MaxUint64 - 1, 1, 1, 0, true},
		{"maximum", math.MaxUint64 - 1, 1, 0, math.MaxUint64, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StorageBudget(tt.artifact, tt.rollback, tt.assets)
			if (err != nil) != tt.fail || (!tt.fail && got != tt.want) {
				t.Fatalf("got %d, %v; want %d, failure %v", got, err, tt.want, tt.fail)
			}
		})
	}
}
