package routertraffic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// RunExperimentalCapture creates a dedicated, initially inactive firewall
// policy, then renews its kernel-expiring lease while health succeeds. It
// refuses to replace an existing table. The caller must ensure the policy's
// interface and bypass list are correct for the router.
func RunExperimentalCapture(ctx context.Context, policy ExperimentalNFTPolicy, health func(context.Context) error) error {
	script, err := policy.Script()
	if err != nil {
		return err
	}
	if health == nil {
		return errors.New("routertraffic: missing capture health check")
	}
	run := func(ctx context.Context, input string) error {
		cmd := exec.CommandContext(ctx, "/usr/sbin/nft", "-f", "-")
		cmd.Stdin = bytes.NewBufferString(input)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("nft: %w: %s", err, out)
		}
		return nil
	}
	// create table fails if a same-named table already exists; never delete it.
	if err := run(ctx, script); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = run(cleanup, "delete table inet starlight_router\n")
	}()
	lease, err := newExperimentalNFTLease(policy.LANInterface, run)
	if err != nil {
		return err
	}
	return RunLease(ctx, LeaseConfig{
		Backend: captureLease{lease}, Health: health,
		Interval: 3 * time.Second, Timeout: 3 * time.Second, TTL: 15 * time.Second,
	})
}

type captureLease struct{ lease *ExperimentalNFTLease }

func (l captureLease) Renew(ctx context.Context, ttl time.Duration) error {
	return l.lease.RenewExperimental(ctx, ttl)
}
func (l captureLease) Revoke(ctx context.Context) error { return l.lease.RevokeExperimental(ctx) }
