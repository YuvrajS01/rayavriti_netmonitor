package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type AuthHandler struct {
	db  database.Database
	cfg *config.Config
}

func NewAuthHandler(db database.Database, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg}
}

func (h *AuthHandler) authenticate(w http.ResponseWriter, r *http.Request, includeExpires bool) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.db.GetUserByUsername(r.Context(), body.Username)
	if err != nil || !auth.CheckPassword(body.Password, user.PasswordHash) {
		httputil.SendError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !user.Enabled {
		httputil.SendError(w, http.StatusForbidden, "account disabled")
		return
	}

	// Load permissions from the roles table
	var permissions []string
	if user.RoleID != nil {
		if perms, err := h.db.GetRolePermissions(r.Context(), *user.RoleID); err == nil {
			permissions = perms
		}
	}
	// Fallback: derive permissions from the legacy role string
	if len(permissions) == 0 {
		permissions = permissionsForRole(user.Role)
	}

	at, rt, err := auth.GenerateTokenPairWithPerms(user.ID, user.Username, user.Role, permissions,
		h.cfg.Auth.JWTSecret, h.cfg.Auth.AccessTokenExpiry, h.cfg.Auth.RefreshTokenExpiry)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	tokenHash := auth.HashToken(rt)
	expiresAt := time.Now().Add(h.cfg.Auth.RefreshTokenExpiry)
	if err := h.db.CreateRefreshToken(r.Context(), tokenHash, user.ID, expiresAt); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "session storage failed")
		return
	}
	resp := map[string]any{
		"accessToken":  at,
		"refreshToken": rt,
		"user": map[string]any{
			"id":          user.ID,
			"username":    user.Username,
			"role":        user.Role,
			"permissions": permissions,
		},
	}
	if includeExpires {
		resp["expiresIn"] = int(h.cfg.Auth.AccessTokenExpiry.Seconds())
	}
	httputil.SendOK(w, resp)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	h.authenticate(w, r, false)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = httputil.ParseJSON(r, &body)
	if body.RefreshToken != "" {
		tokenHash := auth.HashToken(body.RefreshToken)
		_ = h.db.DeleteRefreshToken(r.Context(), tokenHash)
	}
	httputil.SendOK(w, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httputil.SendError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	user, err := h.db.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "user not found")
		return
	}
	httputil.SendOK(w, user)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	claims, err := auth.ValidateToken(body.RefreshToken, h.cfg.Auth.JWTSecret)
	if err != nil {
		httputil.SendError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	// Verify token exists in DB (prevents reuse after revocation)
	tokenHash := auth.HashToken(body.RefreshToken)
	existing, err := h.db.GetRefreshToken(r.Context(), tokenHash)
	if err != nil || existing == nil {
		httputil.SendError(w, http.StatusUnauthorized, "refresh token revoked")
		return
	}

	// Re-fetch user to check current state (enabled, role, permissions)
	user, err := h.db.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		httputil.SendError(w, http.StatusUnauthorized, "user not found")
		return
	}
	if !user.Enabled {
		// Revoke the token and reject
		_ = h.db.DeleteRefreshToken(r.Context(), tokenHash)
		httputil.SendError(w, http.StatusForbidden, "account disabled")
		return
	}

	// Reload current permissions from DB
	var permissions []string
	if user.RoleID != nil {
		if perms, err := h.db.GetRolePermissions(r.Context(), *user.RoleID); err == nil {
			permissions = perms
		}
	}
	if len(permissions) == 0 {
		permissions = permissionsForRole(user.Role)
	}

	// Delete old token (rotation)
	_ = h.db.DeleteRefreshToken(r.Context(), tokenHash)
	// Issue new pair from current user state (not stale claims)
	at, rt, err := auth.GenerateTokenPairWithPerms(user.ID, user.Username, user.Role, permissions,
		h.cfg.Auth.JWTSecret, h.cfg.Auth.AccessTokenExpiry, h.cfg.Auth.RefreshTokenExpiry)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	// Store new refresh token
	newHash := auth.HashToken(rt)
	expiresAt := time.Now().Add(h.cfg.Auth.RefreshTokenExpiry)
	_ = h.db.CreateRefreshToken(r.Context(), newHash, user.ID, expiresAt)
	httputil.SendOK(w, map[string]any{
		"accessToken":  at,
		"refreshToken": rt,
		"user": map[string]any{
			"id":          user.ID,
			"username":    user.Username,
			"role":        user.Role,
			"permissions": permissions,
		},
	})
}

