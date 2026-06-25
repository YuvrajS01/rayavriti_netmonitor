package collectors

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type HTTPCollector struct{}

func (HTTPCollector) Name() string { return "http" }

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

func (HTTPCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	host := normalizeHost(device.IPAddress)
	port := device.Port
	protocol := device.Protocol

	// Determine scheme: use https if protocol is "https" or port is 443
	scheme := "http"
	if protocol == "https" || port == 443 {
		scheme = "https"
	}

	// Build URL with explicit port
	var url string
	if port > 0 && (scheme != "http" || port != 80) && (scheme != "https" || port != 443) {
		url = fmt.Sprintf("%s://%s:%d%s", scheme, host, port, device.HTTPPath)
	} else if device.HTTPPath != "" {
		url = fmt.Sprintf("%s://%s%s", scheme, host, device.HTTPPath)
	} else {
		url = fmt.Sprintf("%s://%s", scheme, host)
	}

	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	req.Header.Set("User-Agent", "NetMonitor/1.0")

	// For HTTPS, skip TLS verification for self-signed certs
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // Intentional: self-signed certs on local network devices
		},
	}
	resp, err := client.Do(req)
	dur := time.Since(start)
	if err != nil {
		return &Result{Status: "down"}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	rt := float64(dur.Milliseconds())
	status := "up"
	if device.HTTPExpectedStatus > 0 && resp.StatusCode != device.HTTPExpectedStatus {
		status = "down"
	}
	return &Result{Status: status, ResponseTime: &rt, Details: map[string]any{"http_status": resp.StatusCode}}, nil
}
