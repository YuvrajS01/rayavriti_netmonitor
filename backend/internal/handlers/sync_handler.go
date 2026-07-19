package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/monitoring"
)

type SyncHandler struct {
	pool   *pgxpool.Pool
	config monitoring.SysConfigStore
}

func NewSyncHandler(pool *pgxpool.Pool, config monitoring.SysConfigStore) *SyncHandler {
	return &SyncHandler{pool: pool, config: config}
}
func (h *SyncHandler) Config(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil || body.Fingerprint == "" {
		httputil.SendError(w, 400, "fingerprint is required")
		return
	}
	var mode string
	err := h.pool.QueryRow(r.Context(), "SELECT service_mode FROM remote_instances WHERE sync_fingerprint=$1", body.Fingerprint).Scan(&mode)
	if err != nil {
		mode = "active"
	}
	httputil.SendOK(w, map[string]string{"service_mode": mode})
}
func (h *SyncHandler) Identity(w http.ResponseWriter, r *http.Request) {
	fingerprint, err := h.config.GetSysConfig(r.Context(), "sys_fingerprint")
	if err != nil {
		httputil.SendError(w, 500, "unable to read system identity")
		return
	}
	httputil.SendOK(w, map[string]string{"fingerprint": fingerprint})
}
