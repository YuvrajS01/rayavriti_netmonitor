package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type AuthHandler struct {
	db  database.Database
	cfg *config.Config
}

func NewAuthHandler(db database.Database, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
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
	at, rt, err := auth.GenerateTokenPair(user.ID, user.Username, user.Role,
		h.cfg.Auth.JWTSecret, h.cfg.Auth.AccessTokenExpiry, h.cfg.Auth.RefreshTokenExpiry)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	httputil.SendOK(w, map[string]any{
		"accessToken":  at,
		"refreshToken": rt,
		"user": map[string]any{
			"id": user.ID, "username": user.Username, "role": user.Role,
		},
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var body struct{ RefreshToken string `json:"refreshToken"` }
	_ = httputil.ParseJSON(r, &body)
	if body.RefreshToken != "" {
		auth.DeleteSession(body.RefreshToken)
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
	var body struct{ RefreshToken string `json:"refreshToken"` }
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	claims, err := auth.ValidateToken(body.RefreshToken, h.cfg.Auth.JWTSecret)
	if err != nil {
		httputil.SendError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	at, rt, err := auth.GenerateTokenPair(claims.UserID, claims.Username, claims.Role,
		h.cfg.Auth.JWTSecret, h.cfg.Auth.AccessTokenExpiry, h.cfg.Auth.RefreshTokenExpiry)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	httputil.SendOK(w, map[string]string{"accessToken": at, "refreshToken": rt})
}

func (h *AuthHandler) V1Login(w http.ResponseWriter, r *http.Request) {
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
	at, rt, err := auth.GenerateTokenPair(user.ID, user.Username, user.Role,
		h.cfg.Auth.JWTSecret, h.cfg.Auth.AccessTokenExpiry, h.cfg.Auth.RefreshTokenExpiry)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	expiresIn := int(h.cfg.Auth.AccessTokenExpiry.Seconds())
	httputil.SendOK(w, map[string]any{
		"accessToken":  at,
		"refreshToken": rt,
		"expiresIn":    expiresIn,
		"user": map[string]any{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

func (h *AuthHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	httputil.SendOK(w, map[string]bool{"verified": true})
}

func (h *AuthHandler) V1Logout(w http.ResponseWriter, r *http.Request) {
	var body struct{ RefreshToken string `json:"refreshToken"` }
	_ = httputil.ParseJSON(r, &body)
	if body.RefreshToken != "" {
		auth.DeleteSession(body.RefreshToken)
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
	var body struct{ Description string `json:"description"` }
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
