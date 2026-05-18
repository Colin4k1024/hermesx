// Package tools provides url_safety — SSRF prevention for agent HTTP requests.
//
// Blocks requests to private/internal network addresses (169.254.169.254,
// localhost, 10.x, 172.16-31.x, 192.168.x, CGNAT 100.64.0.0/10).
//
// Limitations:
//   - DNS rebinding (TOCTOU): attacker-controlled DNS can return public IP
//     for check, then private IP for connection. Fixing requires connection-level
//     validation or an egress proxy.
package tools

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// blockedHostnames are always blocked regardless of IP resolution.
var blockedHostnames = map[string]bool{
	"metadata.google.internal": true,
	"metadata.goog":            true,
}

// cgnatNetwork is 100.64.0.0/10 (CGNAT / Shared Address Space, RFC 6598).
// Not covered by net.IP.IsPrivate() — must be checked explicitly.
var cgnatNetwork = mustParseCIDR("100.64.0.0/10")

func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(fmt.Sprintf("bad CIDR: %s", s))
	}
	return n
}

// isBlockedIP returns true if the IP should be blocked for SSRF protection.
// Deprecated: use egress.IsBlockedIP for new code.
func isBlockedIP(ip net.IP) bool {
	return IsBlockedIP(ip)
}

// IsBlockedIP is the exported version of SSRF IP blocking logic.
func IsBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if cgnatNetwork.Contains(ip) {
		return true
	}
	return false
}

// IsSafeURL checks if a URL target is not a private/internal address.
// Resolves the hostname and checks against private ranges.
// Fails closed: DNS errors and unexpected exceptions block the request.
func IsSafeURL(rawURL string) (bool, string) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false, fmt.Sprintf("invalid URL: %v", err)
	}

	// Block dangerous schemes.
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false, fmt.Sprintf("blocked scheme: %s", scheme)
	}

	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return false, "empty hostname"
	}

	// Block known internal hostnames.
	if blockedHostnames[hostname] {
		return false, fmt.Sprintf("blocked internal hostname: %s", hostname)
	}

	// Try direct IP parse first (skip DNS for literal IPs).
	if ip := net.ParseIP(hostname); ip != nil {
		if isBlockedIP(ip) {
			return false, fmt.Sprintf("blocked private/internal IP: %s", hostname)
		}
		return true, ""
	}

	// Resolve hostname and check all IPs.
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		// Fail closed: DNS failure = block.
		return false, fmt.Sprintf("DNS resolution failed for %s: %v", hostname, err)
	}

	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if isBlockedIP(ip) {
			return false, fmt.Sprintf("blocked private/internal address: %s -> %s", hostname, addr)
		}
	}

	return true, ""
}
