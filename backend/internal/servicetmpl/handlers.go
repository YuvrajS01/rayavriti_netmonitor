package servicetmpl

import (
	"encoding/json"
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
		httputil.SendError(w, http.StatusNotFound, err.Error())
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

	result, err := h.svc.Apply(r.Context(), req)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.SendCreated(w, result)
}
