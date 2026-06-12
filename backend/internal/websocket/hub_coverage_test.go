package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wsCoverageSecret = "coverage-test-secret"

func wsCoverageToken(t *testing.T) string {
	t.Helper()
	token, _, err := auth.GenerateTokenPair(1, "testuser", "admin", wsCoverageSecret, 1*time.Hour, 24*time.Hour)
	require.NoError(t, err)
	return token
}

func wsCoverageConnect(t *testing.T, url string, headers http.Header) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(url, headers)
	require.NoError(t, err)
	return conn
}

func wsCoverageHubWithServer(t *testing.T) (*Hub, *httptest.Server, string) {
	t.Helper()
	hub := NewHub(wsCoverageSecret, nil, nil)
	go hub.Run()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r)
	}))
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	t.Cleanup(func() {
		srv.Close()
		hub.Stop()
	})
	return hub, srv, wsURL
}

// ── Hub.Run: broadcast to multiple clients ────────────────────────────────────

func TestHub_Coverage_BroadcastMultipleClients(t *testing.T) {
	t.Parallel()
	hub, _, wsURL := wsCoverageHubWithServer(t)
	token := wsCoverageToken(t)

	conn1 := wsCoverageConnect(t, wsURL+"?token="+token, nil)
	conn2 := wsCoverageConnect(t, wsURL+"?token="+token, nil)
	time.Sleep(20 * time.Millisecond)

	msg := Message{Type: EventMetricUpdate, Data: map[string]any{"value": 42}}
	hub.Broadcast(msg)

	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn1.ReadMessage()
	require.NoError(t, err)

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn2.ReadMessage()
	require.NoError(t, err)

	conn1.Close()
	conn2.Close()
}

// ── Hub.Broadcast: mixed dead/alive clients ──────────────────────────────────

func TestHub_Coverage_MixedDeadAlive(t *testing.T) {
	t.Parallel()
	hub, _, wsURL := wsCoverageHubWithServer(t)
	token := wsCoverageToken(t)

	conn1 := wsCoverageConnect(t, wsURL+"?token="+token, nil)
	conn2 := wsCoverageConnect(t, wsURL+"?token="+token, nil)
	time.Sleep(20 * time.Millisecond)

	assert.Equal(t, 2, hub.ConnectionCount())

	conn2.Close()
	time.Sleep(20 * time.Millisecond)

	msg := Message{Type: EventDeviceStatus, Data: "test"}
	hub.Broadcast(msg)

	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn1.ReadMessage()
	require.NoError(t, err)

	conn1.Close()
}

// ── Hub.SendToClient: matching user ID ───────────────────────────────────────

func TestHub_Coverage_SendToClientMatchingUser(t *testing.T) {
	t.Parallel()
	hub, _, wsURL := wsCoverageHubWithServer(t)
	token := wsCoverageToken(t)

	conn := wsCoverageConnect(t, wsURL+"?token="+token, nil)
	time.Sleep(20 * time.Millisecond)

	hub.SendToClient(1, Message{Type: EventAlertTriggered, Data: "test"})

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	require.NoError(t, err)

	conn.Close()
}

// ── Hub.SendToClient: no matching user ID ────────────────────────────────────

func TestHub_Coverage_SendToClientNoMatch(t *testing.T) {
	t.Parallel()
	hub, _, wsURL := wsCoverageHubWithServer(t)
	token := wsCoverageToken(t)

	conn := wsCoverageConnect(t, wsURL+"?token="+token, nil)
	time.Sleep(20 * time.Millisecond)

	hub.SendToClient(99999, Message{Type: EventAlertTriggered, Data: "test"})
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	assert.Error(t, err)

	conn.Close()
}

// ── Hub.Stop: closes all connections ──────────────────────────────────────────

func TestHub_Coverage_StopClosesConnections(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsCoverageSecret, nil, nil)
	go hub.Run()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	token := wsCoverageToken(t)

	conn := wsCoverageConnect(t, wsURL+"?token="+token, nil)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 1, hub.ConnectionCount())

	hub.Stop()
	time.Sleep(50 * time.Millisecond)

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	assert.Error(t, err)
	conn.Close()
}

// ── Hub.extractToken: additional edge cases ──────────────────────────────────

