package collectors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── normalizeHost ─────────────────────────────────────────────────────────────

func TestNormalizeHost(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"http_prefix", "http://example.com", "example.com"},
		{"https_prefix", "https://example.com", "example.com"},
		{"bare_host", "example.com", "example.com"},
		{"trailing_slash", "example.com/", "example.com"},
		{"whitespace_and_http", "  http://example.com  ", "example.com"},
		{"whitespace_and_https", "  https://example.com  ", "example.com"},
		{"http_mixed_case", "HTTP://Example.COM", "Example.COM"},
		{"https_mixed_case", "HTTPS://Example.COM", "Example.COM"},
		{"empty_string", "", ""},
		{"http_with_path", "http://example.com/path", "example.com/path"},
		{"trailing_slashes", "example.com///", "example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, normalizeHost(tt.input))
		})
	}
}

// ── HTTPCollector.Collect ─────────────────────────────────────────────────────

func TestHTTPCollector_Collect_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "HEAD", r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := HTTPCollector{}
	device := &models.Device{IPAddress: server.URL, Port: 0}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	assert.Equal(t, "up", result.Status)
	assert.NotNil(t, result.ResponseTime)
	assert.NotNil(t, result.Details)
	assert.Equal(t, http.StatusOK, result.Details["http_status"])
}

func TestHTTPCollector_Collect_Timeout(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := HTTPCollector{}
	device := &models.Device{IPAddress: server.URL, Port: 0}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := c.Collect(ctx, device)
	require.NoError(t, err)
	assert.Equal(t, "down", result.Status)
}

func TestHTTPCollector_Collect_CustomPath(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := HTTPCollector{}
	device := &models.Device{IPAddress: server.URL, HTTPPath: "/health"}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	assert.Equal(t, "up", result.Status)
}

func TestHTTPCollector_Collect_ExpectedStatusMismatch(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := HTTPCollector{}
	device := &models.Device{
		IPAddress:          server.URL,
		Port:               0,
		HTTPExpectedStatus: http.StatusOK,
	}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	assert.Equal(t, "down", result.Status)
}

func TestHTTPCollector_Collect_ExplicitPort(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := HTTPCollector{}
	device := &models.Device{IPAddress: "127.0.0.1", Port: 19876}
	// Unreachable port — should return down
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	assert.Equal(t, "down", result.Status)
}

func TestHTTPCollector_Collect_ExpectedStatusMatches(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	c := HTTPCollector{}
	device := &models.Device{
		IPAddress:          server.URL,
		HTTPExpectedStatus: http.StatusAccepted,
	}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	assert.Equal(t, "up", result.Status)
}

// ── HTTPSCollector.Collect ────────────────────────────────────────────────────

func TestHTTPSCollector_Collect_DelegatesToHTTP(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := HTTPSCollector{}
	// Use server.URL which will be https://127.0.0.1:<port>
	device := &models.Device{IPAddress: server.URL, Protocol: "https"}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	assert.Equal(t, "up", result.Status)
}

// ── PingCollector.Collect ─────────────────────────────────────────────────────

func TestPingCollector_Collect_InvalidHost(t *testing.T) {
	t.Parallel()
	c := PingCollector{}
	device := &models.Device{IPAddress: "192.0.2.1"} // TEST-NET-1, unreachable
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	assert.Equal(t, "down", result.Status)
}

func TestPingCollector_Collect_InvalidHostname(t *testing.T) {
	t.Parallel()
	c := PingCollector{}
	device := &models.Device{IPAddress: "this.host.does.not.exist.invalid"}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	assert.Equal(t, "down", result.Status)
}

// ── CaptureCollector ──────────────────────────────────────────────────────────

func TestCaptureCollector_Name(t *testing.T) {
	t.Parallel()
	c := &CaptureCollector{}
	assert.Equal(t, "packet_capture", c.Name())
}

func TestCaptureCollector_Collect(t *testing.T) {
	t.Parallel()
	c := &CaptureCollector{}
	result, err := c.Collect(context.Background(), &models.Device{})
	require.NoError(t, err)
	assert.Equal(t, "up", result.Status)
}

