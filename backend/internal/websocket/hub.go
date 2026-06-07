package websocket

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
)

type EventType string

const (
	EventMetricUpdate   EventType = "metric:update"
	EventAlertTriggered EventType = "alert:triggered"
	EventAlertUpdated   EventType = "alert:updated"
	EventDeviceStatus   EventType = "device:status"
	EventFlowUpdate     EventType = "flow:update"
	EventCaptureStats   EventType = "capture:stats"
)

type Message struct {
	Type EventType `json:"type"`
	Data any       `json:"data"`
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	mu        sync.RWMutex
	clients   map[*client]struct{}
	broadcast chan Message
	upgrader  websocket.Upgrader
	jwtSecret string
}

func NewHub(jwtSecret string) *Hub {
	return &Hub{
		clients:   make(map[*client]struct{}),
		broadcast: make(chan Message, 256),
		jwtSecret: jwtSecret,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *Hub) Run() {
	for msg := range h.broadcast {
		b, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		h.mu.RLock()
		for c := range h.clients {
			select {
			case c.send <- b:
			default:
				// slow client: drop
			}
		}
		h.mu.RUnlock()
	}
}

func (h *Hub) Broadcast(msg Message) {
	select {
	case h.broadcast <- msg:
	default:
	}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	// Authenticate via ?token= query param
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	if _, err := auth.ValidateToken(token, h.jwtSecret); err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 64)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	// writer goroutine
	go func() {
		defer func() {
			conn.Close()
			h.mu.Lock()
			delete(h.clients, c)
			h.mu.Unlock()
		}()
		for b := range c.send {
			if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
				return
			}
		}
	}()

	// reader (keep-alive, discard incoming)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c.send)
}
