package collectors

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type PortCollector struct{}

func (PortCollector) Name() string { return "port" }

func (PortCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	port := device.Port
	if port == 0 {
		port = 80
	}
	addr := net.JoinHostPort(device.IPAddress, strconv.Itoa(port))

	// Use a context-aware dialer so the poll is cancelled when ctx is
	// done (e.g. shutdown). Cap the dial deadline at 5 seconds as a
	// fallback (M26 — previously used net.DialTimeout which ignored ctx).
	dialer := net.Dialer{Timeout: 5 * time.Second}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	dur := time.Since(start)
	if err != nil {
		return &Result{Status: "down"}, nil //nolint:nilerr // intentional: return down status, not error
	}
	_ = conn.Close()
	rt := float64(dur.Milliseconds())
	return &Result{Status: "up", ResponseTime: &rt}, nil
}
