package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type PortsHandler struct{ db database.Database }

func NewPortsHandler(db database.Database) *PortsHandler { return &PortsHandler{db: db} }

func (h *PortsHandler) ForDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	results, err := h.db.GetPortScanResults(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, results)
}
