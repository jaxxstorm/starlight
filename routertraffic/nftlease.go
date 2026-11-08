package routertraffic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"time"
)

// ExperimentalNFTLease updates only the dedicated inet starlight_router
// capture_lease set, which must already exist with type ifname and flags timeout.
// It does not create a table, rules, exclusions, or a listener. It has NOT been
// validated for crash safety, delayed netlink effects, ownership, fw4 reload,
// or compatibility on the target router. Do not use it to activate capture.
type ExperimentalNFTLease struct {
	iface string
	run   func(context.Context, string) error
}

var nftInterfaceName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,15}$`)

// NewExperimentalNFTLease constructs an unvalidated nftables lease updater.
// The table and set must be exclusively owned by the caller; this constructor
// does not establish ownership or ensure firewall/management safety.
func NewExperimentalNFTLease(iface string) (*ExperimentalNFTLease, error) {
	return newExperimentalNFTLease(iface, func(ctx context.Context, script string) error {
		cmd := exec.CommandContext(ctx, "/usr/sbin/nft", "-f", "-")
		cmd.Stdin = bytes.NewBufferString(script)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("nft transaction: %w: %s", err, out)
		}
		return nil
	})
}

func newExperimentalNFTLease(iface string, run func(context.Context, string) error) (*ExperimentalNFTLease, error) {
	if !nftInterfaceName.MatchString(iface) || run == nil {
		return nil, errors.New("routertraffic: invalid nft lease interface or runner")
	}
	return &ExperimentalNFTLease{iface: iface, run: run}, nil
}

// RenewExperimental submits one transaction to replace the set contents with
// one expiring interface key. The distinct method name deliberately prevents
// this unverified updater from satisfying LeaseBackend.
// The TTL is rounded down to milliseconds, never up. Errors, including context
// cancellation, can leave the kernel transaction outcome ambiguous; callers
// must not assume this experimental backend satisfies the LeaseBackend contract.
func (n *ExperimentalNFTLease) RenewExperimental(ctx context.Context, ttl time.Duration) error {
	if ttl < time.Millisecond || ttl > 24*time.Hour {
		return errors.New("routertraffic: nft lease TTL must be between 1ms and 24h")
	}
	return n.run(ctx, fmt.Sprintf("flush set inet starlight_router capture_lease\nadd element inet starlight_router capture_lease { \"%s\" timeout %dms }\n", n.iface, ttl.Milliseconds()))
}

// RevokeExperimental removes only entries in the dedicated set; it leaves all rules and
// tables untouched. Ownership and failure behavior are not yet verified.
func (n *ExperimentalNFTLease) RevokeExperimental(ctx context.Context) error {
	return n.run(ctx, "flush set inet starlight_router capture_lease\n")
}
