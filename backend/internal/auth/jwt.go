package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Token types carried in the "typ" claim. Access and refresh tokens are signed
// with the same secret, so the type must be checked to stop a long-lived
// refresh token from being used as a bearer token.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID      int64    `json:"uid"`
	Username    string   `json:"username"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions,omitempty"`
	TokenType   string   `json:"typ,omitempty"`
}

func GenerateTokenPair(userID int64, username, role, secret string, accessExpiry, refreshExpiry time.Duration) (accessToken, refreshToken string, err error) {
	return GenerateTokenPairWithPerms(userID, username, role, nil, secret, accessExpiry, refreshExpiry)
}

func GenerateTokenPairWithPerms(userID int64, username, role string, permissions []string, secret string, accessExpiry, refreshExpiry time.Duration) (accessToken, refreshToken string, err error) {
	now := time.Now()
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(accessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   username,
			ID:        uuid.NewString(),
		},
		UserID:      userID,
		Username:    username,
		Role:        role,
		Permissions: permissions,
		TokenType:   TokenTypeAccess,
	})
	accessToken, err = access.SignedString([]byte(secret))
	if err != nil {
		return
	}
	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   username,
			ID:        uuid.NewString(),
		},
		UserID:      userID,
		Username:    username,
		Role:        role,
		Permissions: permissions,
		TokenType:   TokenTypeRefresh,
	})
	refreshToken, err = refresh.SignedString([]byte(secret))
	return
}

func ValidateToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

// ValidateAccessToken validates a token and requires it to be an access token.
func ValidateAccessToken(tokenString, secret string) (*Claims, error) {
	return validateTyped(tokenString, secret, TokenTypeAccess)
}

// ValidateRefreshToken validates a token and requires it to be a refresh token.
func ValidateRefreshToken(tokenString, secret string) (*Claims, error) {
	return validateTyped(tokenString, secret, TokenTypeRefresh)
}

func validateTyped(tokenString, secret, typ string) (*Claims, error) {
	claims, err := ValidateToken(tokenString, secret)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != typ {
		return nil, fmt.Errorf("invalid token type")
	}
	return claims, nil
}
