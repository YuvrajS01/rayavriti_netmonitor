package auth

import (
	"net/http"
	"time"
)

const (
	RefreshCookieName = "nm_refresh_token"
	AccessCookieName  = "nm_access_token"
)

// SetRefreshCookie sets an HttpOnly, Secure, SameSite cookie for the refresh token.
//
//nolint:gosec // Secure is set by caller; gosec cannot verify parameterized values.
func SetRefreshCookie(w http.ResponseWriter, token string, expiry time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    token,
		Path:     "/api/v1/auth",
		MaxAge:   int(expiry.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// SetAccessCookie sets a short-lived cookie for the access token.
// The access token is still sent via Authorization header for API key compat,
// but this cookie provides a fallback for same-origin browser requests.
//
//nolint:gosec // Secure is set by caller; gosec cannot verify parameterized values.
func SetAccessCookie(w http.ResponseWriter, token string, expiry time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(expiry.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearRefreshCookie clears the refresh token cookie.
func ClearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     "/api/v1/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearAccessCookie clears the access token cookie.
func ClearAccessCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// GetRefreshTokenFromCookie extracts the refresh token from the cookie.
func GetRefreshTokenFromCookie(r *http.Request) string {
	c, err := r.Cookie(RefreshCookieName)
	if err != nil || c.Value == "" {
		return ""
	}
	return c.Value
}
