package collectors

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemCollector_Collect(t *testing.T) {
	t.Parallel()
	c := SystemCollector{}
	device := &models.Device{}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, []string{"up", "warning"}, result.Status)
	assert.NotNil(t, result.CPUUsage)
	assert.NotNil(t, result.MemoryUsage)
	assert.NotNil(t, result.Details)
	assert.NotNil(t, result.ResponseTime)
}

func TestSystemCollector_Collect_Details(t *testing.T) {
	t.Parallel()
	c := SystemCollector{}
	device := &models.Device{}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	require.NotNil(t, result)

	details := result.Details
	assert.Contains(t, details, "cpu_usage")
	assert.Contains(t, details, "cpu_cores")
	assert.Contains(t, details, "memory_percent")
	assert.Contains(t, details, "disk_percent")
	assert.Contains(t, details, "uptime_seconds")
	assert.Contains(t, details, "load_avg_1m")
	assert.Contains(t, details, "load_avg_5m")
	assert.Contains(t, details, "load_avg_15m")
	assert.Contains(t, details, "system_info")

	sysInfo, ok := details["system_info"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, sysInfo, "cpu")
	assert.Contains(t, sysInfo, "memory")
	assert.Contains(t, sysInfo, "disk")
	assert.Contains(t, sysInfo, "uptime")
	assert.Contains(t, sysInfo, "loadAvg")
	assert.Contains(t, sysInfo, "hostname")
	assert.Contains(t, sysInfo, "goVersion")
	assert.Contains(t, sysInfo, "numCPU")
	assert.Contains(t, sysInfo, "numGoroutine")
}

func TestSystemCollector_Collect_ResponseTime(t *testing.T) {
	t.Parallel()
	c := SystemCollector{}
	device := &models.Device{}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.ResponseTime)
	assert.Equal(t, 0.0, *result.ResponseTime)
}

func TestSystemCollector_Collect_SecondCall(t *testing.T) {
	t.Parallel()
	c := SystemCollector{}
	device := &models.Device{}

	result1, err := c.Collect(context.Background(), device)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	result2, err := c.Collect(context.Background(), device)
	require.NoError(t, err)

	assert.Equal(t, result1.Status, result2.Status)
}

func TestSNMPCollector_Collect_ConnectionRefused(t *testing.T) {
	t.Parallel()
	c := SNMPCollector{}
	device := &models.Device{
		IPAddress: "127.0.0.1",
		SNMPPort:  19999, // Not listening
	}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	require.NotNil(t, result)
	// SNMP uses UDP - ConnectIPv4 may succeed even if nothing listens.
	// The goroutines will timeout and return zero values.
	assert.Contains(t, []string{"up", "down"}, result.Status)
}

func TestSNMPCollector_Collect_ConnectionRefused_v2c(t *testing.T) {
	t.Parallel()
	c := SNMPCollector{}
	device := &models.Device{
		IPAddress:     "127.0.0.1",
		SNMPPort:      19998,
		SNMPCommunity: "private",
		SNMPVersion:   "2c",
	}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, []string{"up", "down"}, result.Status)
}

func TestSNMPCollector_Collect_ConnectionRefused_v1(t *testing.T) {
	t.Parallel()
	c := SNMPCollector{}
	device := &models.Device{
		IPAddress:     "127.0.0.1",
		SNMPPort:      19997,
		SNMPCommunity: "public",
		SNMPVersion:   "1",
	}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, []string{"up", "down"}, result.Status)
}

func TestSNMPCollector_Collect_DefaultCommunity(t *testing.T) {
	t.Parallel()
	c := SNMPCollector{}
	device := &models.Device{
		IPAddress:     "127.0.0.1",
		SNMPPort:      19996,
		SNMPCommunity: "", // Should default to "public"
	}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, []string{"up", "down"}, result.Status)
}

func TestCollectInterfaces_ManyEntries(t *testing.T) {
	t.Parallel()
	baseTable := map[string]map[string]gosnmp.SnmpPDU{}
	xTable := map[string]map[string]gosnmp.SnmpPDU{}

	// Create more than 12 interfaces to test truncation
	for i := 1; i <= 15; i++ {
		idx := fmt.Sprintf("%d", i)
		baseTable[idx] = map[string]gosnmp.SnmpPDU{
			"2":  {Value: []byte(fmt.Sprintf("eth%d", i))},
			"5":  {Value: uint32(1000000)},
			"8":  {Value: uint32(1)},
			"10": {Value: uint64(i * 1000)},
			"16": {Value: uint64(i * 500)},
		}
	}

	interfaces := collectInterfaces(baseTable, xTable)
	assert.LessOrEqual(t, len(interfaces), 12)
}

func TestCollectInterfaces_SortByTraffic_Descending(t *testing.T) {
	t.Parallel()
	baseTable := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"2":  {Value: []byte("low-traffic")},
			"5":  {Value: uint32(1000)},
			"8":  {Value: uint32(1)},
			"10": {Value: uint64(100)},
			"16": {Value: uint64(100)},
		},
		"2": {
			"2":  {Value: []byte("high-traffic")},
			"5":  {Value: uint32(1000)},
			"8":  {Value: uint32(1)},
			"10": {Value: uint64(99999)},
			"16": {Value: uint64(99999)},
		},
		"3": {
			"2":  {Value: []byte("medium-traffic")},
			"5":  {Value: uint32(1000)},
			"8":  {Value: uint32(1)},
			"10": {Value: uint64(50000)},
			"16": {Value: uint64(50000)},
		},
	}

	interfaces := collectInterfaces(baseTable, nil)
	require.Len(t, interfaces, 3)
	assert.Equal(t, "high-traffic", interfaces[0]["name"])
	assert.Equal(t, "medium-traffic", interfaces[1]["name"])
	assert.Equal(t, "low-traffic", interfaces[2]["name"])
}