func TestCaptureCollector_StartStopIsRunning(t *testing.T) {
	t.Parallel()
	c := &CaptureCollector{Interface: "eth0", Filter: "tcp"}

	assert.False(t, c.IsRunning())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Start(ctx)
	require.NoError(t, err)
	assert.True(t, c.IsRunning())

	// Start again — should be idempotent
	err = c.Start(ctx)
	require.NoError(t, err)
	assert.True(t, c.IsRunning())

	c.Stop()
	assert.False(t, c.IsRunning())
}

func TestCaptureCollector_Stats(t *testing.T) {
	t.Parallel()
	c := &CaptureCollector{}
	stats := c.Stats()
	assert.NotNil(t, stats)
}

func TestCaptureCollector_Start_ContextCancel(t *testing.T) {
	t.Parallel()
	c := &CaptureCollector{Interface: "eth0"}
	ctx, cancel := context.WithCancel(context.Background())

	err := c.Start(ctx)
	require.NoError(t, err)
	assert.True(t, c.IsRunning())

	cancel()
	time.Sleep(10 * time.Millisecond)
	assert.False(t, c.IsRunning())
}

// ── pduToFloat64 ─────────────────────────────────────────────────────────────

func TestPduToFloat64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pdu      gosnmp.SnmpPDU
		expected float64
	}{
		{"uint32", gosnmp.SnmpPDU{Value: uint32(42)}, 42},
		{"uint64", gosnmp.SnmpPDU{Value: uint64(100)}, 100},
		{"uint", gosnmp.SnmpPDU{Value: uint(7)}, 7},
		{"int", gosnmp.SnmpPDU{Value: int(-3)}, -3},
		{"int64", gosnmp.SnmpPDU{Value: int64(999)}, 999},
		{"bytes_single", gosnmp.SnmpPDU{Value: []byte{0xFF}}, 255},
		{"bytes_two", gosnmp.SnmpPDU{Value: []byte{0x01, 0x00}}, 256},
		{"bytes_empty", gosnmp.SnmpPDU{Value: []byte{}}, 0},
		{"bytes_long", gosnmp.SnmpPDU{Value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}}, 0},
		{"string_fallback", gosnmp.SnmpPDU{Value: "hello"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, pduToFloat64(tt.pdu))
		})
	}
}

// ── sumStorageByType ─────────────────────────────────────────────────────────

func TestSumStorageByType(t *testing.T) {
	t.Parallel()

	ramOID := ".1.3.6.1.2.1.25.2.1.2"
	diskOID := ".1.3.6.1.2.1.25.2.1.4"

	table := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"2": {Value: ramOID},
			"4": {Value: uint32(1024)},
			"5": {Value: uint32(1000)},
			"6": {Value: uint32(500)},
		},
		"2": {
			"2": {Value: diskOID},
			"4": {Value: uint32(512)},
			"5": {Value: uint32(2000)},
			"6": {Value: uint32(1000)},
		},
		"3": {
			"2": {Value: ramOID},
			"4": {Value: uint32(512)},
			"5": {Value: uint32(800)},
			"6": {Value: uint32(200)},
		},
	}

	totalRam, usedRam := sumStorageByType(table, ramOID)
	assert.Equal(t, float64(1024*1000+512*800), totalRam)
	assert.Equal(t, float64(1024*500+512*200), usedRam)

	totalDisk, usedDisk := sumStorageByType(table, diskOID)
	assert.Equal(t, float64(512*2000), totalDisk)
	assert.Equal(t, float64(512*1000), usedDisk)
}

func TestSumStorageByType_EmptyTable(t *testing.T) {
	t.Parallel()
	table := map[string]map[string]gosnmp.SnmpPDU{}
	total, used := sumStorageByType(table, ".1.3.6.1.2.1.25.2.1.2")
	assert.Equal(t, float64(0), total)
	assert.Equal(t, float64(0), used)
}

