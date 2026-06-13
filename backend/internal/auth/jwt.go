package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func GenerateTokenPair(userID int64, username, role, secret string, accessExpiry, refreshExpiry time.Duration) (accessToken, refreshToken string, err error) {
	now := time.Now()
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(accessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   username,
		},
		UserID:   userID,
		Username: username,
		Role:     role,
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
		},
		UserID:   userID,
		Username: username,
		Role:     role,
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
