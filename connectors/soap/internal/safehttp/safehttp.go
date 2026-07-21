// Package safehttp provides the SSRF-hardening helpers shared by the connector
// tool handlers: an *http.Client whose redirect policy (a) caps the number of
// redirects, (b) strips the configured custom auth header on any cross-host
// redirect, and (c) refuses redirects whose target resolves to a private,
// loopback, or link-local address; plus IsDisallowedHost, reused to screen the
// initial upstream target as well.
package safehttp

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// MaxRedirects caps the number of redirects the client follows before it
// returns an error.
const MaxRedirects = 5

// lookupIP resolves a host to its IP addresses. It is a package variable so
// tests can substitute a deterministic resolver.
var lookupIP = net.LookupIP

// IsDisallowedHost reports whether host is, or resolves to, an address that a
// connector must never reach: 10/8, 172.16/12, 192.168/16, fc00::/7 (private),
// 127/8, ::1 (loopback), 169.254/16, fe80::/10 (link-local), and 0.0.0.0 / ::
// (unspecified). A host that resolves to multiple addresses is disallowed if
// ANY of them is disallowed. Literal IPs are checked directly. A host that is
// empty or cannot be resolved is treated as disallowed (fail closed).
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

// isDisallowedIP reports whether ip falls in a range that must never be reached
// from a connector. IsPrivate covers 10/8, 172.16/12, 192.168/16 and fc00::/7;
// IsLoopback covers 127/8 and ::1; IsLinkLocalUnicast covers 169.254/16 and
// fe80::/10; IsUnspecified covers 0.0.0.0 and ::. Link-local multicast is
// rejected defensively.
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

// NewClient returns an *http.Client with the given timeout whose CheckRedirect:
//   - caps redirects at MaxRedirects;
//   - deletes authHeaderName from the follow-up request whenever the redirect
//     target host differs from the original request host (Go strips
//     Authorization/Cookie itself but not a custom header name);
//   - refuses (errors) any redirect whose target host is disallowed.
//
// isDisallowed is injected (nil defaults to IsDisallowedHost) so tests can
// exercise the policy while loopback httptest servers still work.
func NewClient(timeout time.Duration, authHeaderName string, isDisallowed func(host string) bool) *http.Client {
	if isDisallowed == nil {
		isDisallowed = IsDisallowedHost
	}
	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: checkRedirect(authHeaderName, isDisallowed),
	}
}

func checkRedirect(authHeaderName string, isDisallowed func(host string) bool) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= MaxRedirects {
			return fmt.Errorf("stopped after %d redirects", MaxRedirects)
		}
		target := req.URL.Hostname()
		if authHeaderName != "" && len(via) > 0 {
			if orig := via[0].URL.Hostname(); !strings.EqualFold(target, orig) {
				req.Header.Del(authHeaderName)
			}
		}
		if isDisallowed(target) {
			return fmt.Errorf("redirect to disallowed host %q refused", target)
		}
		return nil
	}
}
