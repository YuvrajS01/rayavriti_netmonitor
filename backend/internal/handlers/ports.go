package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

// PortsHandler returns port scan results (stored by scanner).
type PortsHandler struct{}

func NewPortsHandler() *PortsHandler { return &PortsHandler{} }

func (h *PortsHandler) ForDevice(w http.ResponseWriter, r *http.Request) {
	_, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	// Port scan results are stored during background scan; return empty for now.
	httputil.SendOK(w, []any{})
}
