// Package safehttp screens hosts for SSRF before the gateway proxies to them.
//
// It is a trimmed third copy of the package the REST and SOAP connectors carry.
// The connectors' versions live under their own modules' internal/ trees and
// cannot be imported here, and the gateway needs only the host screen — it
// makes no connector-style redirect-following calls — so only IsDisallowedHost
// and its helper are ported. Keep the three copies' screening logic identical:
// a provider registry entry pointing at a private address is exactly the SSRF
// the connectors already defend against.
package safehttp

import (
	"net"
	"strings"
)

// lookupIP resolves a host to its IP addresses. It is a package variable so
// tests can substitute a deterministic resolver (see SetLookupIPForTest).
var lookupIP = net.LookupIP

// IsDisallowedHost reports whether host is, or resolves to, an address the
// gateway must never reach: 10/8, 172.16/12, 192.168/16, fc00::/7 (private),
// 127/8, ::1 (loopback), 169.254/16, fe80::/10 (link-local), and 0.0.0.0 / ::
// (unspecified). A host resolving to multiple addresses is disallowed if ANY of
// them is. Literal IPs are checked directly. An empty or unresolvable host is
// disallowed — fail closed, so a registry entry the gateway cannot vet is never
// dialed.
func IsDisallowedHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return isDisallowedIP(ip)
	}
	ips, err := lookupIP(host)
	if err != nil || len(ips) == 0 {
		return true
	}
	for _, ip := range ips {
		if isDisallowedIP(ip) {
			return true
		}
	}
	return false
}

// isDisallowedIP reports whether ip falls in a range that must never be reached.
// IsPrivate covers 10/8, 172.16/12, 192.168/16 and fc00::/7; IsLoopback covers
// 127/8 and ::1; IsLinkLocalUnicast covers 169.254/16 and fe80::/10;
// IsUnspecified covers 0.0.0.0 and ::. Link-local multicast is rejected
// defensively.
func isDisallowedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

// SetLookupIPForTest substitutes the DNS resolver so a caller can screen a
// hostname against controlled addresses without real DNS. Test-only, but
// exported because the registry's tests live in a sibling package. Restore the
// original with SetLookupIPForTest(net.LookupIP) in a t.Cleanup.
func SetLookupIPForTest(fn func(host string) ([]net.IP, error)) {
	lookupIP = fn
}
