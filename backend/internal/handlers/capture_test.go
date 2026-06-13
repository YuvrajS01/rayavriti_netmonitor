package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

func newTestCaptureHandler(db *mockDB) *CaptureHandler {
	hub := websocket.NewHub("test-secret", nil, nil)
	go hub.Run()
	return NewCaptureHandler(db, hub)
}

func TestCaptureStats_NotRunning(t *testing.T) {
	db := &mockDB{}
	h := newTestCaptureHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/capture/stats", "")
	h.Stats(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["running"] != false {
		t.Fatalf("expected running=false, got %v", data["running"])
	}
}

func TestCaptureInterfaces(t *testing.T) {
	db := &mockDB{}
	h := newTestCaptureHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/capture/interfaces", "")
	h.Interfaces(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCaptureGetSession_Valid(t *testing.T) {
	db := &mockDB{
		getCaptureSessionFn: func(ctx context.Context, id int64) (*models.CaptureSession, error) {
			if id == 1 {
				return &models.CaptureSession{ID: 1, InterfaceName: "eth0", Status: "running"}, nil
			}
			return nil, errors.New("not found")
		},
	}
	h := newTestCaptureHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/capture/sessions/1", "", "id", "1")
	h.GetSession(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCaptureGetSession_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := newTestCaptureHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/capture/sessions/abc", "", "id", "abc")
	h.GetSession(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCaptureGetSession_NotFound(t *testing.T) {
	db := &mockDB{
		getCaptureSessionFn: func(ctx context.Context, id int64) (*models.CaptureSession, error) {
			return nil, errors.New("not found")
		},
	}
	h := newTestCaptureHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/capture/sessions/999", "", "id", "999")
	h.GetSession(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCaptureGetPackets(t *testing.T) {
	db := &mockDB{
		getCapturePacketsFn: func(ctx context.Context, sessionID int64, limit, offset int) ([]models.CapturePacket, error) {
			return []models.CapturePacket{
				{ID: 1, SrcIP: "10.0.0.1", DstIP: "10.0.0.2", Protocol: "TCP"},
			}, nil
		},
	}
	h := newTestCaptureHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/capture/sessions/1/packets", "", "id", "1")
	h.GetPackets(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCaptureGetPackets_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := newTestCaptureHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/capture/sessions/abc/packets", "", "id", "abc")
	h.GetPackets(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCaptureGetPackets_DBError(t *testing.T) {
	db := &mockDB{
		getCapturePacketsFn: func(ctx context.Context, sessionID int64, limit, offset int) ([]models.CapturePacket, error) {
			return nil, errors.New("db error")
		},
	}
	h := newTestCaptureHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/capture/sessions/1/packets", "", "id", "1")
	h.GetPackets(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCaptureListSessions(t *testing.T) {
	db := &mockDB{
		getCaptureSessionsFn: func(ctx context.Context) ([]models.CaptureSession, error) {
			return []models.CaptureSession{
				{ID: 1, InterfaceName: "eth0", Status: "running"},
				{ID: 2, InterfaceName: "eth1", Status: "stopped"},
			}, nil
		},
	}
	h := newTestCaptureHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/capture/sessions", "")
	h.ListSessions(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCaptureListSessions_DBError(t *testing.T) {
	db := &mockDB{
		getCaptureSessionsFn: func(ctx context.Context) ([]models.CaptureSession, error) {
			return nil, errors.New("db error")
		},
	}
	h := newTestCaptureHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/capture/sessions", "")
	h.ListSessions(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- parseFlags ---

func TestParseFlags(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"UP,BROADCAST,RUNNING,MULTICAST", 4},
		{"", 0},
		{"  UP  , BROADCAST  ", 2},
		{"LOOPBACK", 1},
	}
	for _, tt := range tests {
		flags := parseFlags(tt.input)
		if len(flags) != tt.expected {
			t.Errorf("parseFlags(%q) returned %d flags, expected %d", tt.input, len(flags), tt.expected)
		}
	}
}

// --- isTcpdumpHeader ---

func TestIsTcpdumpHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"2026-06-11 14:23:37.123456 eth0 In IP 192.168.1.1 > 10.0.0.1: Flags [S]", true},
		{"2026-01-01 00:00:00.000000", true},
		{"tcpdump: listening on eth0", false},
		{"0x0000: 4500 003c", false},
		{"short", false},
		{"", false},
		{"not a timestamp at all!!!!", false},
	}
	for _, tt := range tests {
		got := isTcpdumpHeader(tt.input)
		if got != tt.expected {
			t.Errorf("isTcpdumpHeader(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

// --- parseTcpdumpHeader ---

func TestParseTcpdumpHeader(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In  ethertype IPv4 (0x0800), length 74: 192.168.1.1.443 > 10.0.0.1.8080: Flags [S], seq 12345, win 65535"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.SrcIP != "192.168.1.1" {
		t.Fatalf("expected src IP 192.168.1.1, got %s", pkt.SrcIP)
	}
	if pkt.DstIP != "10.0.0.1" {
		t.Fatalf("expected dst IP 10.0.0.1, got %s", pkt.DstIP)
	}
	if pkt.SrcPort != 443 {
		t.Fatalf("expected src port 443, got %d", pkt.SrcPort)
	}
	if pkt.DstPort != 8080 {
		t.Fatalf("expected dst port 8080, got %d", pkt.DstPort)
	}
}

func TestParseTcpdumpHeader_Empty(t *testing.T) {
	if pkt := parseTcpdumpHeader(""); pkt != nil {
		t.Fatal("expected nil for empty string")
	}
}

func TestParseTcpdumpHeader_TcpdumpPrefix(t *testing.T) {
	if pkt := parseTcpdumpHeader("tcpdump: listening on eth0"); pkt != nil {
		t.Fatal("expected nil for tcpdump prefix")
	}
}

func TestParseTcpdumpHeader_Listening(t *testing.T) {
	if pkt := parseTcpdumpHeader("listening on eth0"); pkt != nil {
		t.Fatal("expected nil for listening prefix")
	}
}

func TestParseTcpdumpHeader_UDP(t *testing.T) {
	pkt := parseTcpdumpHeader("2026-06-11 14:23:37.123456 eth0 In  ethertype IPv4 (0x0800), length 86: 192.168.1.1.53 > 10.0.0.1.12345: UDP")
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Protocol != "UDP" {
		t.Fatalf("expected UDP, got %s", pkt.Protocol)
	}
	if pkt.SrcIP != "192.168.1.1" {
		t.Fatalf("expected src IP 192.168.1.1, got %s", pkt.SrcIP)
	}
}

func TestParseTcpdumpHeader_ARP(t *testing.T) {
	pkt := parseTcpdumpHeader("2026-06-11 14:23:37.123456 eth0 In ARP, Request who-has 192.168.0.115 tell 192.168.0.1, length 46")
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Protocol != "ARP" {
		t.Fatalf("expected ARP, got %s", pkt.Protocol)
	}
}

func TestParseTcpdumpHeader_ICMP(t *testing.T) {
	pkt := parseTcpdumpHeader("2026-06-11 14:23:37.123456 eth0 In  ethertype IPv4 (0x0800), length 98: 192.168.1.1 > 10.0.0.1: ICMP")
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Protocol != "ICMP" {
		t.Fatalf("expected ICMP, got %s", pkt.Protocol)
	}
	if pkt.SrcIP != "192.168.1.1" {
		t.Fatalf("expected src IP 192.168.1.1, got %s", pkt.SrcIP)
	}
}

func TestParseTcpdumpHeader_LengthParsing(t *testing.T) {
	pkt := parseTcpdumpHeader("2026-06-11 14:23:37.123456 eth0 In IP 192.168.1.1 > 10.0.0.1: Flags [.], length 1500,")
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Length != 1500 {
		t.Fatalf("expected length 1500, got %d", pkt.Length)
	}
}

func TestParseTcpdumpHeader_DefaultLength(t *testing.T) {
	pkt := parseTcpdumpHeader("2026-06-11 14:23:37.123456 eth0 In  ethertype IPv4 (0x0800), length 64: 192.168.1.1 > 10.0.0.1: Flags [S]")
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Length != 64 {
		t.Fatalf("expected default length 64, got %d", pkt.Length)
	}
}

func TestParseTcpdumpHeader_FlagsParsing(t *testing.T) {
	pkt := parseTcpdumpHeader("2026-06-11 14:23:37.123456 eth0 In  ethertype IPv4 (0x0800), length 74: 192.168.1.1 > 10.0.0.1: Flags [S.], seq 1, ack 2, win 65535")
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Flags != "S." {
		t.Fatalf("expected flags 'S.', got '%s'", pkt.Flags)
	}
}

// --- splitIPPortV2 ---

func TestSplitIPPortV2(t *testing.T) {
	tests := []struct {
		input    string
		wantIP   string
		wantPort int
	}{
		{"192.168.1.1.443", "192.168.1.1", 443},
		{"192.168.1.1", "192.168.1.1", 0},
		{"[fe80::1]:80", "fe80::1", 80},
		{"fe80::1", "fe80::1", 0},
		{"10.0.0.1.8080", "10.0.0.1", 8080},
		{"", "", 0},
	}
	for _, tt := range tests {
		ip, port := splitIPPortV2(tt.input)
		if ip != tt.wantIP || port != tt.wantPort {
			t.Errorf("splitIPPortV2(%q) = (%q, %d), want (%q, %d)", tt.input, ip, port, tt.wantIP, tt.wantPort)
		}
	}
}

// --- isValidIPv4 ---

func TestIsValidIPv4(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"255.255.255.255", true},
		{"256.1.1.1", false},
		{"1.1.1.256", false},
		{"abc.def.ghi.jkl", false},
		{"1.2.3", false},
		{"1.2.3.4.5", false},
	}
	for _, tt := range tests {
		got := isValidIPv4(tt.input)
		if got != tt.expected {
			t.Errorf("isValidIPv4(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

// --- isValidIPv6 ---

func TestIsValidIPv6(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"fe80::1", true},
		{"::1", true},
		{"2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"192.168.1.1", false}, // has dots
		{"abc", false},         // not enough colons
		{"", false},
	}
	for _, tt := range tests {
		got := isValidIPv6(tt.input)
		if got != tt.expected {
			t.Errorf("isValidIPv6(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestCaptureStop(t *testing.T) {
	db := &mockDB{
		stopCaptureSessionFn: func(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
			return nil
		},
	}
	h := newTestCaptureHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/capture/sessions/1/stop", "", "id", "1")
	h.Stop(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCaptureStop_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := newTestCaptureHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/capture/sessions/abc/stop", "", "id", "abc")
	h.Stop(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCaptureStart_InvalidBody(t *testing.T) {
	db := &mockDB{}
	h := newTestCaptureHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/capture/start", "not-json")
	h.Start(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCaptureStart_EmptyInterface(t *testing.T) {
	db := &mockDB{}
	h := newTestCaptureHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/capture/start", `{"interface":""}`)
	h.Start(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCaptureStart_AlreadyRunning(t *testing.T) {
	db := &mockDB{
		createCaptureSessionFn: func(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error) {
			cs.ID = 1
			return cs, nil
		},
	}
	h := newTestCaptureHandler(db)

	// Start once to set running=1
	h.running = 1
	w, req := authenticatedRequest("POST", "/api/v1/capture/start", `{"interface":"lo"}`)
	h.Start(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

// --- test timestamp ---
func TestParseTcpdumpHeader_TimestampParsing(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In  ethertype IPv4 (0x0800), length 74: 192.168.1.1 > 10.0.0.1: Flags [S]"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	expected := time.Date(2026, 6, 11, 14, 23, 37, 123456000, time.UTC)
	if !pkt.Timestamp.Equal(expected) {
		t.Fatalf("expected timestamp %v, got %v", expected, pkt.Timestamp)
	}
}

// --- test len= format ---
func TestParseTcpdumpHeader_LenFormat(t *testing.T) {
	line := "2026-06-11 14:23:37.123456 eth0 In IP 192.168.1.1 > 10.0.0.1: Flags [.], ack 12345, win 65535, len=100,"
	pkt := parseTcpdumpHeader(line)
	if pkt == nil {
		t.Fatal("expected non-nil packet")
	}
	if pkt.Length != 100 {
		t.Fatalf("expected length 100, got %d", pkt.Length)
	}
}
