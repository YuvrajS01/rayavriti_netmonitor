package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/rbac"
)

// DashboardHandler serves user dashboards. Every user manages their own;
// a shared dashboard is readable by everyone but only its owner can change
// or delete it. Widgets fetch their data from the regular (scoped) APIs, so
// a shared layout never exposes data the viewer could not already see.
type DashboardHandler struct{ db database.Database }

func NewDashboardHandler(db database.Database) *DashboardHandler { return &DashboardHandler{db: db} }

func (h *DashboardHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	ds, err := h.db.GetDashboards(r.Context(), claims.UserID)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	if ds == nil {
		ds = []models.Dashboard{}
	}
	httputil.SendOK(w, ds)
}

// visible loads a dashboard the caller may read: their own or a shared one.
// Anything else is reported as not found.
func (h *DashboardHandler) visible(w http.ResponseWriter, r *http.Request) (*models.Dashboard, bool) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return nil, false
	}
	d, err := h.db.GetDashboard(r.Context(), id)
	if err != nil || d == nil {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			httputil.SendInternalError(w, err)
			return nil, false
		}
		httputil.SendError(w, http.StatusNotFound, "not found")
		return nil, false
	}
	if d.UserID != auth.GetClaims(r.Context()).UserID && !d.Shared {
		httputil.SendError(w, http.StatusNotFound, "not found")
		return nil, false
	}
	return d, true
}

// owned loads a dashboard the caller may change. Someone else's shared
// dashboard is visible, so changing it is forbidden rather than not found.
func (h *DashboardHandler) owned(w http.ResponseWriter, r *http.Request) (*models.Dashboard, bool) {
	d, ok := h.visible(w, r)
	if !ok {
		return nil, false
	}
	if d.UserID != auth.GetClaims(r.Context()).UserID {
		httputil.SendError(w, http.StatusForbidden, "only the owner can change this dashboard")
		return nil, false
	}
	return d, true
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	d, ok := h.visible(w, r)
	if !ok {
		return
	}
	h.markDefault(r, d)
	httputil.SendOK(w, d)
}

// markDefault sets IsDefault for the caller (GetDashboard has no caller).
func (h *DashboardHandler) markDefault(r *http.Request, d *models.Dashboard) {
	ds, err := h.db.GetDashboards(r.Context(), auth.GetClaims(r.Context()).UserID)
	if err != nil {
		return
	}
	for _, x := range ds {
		if x.ID == d.ID {
			d.IsDefault = x.IsDefault
			return
		}
	}
}

type dashboardInput struct {
	Name   *string         `json:"name"`
	Layout json.RawMessage `json:"layout"`
	Shared *bool           `json:"shared"`
}

// canShare reports whether the caller may share dashboards with everyone:
// operators who can change devices. Viewers keep their dashboards private.
func canShare(r *http.Request) bool {
	claims := auth.GetClaims(r.Context())
	return claims != nil && (rbac.IsUnscopedRole(claims.Role) || rbac.HasPermission(claims.Permissions, models.PermDevicesWrite))
}

// Save creates a dashboard (POST) or updates one the caller owns (PUT).
// On update, omitted fields keep their stored values.
func (h *DashboardHandler) Save(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, models.DashboardMaxLayoutBytes+4096))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	var in dashboardInput
	if err := json.Unmarshal(body, &in); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}

	d := &models.Dashboard{UserID: auth.GetClaims(r.Context()).UserID, Layout: models.EmptyDashboardLayout}
	if chi.URLParam(r, "id") != "" {
		existing, ok := h.owned(w, r)
		if !ok {
			return
		}
		d = existing
	}
	if in.Name != nil {
		d.Name = strings.TrimSpace(*in.Name)
	}
	if d.Name == "" || len([]rune(d.Name)) > models.DashboardMaxNameLength {
		httputil.SendError(w, http.StatusBadRequest, "name is required and must be at most 100 characters")
		return
	}
	if len(in.Layout) > 0 && string(in.Layout) != "null" {
		layout, err := models.ValidateDashboardLayout(in.Layout)
		if err != nil {
			httputil.SendError(w, http.StatusBadRequest, err.Error())
			return
		}
		d.Layout = layout
	}
	if in.Shared != nil {
		if *in.Shared && !d.Shared && !canShare(r) {
			httputil.SendError(w, http.StatusForbidden, "you need device write access to share dashboards")
			return
		}
		d.Shared = *in.Shared
	}

	saved, err := h.db.SaveDashboard(r.Context(), d)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	h.markDefault(r, saved)
	httputil.SendOK(w, saved)
}

func (h *DashboardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	d, ok := h.owned(w, r)
	if !ok {
		return
	}
	if err := h.db.DeleteDashboard(r.Context(), d.ID); err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, map[string]string{"message": "deleted"})
}

// SetDefault makes a visible dashboard (own or shared) open first for the
// caller.
func (h *DashboardHandler) SetDefault(w http.ResponseWriter, r *http.Request) {
	d, ok := h.visible(w, r)
	if !ok {
		return
	}
	if err := h.db.SetDefaultDashboard(r.Context(), auth.GetClaims(r.Context()).UserID, &d.ID); err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, map[string]any{"defaultDashboardId": d.ID})
}

// ClearDefault returns the caller to the built-in overview.
func (h *DashboardHandler) ClearDefault(w http.ResponseWriter, r *http.Request) {
	if err := h.db.SetDefaultDashboard(r.Context(), auth.GetClaims(r.Context()).UserID, nil); err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, map[string]any{"defaultDashboardId": nil})
}
