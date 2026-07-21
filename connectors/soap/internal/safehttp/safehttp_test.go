package safehttp

import (
	"net"
	"testing"
)

func TestIsDisallowedHostLiterals(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"169.254.169.254", true},     // link-local (cloud metadata)
		{"127.0.0.1", true},           // loopback
		{"10.1.2.3", true},            // private 10/8
		{"172.16.5.9", true},          // private 172.16/12
		{"192.168.1.1", true},         // private 192.168/16
		{"0.0.0.0", true},             // unspecified
		{"::1", true},                 // IPv6 loopback
		{"fe80::1", true},             // IPv6 link-local
		{"fc00::1", true},             // IPv6 unique-local
		{"::", true},                  // IPv6 unspecified
		{"", true},                    // empty fails closed
		{"8.8.8.8", false},            // public
		{"93.184.216.34", false},      // public
		{"2606:2800:220:1::1", false}, // public IPv6
	}
	for _, tt := range tests {
		if got := IsDisallowedHost(tt.host); got != tt.want {
			t.Errorf("IsDisallowedHost(%q) = %t, want %t", tt.host, got, tt.want)
		}
	}
}

func TestIsDisallowedHostResolves(t *testing.T) {
	orig := lookupIP
	t.Cleanup(func() { lookupIP = orig })

	tests := []struct {
		name string
		ips  []net.IP
		err  error
		want bool
	}{
		{"public only", []net.IP{net.ParseIP("8.8.8.8")}, nil, false},
		{"any private rejects", []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("10.0.0.5")}, nil, true},
		{"all private", []net.IP{net.ParseIP("192.168.0.1")}, nil, true},
		{"resolve error fails closed", nil, &net.DNSError{Err: "no such host"}, true},
		{"empty result fails closed", nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookupIP = func(string) ([]net.IP, error) { return tt.ips, tt.err }
			if got := IsDisallowedHost("example.test"); got != tt.want {
				t.Errorf("IsDisallowedHost = %t, want %t", got, tt.want)
			}
		})
	}
}
