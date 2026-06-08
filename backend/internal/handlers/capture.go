package handlers

import (
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// CaptureHandler provides start/stop/stats for packet capture.
// It stores an atomic running flag; real capture is wired via the collector in Phase 1.
type CaptureHandler struct {
	running int32
	db      database.Database
}

func NewCaptureHandler(db database.Database) *CaptureHandler { return &CaptureHandler{db: db} }

func (h *CaptureHandler) Start(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Interface string `json:"interface"`
		Filter    string `json:"filter"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	if body.Interface == "" {
		httputil.SendError(w, 400, "interface is required")
		return
	}

	if !atomic.CompareAndSwapInt32(&h.running, 0, 1) {
		httputil.SendError(w, 409, "capture already running")
		return
	}

	session := &models.CaptureSession{
		InterfaceName: body.Interface,
		Filter:        body.Filter,
		Status:        "running",
		TotalPackets:  0,
		TotalBytes:    0,
	}
	created, err := h.db.CreateCaptureSession(r.Context(), session)
	if err != nil {
		atomic.StoreInt32(&h.running, 0)
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendCreated(w, created)
}

func (h *CaptureHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}

	stats := models.CaptureSessionStats{
		TotalPackets: 0,
		TotalBytes:   0,
	}
	if err := h.db.StopCaptureSession(r.Context(), id, stats); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	atomic.StoreInt32(&h.running, 0)
	httputil.SendOK(w, map[string]string{"status": "stopped"})
}

func (h *CaptureHandler) Stats(w http.ResponseWriter, r *http.Request) {
	httputil.SendOK(w, map[string]any{
		"running":      atomic.LoadInt32(&h.running) == 1,
		"totalPackets": 0,
		"totalBytes":   0,
		"protocols":    map[string]int{},
	})
}

func (h *CaptureHandler) Interfaces(w http.ResponseWriter, r *http.Request) {
	interfaces := []map[string]string{
		{"name": "eth0", "description": "Ethernet interface"},
		{"name": "lo", "description": "Loopback interface"},
		{"name": "wlan0", "description": "Wireless interface"},
	}
	httputil.SendOK(w, interfaces)
}

func (h *CaptureHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	session, err := h.db.GetCaptureSession(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "session not found")
		return
	}
	httputil.SendOK(w, session)
}

func (h *CaptureHandler) GetPackets(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 200
	}
	packets, err := h.db.GetCapturePackets(r.Context(), id, limit, offset)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, packets)
}

func (h *CaptureHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	sessions, err := h.db.GetCaptureSessions(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	httputil.SendOK(w, sessions)
}
