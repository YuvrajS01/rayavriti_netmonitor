package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
)

func TestServeWS_StaleSubprotocolFallsBackToCookie(t *testing.T) {
	const secret = "test-secret-that-is-at-least-32-characters"
	h := NewHub(secret, nil, nil)
	go h.Run()
	defer h.Stop()
	srv := httptest.NewServer(http.HandlerFunc(h.ServeWS))
	defer srv.Close()

	valid, _, err := auth.GenerateTokenPair(1, "alice", "admin", secret, time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	expired, _, _ := auth.GenerateTokenPair(1, "alice", "admin", secret, -time.Minute, time.Hour)

	d := gws.Dialer{Subprotocols: []string{expired}}
	header := http.Header{"Cookie": []string{auth.AccessCookieName + "=" + valid}}
	conn, resp, err := d.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), header)
	if err != nil {
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		t.Fatalf("expected the valid cookie to authenticate despite a stale subprotocol token (status %d): %v", code, err)
	}
	_ = conn.Close()
}
