package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// --- Login ---

func TestLogin_HappyPath(t *testing.T) {
	passwordHash, _ := auth.HashPassword(testPassword)
	db := &mockDB{
		getUserByUsernameFn: func(ctx context.Context, username string) (*models.User, error) {
			if username == testUsername {
				return &models.User{ID: testUserID, Username: testUsername, PasswordHash: passwordHash, Role: testUserRole, Enabled: true}, nil
			}
			return nil, errors.New("not found")
		},
	}
	h := NewAuthHandler(db, testConfig())
	body, _ := json.Marshal(map[string]string{"username": testUsername, "password": testPassword})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	h.Login(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp["success"].(bool) {
		t.Fatal("expected success=true")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	passwordHash, _ := auth.HashPassword("correct")
	db := &mockDB{
		getUserByUsernameFn: func(ctx context.Context, username string) (*models.User, error) {
			return &models.User{ID: testUserID, PasswordHash: passwordHash, Enabled: true}, nil
		},
	}
	h := NewAuthHandler(db, testConfig())
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrong"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	h.Login(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLogin_UnknownUser(t *testing.T) {
	db := &mockDB{
		getUserByUsernameFn: func(ctx context.Context, username string) (*models.User, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewAuthHandler(db, testConfig())
	body, _ := json.Marshal(map[string]string{"username": "unknown", "password": "pass"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	h.Login(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLogin_DisabledUser(t *testing.T) {
	passwordHash, _ := auth.HashPassword(testPassword)
	db := &mockDB{
		getUserByUsernameFn: func(ctx context.Context, username string) (*models.User, error) {
			return &models.User{ID: testUserID, PasswordHash: passwordHash, Enabled: false}, nil
		},
	}
	h := NewAuthHandler(db, testConfig())
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": testPassword})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	h.Login(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestLogin_EmptyBody(t *testing.T) {
	db := &mockDB{
		getUserByUsernameFn: func(ctx context.Context, username string) (*models.User, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewAuthHandler(db, testConfig())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/login", nil)
	r.Header.Set("Content-Type", "application/json")
	h.Login(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- Logout ---

func TestLogout(t *testing.T) {
	db := &mockDB{}
	h := NewAuthHandler(db, testConfig())
	body, _ := json.Marshal(map[string]string{"refreshToken": "some-token"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/logout", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	h.Logout(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- Me ---

func TestMe_Authenticated(t *testing.T) {
	db := &mockDB{
		getUserByIDFn: func(ctx context.Context, id int64) (*models.User, error) {
			if id == testUserID {
				return &models.User{ID: testUserID, Username: testUsername, Role: testUserRole}, nil
			}
			return nil, errors.New("not found")
		},
	}
	h := NewAuthHandler(db, testConfig())
	w, req := authenticatedRequest("GET", "/api/auth/me", "")
	callWithAuth(h.Me, w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMe_Unauthenticated(t *testing.T) {
	db := &mockDB{}
	h := NewAuthHandler(db, testConfig())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/auth/me", nil)
	h.Me(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMe_UserNotFound(t *testing.T) {
	db := &mockDB{
		getUserByIDFn: func(ctx context.Context, id int64) (*models.User, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewAuthHandler(db, testConfig())
	w, req := authenticatedRequest("GET", "/api/auth/me", "")
	callWithAuth(h.Me, w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- Refresh ---

func TestRefresh_Valid(t *testing.T) {
	db := &mockDB{
		getRefreshTokenFn: func(ctx context.Context, tokenHash string) (*database.RefreshToken, error) {
			return &database.RefreshToken{TokenHash: tokenHash, UserID: testUserID}, nil
		},
		getUserByIDFn: func(ctx context.Context, id int64) (*models.User, error) {
			return &models.User{ID: id, Username: testUsername, Role: testUserRole, Enabled: true}, nil
		},
	}
	h := NewAuthHandler(db, testConfig())
	refreshToken, _, _ := auth.GenerateTokenPair(testUserID, testUsername, testUserRole, testJWTSecret, 15*time.Minute, 7*24*time.Hour)
	body, _ := json.Marshal(map[string]string{"refreshToken": refreshToken})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/refresh", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	h.Refresh(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRefresh_Invalid(t *testing.T) {
	db := &mockDB{}
	h := NewAuthHandler(db, testConfig())
	body, _ := json.Marshal(map[string]string{"refreshToken": "invalid-token"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/refresh", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	h.Refresh(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRefresh_EmptyBody(t *testing.T) {
	db := &mockDB{}
	h := NewAuthHandler(db, testConfig())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/refresh", nil)
	r.Header.Set("Content-Type", "application/json")
	h.Refresh(w, r)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 400 or 401, got %d", w.Code)
	}
}

// --- V1Login ---

func TestV1Login_HappyPath(t *testing.T) {
	passwordHash, _ := auth.HashPassword(testPassword)
	db := &mockDB{
		getUserByUsernameFn: func(ctx context.Context, username string) (*models.User, error) {
			return &models.User{ID: testUserID, Username: testUsername, PasswordHash: passwordHash, Role: testUserRole, Enabled: true}, nil
		},
	}
	h := NewAuthHandler(db, testConfig())
	body, _ := json.Marshal(map[string]string{"username": testUsername, "password": testPassword})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	h.V1Login(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if _, ok := data["expiresIn"]; !ok {
		t.Fatal("V1Login response should include expiresIn")
	}
}

func TestV1Login_WrongPassword(t *testing.T) {
	passwordHash, _ := auth.HashPassword("correct")
	db := &mockDB{
		getUserByUsernameFn: func(ctx context.Context, username string) (*models.User, error) {
			return &models.User{ID: 1, PasswordHash: passwordHash, Enabled: true}, nil
		},
	}
	h := NewAuthHandler(db, testConfig())
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrong"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	h.V1Login(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestV1Login_Disabled(t *testing.T) {
	passwordHash, _ := auth.HashPassword(testPassword)
	db := &mockDB{
		getUserByUsernameFn: func(ctx context.Context, username string) (*models.User, error) {
			return &models.User{ID: 1, PasswordHash: passwordHash, Enabled: false}, nil
		},
	}
	h := NewAuthHandler(db, testConfig())
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": testPassword})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	h.V1Login(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// --- Verify2FA ---

func TestVerify2FA(t *testing.T) {
	db := &mockDB{}
	h := NewAuthHandler(db, testConfig())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/verify-2fa", nil)
	h.Verify2FA(w, r)
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}
}

// --- V1Logout ---

func TestV1Logout(t *testing.T) {
	db := &mockDB{}
	h := NewAuthHandler(db, testConfig())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	h.V1Logout(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- ListAPIKeys ---

func TestListAPIKeys(t *testing.T) {
	db := &mockDB{
		getAPIKeysByUserFn: func(ctx context.Context, userID int64) ([]models.APIKey, error) {
			return []models.APIKey{
				{ID: 1, UserID: userID, Description: "key1"},
				{ID: 2, UserID: userID, Description: "key2"},
			}, nil
		},
	}
	h := NewAuthHandler(db, testConfig())
	w, req := authenticatedRequest("GET", "/api/auth/apikeys", "")
	callWithAuth(h.ListAPIKeys, w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListAPIKeys_Error(t *testing.T) {
	db := &mockDB{
		getAPIKeysByUserFn: func(ctx context.Context, userID int64) ([]models.APIKey, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewAuthHandler(db, testConfig())
	w, req := authenticatedRequest("GET", "/api/auth/apikeys", "")
	callWithAuth(h.ListAPIKeys, w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- CreateAPIKey ---

func TestCreateAPIKey(t *testing.T) {
	db := &mockDB{
		createAPIKeyFn: func(ctx context.Context, k *models.APIKey) (*models.APIKey, error) {
			k.ID = 42
			return k, nil
		},
	}
	h := NewAuthHandler(db, testConfig())
	body, _ := json.Marshal(map[string]string{"description": "test key"})
	w, req := authenticatedRequest("POST", "/api/auth/apikeys", string(body))
	callWithAuth(h.CreateAPIKey, w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateAPIKey_DBError(t *testing.T) {
	db := &mockDB{
		createAPIKeyFn: func(ctx context.Context, k *models.APIKey) (*models.APIKey, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewAuthHandler(db, testConfig())
	w, req := authenticatedRequest("POST", "/api/auth/apikeys", `{"description":"k"}`)
	callWithAuth(h.CreateAPIKey, w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- DeleteAPIKey ---

func TestDeleteAPIKey(t *testing.T) {
	db := &mockDB{
		deleteAPIKeyFn: func(ctx context.Context, id int64) error { return nil },
		getAPIKeyByIDFn: func(ctx context.Context, id int64) (*models.APIKey, error) {
			return &models.APIKey{ID: id, UserID: testUserID}, nil
		},
	}
	h := NewAuthHandler(db, testConfig())
	w, req := authenticatedRequest("DELETE", "/api/auth/apikeys/1", "")
	req = withChiParams(req, "id", "1")
	callWithAuth(h.DeleteAPIKey, w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeleteAPIKey_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewAuthHandler(db, testConfig())
	w, req := authenticatedRequest("DELETE", "/api/auth/apikeys/abc", "")
	req = withChiParams(req, "id", "abc")
	callWithAuth(h.DeleteAPIKey, w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- parseID ---

func TestParseID(t *testing.T) {
	id, err := parseID("42")
	if err != nil || id != 42 {
		t.Fatalf("expected 42, got %d, err: %v", id, err)
	}
	_, err = parseID("abc")
	if err == nil {
		t.Fatal("expected error for non-numeric id")
	}
}
