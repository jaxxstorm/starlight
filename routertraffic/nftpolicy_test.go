package routertraffic

import (
	"net/netip"
	"strings"
	"testing"
)

func TestExperimentalNFTPolicy(t *testing.T) {
	p := ExperimentalNFTPolicy{LANInterface: "br-lan", RelayPort: 19090, Bypass: []netip.Prefix{netip.MustParsePrefix("192.168.8.3/24"), netip.MustParsePrefix("143.198.79.144/32")}}
	s, err := p.Script()
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"flags timeout", "iifname != \"br-lan\" return", "fib daddr type { local, broadcast, multicast } return", "tcp dport 53 return", "ip daddr 192.168.8.0/24 return", "ip daddr 143.198.79.144/32 return", "iifname @capture_lease meta l4proto tcp redirect to :19090"} {
		if !strings.Contains(s, part) {
			t.Fatalf("missing %q in %s", part, s)
		}
	}
	if strings.Contains(s, "add element") {
		t.Fatal("policy must start without an active lease")
	}
}

func TestExperimentalNFTPolicyRejectsInvalid(t *testing.T) {
	for _, p := range []ExperimentalNFTPolicy{
		{LANInterface: "br-lan\nflush ruleset", RelayPort: 1},
		{LANInterface: "br-lan"},
		{LANInterface: "br-lan", RelayPort: 1, Bypass: []netip.Prefix{netip.MustParsePrefix("::1/128")}},
		{LANInterface: "br-lan", RelayPort: 1, Bypass: []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")}},
	} {
		if _, err := p.Script(); err == nil {
			t.Fatalf("accepted invalid policy %+v", p)
		}
	}
}