func TestSumStorageByType_NoMatchingType(t *testing.T) {
	t.Parallel()
	table := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"2": {Value: ".1.3.6.1.2.1.25.2.1.4"},
			"4": {Value: uint32(1024)},
			"5": {Value: uint32(1000)},
			"6": {Value: uint32(500)},
		},
	}
	total, used := sumStorageByType(table, ".1.3.6.1.2.1.25.2.1.2")
	assert.Equal(t, float64(0), total)
	assert.Equal(t, float64(0), used)
}

func TestSumStorageByType_MissingColumn(t *testing.T) {
	t.Parallel()
	table := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"2": {Value: ".1.3.6.1.2.1.25.2.1.2"},
		},
	}
	total, used := sumStorageByType(table, ".1.3.6.1.2.1.25.2.1.2")
	assert.Equal(t, float64(0), total)
	assert.Equal(t, float64(0), used)
}

// ── collectInterfaces ─────────────────────────────────────────────────────────

func TestCollectInterfaces(t *testing.T) {
	t.Parallel()
	baseTable := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"2":  {Value: []byte("eth0")},
			"5":  {Value: uint32(1000000000)},
			"8":  {Value: uint32(1)},
			"10": {Value: uint64(500000)},
			"16": {Value: uint64(300000)},
		},
		"2": {
			"2":  {Value: []byte("lo")},
			"5":  {Value: uint32(10000000)},
			"8":  {Value: uint32(1)},
			"10": {Value: uint64(100)},
			"16": {Value: uint64(100)},
		},
	}
	xTable := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"1":  {Value: []byte("eth0")},
			"6":  {Value: uint64(1000000)},
			"10": {Value: uint64(800000)},
			"15": {Value: uint32(10)},
		},
	}

	interfaces := collectInterfaces(baseTable, xTable)
	require.Len(t, interfaces, 2)
	// eth0 has more traffic, should be first
	assert.Equal(t, "eth0", interfaces[0]["name"])
	assert.Equal(t, int64(1000000), interfaces[0]["inOctets"])
	assert.Equal(t, int64(800000), interfaces[0]["outOctets"])
	assert.Equal(t, int64(10000000), interfaces[0]["speed"]) // 10 Mbps * 1e6
}

func TestCollectInterfaces_Empty(t *testing.T) {
	t.Parallel()
	interfaces := collectInterfaces(nil, nil)
	assert.Nil(t, interfaces)
}

func TestCollectInterfaces_NameFromNonBytes(t *testing.T) {
	t.Parallel()
	baseTable := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"2":  {Value: uint32(99)},
			"5":  {Value: uint32(1000)},
			"8":  {Value: uint32(1)},
			"10": {Value: uint64(50)},
			"16": {Value: uint64(25)},
		},
	}
	interfaces := collectInterfaces(baseTable, nil)
	require.Len(t, interfaces, 1)
	assert.Equal(t, "if1", interfaces[0]["name"])
}

func TestCollectInterfaces_SortByTraffic(t *testing.T) {
	t.Parallel()
	baseTable := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"2":  {Value: []byte("low")},
			"5":  {Value: uint32(1000)},
			"8":  {Value: uint32(1)},
			"10": {Value: uint64(10)},
			"16": {Value: uint64(10)},
		},
		"2": {
			"2":  {Value: []byte("high")},
			"5":  {Value: uint32(1000)},
			"8":  {Value: uint32(1)},
			"10": {Value: uint64(999999)},
			"16": {Value: uint64(999999)},
		},
	}
	interfaces := collectInterfaces(baseTable, nil)
	require.Len(t, interfaces, 2)
	assert.Equal(t, "high", interfaces[0]["name"])
}

func TestCollectInterfaces_InactiveFiltered(t *testing.T) {
	t.Parallel()
	baseTable := map[string]map[string]gosnmp.SnmpPDU{
		"1": {
			"2":  {Value: []byte("down-if")},
			"5":  {Value: uint32(1000)},
			"8":  {Value: uint32(2)}, // operStatus != 1
			"10": {Value: uint64(0)},
			"16": {Value: uint64(0)},
		},
	}
	interfaces := collectInterfaces(baseTable, nil)
	assert.Len(t, interfaces, 0)
}
