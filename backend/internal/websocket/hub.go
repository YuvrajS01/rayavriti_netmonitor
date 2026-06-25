package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

type EventType string

const (
	EventMetricUpdate   EventType = "metric:update"
	EventAlertTriggered EventType = "alert:triggered"
	EventAlertUpdated   EventType = "alert:updated"
	EventAlertResolved  EventType = "alert:resolved"
	EventDeviceStatus   EventType = "device:status"
	EventFlowUpdate     EventType = "flow:update"
	EventCapturePacket  EventType = "capture:packet"
	EventCaptureStatus  EventType = "capture:status"
	EventPortsScanned   EventType = "ports:scanned"
	EventBootstrap      EventType = "bootstrap"
)

type Message struct {
	Type      EventType `json:"type"`
	RequestID string    `json:"request_id,omitempty"`
	Data      any       `json:"data"`
}

type ClientInfo struct {
	UserID      int64
	Username    string
	Role        string
	Permissions []string
	Scopes      []ScopeEntry
}

// ScopeEntry represents a user scope for filtering events.
type ScopeEntry struct {
	ScopeType  string
	ScopeValue string
}

type client struct {
	conn   *websocket.Conn
	send   chan []byte
	info   ClientInfo
	closed chan struct{}
	mu     sync.Mutex
	dead   bool
}

type Hub struct {
	mu          sync.RWMutex
	clients     map[*client]struct{}
	broadcast   chan Message
	upgrader    websocket.Upgrader
	jwtSecret   string
	bootstrap   BootstrapFunc
	publisher   func(ctx context.Context, msg Message)
	scopeFilter ScopeFilterFunc
}

// ScopeFilterFunc determines whether a client should receive a message.
// Returns true if the message should be delivered to the client.
type ScopeFilterFunc func(clientInfo ClientInfo, msg Message) bool

// BootstrapFunc generates the initial bootstrap payload for a newly connected client.
type BootstrapFunc func(ctx context.Context, userID int64, username, role string) (map[string]any, error)

func NewHub(jwtSecret string, bootstrap BootstrapFunc, allowedOrigins []string) *Hub {
	allowAll := len(allowedOrigins) == 0
	return &Hub{
		clients:   make(map[*client]struct{}),
		broadcast: make(chan Message, 256),
		jwtSecret: jwtSecret,
		bootstrap: bootstrap,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				if allowAll {
					return true
				}
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true // non-browser clients
				}
				for _, o := range allowedOrigins {
					if o == origin {
						return true
					}
				}
				return false
			},
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
		count := 0
		for c := range h.clients {
			c.mu.Lock()
			dead := c.dead
			info := c.info
			c.mu.Unlock()
			if dead {
				continue
			}
			if h.scopeFilter != nil && !h.scopeFilter(info, msg) {
				continue
			}
			select {
			case c.send <- b:
				count++
			default:
				// slow client: drop message for this client
			}
		}
		h.mu.RUnlock()

		if count > 0 {
			slog.Debug("WebSocket broadcast",
				"event", string(msg.Type),
				"payload_bytes", len(b),
				"recipients", count,
			)
		}
	}
}

// SetScopeFilter sets a function that filters which clients receive which messages.
func (h *Hub) SetScopeFilter(fn ScopeFilterFunc) {
	h.scopeFilter = fn
}

func (h *Hub) Broadcast(msg Message) {
	if h.publisher != nil {
		h.publisher(context.Background(), msg)
		return
	}
	select {
	case h.broadcast <- msg:
	default:
		slog.Warn("WebSocket broadcast channel full, dropping message", "event", string(msg.Type))
	}
}

// BroadcastLocal sends a message to locally connected clients only (no Redis publish).
// Used by the Pub/Sub subscriber to deliver messages from other instances.
func (h *Hub) BroadcastLocal(msg Message) {
	select {
	case h.broadcast <- msg:
	default:
		slog.Warn("WebSocket broadcast channel full, dropping message", "event", string(msg.Type))
	}
}

