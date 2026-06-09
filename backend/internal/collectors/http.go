package collectors

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type HTTPCollector struct{}

func (HTTPCollector) Name() string { return "http" }

func (HTTPCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	host := device.IPAddress
	port := device.Port
	protocol := device.Protocol

	// Determine scheme: use https if protocol is "https" or port is 443
	scheme := "http"
	if protocol == "https" || port == 443 {
		scheme = "https"
	}

	// Build URL with explicit port
	var url string
	if port > 0 && !((scheme == "http" && port == 80) || (scheme == "https" && port == 443)) {
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
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	dur := time.Since(start)
	if err != nil {
		return &Result{Status: "down"}, nil
	}
	defer resp.Body.Close()
	rt := float64(dur.Milliseconds())
	status := "up"
	if device.HTTPExpectedStatus > 0 && resp.StatusCode != device.HTTPExpectedStatus {
		status = "down"
	}
	return &Result{Status: status, ResponseTime: &rt, Details: map[string]any{"http_status": resp.StatusCode}}, nil
}
