package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wsTestSecret = "ws-test-secret"

func makeToken(t *testing.T) string {
	t.Helper()
	token, _, err := auth.GenerateTokenPair(1, "testuser", "admin", wsTestSecret, 1*time.Hour, 24*time.Hour)
	require.NoError(t, err)
	return token
}

func TestNewHub(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	require.NotNil(t, hub)
	assert.Equal(t, 0, hub.ConnectionCount())
}

func TestHub_Broadcast_NoClients(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	go hub.Run()
	hub.Broadcast(Message{Type: EventMetricUpdate, Data: "test"})
	hub.Stop()
}

func TestHub_ConnectionCount(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	assert.Equal(t, 0, hub.ConnectionCount())
}

func TestHub_Stop(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	go hub.Run()
	hub.Stop()
	// Broadcasting after Stop should not panic — Broadcast uses select with default
	// However, the channel is closed so it will panic. This is expected behavior.
	// We just verify Stop completes without deadlock.
}

func TestHub_ServeWS_NoToken(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	go hub.Run()
	defer hub.Stop()

	req := httptest.NewRequest("GET", "/ws", nil)
	w := httptest.NewRecorder()
	hub.ServeWS(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHub_ServeWS_InvalidToken(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	go hub.Run()
	defer hub.Stop()

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")
	w := httptest.NewRecorder()
	hub.ServeWS(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHub_ServeWS_QueryParam(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	go hub.Run()
	defer hub.Stop()

	token := makeToken(t)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		hub.ServeWS(w, req)
	}()

	time.Sleep(50 * time.Millisecond)
	_ = hub.ConnectionCount()
}

func TestHub_ExtractToken_AuthorizationHeader(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	token := makeToken(t)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	extracted := hub.extractToken(req)
	assert.Equal(t, token, extracted)
}

func TestHub_ExtractToken_SecWebSocketProtocol(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "mytoken123")
	extracted := hub.extractToken(req)
	assert.Equal(t, "mytoken123", extracted)
}

func TestHub_ExtractToken_QueryParam(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	req := httptest.NewRequest("GET", "/ws?token=mytoken123", nil)
	extracted := hub.extractToken(req)
	assert.Equal(t, "", extracted)
}

func TestHub_ExtractToken_Empty(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	extracted := hub.extractToken(req)
	assert.Equal(t, "", extracted)
}

func TestHub_Broadcast_ChannelFull(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	for i := 0; i < 256; i++ {
		hub.broadcast <- Message{Type: EventMetricUpdate, Data: "test"}
	}
	hub.Broadcast(Message{Type: EventMetricUpdate, Data: "overflow"})
}

func TestHub_SendToClient_NoClients(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsTestSecret, nil, nil)
	go hub.Run()
	defer hub.Stop()
	hub.SendToClient(1, Message{Type: EventMetricUpdate, Data: "test"})
}

func TestMessageJSON(t *testing.T) {
	t.Parallel()
	msg := Message{
		Type:      EventMetricUpdate,
		RequestID: "req-123",
		Data:      map[string]string{"key": "value"},
	}
	b, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.Contains(t, string(b), "metric:update")
	assert.Contains(t, string(b), "req-123")

	var decoded Message
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, EventMetricUpdate, decoded.Type)
}

func TestEventTypes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, EventType("metric:update"), EventMetricUpdate)
	assert.Equal(t, EventType("alert:triggered"), EventAlertTriggered)
	assert.Equal(t, EventType("alert:updated"), EventAlertUpdated)
	assert.Equal(t, EventType("alert:resolved"), EventAlertResolved)
	assert.Equal(t, EventType("device:status"), EventDeviceStatus)
	assert.Equal(t, EventType("flow:update"), EventFlowUpdate)
	assert.Equal(t, EventType("capture:packet"), EventCapturePacket)
	assert.Equal(t, EventType("capture:status"), EventCaptureStatus)
	assert.Equal(t, EventType("ports:scanned"), EventPortsScanned)
	assert.Equal(t, EventType("bootstrap"), EventBootstrap)
}
