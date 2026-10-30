package collectors

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestSNMPCollector_UnreachableAgentIsDown(t *testing.T) {
	// Grab a free UDP port and release it so nothing answers there.
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot open UDP socket:", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	_ = conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := SNMPCollector{}.Collect(ctx, &models.Device{IPAddress: "127.0.0.1", SNMPPort: port})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "down" {
		t.Fatalf("status = %q, want down", result.Status)
	}
}

func TestSNMPCollector_HasTimeoutMeta(t *testing.T) {
	var c Collector = SNMPCollector{}
	meta, ok := c.(CollectorMeta)
	if !ok || meta.DefaultTimeout() <= 0 {
		t.Fatal("SNMPCollector must declare a default timeout")
	}
}
