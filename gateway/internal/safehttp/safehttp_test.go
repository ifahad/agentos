package safehttp

import (
	"errors"
	"net"
	"testing"
)

func TestIsDisallowedHostLiteralIPs(t *testing.T) {
	cases := []struct {
		host       string
		disallowed bool
	}{
		{"127.0.0.1", true},             // loopback
		{"::1", true},                   // loopback v6
		{"10.0.0.5", true},              // private
		{"172.16.4.4", true},            // private
		{"192.168.1.1", true},           // private
		{"169.254.169.254", true},       // link-local — the cloud metadata endpoint
		{"fe80::1", true},               // link-local v6
		{"0.0.0.0", true},               // unspecified
		{"::", true},                    // unspecified v6
		{"8.8.8.8", false},              // public
		{"1.1.1.1", false},              // public
		{"2606:4700:4700::1111", false}, // public v6
	}
	for _, tc := range cases {
		if got := IsDisallowedHost(tc.host); got != tc.disallowed {
			t.Errorf("IsDisallowedHost(%q) = %v, want %v", tc.host, got, tc.disallowed)
		}
	}
}

func TestIsDisallowedHostEmptyIsFailClosed(t *testing.T) {
	if !IsDisallowedHost("") {
		t.Error("empty host must be disallowed (fail closed)")
	}
	if !IsDisallowedHost("   ") {
		t.Error("whitespace host must be disallowed (fail closed)")
	}
}

func TestIsDisallowedHostUnresolvableIsFailClosed(t *testing.T) {
	SetLookupIPForTest(func(string) ([]net.IP, error) {
		return nil, errors.New("no such host")
	})
	t.Cleanup(func() { SetLookupIPForTest(net.LookupIP) })
	if !IsDisallowedHost("nonexistent.invalid") {
		t.Error("unresolvable host must be disallowed (fail closed)")
	}
}

func TestIsDisallowedHostResolvesToPrivate(t *testing.T) {
	// A perfectly public-looking name that resolves to a private address is the
	// classic DNS-rebinding SSRF; it must be refused.
	SetLookupIPForTest(func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.1.2.3")}, nil
	})
	t.Cleanup(func() { SetLookupIPForTest(net.LookupIP) })
	if !IsDisallowedHost("api.evil.example") {
		t.Error("host resolving to a private address must be disallowed")
	}
}

func TestIsDisallowedHostAnyPrivateAddressDisqualifies(t *testing.T) {
	// One public and one private A record: the private one wins, because the
	// dialer could pick either.
	SetLookupIPForTest(func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("127.0.0.1")}, nil
	})
	t.Cleanup(func() { SetLookupIPForTest(net.LookupIP) })
	if !IsDisallowedHost("mixed.example") {
		t.Error("a host with any disallowed address must be disallowed")
	}
}

func TestIsDisallowedHostResolvesToPublic(t *testing.T) {
	SetLookupIPForTest(func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	})
	t.Cleanup(func() { SetLookupIPForTest(net.LookupIP) })
	if IsDisallowedHost("example.com") {
		t.Error("host resolving only to a public address must be allowed")
	}
}
