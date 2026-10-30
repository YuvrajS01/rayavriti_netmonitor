package collectors

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/netutil"
)

// HTTPCollector polls HTTP(S) endpoints. The zero value uses the shared,
// SSRF-guarded client; client is only overridden in tests.
type HTTPCollector struct {
	client *http.Client
}

func (HTTPCollector) Name() string { return "http" }

// sharedHTTPClient reuses a single Transport across all HTTP polls so idle
// connections are pooled and goroutine cleanup doesn't accumulate (M25).
// Every connection is re-checked against the default host policy (no
// loopback or cloud metadata, even via DNS rebinding) and redirects are not
// followed: the 3xx itself is the check result.
var sharedHTTPClient = newHTTPClient(netutil.DefaultHostPolicy())

func newHTTPClient(policy netutil.HostPolicy) *http.Client {
	transport := netutil.SafeTransport(policy)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // Intentional: self-signed certs on local network devices
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// normalizeHost strips any scheme prefix from the host field, in case
// the user pasted a full URL like "https://example.com" as the IP address.
func normalizeHost(raw string) string {
	h := strings.TrimSpace(raw)
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(strings.ToLower(h), prefix) {
			h = h[len(prefix):]
			break
		}
	}
	h = strings.TrimRight(h, "/")
	return h
}

func (c HTTPCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	host := normalizeHost(device.IPAddress)
	port := device.Port
	protocol := device.Protocol

	// Determine scheme: use https if protocol is "https" or port is 443
	scheme := "http"
	if protocol == "https" || port == 443 {
		scheme = "https"
	}

	// Build the URL structurally: a path like "@169.254.169.254/" spliced
	// into a format string would otherwise change the request's host.
	if port > 0 && (scheme != "http" || port != 80) && (scheme != "https" || port != 443) {
		if _, _, err := net.SplitHostPort(host); err != nil {
			host = net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(port))
		}
	}
	target := url.URL{Scheme: scheme, Host: host}
	if device.HTTPPath != "" {
		if !strings.HasPrefix(device.HTTPPath, "/") || strings.HasPrefix(device.HTTPPath, "//") {
			return &Result{Status: "down", Details: map[string]any{"error": "httpPath must start with a single /"}}, nil
		}
		parsed, err := url.Parse(device.HTTPPath)
		if err != nil {
			return &Result{Status: "down", Details: map[string]any{"error": "invalid httpPath"}}, nil
		}
		target.Path, target.RawQuery = parsed.Path, parsed.RawQuery
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "HEAD", target.String(), nil)
	if err != nil {
		return &Result{Status: "down", Details: map[string]any{"error": "invalid request URL"}}, nil
	}
	req.Header.Set("User-Agent", "NetMonitor/1.0")

	client := c.client
	if client == nil {
		client = sharedHTTPClient
	}
	resp, err := client.Do(req)
	dur := time.Since(start)
	if err != nil {
		return &Result{Status: "down"}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	rt := float64(dur.Microseconds()) / 1000
	status := "up"
	switch {
	case device.HTTPExpectedStatus > 0:
		if resp.StatusCode != device.HTTPExpectedStatus {
			status = "down"
		}
	case resp.StatusCode >= 500:
		// A server error is not a healthy service, even without an
		// explicitly configured expected status.
		status = "down"
	}
	return &Result{Status: status, ResponseTime: &rt, Details: map[string]any{"http_status": resp.StatusCode}}, nil
}
