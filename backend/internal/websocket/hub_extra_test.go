package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wsExtraSecret = "extra-test-secret" //nolint:gosec // Test fixture, not a real credential

func wsExtraToken(t *testing.T) string {
	t.Helper()
	token, _, err := auth.GenerateTokenPair(1, "testuser", "admin", wsExtraSecret, 1*time.Hour, 24*time.Hour)
	require.NoError(t, err)
	return token
}

func wsExtraTokenForUser(t *testing.T, userID int64, username string) string {
	t.Helper()
	token, _, err := auth.GenerateTokenPair(userID, username, "admin", wsExtraSecret, 1*time.Hour, 24*time.Hour)
	require.NoError(t, err)
	return token
}

func wsExtraConnect(t *testing.T, url string, headers http.Header) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(url, headers)
	require.NoError(t, err)
	return conn
}

func wsExtraConnectWithToken(t *testing.T, url string, token string) *websocket.Conn {
	t.Helper()
	headers := http.Header{"Authorization": []string{"Bearer " + token}}
	return wsExtraConnect(t, url, headers)
}

func wsExtraHubWithServer(t *testing.T) (*Hub, string) {
	t.Helper()
	hub := NewHub(wsExtraSecret, nil, nil)
	go hub.Run()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r)
	}))
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	t.Cleanup(func() {
		srv.Close()
		hub.Stop()
	})
	return hub, wsURL
}

// ── ServeWS with valid JWT token ─────────────────────────────────────────────

func TestHub_Extra_ServeWSValidToken(t *testing.T) {
	t.Parallel()
	hub, wsURL := wsExtraHubWithServer(t)
	token := wsExtraToken(t)

	conn := wsExtraConnectWithToken(t, wsURL, token)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 1, hub.ConnectionCount())
	conn.Close()
}

// ── ServeWS with Sec-WebSocket-Protocol header for auth ──────────────────────

func TestHub_Extra_ServeWSProtocolHeader(t *testing.T) {
	t.Parallel()
	hub, wsURL := wsExtraHubWithServer(t)
	token := wsExtraToken(t)

	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", token)
	conn := wsExtraConnect(t, wsURL, headers)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 1, hub.ConnectionCount())
	conn.Close()
}

// ── Run loop: broadcast to removed client ────────────────────────────────────

func TestHub_Extra_BroadcastToRemovedClient(t *testing.T) {
	t.Parallel()
	hub, wsURL := wsExtraHubWithServer(t)
	token := wsExtraToken(t)

	conn := wsExtraConnectWithToken(t, wsURL, token)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 1, hub.ConnectionCount())

	conn.Close()
	time.Sleep(100 * time.Millisecond)

	hub.Broadcast(Message{Type: EventMetricUpdate, Data: "test"})
	time.Sleep(50 * time.Millisecond)
}

// ── Run loop: broadcast to dead client (send channel full) ───────────────────

func TestHub_Extra_BroadcastToDeadClient(t *testing.T) {
	t.Parallel()
	hub, wsURL := wsExtraHubWithServer(t)
	token := wsExtraToken(t)

	conn := wsExtraConnectWithToken(t, wsURL, token)
	time.Sleep(20 * time.Millisecond)

	hub.mu.RLock()
	for c := range hub.clients {
		c.mu.Lock()
		c.dead = true
		c.mu.Unlock()
		break
	}
	hub.mu.RUnlock()

	hub.Broadcast(Message{Type: EventMetricUpdate, Data: "test"})
	time.Sleep(50 * time.Millisecond)

	conn.Close()
}

// ── SendToClient with concurrent sends ───────────────────────────────────────

func TestHub_Extra_SendToClientConcurrent(t *testing.T) {
	t.Parallel()
	hub, wsURL := wsExtraHubWithServer(t)
	token := wsExtraToken(t)

	conn := wsExtraConnectWithToken(t, wsURL, token)
	time.Sleep(20 * time.Millisecond)

	for i := 0; i < 10; i++ {
		go func() {
			hub.SendToClient(1, Message{Type: EventMetricUpdate, Data: "concurrent"})
		}()
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	require.NoError(t, err)

	conn.Close()
}

// ── Multiple clients connecting and disconnecting ────────────────────────────

func TestHub_Extra_MultipleClientsConnectDisconnect(t *testing.T) {
	t.Parallel()
	hub, wsURL := wsExtraHubWithServer(t)
	token1 := wsExtraTokenForUser(t, 1, "user1")
	token2 := wsExtraTokenForUser(t, 2, "user2")
	token3 := wsExtraTokenForUser(t, 3, "user3")

	conn1 := wsExtraConnectWithToken(t, wsURL, token1)
	conn2 := wsExtraConnectWithToken(t, wsURL, token2)
	conn3 := wsExtraConnectWithToken(t, wsURL, token3)
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, 3, hub.ConnectionCount())

	hub.Broadcast(Message{Type: EventDeviceStatus, Data: "test"})

	for _, conn := range []*websocket.Conn{conn1, conn2, conn3} {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, err := conn.ReadMessage()
		require.NoError(t, err)
	}

	conn2.Close()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 2, hub.ConnectionCount())

	hub.Broadcast(Message{Type: EventMetricUpdate, Data: "remaining"})
	for _, conn := range []*websocket.Conn{conn1, conn3} {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, err := conn.ReadMessage()
		require.NoError(t, err)
	}

	conn1.Close()
	conn3.Close()
}

