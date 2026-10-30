package servicetmpl

import (
	"encoding/json"
	"github.com/rayavriti/netmonitor-backend/internal/netutil"
	"net/http"

	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	tmpls := ListTemplates()
	httputil.SendOK(w, tmpls)
}

func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tmpl, err := GetTemplate(name)
	if err != nil {
		httputil.SendStoreError(w, http.StatusNotFound, err)
		return
	}
	httputil.SendOK(w, tmpl)
}

func (h *Handler) ApplyTemplate(w http.ResponseWriter, r *http.Request) {
	var req ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Template == "" || req.Host == "" {
		httputil.SendError(w, http.StatusBadRequest, "template and host are required")
		return
	}
	if err := netutil.ValidateHost(req.Host, netutil.DefaultHostPolicy()); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "host is not allowed")
		return
	}

	result, err := h.svc.Apply(r.Context(), req)
	if err != nil {
		httputil.SendStoreError(w, http.StatusBadRequest, err)
		return
	}
	httputil.SendCreated(w, result)
}
