package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/remote"
)

type RemoteHandler struct {
	store     *remote.Store
	collector *remote.Collector
}

func NewRemoteHandler(store *remote.Store, collector *remote.Collector) *RemoteHandler {
	return &RemoteHandler{store: store, collector: collector}
}
func remoteID(r *http.Request) (int64, error) { return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64) }
func remoteError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		httputil.SendError(w, http.StatusNotFound, "remote instance not found")
	} else {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
	}
}
func (h *RemoteHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.List(r.Context())
	if err != nil {
		remoteError(w, err)
		return
	}
	httputil.SendOK(w, items)
}
func (h *RemoteHandler) Overview(w http.ResponseWriter, r *http.Request) {
	value, err := h.store.Overview(r.Context())
	if err != nil {
		remoteError(w, err)
		return
	}
	httputil.SendOK(w, value)
}
func (h *RemoteHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := remoteID(r)
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	item, err := h.store.Get(r.Context(), id)
	if err != nil {
		remoteError(w, err)
		return
	}
	snapshots, err := h.store.Snapshots(r.Context(), id, 1)
	if err != nil {
		remoteError(w, err)
		return
	}
	httputil.SendOK(w, map[string]any{"instance": item, "snapshot": func() any {
		if len(snapshots) > 0 {
			return snapshots[0]
		}
		return nil
	}()})
}
func (h *RemoteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input remote.CreateInstance
	if err := httputil.ParseJSON(r, &input); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	item, err := h.store.Create(r.Context(), input)
	if err != nil {
		httputil.SendError(w, 400, err.Error())
		return
	}
	httputil.SendCreated(w, item)
}
func (h *RemoteHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := remoteID(r)
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	var input remote.CreateInstance
	if err := httputil.ParseJSON(r, &input); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	item, err := h.store.Update(r.Context(), id, input)
	if err != nil {
		remoteError(w, err)
		return
	}
	httputil.SendOK(w, item)
}
func (h *RemoteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := remoteID(r)
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	if err = h.store.Delete(r.Context(), id); err != nil {
		remoteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *RemoteHandler) Test(w http.ResponseWriter, r *http.Request) {
	id, err := remoteID(r)
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	raw, err := h.collector.Proxy(r.Context(), id, "/health")
	if err != nil {
		httputil.SendError(w, http.StatusBadGateway, err.Error())
		return
	}
	httputil.SendOK(w, map[string]any{"reachable": true, "health": raw})
}
func (h *RemoteHandler) Snapshots(w http.ResponseWriter, r *http.Request) {
	id, err := remoteID(r)
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	items, err := h.store.Snapshots(r.Context(), id, httputil.QueryParamInt(r, "limit", 100, 1, 1000))
	if err != nil {
		remoteError(w, err)
		return
	}
	httputil.SendOK(w, items)
}
func (h *RemoteHandler) Devices(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/api/v1/devices")
}
func (h *RemoteHandler) Alerts(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, "/api/v1/alerts?status=active&limit=200")
}

func (h *RemoteHandler) SetMode(w http.ResponseWriter, r *http.Request) {
	id, err := remoteID(r)
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	var input struct {
		Mode string `json:"mode"`
	}
	if err := httputil.ParseJSON(r, &input); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	if input.Mode != "active" && input.Mode != "maintenance" && input.Mode != "readonly" {
		httputil.SendError(w, 400, "mode must be active, maintenance, or readonly")
		return
	}
	if err := h.store.SetServiceMode(r.Context(), id, input.Mode); err != nil {
		remoteError(w, err)
		return
	}
	httputil.SendOK(w, map[string]string{"mode": input.Mode})
}
func (h *RemoteHandler) proxy(w http.ResponseWriter, r *http.Request, path string) {
	id, err := remoteID(r)
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	raw, err := h.collector.Proxy(r.Context(), id, path)
	if err != nil {
		httputil.SendError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(raw); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "write error")
	}
}