// ── extractToken with empty Authorization header ─────────────────────────────

func TestHub_Extra_ExtractTokenEmptyAuthHeader(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsExtraSecret, nil, nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Authorization", "")
	token := hub.extractToken(req)
	assert.Equal(t, "", token)
}

// ── extractToken: only query param ───────────────────────────────────────────

func TestHub_Extra_ExtractTokenOnlyQueryParam(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsExtraSecret, nil, nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	token := hub.extractToken(req)
	assert.Equal(t, "", token)
}

// ── extractToken: Bearer without space ───────────────────────────────────────

func TestHub_Extra_ExtractTokenBearerNoSpace(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsExtraSecret, nil, nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Authorization", "BearerToken123")
	token := hub.extractToken(req)
	assert.Equal(t, "", token)
}

// ── ServeWS with invalid token on Authorization header ───────────────────────

func TestHub_Extra_ServeWSInvalidBearerToken(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsExtraSecret, nil, nil)
	go hub.Run()
	defer hub.Stop()

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")
	w := httptest.NewRecorder()
	hub.ServeWS(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ── ServeWS with no token at all ────────────────────────────────────────────

func TestHub_Extra_ServeWSNoTokenAtAll(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsExtraSecret, nil, nil)
	go hub.Run()
	defer hub.Stop()

	req := httptest.NewRequest("GET", "/ws", nil)
	w := httptest.NewRecorder()
	hub.ServeWS(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "missing authentication token")
}

// ── Hub with bootstrap function ──────────────────────────────────────────────

func TestHub_Extra_WithBootstrap(t *testing.T) {
	t.Parallel()
	bootstrapCalled := false
	hub := NewHub(wsExtraSecret, func(ctx context.Context, userID int64, username, role string) (map[string]any, error) {
		bootstrapCalled = true
		return map[string]any{"devices": []string{"server1", "server2"}}, nil
	}, nil)
	go hub.Run()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	token := wsExtraToken(t)

	conn := wsExtraConnectWithToken(t, wsURL, token)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(msg), "bootstrap")
	assert.True(t, bootstrapCalled)

	conn.Close()
	hub.Stop()
}

// ── Hub.Broadcast: slow client (send buffer full) ────────────────────────────

func TestHub_Extra_BroadcastSlowClient(t *testing.T) {
	t.Parallel()
	hub, wsURL := wsExtraHubWithServer(t)
	token := wsExtraToken(t)

	conn := wsExtraConnectWithToken(t, wsURL, token)
	time.Sleep(20 * time.Millisecond)

	for i := 0; i < 70; i++ {
		hub.Broadcast(Message{Type: EventMetricUpdate, Data: i})
	}

	time.Sleep(100 * time.Millisecond)
	conn.Close()
}

// ── Hub.SendToClient: dead client in map ─────────────────────────────────────

func TestHub_Extra_SendToClientDeadClientInMap(t *testing.T) {
	t.Parallel()
	hub, wsURL := wsExtraHubWithServer(t)
	token := wsExtraToken(t)

	conn := wsExtraConnectWithToken(t, wsURL, token)
	time.Sleep(20 * time.Millisecond)

	conn.Close()
	time.Sleep(100 * time.Millisecond)

	hub.SendToClient(1, Message{Type: EventMetricUpdate, Data: "gone"})
	time.Sleep(50 * time.Millisecond)
}

// ── Hub.Run: broadcast count with active clients ─────────────────────────────

func TestHub_Extra_BroadcastCount(t *testing.T) {
	t.Parallel()
	hub, wsURL := wsExtraHubWithServer(t)
	token := wsExtraToken(t)

	conn := wsExtraConnectWithToken(t, wsURL, token)
	time.Sleep(20 * time.Millisecond)

	hub.Broadcast(Message{Type: EventAlertTriggered, Data: "alert"})

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	require.NoError(t, err)

	conn.Close()
}
