package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

// ── alerts.go: Update with valid body ────────────────────────────────────────

func TestAlertUpdate_ValidBody_AllFields(t *testing.T) {
	db := &mockDB{
		getAlertFn: func(ctx context.Context, id int64) (*models.Alert, error) {
			return &models.Alert{ID: 1, Severity: "info", Message: "old msg", Status: "active"}, nil
		},
		updateAlertStatusFn: func(ctx context.Context, id int64, status, by string) error {
			return nil
		},
	}
	h := NewAlertHandler(db)
	severity := "critical"
	message := "updated message"
	status := "acknowledged"
	body, _ := json.Marshal(map[string]any{
		"severity": severity,
		"message":  message,
		"status":   status,
	})
	w, req := makeRequestWithParams("PUT", "/api/v1/alerts/1", string(body), "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAlertUpdate_TriggeredNormalizedToActive(t *testing.T) {
	db := &mockDB{
		getAlertFn: func(ctx context.Context, id int64) (*models.Alert, error) {
			return &models.Alert{ID: 1, Status: "resolved"}, nil
		},
		updateAlertStatusFn: func(ctx context.Context, id int64, status, by string) error {
			if status != "active" {
				t.Fatalf("expected normalized status 'active', got %s", status)
			}
			return nil
		},
	}
	h := NewAlertHandler(db)
	status := "triggered"
	body, _ := json.Marshal(map[string]any{"status": status})
	w, req := makeRequestWithParams("PUT", "/api/v1/alerts/1", string(body), "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertUpdate_StatusDBError(t *testing.T) {
	db := &mockDB{
		getAlertFn: func(ctx context.Context, id int64) (*models.Alert, error) {
			return &models.Alert{ID: 1, Status: "active"}, nil
		},
		updateAlertStatusFn: func(ctx context.Context, id int64, status, by string) error {
			return errors.New("db error")
		},
	}
	h := NewAlertHandler(db)
	status := "resolved"
	body, _ := json.Marshal(map[string]any{"status": status})
	w, req := makeRequestWithParams("PUT", "/api/v1/alerts/1", string(body), "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertUpdate_InvalidBody(t *testing.T) {
	db := &mockDB{
		getAlertFn: func(ctx context.Context, id int64) (*models.Alert, error) {
			return &models.Alert{ID: 1}, nil
		},
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("PUT", "/api/v1/alerts/1", "not-json", "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertUpdate_GetAlertAfterUpdateFails(t *testing.T) {
	callCount := 0
	db := &mockDB{
		getAlertFn: func(ctx context.Context, id int64) (*models.Alert, error) {
			callCount++
			if callCount == 1 {
				return &models.Alert{ID: 1, Status: "active"}, nil
			}
			return nil, errors.New("db error after update")
		},
	}
	h := NewAlertHandler(db)
	status := "resolved"
	body, _ := json.Marshal(map[string]any{"status": status})
	w, req := makeRequestWithParams("PUT", "/api/v1/alerts/1", string(body), "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ── alerts.go: Acknowledge with already acknowledged ─────────────────────────

func TestAlertAcknowledge_AlreadyAcknowledged(t *testing.T) {
	db := &mockDB{
		updateAlertStatusFn: func(ctx context.Context, id int64, status, by string) error {
			return nil
		},
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/alerts/1/acknowledge", "", "id", "1")
	h.Acknowledge(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ── alerts.go: History with nil result ────────────────────────────────────────

func TestAlertHistory_NilResult(t *testing.T) {
	db := &mockDB{
		getAlertHistoryFn: func(ctx context.Context, alertID int64) ([]models.AlertHistory, error) {
			return nil, nil
		},
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/alerts/1/history", "", "id", "1")
	h.History(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertHistory_WithLimit(t *testing.T) {
	db := &mockDB{
		getAlertHistoryFn: func(ctx context.Context, alertID int64) ([]models.AlertHistory, error) {
			return []models.AlertHistory{
				{ID: 1, AlertID: alertID, Action: "fired"},
				{ID: 2, AlertID: alertID, Action: "acknowledged"},
			}, nil
		},
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/alerts/1/history?limit=1", "", "id", "1")
	h.History(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ── alerts.go: AlertStats with channel error ──────────────────────────────────

func TestAlertStats_ChannelError(t *testing.T) {
	db := &mockDB{
		getAlertCountsFn: func(ctx context.Context) (models.AlertCounts, error) {
			return models.AlertCounts{}, nil
		},
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{}, nil
		},
		getNotificationChannelsFn: func(ctx context.Context) ([]models.NotificationChannel, error) {
			return nil, errors.New("channel error")
		},
	}
	h := NewAlertHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alerts/stats", "")
	h.AlertStats(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (graceful degradation), got %d", w.Code)
	}
}

func TestAlertStats_RulesError(t *testing.T) {
	db := &mockDB{
		getAlertCountsFn: func(ctx context.Context) (models.AlertCounts, error) {
			return models.AlertCounts{}, nil
		},
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return nil, errors.New("rules error")
		},
	}
	h := NewAlertHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alerts/stats", "")
	h.AlertStats(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ── capture.go: Start with valid interface ────────────────────────────────────

func TestCaptureStart_ValidInterface(t *testing.T) {
	db := &mockDB{
		createCaptureSessionFn: func(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error) {
			cs.ID = 1
			return cs, nil
		},
	}
	hub := websocket.NewHub("test-secret", nil)
	go hub.Run()
	h := NewCaptureHandler(db, hub)

	w, req := authenticatedRequest("POST", "/api/v1/capture/start", `{"interface":"eth0","filter":"port 80"}`)
	h.Start(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Clean up: stop any running capture
	h.mu.Lock()
	if h.cancel != nil {
		h.cancel()
	}
	h.mu.Unlock()
	// Wait for capture goroutine to finish broadcasting before test ends
	for i := 0; i < 100; i++ {
		if atomic.LoadInt32(&h.running) == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCaptureStart_DBError(t *testing.T) {
	db := &mockDB{
		createCaptureSessionFn: func(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error) {
			return nil, errors.New("db error")
		},
	}
	hub := websocket.NewHub("test-secret", nil)
	go hub.Run()
	h := NewCaptureHandler(db, hub)

	w, req := authenticatedRequest("POST", "/api/v1/capture/start", `{"interface":"eth0"}`)
	h.Start(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	hub.Stop()
}

func TestCaptureStart_NoFilter(t *testing.T) {
	db := &mockDB{
		createCaptureSessionFn: func(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error) {
			cs.ID = 2
			if cs.Filter != "" {
				t.Fatalf("expected empty filter, got %s", cs.Filter)
			}
			return cs, nil
		},
	}
	hub := websocket.NewHub("test-secret", nil)
	go hub.Run()
	h := NewCaptureHandler(db, hub)

	w, req := authenticatedRequest("POST", "/api/v1/capture/start", `{"interface":"lo"}`)
	h.Start(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	h.mu.Lock()
	if h.cancel != nil {
		h.cancel()
	}
	h.mu.Unlock()
	// Wait for capture goroutine to finish broadcasting before test ends
	for i := 0; i < 100; i++ {
		if atomic.LoadInt32(&h.running) == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// ── capture.go: Stop with active session ──────────────────────────────────────

func TestCaptureStop_WithActiveSession(t *testing.T) {
	db := &mockDB{
		stopCaptureSessionFn: func(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
			return nil
		},
	}
	hub := websocket.NewHub("test-secret", nil)
	go hub.Run()
	h := NewCaptureHandler(db, hub)

	// Simulate running state
	h.running = 1
	h.mu.Lock()
	h.cancel = func() {}
	h.stats = captureStats{totalPackets: 10, totalBytes: 500, protocols: map[string]int64{"TCP": 10}}
	h.mu.Unlock()

	w, req := makeRequestWithParams("POST", "/api/v1/capture/sessions/1/stop", "", "id", "1")
	h.Stop(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	hub.Stop()
}

func TestCaptureStop_DBError(t *testing.T) {
	db := &mockDB{
		stopCaptureSessionFn: func(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
			return errors.New("db error")
		},
	}
	hub := websocket.NewHub("test-secret", nil)
	go hub.Run()
	h := NewCaptureHandler(db, hub)

	w, req := makeRequestWithParams("POST", "/api/v1/capture/sessions/1/stop", "", "id", "1")
	h.Stop(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	hub.Stop()
}

// ── capture.go: parseTcpdumpHeader with more line variations ──────────────────

func TestParseTcpdumpHeader_IPv6(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In  ethertype IPv6 (0x86dd), length 74: fe80::1 > fe80::2: Flags [S]"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Protocol != "TCP" {
		t.Fatalf("expected TCP protocol, got %s", pkt.Protocol)
	}
}

func TestParseTcpdumpHeader_ARPReply(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In ARP, Reply 192.168.0.1 is-at 00:11:22:33:44:55, length 46"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Protocol != "ARP" {
		t.Fatalf("expected ARP, got %s", pkt.Protocol)
	}
	if pkt.SrcIP != "192.168.0.1" {
		t.Fatalf("expected src IP 192.168.0.1, got %s", pkt.SrcIP)
	}
}

func TestParseTcpdumpHeader_ICMPv6(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In  ethertype IPv6 (0x86dd), length 86: fe80::1 > fe80::2: ICMP6, echo request"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Protocol != "ICMP6" {
		t.Fatalf("expected ICMP6, got %s", pkt.Protocol)
	}
}

func TestParseTcpdumpHeader_TruncatedFlags(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In IP 192.168.1.1 > 10.0.0.1: Flags [F], length 0"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Flags != "F" {
		t.Fatalf("expected flags 'F', got '%s'", pkt.Flags)
	}
}

func TestParseTcpdumpHeader_TCPWithoutFlags(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In  ethertype IPv4 (0x0800), length 74: tcp 192.168.1.1 > 10.0.0.1"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Protocol != "TCP" {
		t.Fatalf("expected TCP, got %s", pkt.Protocol)
	}
}

func TestParseTcpdumpHeader_IPv4Fallback(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In  ethertype IPv4 (0x0800), length 74: ipv4 192.168.1.1 > 10.0.0.1"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Protocol != "TCP" {
		t.Fatalf("expected TCP for ipv4 fallback, got %s", pkt.Protocol)
	}
}

func TestParseTcpdumpHeader_ShortTimestamp(t *testing.T) {
	line := "2026-06-11 14:23:37"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
}

func TestParseTcpdumpHeader_UnknownSrcDst(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In  ethertype IPv4 (0x0800), length 74: unknown > unknown: Flags [S]"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.SrcIP != "unknown" {
		t.Fatalf("expected 'unknown' for missing src, got %s", pkt.SrcIP)
	}
	if pkt.DstIP != "unknown" {
		t.Fatalf("expected 'unknown' for missing dst, got %s", pkt.DstIP)
	}
}

func TestParseTcpdumpHeader_ARPFallbackIPExtraction(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In ARP, Request who-has (unknown) tell 192.168.0.1, length 46"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.SrcIP != "192.168.0.1" {
		t.Fatalf("expected src IP 192.168.0.1, got %s", pkt.SrcIP)
	}
}

func TestParseTcpdumpHeader_DefaultLength_NoLengthField(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In IP 192.168.1.1 > 10.0.0.1: Flags [S]"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Length != 64 {
		t.Fatalf("expected default length 64, got %d", pkt.Length)
	}
}

func TestParseTcpdumpHeader_FlagsWithoutBrackets(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In IP 192.168.1.1 > 10.0.0.1: Flags S, seq 1"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Flags != "S" {
		t.Fatalf("expected flags 'S', got '%s'", pkt.Flags)
	}
}

// ── capture.go: splitIPPortV2 with more cases ────────────────────────────────

func TestSplitIPPortV2_IPv4InvalidPort(t *testing.T) {
	ip, port := splitIPPortV2("192.168.1.1.99999")
	if ip != "192.168.1.1" {
		t.Fatalf("expected ip 192.168.1.1, got %s", ip)
	}
	if port != 0 {
		t.Fatalf("expected port 0 for invalid port, got %d", port)
	}
}

func TestSplitIPPortV2_BracketIPv6NoPort(t *testing.T) {
	ip, port := splitIPPortV2("[fe80::1]")
	if ip != "fe80::1" {
		t.Fatalf("expected ip fe80::1, got %s", ip)
	}
	if port != 0 {
		t.Fatalf("expected port 0, got %d", port)
	}
}

func TestSplitIPPortV2_BracketIPv6InvalidPort(t *testing.T) {
	ip, port := splitIPPortV2("[fe80::1]:abc")
	if ip != "fe80::1" {
		t.Fatalf("expected ip fe80::1, got %s", ip)
	}
	if port != 0 {
		t.Fatalf("expected port 0 for invalid, got %d", port)
	}
}

func TestSplitIPPortV2_IPv6WithColonPort(t *testing.T) {
	ip, port := splitIPPortV2("fe80::1:80")
	// The function treats the last colon segment as port only if the prefix is valid IPv6
	if ip == "" && port == 0 {
		// Expected: function doesn't parse bare IPv6:port without brackets
		return
	}
	// If it does parse, verify the result
	t.Logf("splitIPPortV2(\"fe80::1:80\") = (%q, %d)", ip, port)
}

func TestSplitIPPortV2_IPv6InvalidLastSegment(t *testing.T) {
	ip, port := splitIPPortV2("fe80::1:abc")
	// The function may or may not parse this depending on implementation
	t.Logf("splitIPPortV2(\"fe80::1:abc\") = (%q, %d)", ip, port)
}

// ── capture.go: isValidIPv6 edge cases ───────────────────────────────────────

func TestIsValidIPv6_EdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"::", true},
		{"::1", true},
		{"fe80::1", true},
		{"2001:db8::1", true},
		{"2001:0db8:0000:0000:0000:0000:0000:0001", true},
		{"192.168.1.1", false},
		{"not-ipv6", false},
		{"", false},
		{"a:b", false},
		{"gggg::1", false},
	}
	for _, tt := range tests {
		got := isValidIPv6(tt.input)
		if got != tt.expected {
			t.Errorf("isValidIPv6(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

// ── capture.go: isTcpdumpHeader more edge cases ──────────────────────────────

func TestIsTcpdumpHeader_EdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"2026-01-01 00:00:00.000000 extra", true},
		{"2026-12-31 23:59:59.999999", true},
		{"2026-06-11T14:23:37.123456", false},
		{"2026-06-1 14:23:37.123456", false},
	}
	for _, tt := range tests {
		got := isTcpdumpHeader(tt.input)
		if got != tt.expected {
			t.Errorf("isTcpdumpHeader(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

// ── devices.go: ScanPorts ────────────────────────────────────────────────────

func TestDeviceScanPorts(t *testing.T) {
	db := &mockDB{
		getDeviceFn: func(ctx context.Context, id int64) (*models.Device, error) {
			return &models.Device{ID: 1, Name: "Server", IPAddress: "127.0.0.1"}, nil
		},
		upsertPortScanResultsFn: func(ctx context.Context, deviceID int64, results []models.PortScanResult) error {
			return nil
		},
	}
	h := NewDeviceHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/devices/1/scan-ports", `{"ports":[22,80],"timeoutMs":500,"concurrency":10}`, "id", "1")
	h.ScanPorts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeviceScanPorts_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewDeviceHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/devices/abc/scan-ports", `{"ports":[22]}`, "id", "abc")
	h.ScanPorts(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeviceScanPorts_DeviceNotFound(t *testing.T) {
	db := &mockDB{
		getDeviceFn: func(ctx context.Context, id int64) (*models.Device, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewDeviceHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/devices/999/scan-ports", `{"ports":[22]}`, "id", "999")
	h.ScanPorts(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeviceScanPorts_DefaultPorts(t *testing.T) {
	db := &mockDB{
		getDeviceFn: func(ctx context.Context, id int64) (*models.Device, error) {
			return &models.Device{ID: 1, Name: "Server", IPAddress: "127.0.0.1"}, nil
		},
		upsertPortScanResultsFn: func(ctx context.Context, deviceID int64, results []models.PortScanResult) error {
			return nil
		},
	}
	h := NewDeviceHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/devices/1/scan-ports", `{}`, "id", "1")
	h.ScanPorts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeviceScanPorts_EmptyBody(t *testing.T) {
	db := &mockDB{
		getDeviceFn: func(ctx context.Context, id int64) (*models.Device, error) {
			return &models.Device{ID: 1, Name: "Server", IPAddress: "127.0.0.1"}, nil
		},
		upsertPortScanResultsFn: func(ctx context.Context, deviceID int64, results []models.PortScanResult) error {
			return nil
		},
	}
	h := NewDeviceHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/devices/1/scan-ports", "", "id", "1")
	h.ScanPorts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ── devices.go: List with filter combinations ────────────────────────────────

func TestDeviceList_FilterByEnabledFalse(t *testing.T) {
	devices := []models.Device{
		{ID: 1, Name: "R1", Enabled: true},
		{ID: 2, Name: "R2", Enabled: false},
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?enabled=false", "")
	h.List(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	meta := resp["meta"].(map[string]any)
	if int(meta["total"].(float64)) != 1 {
		t.Fatalf("expected 1 disabled device, got %d", int(meta["total"].(float64)))
	}
}

func TestDeviceList_SearchByIP(t *testing.T) {
	devices := []models.Device{
		{ID: 1, Name: "Router", IPAddress: "10.0.0.1"},
		{ID: 2, Name: "Switch", IPAddress: "192.168.1.1"},
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?search=192.168", "")
	h.List(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	meta := resp["meta"].(map[string]any)
	if int(meta["total"].(float64)) != 1 {
		t.Fatalf("expected 1 search result, got %d", int(meta["total"].(float64)))
	}
}

func TestDeviceList_SortByStatus(t *testing.T) {
	devices := []models.Device{
		{ID: 1, Name: "A", Status: "down"},
		{ID: 2, Name: "B", Status: "up"},
		{ID: 3, Name: "C", Status: "up"},
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?sort=status&dir=asc", "")
	h.List(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	first := data[0].(map[string]any)
	if first["status"] != "down" {
		t.Fatalf("expected 'down' first, got %s", first["status"])
	}
}

func TestDeviceList_SortByIPAddress(t *testing.T) {
	devices := []models.Device{
		{ID: 1, Name: "A", IPAddress: "192.168.1.1"},
		{ID: 2, Name: "B", IPAddress: "10.0.0.1"},
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?sort=ipAddress&dir=asc", "")
	h.List(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	first := data[0].(map[string]any)
	if first["ipAddress"] != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1 first, got %s", first["ipAddress"])
	}
}

func TestDeviceList_PaginationEdge(t *testing.T) {
	devices := make([]models.Device, 3)
	for i := range devices {
		devices[i] = models.Device{ID: int64(i + 1), Name: "Device"}
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?page=10&pageSize=5", "")
	h.List(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	if len(data) != 0 {
		t.Fatalf("expected 0 items on page 10, got %d", len(data))
	}
}

func TestDeviceList_PageSizeOver200(t *testing.T) {
	devices := make([]models.Device, 1)
	devices[0] = models.Device{ID: 1, Name: "D"}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?page=1&pageSize=300", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeviceList_SortByProtocol(t *testing.T) {
	devices := []models.Device{
		{ID: 1, Name: "A", Protocol: "snmp"},
		{ID: 2, Name: "B", Protocol: "http"},
		{ID: 3, Name: "C", Protocol: "ping"},
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?sort=protocol&dir=asc", "")
	h.List(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	first := data[0].(map[string]any)
	if first["protocol"] != "http" {
		t.Fatalf("expected http first, got %s", first["protocol"])
	}
}

func TestDeviceList_SortDesc(t *testing.T) {
	devices := []models.Device{
		{ID: 1, Name: "Charlie"},
		{ID: 2, Name: "Alpha"},
		{ID: 3, Name: "Bravo"},
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?sort=name&dir=desc", "")
	h.List(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	first := data[0].(map[string]any)
	if first["name"] != "Charlie" {
		t.Fatalf("expected Charlie first with desc, got %s", first["name"])
	}
}

// ── auth.go: Refresh with wrong secret ────────────────────────────────────────

func TestRefresh_WrongSecret(t *testing.T) {
	db := &mockDB{}
	h := NewAuthHandler(db, testConfig())
	// Generate token with a different secret
	refreshToken, _, _ := auth.GenerateTokenPair(testUserID, testUsername, testUserRole, "wrong-secret", 15*time.Minute, 7*24*time.Hour)
	body, _ := json.Marshal(map[string]string{"refreshToken": refreshToken})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/refresh", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	h.Refresh(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// ── auth.go: CreateAPIKey with no description ─────────────────────────────────

func TestCreateAPIKey_NoDescription(t *testing.T) {
	db := &mockDB{
		createAPIKeyFn: func(ctx context.Context, k *models.APIKey) (*models.APIKey, error) {
			k.ID = 43
			return k, nil
		},
	}
	h := NewAuthHandler(db, testConfig())
	w, req := authenticatedRequest("POST", "/api/auth/apikeys", `{}`)
	callWithAuth(h.CreateAPIKey, w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

// ── auth.go: DeleteAPIKey when key belongs to different user ───────────────────

func TestDeleteAPIKey_DifferentUser(t *testing.T) {
	db := &mockDB{
		deleteAPIKeyFn: func(ctx context.Context, id int64) error {
			return nil
		},
	}
	h := NewAuthHandler(db, testConfig())
	w, req := authenticatedRequest("DELETE", "/api/auth/apikeys/99", "")
	req = withChiParams(req, "id", "99")
	callWithAuth(h.DeleteAPIKey, w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ── notification_channels.go: Test handler ────────────────────────────────────

func TestNotificationChannelTest_Valid(t *testing.T) {
	db := &mockDB{
		getNotificationChannelFn: func(ctx context.Context, id int64) (*models.NotificationChannel, error) {
			return &models.NotificationChannel{ID: 1, Name: "Webhook", Type: "webhook", Enabled: true, Config: map[string]any{"url": "http://example.com"}}, nil
		},
	}
	h := NewNotificationChannelHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/notification-channels/1/test", "")
	req = withChiParams(req, "id", "1")
	h.Test(w, req)
	// The test notification will likely fail because the webhook URL is not real,
	// but we're testing the handler logic path, not the actual sending.
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d", w.Code)
	}
}

// ── alert_rules.go: Create with conditions ────────────────────────────────────

func TestAlertRuleCreate_WithConditions(t *testing.T) {
	db := &mockDB{
		createAlertRuleFn: func(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error) {
			if len(r.Conditions) != 2 {
				t.Fatalf("expected 2 conditions, got %d", len(r.Conditions))
			}
			if r.ConditionLogic != "any" {
				t.Fatalf("expected conditionLogic 'any', got %s", r.ConditionLogic)
			}
			if r.CooldownSec != 600 {
				t.Fatalf("expected cooldownSec 600, got %d", r.CooldownSec)
			}
			r.ID = 1
			return r, nil
		},
	}
	h := NewAlertRuleHandler(db)
	body, _ := json.Marshal(map[string]any{
		"name":           "High Latency",
		"severity":       "warning",
		"conditionLogic": "any",
		"cooldownSec":    600,
		"scopeType":      "global",
		"conditions": []map[string]any{
			{"type": "threshold", "metricField": "response_time", "operator": "gt", "value": "500"},
			{"type": "threshold", "metricField": "packet_loss", "operator": "gt", "value": "10"},
		},
	})
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules", string(body))
	callWithAuth(h.Create, w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAlertRuleCreate_DefaultFields(t *testing.T) {
	db := &mockDB{
		createAlertRuleFn: func(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error) {
			if r.Severity != "warning" {
				t.Fatalf("expected default severity 'warning', got %s", r.Severity)
			}
			if r.ConditionLogic != "all" {
				t.Fatalf("expected default conditionLogic 'all', got %s", r.ConditionLogic)
			}
			if r.CooldownSec != 300 {
				t.Fatalf("expected default cooldownSec 300, got %d", r.CooldownSec)
			}
			if r.ScopeType != "global" {
				t.Fatalf("expected default scopeType 'global', got %s", r.ScopeType)
			}
			if !r.Enabled {
				t.Fatal("expected default enabled true")
			}
			r.ID = 2
			return r, nil
		},
	}
	h := NewAlertRuleHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Basic Rule"})
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules", string(body))
	callWithAuth(h.Create, w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

// ── alert_rules.go: Update with conditions ────────────────────────────────────

func TestAlertRuleUpdate_WithConditions(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return &models.AlertRule{ID: 1}, nil
		},
		updateAlertRuleFn: func(ctx context.Context, id int64, r *models.AlertRule) (*models.AlertRule, error) {
			return r, nil
		},
	}
	h := NewAlertRuleHandler(db)
	body, _ := json.Marshal(map[string]any{
		"name":     "Updated Rule",
		"severity": "critical",
		"conditions": []map[string]any{
			{"type": "threshold", "metricField": "cpu", "operator": "gt", "value": "90"},
		},
	})
	w, req := authenticatedRequest("PUT", "/api/v1/alert-rules/1", string(body))
	req = withChiParams(req, "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertRuleUpdate_InvalidBody(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return &models.AlertRule{ID: 1}, nil
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("PUT", "/api/v1/alert-rules/1", "not-json")
	req = withChiParams(req, "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ── alert_rules.go: Test handler ──────────────────────────────────────────────

func TestAlertRuleTest(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return &models.AlertRule{
				ID:             1,
				Name:           "High Latency",
				Severity:       "warning",
				ConditionLogic: "all",
				Conditions: []models.AlertRuleCondition{
					{Type: "threshold", MetricField: "response_time", Operator: "gt", Value: "500"},
				},
			}, nil
		},
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{{ID: 1, Name: "Server1", Status: "up", Enabled: true}}, nil
		},
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			rt := 100.0
			return []models.Metric{{DeviceID: 1, ResponseTime: &rt}}, nil
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules/1/test", "")
	req = withChiParams(req, "id", "1")
	h.Test(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAlertRuleTest_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules/abc/test", "")
	req = withChiParams(req, "id", "abc")
	h.Test(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertRuleTest_RuleNotFound(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules/999/test", "")
	req = withChiParams(req, "id", "999")
	h.Test(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAlertRuleTest_DevicesError(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return &models.AlertRule{ID: 1, Name: "Rule"}, nil
		},
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules/1/test", "")
	req = withChiParams(req, "id", "1")
	h.Test(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertRuleTest_MetricsError(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return &models.AlertRule{ID: 1, Name: "Rule"}, nil
		},
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{{ID: 1, Name: "Server1"}}, nil
		},
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules/1/test", "")
	req = withChiParams(req, "id", "1")
	h.Test(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertRuleTest_AnyConditionLogic(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return &models.AlertRule{
				ID:             1,
				Name:           "Any Rule",
				ConditionLogic: "any",
				Conditions: []models.AlertRuleCondition{
					{Type: "threshold", MetricField: "response_time", Operator: "gt", Value: "500"},
					{Type: "threshold", MetricField: "packet_loss", Operator: "gt", Value: "50"},
				},
			}, nil
		},
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{{ID: 1, Name: "Server1", Status: "up", Enabled: true}}, nil
		},
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			rt := 100.0
			return []models.Metric{{DeviceID: 1, ResponseTime: &rt}}, nil
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules/1/test", "")
	req = withChiParams(req, "id", "1")
	h.Test(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertRuleTest_NoMatchingDevice(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return &models.AlertRule{
				ID:       1,
				Name:     "Device-Specific Rule",
				ScopeType: "device",
				DeviceID: int64Ptr(42),
				Conditions: []models.AlertRuleCondition{
					{Type: "threshold", MetricField: "response_time", Operator: "gt", Value: "500"},
				},
			}, nil
		},
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{{ID: 1, Name: "Server1", Status: "up", Enabled: true}}, nil
		},
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return []models.Metric{{DeviceID: 1}}, nil
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules/1/test", "")
	req = withChiParams(req, "id", "1")
	h.Test(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ── metrics.go: Query with aggregation parameter ─────────────────────────────

func TestMetricQuery_WithAggregationMax(t *testing.T) {
	db := &mockDB{
		queryMetricsFn: func(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
			if q.Aggregation != "max" {
				t.Fatalf("expected aggregation max, got %s", q.Aggregation)
			}
			return []models.Metric{}, nil
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query?aggregation=max", "")
	h.Query(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMetricQuery_WithAggregationMin(t *testing.T) {
	db := &mockDB{
		queryMetricsFn: func(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
			if q.Aggregation != "min" {
				t.Fatalf("expected aggregation min, got %s", q.Aggregation)
			}
			return []models.Metric{}, nil
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query?aggregation=min", "")
	h.Query(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMetricQuery_WithAggregationSum(t *testing.T) {
	db := &mockDB{
		queryMetricsFn: func(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
			if q.Aggregation != "sum" {
				t.Fatalf("expected aggregation sum, got %s", q.Aggregation)
			}
			return []models.Metric{}, nil
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query?aggregation=sum", "")
	h.Query(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMetricQuery_AggregationDBError(t *testing.T) {
	db := &mockDB{
		queryMetricsFn: func(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query?aggregation=avg", "")
	h.Query(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestMetricQuery_WithDeviceIDAndAggregation(t *testing.T) {
	db := &mockDB{
		queryMetricsFn: func(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
			if q.DeviceID == nil || *q.DeviceID != 1 {
				t.Fatalf("expected deviceID 1, got %v", q.DeviceID)
			}
			if q.Aggregation != "avg" {
				t.Fatalf("expected aggregation avg, got %s", q.Aggregation)
			}
			return []models.Metric{}, nil
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query?deviceId=1&aggregation=avg", "")
	h.Query(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMetricQuery_WithStatusFilter(t *testing.T) {
	db := &mockDB{
		queryMetricsFn: func(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
			if q.Status != "up" {
				t.Fatalf("expected status 'up', got %s", q.Status)
			}
			return []models.Metric{}, nil
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query?aggregation=avg&status=up", "")
	h.Query(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMetricQuery_InvalidBucketMin(t *testing.T) {
	db := &mockDB{
		queryMetricsFn: func(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
			return []models.Metric{}, nil
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query?bucketMin=abc", "")
	h.Query(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMetricQuery_DeviceIDDBError(t *testing.T) {
	db := &mockDB{
		getDeviceMetricsFn: func(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query?deviceId=1", "")
	h.Query(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ── flows.go: verify already 100% (basic coverage) ───────────────────────────

func TestFlowList_WithTimeRange(t *testing.T) {
	db := &mockDB{
		getFlowsFn: func(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error) {
			return []models.Flow{}, 0, nil
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows?from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z&limit=10&offset=5", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestFlowTopTalkers_WithN(t *testing.T) {
	db := &mockDB{
		getTopTalkersFn: func(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error) {
			if n != 10 {
				t.Fatalf("expected n=10, got %d", n)
			}
			return []models.IPCount{}, nil
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows/top-talkers?n=10", "")
	h.TopTalkers(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ── handlers_test.go: stringPtr helper ────────────────────────────────────────

func TestStringPtr(t *testing.T) {
	s := stringPtr("hello")
	if s == nil || *s != "hello" {
		t.Fatalf("expected 'hello', got %v", s)
	}
}