func TestHub_Coverage_ExtractTokenPriorityOrder(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsCoverageSecret, nil, nil)
	req := httptest.NewRequest("GET", "/ws?token=qs", nil)
	req.Header.Set("Authorization", "Bearer hdr")
	req.Header.Set("Sec-WebSocket-Protocol", "proto")
	token := hub.extractToken(req)
	assert.Equal(t, "hdr", token)
}

func TestHub_Coverage_ExtractTokenMultipleProtocols(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsCoverageSecret, nil, nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "first-protocol, second-protocol")
	token := hub.extractToken(req)
	assert.Equal(t, "first-protocol", token)
}

func TestHub_Coverage_ExtractTokenAuthNoBearer(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsCoverageSecret, nil, nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Authorization", "Basic something")
	token := hub.extractToken(req)
	assert.Equal(t, "", token)
}

func TestHub_Coverage_ExtractTokenEmptyProtocol(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsCoverageSecret, nil, nil)
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "")
	token := hub.extractToken(req)
	assert.Equal(t, "", token)
}

func TestHub_Coverage_ExtractTokenProtocolFallsToQueryParam(t *testing.T) {
	t.Parallel()
	hub := NewHub(wsCoverageSecret, nil, nil)
	req := httptest.NewRequest("GET", "/ws?token=qs-token", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "")
	token := hub.extractToken(req)
	assert.Equal(t, "qs-token", token)
}

// ── Hub.ConnectionCount ───────────────────────────────────────────────────────

func TestHub_Coverage_ConnectionCountWithData(t *testing.T) {
	t.Parallel()
	hub, _, wsURL := wsCoverageHubWithServer(t)
	token := wsCoverageToken(t)

	conn := wsCoverageConnect(t, wsURL+"?token="+token, nil)
	time.Sleep(20 * time.Millisecond)

	assert.Equal(t, 1, hub.ConnectionCount())
	conn.Close()
}

// ── Hub.Run: unmarshalable message ────────────────────────────────────────────

func TestHub_Coverage_RunUnmarshalableMessage(t *testing.T) {
	t.Parallel()
	hub, _, wsURL := wsCoverageHubWithServer(t)
	token := wsCoverageToken(t)

	conn := wsCoverageConnect(t, wsURL+"?token="+token, nil)
	time.Sleep(20 * time.Millisecond)

	type badMsg struct{ Ch chan int }
	hub.broadcast <- Message{Type: EventMetricUpdate, Data: badMsg{Ch: make(chan int)}}

	time.Sleep(50 * time.Millisecond)
	conn.Close()
}

// ── Hub: concurrent operations ────────────────────────────────────────────────

func TestHub_Coverage_ConcurrentOperations(t *testing.T) {
	t.Parallel()
	hub, _, wsURL := wsCoverageHubWithServer(t)
	token := wsCoverageToken(t)

	conns := make([]*websocket.Conn, 5)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conns[idx] = wsCoverageConnect(t, wsURL+"?token="+token, nil)
		}(i)
	}
	wg.Wait()
	time.Sleep(20 * time.Millisecond)

	hub.Broadcast(Message{Type: EventMetricUpdate, Data: "test"})
	hub.SendToClient(1, Message{Type: EventAlertTriggered, Data: "test"})

	for _, c := range conns {
		if c != nil {
			c.Close()
		}
	}
}

// ── Hub.Broadcast: dead client skipped ────────────────────────────────────────

func TestHub_Coverage_BroadcastSkipsDeadClient(t *testing.T) {
	t.Parallel()
	hub, _, wsURL := wsCoverageHubWithServer(t)
	token := wsCoverageToken(t)

	conn := wsCoverageConnect(t, wsURL+"?token="+token, nil)
	time.Sleep(20 * time.Millisecond)

	conn.Close()
	time.Sleep(20 * time.Millisecond)

	hub.Broadcast(Message{Type: EventMetricUpdate, Data: "test"})
	time.Sleep(50 * time.Millisecond)
}

// ── Hub.SendToClient: dead client skipped ────────────────────────────────────

func TestHub_Coverage_SendToClientSkipsDeadClient(t *testing.T) {
	t.Parallel()
	hub, _, wsURL := wsCoverageHubWithServer(t)
	token := wsCoverageToken(t)

	conn := wsCoverageConnect(t, wsURL+"?token="+token, nil)
	time.Sleep(20 * time.Millisecond)

	conn.Close()
	time.Sleep(20 * time.Millisecond)

	hub.SendToClient(1, Message{Type: EventMetricUpdate, Data: "test"})
}
