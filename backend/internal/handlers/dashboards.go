package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type DashboardHandler struct{ db database.Database }

func NewDashboardHandler(db database.Database) *DashboardHandler { return &DashboardHandler{db: db} }

func (h *DashboardHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	ds, err := h.db.GetDashboards(r.Context(), claims.UserID)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, ds)
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	d, err := h.db.GetDashboard(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "not found")
		return
	}
	httputil.SendOK(w, d)
}

func (h *DashboardHandler) Save(w http.ResponseWriter, r *http.Request) {
	var d models.Dashboard
	if err := httputil.ParseJSON(r, &d); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	claims := auth.GetClaims(r.Context())
	d.UserID = claims.UserID
	if idStr := chi.URLParam(r, "id"); idStr != "" {
		id, _ := parseID(idStr)
		d.ID = id
	}
	saved, err := h.db.SaveDashboard(r.Context(), &d)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, saved)
}

func (h *DashboardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	if err := h.db.DeleteDashboard(r.Context(), id); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]string{"message": "deleted"})
}
