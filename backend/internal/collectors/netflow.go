package collectors

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// NetFlowCollector listens on a UDP port for NetFlow v5 datagrams and
// writes parsed flow records to the provided channel.
type NetFlowCollector struct {
	Port    int
	FlowsCh chan<- []models.Flow
}

func (c *NetFlowCollector) Name() string { return "netflow" }

// Collect is a no-op for the scheduler – the NetFlow collector is driven by
// its own goroutine started via Listen.
func (c *NetFlowCollector) Collect(_ context.Context, _ *models.Device) (*Result, error) {
	return &Result{Status: "up"}, nil
}

// Listen starts a UDP listener and publishes flows until ctx is cancelled.
func (c *NetFlowCollector) Listen(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", c.Port)
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("netflow listen %s: %w", addr, err)
	}
	slog.Info("NetFlow collector listening", "addr", addr)

	go func() {
		<-ctx.Done()
		_ = pc.Close()
	}()

	buf := make([]byte, 2048)
	for {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				slog.Warn("NetFlow read error", "error", err)
				continue
			}
		}
		flows := parseNetFlowV5(buf[:n])
		if len(flows) > 0 && c.FlowsCh != nil {
			select {
			case c.FlowsCh <- flows:
			default:
			}
		}
	}
}

// parseNetFlowV5 parses a NetFlow v5 packet (simplified – header + records).
func parseNetFlowV5(data []byte) []models.Flow {
	if len(data) < 24 {
		return nil
	}
	version := binary.BigEndian.Uint16(data[0:2])
	if version != 5 {
		return nil
	}
	count := int(binary.BigEndian.Uint16(data[2:4]))
	now := time.Now()
	var flows []models.Flow
	offset := 24
	for i := 0; i < count && offset+48 <= len(data); i++ {
		rec := data[offset : offset+48]                       //nolint:gosec // Bounds guaranteed by loop condition offset+48 <= len(data)
		srcIP := net.IP(rec[0:4]).String()                    //nolint:gosec // Bounds guaranteed by loop condition offset+48 <= len(data)
		dstIP := net.IP(rec[4:8]).String()                    //nolint:gosec // Bounds guaranteed by loop condition offset+48 <= len(data)
		srcPort := int(binary.BigEndian.Uint16(rec[32:34]))   //nolint:gosec // Bounds guaranteed by loop condition offset+48 <= len(data)
		dstPort := int(binary.BigEndian.Uint16(rec[34:36]))   //nolint:gosec // Bounds guaranteed by loop condition offset+48 <= len(data)
		proto := int(rec[38])                                 //nolint:gosec // Bounds guaranteed by loop condition offset+48 <= len(data)
		bytes := int64(binary.BigEndian.Uint32(rec[20:24]))   //nolint:gosec // Bounds guaranteed by loop condition offset+48 <= len(data)
		packets := int64(binary.BigEndian.Uint32(rec[16:20])) //nolint:gosec // Bounds guaranteed by loop condition offset+48 <= len(data)
		flows = append(flows, models.Flow{
			Timestamp: now,
			SrcIP:     srcIP,
			DstIP:     dstIP,
			SrcPort:   srcPort,
			DstPort:   dstPort,
			Protocol:  protoName(proto),
			Bytes:     bytes,
			Packets:   packets,
		})
		offset += 48
	}
	return flows
}

func protoName(p int) string {
	switch p {
	case 6:
		return "TCP"
	case 17:
		return "UDP"
	case 1:
		return "ICMP"
	default:
		return fmt.Sprintf("%d", p)
	}
}