func TestSumStorageByType_MultipleRAM(t *testing.T) {
	t.Parallel()
	ramOID := ".1.3.6.1.2.1.25.2.1.2"
	table := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"2": {Value: ramOID},
			"4": {Value: uint32(1024)},
			"5": {Value: uint32(1000)},
			"6": {Value: uint32(500)},
		},
		"2": {
			"2": {Value: ramOID},
			"4": {Value: uint32(2048)},
			"5": {Value: uint32(2000)},
			"6": {Value: uint32(1000)},
		},
	}

	total, used := sumStorageByType(table, ramOID)
	assert.Equal(t, float64(1024*1000+2048*2000), total)
	assert.Equal(t, float64(1024*500+2048*1000), used)
}

func TestCollectInterfaces_WithXTableOverrides(t *testing.T) {
	t.Parallel()
	baseTable := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"2":  {Value: []byte("eth0")},
			"5":  {Value: uint32(100000000)},
			"8":  {Value: uint32(1)},
			"10": {Value: uint64(1000)},
			"16": {Value: uint64(500)},
		},
	}
	xTable := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"1":  {Value: []byte("eth0-v2")},
			"6":  {Value: uint64(1000000)},
			"10": {Value: uint64(800000)},
			"15": {Value: uint32(10)},
		},
	}

	interfaces := collectInterfaces(baseTable, xTable)
	require.Len(t, interfaces, 1)
	assert.Equal(t, "eth0-v2", interfaces[0]["name"])
	assert.Equal(t, int64(1000000), interfaces[0]["inOctets"])
	assert.Equal(t, int64(800000), interfaces[0]["outOctets"])
	assert.Equal(t, int64(10000000), interfaces[0]["speed"]) // 10 * 1e6
}

func TestCollectInterfaces_OperStatus2Filtered(t *testing.T) {
	t.Parallel()
	baseTable := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"2":  {Value: []byte("down-if")},
			"5":  {Value: uint32(1000)},
			"8":  {Value: uint32(2)}, // down
			"10": {Value: uint64(0)},
			"16": {Value: uint64(0)},
		},
		"2": {
			"2":  {Value: []byte("active-if")},
			"5":  {Value: uint32(1000)},
			"8":  {Value: uint32(1)}, // up
			"10": {Value: uint64(100)},
			"16": {Value: uint64(100)},
		},
	}

	interfaces := collectInterfaces(baseTable, nil)
	require.Len(t, interfaces, 1)
	assert.Equal(t, "active-if", interfaces[0]["name"])
}

func TestNormalizeCounter_LargeBytes(t *testing.T) {
	t.Parallel()
	result := NormalizeCounter([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	assert.Equal(t, int64(-1), result)
}

func TestPduToFloat64_Nil(t *testing.T) {
	t.Parallel()
	result := pduToFloat64(gosnmp.SnmpPDU{Value: nil})
	assert.Equal(t, float64(0), result)
}

func TestProtoNameFromNum_AllCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    int
		expected string
	}{
		{1, "ICMP"},
		{2, "IGMP"},
		{6, "TCP"},
		{17, "UDP"},
		{47, "GRE"},
		{50, "ESP"},
		{51, "AH"},
		{58, "ICMPv6"},
		{89, "OSPF"},
		{132, "SCTP"},
		{255, "PROTO_255"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, protoNameFromNum(tt.input))
		})
	}
}

func TestGetNetflowProtoName_AllCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    int
		expected string
	}{
		{1, "ICMP"},
		{6, "TCP"},
		{17, "UDP"},
		{47, "GRE"},
		{50, "ESP"},
		{51, "AH"},
		{58, "ICMPv6"},
		{89, "OSPF"},
		{132, "SCTP"},
		{0, "PROTO_0"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, GetNetflowProtoName(tt.input))
		})
	}
}

func TestNormalizeCounter_AllTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    any
		expected int64
	}{
		{"nil", nil, 0},
		{"uint64", uint64(42), 42},
		{"uint32", uint32(100), 100},
		{"int", int(7), 7},
		{"int64", int64(-5), -5},
		{"empty_bytes", []byte{}, 0},
		{"single_byte", []byte{0xFF}, 255},
		{"two_bytes", []byte{0x01, 0x00}, 256},
		{"three_bytes", []byte{0x01, 0x00, 0x00}, 65536},
		{"string_value", "test", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, NormalizeCounter(tt.input))
		})
	}
}

func TestF64_Zero(t *testing.T) {
	t.Parallel()
	v := f64(0)
	require.NotNil(t, v)
	assert.Equal(t, 0.0, *v)
}

func TestF64_Negative(t *testing.T) {
	t.Parallel()
	v := f64(-1.5)
	require.NotNil(t, v)
	assert.Equal(t, -1.5, *v)
}
