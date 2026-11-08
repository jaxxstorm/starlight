package routertraffic

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// ExperimentalNFTPolicy describes a disabled-by-default IPv4 NAT policy.
// It does not install the policy or grant a lease. Deployment is unsafe until
// interface selection, rule ordering, health and recovery are validated.
type ExperimentalNFTPolicy struct {
	LANInterface string
	RelayPort    uint16
	Bypass       []netip.Prefix
}

// Script creates an exclusively owned table with an initially empty lease.
// An empty lease means that none of its rules redirect traffic. The caller must
// refuse to overwrite an existing table and own its lifecycle.
func (p ExperimentalNFTPolicy) Script() (string, error) {
	if !nftInterfaceName.MatchString(p.LANInterface) || p.RelayPort == 0 {
		return "", errors.New("routertraffic: invalid firewall policy")
	}
	var b strings.Builder
	b.WriteString("create table inet starlight_router\n")
	b.WriteString("add set inet starlight_router capture_lease { type ifname; flags timeout; }\n")
	b.WriteString("add chain inet starlight_router prerouting { type nat hook prerouting priority -101; policy accept; }\n")
	fmt.Fprintf(&b, "add rule inet starlight_router prerouting iifname != \"%s\" return\n", p.LANInterface)
	b.WriteString("add rule inet starlight_router prerouting meta nfproto != ipv4 return\n")
	b.WriteString("add rule inet starlight_router prerouting fib daddr type { local, broadcast, multicast } return\n")
	b.WriteString("add rule inet starlight_router prerouting tcp dport 53 return\n")
	for _, prefix := range p.Bypass {
		if !prefix.IsValid() || !prefix.Addr().Is4() || prefix.Bits() == 0 {
			return "", errors.New("routertraffic: bypass must be a valid non-default IPv4 prefix")
		}
		fmt.Fprintf(&b, "add rule inet starlight_router prerouting ip daddr %s return\n", prefix.Masked())
	}
	fmt.Fprintf(&b, "add rule inet starlight_router prerouting iifname @capture_lease meta l4proto tcp redirect to :%d\n", p.RelayPort)
	return b.String(), nil
}
