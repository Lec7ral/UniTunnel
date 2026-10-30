//go:build windows

package tun

import "testing"

// cidrToDestMask feeds `route add`, which only understands a destination plus a
// dotted mask. A wrong conversion here silently breaks bypass routing.
func TestCIDRToDestMask(t *testing.T) {
	cases := []struct {
		cidr string
		dest string
		mask string
		ok   bool
	}{
		{"10.0.0.0/8", "10.0.0.0", "255.0.0.0", true},
		{"172.16.0.0/12", "172.16.0.0", "255.240.0.0", true},
		{"192.168.0.0/16", "192.168.0.0", "255.255.0.0", true},
		// A host address inside the network must be normalised to the network.
		{"10.1.2.3/8", "10.0.0.0", "255.0.0.0", true},
		{"0.0.0.0/0", "0.0.0.0", "0.0.0.0", true},
		{"192.168.1.1/32", "192.168.1.1", "255.255.255.255", true},
		// Whitespace from hand-edited config.json is tolerated.
		{" 10.0.0.0/8 ", "10.0.0.0", "255.0.0.0", true},
		// Invalid input must be reported, never silently converted.
		{"10.0.0.0", "", "", false},
		{"10.0.0.0/33", "", "", false},
		{"no-es-una-red/8", "", "", false},
		{"", "", "", false},
		{"2001:db8::/32", "", "", false},
	}

	for _, tc := range cases {
		dest, mask, ok := cidrToDestMask(tc.cidr)
		if ok != tc.ok {
			t.Errorf("cidrToDestMask(%q) ok = %v, esperado %v", tc.cidr, ok, tc.ok)
			continue
		}
		if !tc.ok {
			continue
		}
		if dest != tc.dest || mask != tc.mask {
			t.Errorf("cidrToDestMask(%q) = (%q, %q), esperado (%q, %q)",
				tc.cidr, dest, mask, tc.dest, tc.mask)
		}
	}
}
