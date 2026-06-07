package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type AlertHandler struct{ db database.Database }

func NewAlertHandler(db database.Database) *AlertHandler { return &AlertHandler{db: db} }

func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	alerts, total, err := h.db.GetAlerts(r.Context(), status, limit, offset)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]any{"alerts": alerts, "total": total})
}

func (h *AlertHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	a, err := h.db.GetAlert(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "alert not found")
		return
	}
	httputil.SendOK(w, a)
}

func (h *AlertHandler) Create(w http.ResponseWriter, r *http.Request) {
	var a models.Alert
	if err := httputil.ParseJSON(r, &a); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	a.Status = "active"
	created, err := h.db.CreateAlert(r.Context(), &a)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendCreated(w, created)
}

func (h *AlertHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	claims := auth.GetClaims(r.Context())
	by := ""
	if claims != nil {
		by = claims.Username
	}
	if err := h.db.UpdateAlertStatus(r.Context(), id, "acknowledged", by); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]string{"message": "acknowledged"})
}

func (h *AlertHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	claims := auth.GetClaims(r.Context())
	by := ""
	if claims != nil {
		by = claims.Username
	}
	if err := h.db.UpdateAlertStatus(r.Context(), id, "resolved", by); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]string{"message": "resolved"})
}

func (h *AlertHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	if err := h.db.DeleteAlert(r.Context(), id); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]string{"message": "deleted"})
}
