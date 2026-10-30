package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type AuthHandler struct {
	db           database.Database
	cfg          *config.Config
	loginLimiter *auth.LoginLimiter
}

func NewAuthHandler(db database.Database, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg}
}

// WithLoginLimiter attaches a brute-force throttle to the login endpoints.
func (h *AuthHandler) WithLoginLimiter(ll *auth.LoginLimiter) *AuthHandler {
	h.loginLimiter = ll
	return h
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

	// Brute-force throttle: per-IP + per-username sliding window.
	ip := clientIP(r)
	if h.loginLimiter != nil {
		allowed, reason := h.loginLimiter.Allow(ip, body.Username)
		if !allowed {
			httputil.SendError(w, http.StatusTooManyRequests, reason)
			return
		}
	}

	user, err := h.db.GetUserByUsername(r.Context(), body.Username)
	if err != nil || user == nil {
		// Spend the same scrypt work as a real check so response timing
		// doesn't reveal which usernames exist.
		auth.CheckPassword(body.Password, dummyPasswordHash())
		httputil.SendError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !auth.CheckPassword(body.Password, user.PasswordHash) {
		httputil.SendError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !user.Enabled {
		httputil.SendError(w, http.StatusForbidden, "account disabled")
		return
	}

	// Successful authentication: clear the per-username throttle counter.
	if h.loginLimiter != nil {
		h.loginLimiter.ResetSuccess(ip, body.Username)
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

	// Set HttpOnly cookies for browser clients
	secure := h.cfg.App.AppEnv == "production"
	auth.SetAccessCookie(w, at, h.cfg.Auth.AccessTokenExpiry, secure)
	auth.SetRefreshCookie(w, rt, h.cfg.Auth.RefreshTokenExpiry, secure)

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
	_ = httputil.ParseJSON(r, &body)

	// Accept refresh token from body or cookie
	rt := body.RefreshToken
	if rt == "" {
		rt = auth.GetRefreshTokenFromCookie(r)
	}
	if rt == "" {
		httputil.SendError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	claims, err := auth.ValidateRefreshToken(rt, h.cfg.Auth.JWTSecret)
	if err != nil {
		httputil.SendError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	// Verify token exists in DB (prevents reuse after revocation)
	tokenHash := auth.HashToken(rt)
	existing, err := h.db.GetRefreshToken(r.Context(), tokenHash)
	if err != nil || existing == nil {
		// N4: Refresh-token reuse detection. A valid JWT that's not in
		// the DB means it was already rotated — an attacker is replaying
		// a stolen token. Revoke the entire token family for this user
		// to invalidate the attacker's chain.
		if claims.UserID > 0 {
			_ = h.db.DeleteRefreshTokensByUser(r.Context(), claims.UserID)
		}
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

	// Rotation: atomically claim the old token. If another request already
	// claimed it, this is a replay (or a concurrent refresh) — treat it like
	// reuse and revoke the family rather than forking it.
	claimed, err := h.db.ClaimRefreshToken(r.Context(), tokenHash)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	if !claimed {
		_ = h.db.DeleteRefreshTokensByUser(r.Context(), user.ID)
		httputil.SendError(w, http.StatusUnauthorized, "refresh token revoked")
		return
	}
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

	// Set HttpOnly cookies for browser clients
	secure := h.cfg.App.AppEnv == "production"
	auth.SetAccessCookie(w, at, h.cfg.Auth.AccessTokenExpiry, secure)
	auth.SetRefreshCookie(w, rt, h.cfg.Auth.RefreshTokenExpiry, secure)

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
	// Revoke refresh token from body or cookie
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = httputil.ParseJSON(r, &body)
	rt := body.RefreshToken
	if rt == "" {
		rt = auth.GetRefreshTokenFromCookie(r)
	}
	if rt != "" {
		tokenHash := auth.HashToken(rt)
		_ = h.db.DeleteRefreshToken(r.Context(), tokenHash)
	}
	// Clear cookies
	secure := h.cfg.App.AppEnv == "production"
	auth.ClearRefreshCookie(w, secure)
	if secure {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	auth.ClearAccessCookie(w, secure)
	httputil.SendOK(w, map[string]bool{"loggedOut": true})
}

func (h *AuthHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	keys, err := h.db.GetAPIKeysByUser(r.Context(), claims.UserID)
	if err != nil {
		httputil.SendInternalError(w, err)
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
		httputil.SendInternalError(w, err)
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
		httputil.SendInternalError(w, err)
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
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
		Enabled     *bool  `json:"enabled"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}

	var errs []string
	if msg := httputil.RequiredString(body.Username, "username"); msg != "" {
		errs = append(errs, msg)
	} else if len(body.Username) < 3 || len(body.Username) > 64 {
		errs = append(errs, "username must be 3-64 characters")
	}
	if msg := httputil.RequiredString(body.Password, "password"); msg != "" {
		errs = append(errs, msg)
	} else if len(body.Password) < 8 {
		errs = append(errs, "password must be at least 8 characters")
	}
	if body.Role != "" {
		validRoles := map[string]bool{"viewer": true, "dept_admin": true, "network_admin": true, "admin": true, "super_admin": true}
		if !validRoles[body.Role] {
			errs = append(errs, "role must be one of: viewer, dept_admin, network_admin, admin, super_admin")
		}
	}
	if body.Role != "" {
		if err := canGrantRole(auth.GetClaims(r.Context()), body.Role); err != nil {
			httputil.SendError(w, http.StatusForbidden, err.Error())
			return
		}
	}
	if body.Email != "" && !httputil.IsValidEmail(body.Email) {
		errs = append(errs, "invalid email format")
	}
	if len(body.DisplayName) > 128 {
		errs = append(errs, "displayName must be at most 128 characters")
	}
	if len(errs) > 0 {
		httputil.SendError(w, http.StatusBadRequest, strings.Join(errs, "; "))
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
		httputil.SendInternalError(w, err)
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
	if !outranksOrEqual(claims, user.Role) {
		httputil.SendError(w, http.StatusForbidden, "cannot delete a user with a higher role")
		return
	}
	if err := h.db.DeleteUser(r.Context(), id); err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, map[string]string{"message": "deleted"})
}

// canGrantRole reports whether the caller may assign role to a user: callers
// can only grant roles at or below their own, and only a super_admin can
// create another super_admin.
func canGrantRole(claims *auth.Claims, role string) error {
	if claims == nil {
		return fmt.Errorf("not authenticated")
	}
	if auth.RoleWeight(role) == 0 {
		return fmt.Errorf("unknown role %q", role)
	}
	if auth.RoleWeight(role) > auth.RoleWeight(claims.Role) {
		return fmt.Errorf("cannot grant a role higher than your own")
	}
	return nil
}

func outranksOrEqual(claims *auth.Claims, role string) bool {
	return claims != nil && auth.RoleWeight(claims.Role) >= auth.RoleWeight(role)
}

// GuardUserUpdate protects the generic users update route against privilege
// escalation before delegating to next. It enforces the role hierarchy on the
// target user and on any new role, restricts custom role_id assignment to
// admins, and stops users from changing their own role or enabled flag.
func (h *AuthHandler) GuardUserUpdate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			httputil.SendError(w, http.StatusBadRequest, "invalid id")
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			httputil.SendError(w, http.StatusBadRequest, "invalid body")
			return
		}
		var patch map[string]any
		if err := json.Unmarshal(raw, &patch); err != nil {
			httputil.SendError(w, http.StatusBadRequest, "invalid body")
			return
		}
		fields := map[string]any{}
		for k, v := range patch {
			fields[strings.ReplaceAll(strings.ToLower(k), "_", "")] = v
		}

		claims := auth.GetClaims(r.Context())
		target, err := h.db.GetUserByID(r.Context(), id)
		if err != nil {
			httputil.SendError(w, http.StatusNotFound, "user not found")
			return
		}
		if !outranksOrEqual(claims, target.Role) {
			httputil.SendError(w, http.StatusForbidden, "cannot modify a user with a higher role")
			return
		}
		_, changesRole := fields["role"]
		_, changesRoleID := fields["roleid"]
		_, changesEnabled := fields["enabled"]
		if claims.UserID == id && (changesRole || changesRoleID || changesEnabled) {
			httputil.SendError(w, http.StatusForbidden, "cannot change your own role or enabled status")
			return
		}
		if changesRole {
			role, _ := fields["role"].(string)
			if err := canGrantRole(claims, role); err != nil {
				httputil.SendError(w, http.StatusForbidden, err.Error())
				return
			}
		}
		if changesRoleID && auth.RoleWeight(claims.Role) < auth.RoleWeight("admin") {
			httputil.SendError(w, http.StatusForbidden, "only admins can assign custom roles")
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(raw))
		next(w, r)
	}
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
			"status_page.manage", "sla.manage", "system.monitoring", "system.logs",
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

// clientIP extracts the client IP from a request, accounting for
// X-Forwarded-For when the request is behind a reverse proxy.
// clientIP returns the address used for login throttling. X-Forwarded-For is
// client-controlled, so it is only honoured when the direct peer is a
// reverse proxy on the same host, and then the rightmost entry (the one that
// proxy appended) is used.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil || !peer.IsLoopback() {
		return host
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if last := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(last) != nil {
			return last
		}
	}
	return host
}

var (
	dummyHashOnce sync.Once
	dummyHash     string
)

// dummyPasswordHash returns a fixed scrypt hash used to equalise login timing
// for unknown usernames.
func dummyPasswordHash() string {
	dummyHashOnce.Do(func() {
		dummyHash, _ = auth.HashPassword("netmonitor-timing-equaliser")
	})
	return dummyHash
}
