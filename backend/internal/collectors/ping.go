package collectors

import (
	"context"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type PingCollector struct{}

func (PingCollector) Name() string { return "ping" }

func (PingCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	pinger, err := probing.NewPinger(device.IPAddress)
	if err != nil {
		return &Result{Status: "down"}, nil
	}
	pinger.Count = 3
	pinger.Timeout = 5 * time.Second

	// Try privileged mode first (requires root/CAP_NET_RAW)
	pinger.SetPrivileged(true)
	if err := pinger.RunWithContext(ctx); err != nil {
		// Fallback to unprivileged mode
		pinger2, err2 := probing.NewPinger(device.IPAddress)
		if err2 != nil {
			return &Result{Status: "down"}, nil
		}
		pinger2.Count = 3
		pinger2.Timeout = 5 * time.Second
		pinger2.SetPrivileged(false)
		if err2 = pinger2.RunWithContext(ctx); err2 != nil {
			return &Result{Status: "down"}, nil
		}
		stats := pinger2.Statistics()
		if stats.PacketsRecv == 0 {
			return &Result{Status: "down", PacketLoss: f64(100)}, nil
		}
		rt := float64(stats.AvgRtt.Milliseconds())
		return &Result{Status: "up", ResponseTime: &rt, PacketLoss: &stats.PacketLoss}, nil
	}

	stats := pinger.Statistics()
	if stats.PacketsRecv == 0 {
		return &Result{Status: "down", PacketLoss: f64(100)}, nil
	}
	rt := float64(stats.AvgRtt.Milliseconds())
	return &Result{Status: "up", ResponseTime: &rt, PacketLoss: &stats.PacketLoss}, nil
}
