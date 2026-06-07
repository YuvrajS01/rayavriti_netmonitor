package handlers

import (
	"net/http"
	"sync/atomic"

	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

// CaptureHandler provides start/stop/stats for packet capture.
// It stores an atomic running flag; real capture is wired via the collector in Phase 1.
type CaptureHandler struct {
	running int32
}

func NewCaptureHandler() *CaptureHandler { return &CaptureHandler{} }

func (h *CaptureHandler) Start(w http.ResponseWriter, r *http.Request) {
	if !atomic.CompareAndSwapInt32(&h.running, 0, 1) {
		httputil.SendError(w, 409, "capture already running")
		return
	}
	httputil.SendOK(w, map[string]string{"status": "started"})
}

func (h *CaptureHandler) Stop(w http.ResponseWriter, r *http.Request) {
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
