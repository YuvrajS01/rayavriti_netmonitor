package collectors

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type HTTPCollector struct{}

func (HTTPCollector) Name() string { return "http" }

func (HTTPCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	url := fmt.Sprintf("http://%s%s", device.IPAddress, device.HTTPPath)
	if device.HTTPPath == "" {
		url = fmt.Sprintf("http://%s", device.IPAddress)
	}
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	req.Header.Set("User-Agent", "NetMonitor/1.0")
	client := &http.Client{Timeout: 10 * time.Second}
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
