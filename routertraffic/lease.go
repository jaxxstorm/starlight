package routertraffic

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// LeaseBackend must implement an independently expiring lease: expiration must
// not depend on this process, and a successful Renew must not outlive ttl. No
// production backend is provided; this interface alone does not prove safety.
// Operations must respect context cancellation and must not take effect later.
type LeaseBackend interface {
	Renew(ctx context.Context, ttl time.Duration) error
	Revoke(ctx context.Context) error
}

// LeaseConfig coordinates capture only while both listener and authenticated
// ground health are established by Health. An optimistic stream open is not a
// valid health check. Interval, Timeout and TTL require target measurements.
type LeaseConfig struct {
	Backend  LeaseBackend
	Health   func(context.Context) error
	Interval time.Duration
	Timeout  time.Duration
	TTL      time.Duration
}

// RunLease runs until cancellation or a lease operation fails. It revokes on
// unhealthy status and on exit; independent expiry remains essential for crash
// and revocation failure. Health and backend callbacks must honor context.
func RunLease(ctx context.Context, c LeaseConfig) (result error) {
	if c.Backend == nil || c.Health == nil || c.Interval <= 0 || c.Timeout <= 0 || c.TTL <= c.Interval || c.Timeout >= (c.TTL-c.Interval)/2 {
		return errors.New("routertraffic: invalid lease configuration")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), c.Timeout)
		defer cancel()
		if err := c.Backend.Revoke(cleanup); err != nil {
			result = errors.Join(result, fmt.Errorf("routertraffic: revoke lease: %w", err))
		}
	}()
	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return nil
		}
		check, cancel := context.WithTimeout(ctx, c.Timeout)
		healthErr := c.Health(check)
		if check.Err() != nil {
			healthErr = check.Err()
		}
		cancel()
		if ctx.Err() != nil {
			return nil
		}
		op, cancel := context.WithTimeout(ctx, c.Timeout)
		var err error
		if healthErr == nil {
			err = c.Backend.Renew(op, c.TTL)
		} else {
			err = c.Backend.Revoke(op)
		}
		if op.Err() != nil && err == nil {
			err = op.Err()
		}
		cancel()
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return fmt.Errorf("routertraffic: lease update: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