func (h *AuthHandler) V1Login(w http.ResponseWriter, r *http.Request) {
	h.authenticate(w, r, true)
}

func (h *AuthHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	httputil.SendError(w, http.StatusNotImplemented, "2FA is not implemented")
}

func (h *AuthHandler) V1Logout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = httputil.ParseJSON(r, &body)
	if body.RefreshToken != "" {
		tokenHash := auth.HashToken(body.RefreshToken)
		_ = h.db.DeleteRefreshToken(r.Context(), tokenHash)
	}
	httputil.SendOK(w, map[string]bool{"loggedOut": true})
}

func (h *AuthHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	keys, err := h.db.GetAPIKeysByUser(r.Context(), claims.UserID)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, keys)
}

func (h *AuthHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Description string `json:"description"`
	}
	_ = httputil.ParseJSON(r, &body)
	claims := auth.GetClaims(r.Context())
	rawKey, hash, err := auth.GenerateAPIKey()
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "key generation failed")
		return
	}
	key, err := h.db.CreateAPIKey(r.Context(), &models.APIKey{
		UserID:      claims.UserID,
		KeyHash:     hash,
		Description: body.Description,
	})
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendCreated(w, map[string]any{
		"id": key.ID, "key": rawKey, "description": key.Description, "createdAt": key.CreatedAt,
	})
}

func (h *AuthHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		httputil.SendError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	key, err := h.db.GetAPIKeyByID(r.Context(), id)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "API key not found")
		return
	}
	if key.UserID != claims.UserID {
		httputil.SendError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := h.db.DeleteAPIKey(r.Context(), id); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, map[string]string{"message": "deleted"})
}

func parseID(s string) (int64, error) {
	var id int64
	if _, err := fmt.Sscanf(s, "%d", &id); err != nil {
		return 0, err
	}
	return id, nil
}

func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		Role        string `json:"role"`
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
		Enabled     *bool  `json:"enabled"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.Username == "" || body.Password == "" {
		httputil.SendError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	existing, _ := h.db.GetUserByUsername(r.Context(), body.Username)
	if existing != nil {
		httputil.SendError(w, http.StatusConflict, "username already exists")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "password hashing failed")
		return
	}
	role := body.Role
	if role == "" {
		role = "viewer"
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	user, err := h.db.CreateUser(r.Context(), &models.User{
		Username:     body.Username,
		PasswordHash: hash,
		Role:         role,
		DisplayName:  body.DisplayName,
		Email:        body.Email,
		Phone:        body.Phone,
		Enabled:      enabled,
	})
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendCreated(w, user)
}

func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	claims := auth.GetClaims(r.Context())
	if claims != nil && claims.UserID == id {
		httputil.SendError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}
	user, err := h.db.GetUserByID(r.Context(), id)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "user not found")
		return
	}
	if user.Role == "super_admin" {
		httputil.SendError(w, http.StatusForbidden, "cannot delete super admin")
		return
	}
	if err := h.db.DeleteUser(r.Context(), id); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, map[string]string{"message": "deleted"})
}

// permissionsForRole returns a default permission set for the legacy role string.
func permissionsForRole(role string) []string {
	switch role {
	case "super_admin", "admin":
		return []string{
			"devices.read", "devices.write", "devices.delete",
			"alerts.read", "alerts.create", "alerts.acknowledge", "alerts.resolve",
			"alert_rules.write", "incidents.write", "maintenance.write",
			"contacts.write", "notifications.manage", "reports.read", "reports.write",
			"settings.write", "users.manage", "import.execute", "discovery.execute",
			"capture.execute", "status_page.manage", "sla.manage",
			"system.monitoring", "system.logs",
		}
	case "network_admin":
		return []string{
			"devices.read", "devices.write", "devices.delete",
			"alerts.read", "alerts.create", "alerts.acknowledge", "alerts.resolve",
			"alert_rules.write", "incidents.write", "maintenance.write",
			"contacts.write", "notifications.manage", "reports.read", "reports.write",
			"import.execute", "discovery.execute", "capture.execute",
			"status_page.manage", "sla.manage", "system.monitoring",
		}
	case "dept_admin":
		return []string{
			"devices.read", "alerts.read", "alerts.create", "alerts.acknowledge",
			"incidents.write", "reports.read",
		}
	case "viewer":
		return []string{
			"devices.read", "alerts.read", "reports.read",
		}
	default:
		return nil
	}
}
