package printer

import (
	"net"
	"reflect"
	"testing"
)

func mustParseCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse %q: %v", cidr, err)
	}
	return network
}

// The scan used to always emit .1 through .254 regardless of the mask, which is
// wrong in both directions: it misses most of a larger subnet and probes
// addresses that are not on the local link on a smaller one.
func TestGenerateIPRangeHonoursPrefixLength(t *testing.T) {
	cases := []struct {
		cidr      string
		wantFirst string
		wantLast  string
		wantCount int
	}{
		{"192.0.2.0/24", "192.0.2.1", "192.0.2.254", 254},
		{"192.0.2.128/25", "192.0.2.129", "192.0.2.254", 126},
		{"192.0.2.0/26", "192.0.2.1", "192.0.2.62", 62},
		{"192.0.2.64/27", "192.0.2.65", "192.0.2.94", 30},
		{"192.0.2.240/28", "192.0.2.241", "192.0.2.254", 14},
		{"192.0.2.0/30", "192.0.2.1", "192.0.2.2", 2},
		{"203.0.113.0/29", "203.0.113.1", "203.0.113.6", 6},
	}

	for _, tc := range cases {
		got := generateIPRange(mustParseCIDR(t, tc.cidr))
		if len(got) != tc.wantCount {
			t.Errorf("%s: got %d addresses, want %d", tc.cidr, len(got), tc.wantCount)
			continue
		}
		if got[0] != tc.wantFirst {
			t.Errorf("%s: first address = %s, want %s", tc.cidr, got[0], tc.wantFirst)
		}
		if got[len(got)-1] != tc.wantLast {
			t.Errorf("%s: last address = %s, want %s", tc.cidr, got[len(got)-1], tc.wantLast)
		}
		for _, ip := range got {
			if net.ParseIP(ip) == nil {
				t.Errorf("%s: generated %q is not a valid IP", tc.cidr, ip)
			}
		}
	}
}

// A /31 and a /32 have no usable host range; probing them would be pointless.
func TestGenerateIPRangeReturnsNothingForPointToPoint(t *testing.T) {
	for _, cidr := range []string{"192.0.2.0/31", "192.0.2.5/32"} {
		if got := generateIPRange(mustParseCIDR(t, cidr)); len(got) != 0 {
			t.Errorf("%s: expected no addresses, got %v", cidr, got)
		}
	}
}

// A scan is capped: a /8 would otherwise mean millions of probes. Whatever is
// scanned must still be inside the requested network.
func TestGenerateIPRangeCapsLargeNetworks(t *testing.T) {
	network := mustParseCIDR(t, "203.0.113.0/8")
	got := generateIPRange(network)
	if len(got) != maxScanHosts {
		t.Fatalf("expected the scan to be capped at %d hosts, got %d", maxScanHosts, len(got))
	}
	for _, ip := range got {
		if !network.Contains(net.ParseIP(ip)) {
			t.Fatalf("%s escaped the network %s", ip, network)
		}
	}
	// The cap must start at the beginning of the range, not skip around.
	if got[0] != "203.0.0.1" {
		t.Fatalf("expected scanning to start at the first usable host, got %s", got[0])
	}
}

// Enumerating must be monotonic: a bug that resets the low octet would produce
// duplicates and miss the upper half of the range.
func TestGenerateIPRangeHasNoDuplicatesAndIsOrdered(t *testing.T) {
	got := generateIPRange(mustParseCIDR(t, "192.0.2.0/24"))
	want := make([]string, 0, len(got))
	for i := 1; i <= 254; i++ {
		want = append(want, net.IPv4(192, 0, 2, byte(i)).String())
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("range mismatch:\n got %v\nwant %v", got[:3], want[:3])
	}
	seen := make(map[string]bool, len(got))
	for _, ip := range got {
		if seen[ip] {
			t.Fatalf("duplicate address %s", ip)
		}
		seen[ip] = true
	}
}

// In a container the first non-loopback interface belongs to the container, not
// to the LAN with the printers, so the subnet has to be pin-able.
func TestGetLocalNetworkHonoursPinnedSubnet(t *testing.T) {
	t.Setenv(ScanNetworkEnv, " 198.51.100.0/24 ")
	localIP, network, err := getLocalNetwork()
	if err != nil {
		t.Fatalf("getLocalNetwork: %v", err)
	}
	if localIP != "198.51.100.0" {
		t.Fatalf("local IP = %q, want the address from the pinned CIDR", localIP)
	}
	if network.String() != "198.51.100.0/24" {
		t.Fatalf("network = %q, want 198.51.100.0/24", network)
	}
}

// A typo in the CIDR must be an error, not a silent fallback to the container's
// own network (which would scan the wrong place and report "no printers").
func TestGetLocalNetworkRejectsMalformedSubnet(t *testing.T) {
	t.Setenv(ScanNetworkEnv, "198.51.100.0/33")
	if _, _, err := getLocalNetwork(); err == nil {
		t.Fatal("expected an error for an invalid pinned CIDR")
	}
}
