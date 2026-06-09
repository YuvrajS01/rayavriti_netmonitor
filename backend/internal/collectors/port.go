package collectors

import (
	"context"
	"fmt"
	"net"
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
	addr := fmt.Sprintf("%s:%d", device.IPAddress, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	dur := time.Since(start)
	if err != nil {
		return &Result{Status: "down"}, nil
	}
	conn.Close()
	rt := float64(dur.Milliseconds())
	return &Result{Status: "up", ResponseTime: &rt}, nil
}
