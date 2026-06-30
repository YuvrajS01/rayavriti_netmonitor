package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

// RoleHandler provides typed CRUD for roles with permission validation.
type RoleHandler struct {
	db     database.Database
	phase2 database.Phase2Store
}

func NewRoleHandler(db database.Database) *RoleHandler {
	phase2, _ := db.(database.Phase2Store)
	return &RoleHandler{db: db, phase2: phase2}
}

func (h *RoleHandler) requireStore(w http.ResponseWriter) bool {
	if h.phase2 == nil {
		httputil.SendError(w, http.StatusNotImplemented, "phase 2 storage not available")
		return false
	}
	return true
}

var validPermissions = map[string]bool{
	"devices.read":         true,
	"devices.write":        true,
	"devices.delete":       true,
	"alerts.read":          true,
	"alerts.create":        true,
	"alerts.acknowledge":   true,
	"alerts.resolve":       true,
	"alert_rules.write":    true,
	"capture.execute":      true,
	"discovery.execute":    true,
	"import.execute":       true,
	"incidents.write":      true,
	"maintenance.write":    true,
	"contacts.write":       true,
	"notifications.manage": true,
	"reports.read":         true,
	"reports.write":        true,
	"settings.write":       true,
	"users.manage":         true,
	"status_page.manage":   true,
	"sla.manage":           true,
	"system.monitoring":    true,
	"system.logs":          true,
}

func validatePermissions(perms []string) error {
	for _, p := range perms {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !validPermissions[p] {
			return &httputil.ValidationError{Field: "permissions", Message: "unknown permission: " + p}
		}
	}
	return nil
}

func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	rows, err := h.phase2.ListPhase2(r.Context(), "roles", nil)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, rows)
}

func (h *RoleHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := h.phase2.GetPhase2(r.Context(), "roles", id)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "role not found")
		return
	}
	httputil.SendOK(w, item)
}

func (h *RoleHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	var body struct {
		Name        string   `json:"name"`
		DisplayName string   `json:"display_name"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		httputil.SendError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(body.Name) > 64 {
		httputil.SendError(w, http.StatusBadRequest, "name must be at most 64 characters")
		return
	}
	if body.DisplayName == "" {
		body.DisplayName = body.Name
	}
	if err := validatePermissions(body.Permissions); err != nil {
		httputil.SendError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Check for duplicate name
	existing, _ := h.phase2.ListPhase2(r.Context(), "roles", map[string]string{"name": body.Name})
	if len(existing) > 0 {
		httputil.SendError(w, http.StatusConflict, "role name already exists")
		return
	}
	permJSON, _ := json.Marshal(body.Permissions)
	item, err := h.phase2.CreatePhase2(r.Context(), "roles", map[string]any{
		"name":         body.Name,
		"display_name": body.DisplayName,
		"description":  body.Description,
		"permissions":  string(permJSON),
	})
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.SendCreated(w, item)
}

func (h *RoleHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	// Check existing role
	existing, err := h.phase2.GetPhase2(r.Context(), "roles", id)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "role not found")
		return
	}
	// Prevent editing system roles
	if isSystem, ok := existing["is_system"].(bool); ok && isSystem {
		httputil.SendError(w, http.StatusForbidden, "cannot modify system roles")
		return
	}
	var body struct {
		DisplayName *string  `json:"display_name"`
		Description *string  `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	patch := map[string]any{}
	if body.DisplayName != nil {
		patch["display_name"] = *body.DisplayName
	}
	if body.Description != nil {
		patch["description"] = *body.Description
	}
	if body.Permissions != nil {
		if err := validatePermissions(body.Permissions); err != nil {
			httputil.SendError(w, http.StatusBadRequest, err.Error())
			return
		}
		permJSON, _ := json.Marshal(body.Permissions)
		patch["permissions"] = string(permJSON)
	}
	if len(patch) == 0 {
		httputil.SendError(w, http.StatusBadRequest, "no fields to update")
		return
	}
	item, err := h.phase2.UpdatePhase2(r.Context(), "roles", id, patch)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.SendOK(w, item)
}

func (h *RoleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	// Check existing role
	existing, err := h.phase2.GetPhase2(r.Context(), "roles", id)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "role not found")
		return
	}
	if isSystem, ok := existing["is_system"].(bool); ok && isSystem {
		httputil.SendError(w, http.StatusForbidden, "cannot delete system roles")
		return
	}
	// Check if any users have this role
	users, _ := h.phase2.ListPhase2(r.Context(), "users", map[string]string{"role_id": chi.URLParam(r, "id")})
	if len(users) > 0 {
		httputil.SendError(w, http.StatusConflict, "role is assigned to users")
		return
	}
	if err := h.phase2.DeletePhase2(r.Context(), "roles", id); err != nil {
		httputil.SendError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.SendOK(w, map[string]bool{"deleted": true})
}

// UserScopeHandler provides typed CRUD for user scope assignments.
type UserScopeHandler struct {
	phase2 database.Phase2Store
}

func NewUserScopeHandler(db database.Database) *UserScopeHandler {
	phase2, _ := db.(database.Phase2Store)
	return &UserScopeHandler{phase2: phase2}
}

var validScopeTypes = map[string]bool{
	"location":   true,
	"department": true,
	"device":     true,
}

func (h *UserScopeHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.phase2 == nil {
		httputil.SendError(w, http.StatusNotImplemented, "phase 2 storage not available")
		return
	}
	filters := map[string]string{}
	if uid := r.URL.Query().Get("user_id"); uid != "" {
		filters["user_id"] = uid
	}
	rows, err := h.phase2.ListPhase2(r.Context(), "user_scopes", filters)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, rows)
}

func (h *UserScopeHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h.phase2 == nil {
		httputil.SendError(w, http.StatusNotImplemented, "phase 2 storage not available")
		return
	}
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httputil.SendError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var body struct {
		UserID     int64  `json:"user_id"`
		ScopeType  string `json:"scope_type"`
		ScopeValue string `json:"scope_value"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.UserID == 0 {
		httputil.SendError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if !validScopeTypes[body.ScopeType] {
		httputil.SendError(w, http.StatusBadRequest, "scope_type must be one of: location, department, device")
		return
	}
	if body.ScopeValue == "" {
		httputil.SendError(w, http.StatusBadRequest, "scope_value is required")
		return
	}
	// Check for duplicate
	existing, _ := h.phase2.ListPhase2(r.Context(), "user_scopes", map[string]string{
		"user_id":     fmt.Sprintf("%d", body.UserID),
		"scope_type":  body.ScopeType,
		"scope_value": body.ScopeValue,
	})
	if len(existing) > 0 {
		httputil.SendError(w, http.StatusConflict, "scope already assigned")
		return
	}
	item, err := h.phase2.CreatePhase2(r.Context(), "user_scopes", map[string]any{
		"user_id":     body.UserID,
		"scope_type":  body.ScopeType,
		"scope_value": body.ScopeValue,
	})
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.SendCreated(w, item)
}

func (h *UserScopeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.phase2 == nil {
		httputil.SendError(w, http.StatusNotImplemented, "phase 2 storage not available")
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.phase2.DeletePhase2(r.Context(), "user_scopes", id); err != nil {
		httputil.SendError(w, http.StatusBadRequest, err.Error())
		return
	}
	httputil.SendOK(w, map[string]bool{"deleted": true})
}
