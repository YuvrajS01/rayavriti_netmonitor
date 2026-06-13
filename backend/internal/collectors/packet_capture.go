//go:build !nocapture

package collectors

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// PacketStats holds basic stats about a live capture session.
type PacketStats struct {
	PacketsTotal  int64
	BytesTotal    int64
	PacketsPerSec float64
	TopProtocols  map[string]int64
}

// CaptureCollector is a stub for libpcap-based packet capture.
// Full implementation requires CGo + gopacket; this stub is always compiled
// so the rest of the codebase can reference it. Enable with build tag pcap.
type CaptureCollector struct {
	Interface string
	Filter    string
	running   atomic.Bool
	stats     PacketStats
	startTime time.Time
}

func (c *CaptureCollector) Name() string { return "packet_capture" }

func (c *CaptureCollector) Collect(_ context.Context, _ *models.Device) (*Result, error) {
	return &Result{Status: "up"}, nil
}

// Start begins packet capture on the configured interface.
// Stub: logs intent; real capture requires gopacket/pcap build tag.
func (c *CaptureCollector) Start(ctx context.Context) error {
	if c.running.Swap(true) {
		return nil // already running
	}
	c.startTime = time.Now()
	c.stats = PacketStats{TopProtocols: map[string]int64{}}
	slog.Info("Packet capture started (stub — build with pcap tag for live capture)",
		"interface", c.Interface, "filter", c.Filter)
	go func() {
		<-ctx.Done()
		c.running.Store(false)
		slog.Info("Packet capture stopped")
	}()
	return nil
}

func (c *CaptureCollector) Stop() { c.running.Store(false) }

func (c *CaptureCollector) IsRunning() bool { return c.running.Load() }

func (c *CaptureCollector) Stats() PacketStats { return c.stats }
