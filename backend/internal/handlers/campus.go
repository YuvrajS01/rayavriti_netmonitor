package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/campus"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/importer"
)

// CampusHandler provides typed HTTP handlers for campus topology,
// locations, and device import — replacing generic Phase2 handlers for these routes.
type CampusHandler struct {
	locations *campus.LocationService
	topology  *campus.TopologyService
	importer  *importer.ImportService
}

// NewCampusHandler creates a CampusHandler wired to the Postgres pool.
func NewCampusHandler(db database.Database) *CampusHandler {
	pp, ok := db.(database.PoolProvider)
	if !ok || pp.Pool() == nil {
		slog.Warn("CampusHandler: database does not provide a pool, campus features will be unavailable")
		return &CampusHandler{}
	}
	pool := pp.Pool()
	return &CampusHandler{
		locations: campus.NewLocationService(pool),
		topology:  campus.NewTopologyService(pool),
		importer:  importer.NewImportService(pool),
	}
}

// ── Location endpoints ──────────────────────────────────────────

// ListLocations returns a flat list or tree of all locations.
func (h *CampusHandler) ListLocations(w http.ResponseWriter, r *http.Request) {
	if h.locations == nil {
		httputil.SendError(w, http.StatusNotImplemented, "campus service unavailable")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "tree" || format == "tree_with_status" {
		tree, err := h.locations.GetTreeWithStatus(r.Context())
		if err != nil {
			httputil.SendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httputil.SendOK(w, tree)
		return
	}

	locs, err := h.locations.GetAll(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, locs)
}

// GetLocation returns a single location, or its subtree.
func (h *CampusHandler) GetLocation(w http.ResponseWriter, r *http.Request) {
	if h.locations == nil {
		httputil.SendError(w, http.StatusNotImplemented, "campus service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid location id")
		return
	}
	loc, err := h.locations.GetByID(r.Context(), id)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, err.Error())
		return
	}
	httputil.SendOK(w, loc)
}

// GetLocationTree returns the subtree rooted at the given location.
func (h *CampusHandler) GetLocationTree(w http.ResponseWriter, r *http.Request) {
	if h.locations == nil {
		httputil.SendError(w, http.StatusNotImplemented, "campus service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid location id")
		return
	}
	subtree, err := h.locations.GetSubtree(r.Context(), id)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if subtree == nil {
		httputil.SendError(w, http.StatusNotFound, "location not found")
		return
	}
	httputil.SendOK(w, subtree)
}

// CreateLocation creates a new location.
func (h *CampusHandler) CreateLocation(w http.ResponseWriter, r *http.Request) {
	if h.locations == nil {
		httputil.SendError(w, http.StatusNotImplemented, "campus service unavailable")
		return
	}
	var loc campus.Location
	if err := httputil.ParseJSON(r, &loc); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	created, err := h.locations.Create(r.Context(), &loc)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") {
			status = http.StatusBadRequest
		}
		httputil.SendError(w, status, err.Error())
		return
	}
	httputil.SendCreated(w, created)
}

// UpdateLocation updates an existing location.
func (h *CampusHandler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	if h.locations == nil {
		httputil.SendError(w, http.StatusNotImplemented, "campus service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid location id")
		return
	}
	var loc campus.Location
	if err := httputil.ParseJSON(r, &loc); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	updated, err := h.locations.Update(r.Context(), id, &loc)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, updated)
}

// DeleteLocation removes a location, reassigning its children to its parent.
func (h *CampusHandler) DeleteLocation(w http.ResponseWriter, r *http.Request) {
	if h.locations == nil {
		httputil.SendError(w, http.StatusNotImplemented, "campus service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid location id")
		return
	}
	if err := h.locations.Delete(r.Context(), id); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, map[string]string{"deleted": strconv.FormatInt(id, 10)})
}

