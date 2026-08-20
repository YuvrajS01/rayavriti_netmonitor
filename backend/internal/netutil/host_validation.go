// Package netutil provides network safety helpers for validating target hosts
// before the collectors, port scanner, or remote-fleet fetcher connect to them.
//
// The primary goal is SSRF prevention: reject loopback, link-local (including
// the cloud-metadata endpoint 169.254.169.254), and unspecified addresses so
// that a user with devices.write or remote-instance access cannot exfiltrate
// cloud metadata or probe the server's own loopback services.
//
// Private RFC1918 / ULA addresses are ALLOWED by default because this is a
// network-monitoring product whose core use case is monitoring internal
// infrastructure. Callers that need stricter behaviour (e.g. an
// externally-facing remote-fleet fetcher) can use RejectPrivateHosts.
package netutil

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
)

// ErrBlockedHost is returned when a host resolves to a blocked address.
var ErrBlockedHost = errors.New("host resolves to a blocked address")

// HostPolicy controls which address classes are rejected by ValidateHost.
type HostPolicy struct {
	// RejectLoopback rejects 127.0.0.0/8 and ::1. Almost always true.
	RejectLoopback bool
	// RejectLinkLocal rejects 169.254.0.0/16 and fe80::/10 — this covers the
	// AWS/GCP/Azure cloud-metadata endpoint 169.254.169.254.
	RejectLinkLocal bool
	// RejectUnspecified rejects 0.0.0.0 and ::.
	RejectUnspecified bool
	// RejectPrivate rejects RFC1918 (10/8, 172.16/12, 192.168/16) and
	// fc00::/7 unique-local. Off by default because monitoring private
	// infrastructure is the core use case.
	RejectPrivate bool
}

// DefaultHostPolicy blocks loopback, link-local, and unspecified addresses
// while permitting private ranges (the normal monitoring use case).
func DefaultHostPolicy() HostPolicy {
	return HostPolicy{
		RejectLoopback:    true,
		RejectLinkLocal:   true,
		RejectUnspecified: true,
		RejectPrivate:     false,
	}
}

// StrictHostPolicy additionally rejects private ranges. Use for
// externally-facing fetchers (e.g. remote-fleet collector).
func StrictHostPolicy() HostPolicy {
	return HostPolicy{
		RejectLoopback:    true,
		RejectLinkLocal:   true,
		RejectUnspecified: true,
		RejectPrivate:     true,
	}
}

// ValidateHost validates that every IP the host resolves to is permitted by
// the policy. The host may be an IP literal, a bare hostname, or a host:port
// pair (the port is stripped before resolution). For hostnames, all A/AAAA
// records are checked and the host is rejected if ANY resolved address is
// blocked (no DNS-rebinding to a private IP after a public lookup).
//
// DNS resolution here is only for validation; the caller still performs its
// own connection. To prevent TOCTOU, callers should connect to the resolved
// IP directly when feasible, but the resolution-then-connect gap is bounded
// by the connection timeout in the collectors.
func ValidateHost(host string, policy HostPolicy) error {
	hostname := splitHostPort(host)
	if hostname == "" {
		return errors.New("empty host")
	}

	// Fast path: if it's already an IP literal, validate directly.
	if ip, err := netip.ParseAddr(hostname); err == nil {
		return checkAddr(ip, policy)
	}

	// Hostname: resolve and check every returned address.
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve host %q: %w", hostname, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("no addresses resolved for host %q", hostname)
	}
	for _, resolved := range ips {
		ip, ok := netip.AddrFromSlice(resolved)
		if !ok {
			continue
		}
		if err := checkAddr(ip, policy); err != nil {
			return err
		}
	}
	return nil
}

// ValidateURL validates the host portion of an absolute http(s) URL.
func ValidateURL(rawURL string, policy HostPolicy) error {
	// url.Parse is more lenient than url.ParseRequestURI for bare hosts,
	// but we require an absolute URL with a host.
	idx := strings.Index(rawURL, "://")
	if idx < 0 {
		return errors.New("URL is not absolute")
	}
	scheme := strings.ToLower(rawURL[:idx])
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", scheme)
	}
	rest := rawURL[idx+3:]
	// Strip path/query/fragment.
	host := rest
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return errors.New("URL has no host")
	}
	return ValidateHost(host, policy)
}

func splitHostPort(host string) string {
	// Strip port if present. net.SplitHostPort handles [::1]:80 etc.
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func checkAddr(ip netip.Addr, policy HostPolicy) error {
	if policy.RejectLoopback && ip.IsLoopback() {
		return fmt.Errorf("%w: loopback address %s", ErrBlockedHost, ip)
	}
	if policy.RejectLinkLocal && (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		return fmt.Errorf("%w: link-local address %s", ErrBlockedHost, ip)
	}
	if policy.RejectUnspecified && ip.IsUnspecified() {
		return fmt.Errorf("%w: unspecified address %s", ErrBlockedHost, ip)
	}
	if policy.RejectPrivate && isPrivate(ip) {
		return fmt.Errorf("%w: private address %s", ErrBlockedHost, ip)
	}
	return nil
}

func isPrivate(ip netip.Addr) bool {
	// IsPrivate covers RFC1918, fc00::/7, and loopback in Go's net/netip.
	// We exclude loopback here because RejectLoopback handles it separately
	// (with a more specific error message).
	if ip.IsLoopback() {
		return false
	}
	return ip.IsPrivate()
}
