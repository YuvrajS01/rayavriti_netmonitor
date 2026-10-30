package netutil

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// ErrRedirectBlocked is returned by clients built with SafeHTTPClient when the
// server answers with a redirect.
var ErrRedirectBlocked = errors.New("redirects are not followed")

// SafeDialControl returns a net.Dialer Control hook that re-checks every
// address the dialer actually connects to. Unlike ValidateHost, this runs after
// DNS resolution, so it also defeats DNS rebinding between validation and use.
func SafeDialControl(policy HostPolicy) func(network, address string, c syscall.RawConn) error {
	return func(_ string, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		ip, err := netip.ParseAddr(host)
		if err != nil {
			return fmt.Errorf("unexpected dial address %q: %w", address, err)
		}
		return checkAddr(ip, policy)
	}
}

// SafeDialer returns a dialer whose connections are restricted by policy.
func SafeDialer(policy HostPolicy, timeout time.Duration) *net.Dialer {
	return &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second, Control: SafeDialControl(policy)}
}

// SafeTransport returns an http.Transport that only connects to addresses
// permitted by policy.
func SafeTransport(policy HostPolicy) *http.Transport {
	dialer := SafeDialer(policy, 10*time.Second)
	return &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
}

// SafeHTTPClient wraps transport in a client that refuses redirects, so
// credentials and request bodies are never forwarded to a different host.
func SafeHTTPClient(transport http.RoundTripper, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return ErrRedirectBlocked
		},
	}
}