// MoveLocation moves a location to a new parent (with circular dependency check).
func (h *CampusHandler) MoveLocation(w http.ResponseWriter, r *http.Request) {
	if h.locations == nil {
		httputil.SendError(w, http.StatusNotImplemented, "campus service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid location id")
		return
	}
	var body struct {
		ParentID *int64 `json:"parentId"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.locations.Move(r.Context(), id, body.ParentID); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "circular") {
			status = http.StatusConflict
		}
		httputil.SendError(w, status, err.Error())
		return
	}
	httputil.SendOK(w, map[string]string{"moved": strconv.FormatInt(id, 10)})
}

// LocationStatus returns aggregated device status for a location and descendants.
func (h *CampusHandler) LocationStatus(w http.ResponseWriter, r *http.Request) {
	if h.locations == nil {
		httputil.SendError(w, http.StatusNotImplemented, "campus service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid location id")
		return
	}
	status, err := h.locations.GetLocationStatus(r.Context(), id)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, status)
}

// LocationDevices returns all devices at the given location (non-recursive).
func (h *CampusHandler) LocationDevices(w http.ResponseWriter, r *http.Request) {
	if h.locations == nil {
		httputil.SendError(w, http.StatusNotImplemented, "campus service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid location id")
		return
	}
	ids, err := h.locations.GetDevicesAtLocation(r.Context(), id, false)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, map[string]any{"deviceIds": ids})
}

// ── Topology endpoints ──────────────────────────────────────────

// DependencyTree returns the full device dependency tree.
func (h *CampusHandler) DependencyTree(w http.ResponseWriter, r *http.Request) {
	if h.topology == nil {
		httputil.SendError(w, http.StatusNotImplemented, "topology service unavailable")
		return
	}
	tree, err := h.topology.BuildDependencyTree(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, tree)
}

// DeviceDependencies returns ancestors and descendants of a specific device.
func (h *CampusHandler) DeviceDependencies(w http.ResponseWriter, r *http.Request) {
	if h.topology == nil {
		httputil.SendError(w, http.StatusNotImplemented, "topology service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid device id")
		return
	}
	ancestors, descendants, err := h.topology.GetDeviceDependencies(r.Context(), id)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, map[string]any{
		"ancestors":   ancestors,
		"descendants": descendants,
	})
}

// RootCauseOutages returns all root-cause outage groups.
func (h *CampusHandler) RootCauseOutages(w http.ResponseWriter, r *http.Request) {
	if h.topology == nil {
		httputil.SendError(w, http.StatusNotImplemented, "topology service unavailable")
		return
	}
	outages, err := h.topology.GetRootCauseOutages(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, outages)
}

// ── Import endpoints ────────────────────────────────────────────

// ImportTemplate serves the CSV template download.
func (h *CampusHandler) ImportTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="netmonitor-device-import.csv"`)
	_, _ = w.Write([]byte(importer.TemplateCSV()))
}

// ImportPreview accepts a CSV upload (multipart form "file") and returns
// validation results without creating any devices.
func (h *CampusHandler) ImportPreview(w http.ResponseWriter, r *http.Request) {
	if h.importer == nil {
		httputil.SendError(w, http.StatusNotImplemented, "import service unavailable")
		return
	}

	// Try multipart first, then raw body.
	var rows []importer.ImportRow
	if strings.Contains(r.Header.Get("Content-Type"), "multipart") {
		file, _, err := r.FormFile("file")
		if err != nil {
			httputil.SendError(w, http.StatusBadRequest, "no file uploaded: "+err.Error())
			return
		}
		defer file.Close()
		rows, err = h.importer.ParseCSV(file)
		if err != nil {
			httputil.SendError(w, http.StatusBadRequest, "CSV parse error: "+err.Error())
			return
		}
	} else {
		// Accept raw JSON array of rows for API-only usage.
		if err := json.NewDecoder(r.Body).Decode(&rows); err != nil {
			httputil.SendError(w, http.StatusBadRequest, "expected multipart CSV or JSON array")
			return
		}
	}

	preview, err := h.importer.Validate(r.Context(), rows)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, preview)
}

// ImportExecute accepts validated rows (as JSON) and creates devices in a transaction.
func (h *CampusHandler) ImportExecute(w http.ResponseWriter, r *http.Request) {
	if h.importer == nil {
		httputil.SendError(w, http.StatusNotImplemented, "import service unavailable")
		return
	}
	var rows []importer.ImportRow
	if err := httputil.ParseJSON(r, &rows); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	result, err := h.importer.Execute(r.Context(), rows)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendCreated(w, result)
}