// SetPublisher sets an optional function that intercepts Broadcast calls
// and publishes them via Redis Pub/Sub instead of local delivery.
func (h *Hub) SetPublisher(fn func(ctx context.Context, msg Message)) {
	h.publisher = fn
}

func (h *Hub) Stop() {
	close(h.broadcast)
	h.mu.Lock()
	for c := range h.clients {
		c.mu.Lock()
		c.dead = true
		c.mu.Unlock()
		_ = c.conn.Close()
	}
	h.mu.Unlock()
	slog.Info("WebSocket hub stopped")
}

func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// extractToken tries to extract a JWT from the request using multiple methods:
// 1. Authorization: Bearer <token> header
// 2. Sec-WebSocket-Protocol: <token>
// 3. ?token=<token> query parameter
func (h *Hub) extractToken(r *http.Request) string {
	// Method 1: Authorization header
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	// Method 2: Sec-WebSocket-Protocol header
	if proto := r.Header.Get("Sec-WebSocket-Protocol"); proto != "" {
		// The protocol header may contain multiple values comma-separated
		// The first one is typically the token
		parts := strings.SplitN(proto, ",", 2)
		token := strings.TrimSpace(parts[0])
		if token != "" {
			return token
		}
	}

	// Method 3: query parameter
	return r.URL.Query().Get("token")
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	token := h.extractToken(r)
	if token == "" {
		http.Error(w, "missing authentication token", http.StatusUnauthorized)
		return
	}

	claims, err := auth.ValidateToken(token, h.jwtSecret)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("WebSocket upgrade failed", "error", err)
		return
	}

	c := &client{
		conn: conn,
		send: make(chan []byte, 64),
		info: ClientInfo{
			UserID:      claims.UserID,
			Username:    claims.Username,
			Role:        claims.Role,
			Permissions: claims.Permissions,
		},
		closed: make(chan struct{}),
	}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	totalConns := h.ConnectionCount()
	slog.Info("WebSocket client connected",
		"user_id", c.info.UserID,
		"username", c.info.Username,
		"remote_addr", r.RemoteAddr,
		"total_connections", totalConns,
	)

	// Send bootstrap data
	if h.bootstrap != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			data, err := h.bootstrap(ctx, c.info.UserID, c.info.Username, c.info.Role)
			if err != nil {
				slog.Error("Failed to generate bootstrap data", "error", err, "user_id", c.info.UserID)
				return
			}
			msg := Message{Type: EventBootstrap, Data: data}
			b, err := json.Marshal(msg)
			if err != nil {
				return
			}
			c.mu.Lock()
			dead := c.dead
			c.mu.Unlock()
			if dead {
				return
			}
			select {
			case c.send <- b:
			default:
			}
		}()
	}

	// Writer goroutine with ping/pong
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer func() {
			ticker.Stop()
			_ = conn.Close()
			close(c.closed)
			slog.Info("WebSocket client disconnected",
				"user_id", c.info.UserID,
				"username", c.info.Username,
				"total_connections", h.ConnectionCount(),
			)
		}()

		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
		for {
			c.mu.Lock()
			dead := c.dead
			c.mu.Unlock()
			if dead {
				return
			}
			select {
			case msg, ok := <-c.send:
				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				if !ok {
					_ = conn.WriteMessage(websocket.CloseMessage, nil)
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-ticker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// Reader goroutine (handles pong, reads for keep-alive)
	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	c.mu.Lock()
	c.dead = true
	c.mu.Unlock()
	// Don't close c.send here — the writer goroutine detects disconnection
	// via conn.ReadMessage() and handles cleanup safely.
}

// SendToClient sends a message to a specific client (by user ID).
// Useful for targeted notifications.
func (h *Hub) SendToClient(userID int64, msg Message) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.info.UserID == userID {
			c.mu.Lock()
			dead := c.dead
			c.mu.Unlock()
			if dead {
				continue
			}
			select {
			case c.send <- b:
			default:
			}
		}
	}
}
