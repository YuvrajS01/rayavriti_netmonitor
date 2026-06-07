package collectors

import (
	"context"
	"fmt"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// OID for sysUpTime (in centiseconds) — used as a basic connectivity check.
const oidSysUpTime = ".1.3.6.1.2.1.1.3.0"

type SNMPCollector struct{}

func (SNMPCollector) Name() string { return "snmp" }

func (SNMPCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	community := "public"
	if device.SNMPCommunity != "" {
		community = device.SNMPCommunity
	}
	port := uint16(161)
	if device.SNMPPort > 0 {
		port = uint16(device.SNMPPort)
	}

	version := gosnmp.Version2c
	if device.SNMPVersion == "1" {
		version = gosnmp.Version1
	}

	g := &gosnmp.GoSNMP{
		Target:    device.IPAddress,
		Port:      port,
		Community: community,
		Version:   version,
		Timeout:   5 * time.Second,
		Retries:   1,
	}

	start := time.Now()
	if err := g.ConnectIPv4(); err != nil {
		return &Result{Status: "down", Details: map[string]any{"error": err.Error()}}, nil
	}
	defer g.Conn.Close()

	result, err := g.Get([]string{oidSysUpTime})
	elapsed := float64(time.Since(start).Milliseconds())
	if err != nil || result == nil || len(result.Variables) == 0 {
		return &Result{
			Status:       "down",
			ResponseTime: f64(elapsed),
			Details:      map[string]any{"error": fmt.Sprintf("SNMP get failed: %v", err)},
		}, nil
	}

	details := map[string]any{}
	for _, v := range result.Variables {
		details[v.Name] = gosnmp.ToBigInt(v.Value).String()
	}
	return &Result{Status: "up", ResponseTime: f64(elapsed), Details: details}, nil
}
