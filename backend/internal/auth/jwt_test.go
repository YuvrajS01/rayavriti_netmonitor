package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSecret   = "test-secret-key-for-jwt"
	testUserID   = int64(42)
	testUsername = "admin"
	testRole     = "admin"
)

func TestGenerateTokenPair_HappyPath(t *testing.T) {
	t.Parallel()
	accessToken, refreshToken, err := GenerateTokenPair(testUserID, testUsername, testRole, testSecret, 15*time.Minute, 7*24*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.NotEqual(t, accessToken, refreshToken)

	accessClaims, err := ValidateToken(accessToken, testSecret)
	require.NoError(t, err)
	assert.Equal(t, testUserID, accessClaims.UserID)
	assert.Equal(t, testUsername, accessClaims.Username)
	assert.Equal(t, testRole, accessClaims.Role)
	assert.Equal(t, testUsername, accessClaims.Subject)

	refreshClaims, err := ValidateToken(refreshToken, testSecret)
	require.NoError(t, err)
	assert.Equal(t, testUserID, refreshClaims.UserID)
	assert.Equal(t, testUsername, refreshClaims.Username)
	assert.Equal(t, testRole, refreshClaims.Role)
}

func TestGenerateTokenPair_EmptySecret(t *testing.T) {
	t.Parallel()
	// Empty secret is technically valid for HS256 (it's just a bad practice).
	// Verify the token pair is generated successfully but ValidateToken with
	// a non-empty secret will fail.
	accessToken, _, err := GenerateTokenPair(testUserID, testUsername, testRole, "", 15*time.Minute, 7*24*time.Hour)
	require.NoError(t, err)
	// Token signed with empty secret should fail validation with a real secret
	_, err = ValidateToken(accessToken, "real-secret")
	require.Error(t, err)
}

func TestValidateToken_ValidToken(t *testing.T) {
	t.Parallel()
	accessToken, _, err := GenerateTokenPair(testUserID, testUsername, testRole, testSecret, 15*time.Minute, 7*24*time.Hour)
	require.NoError(t, err)

	claims, err := ValidateToken(accessToken, testSecret)
	require.NoError(t, err)
	assert.Equal(t, testUserID, claims.UserID)
	assert.Equal(t, testUsername, claims.Username)
	assert.Equal(t, testRole, claims.Role)
	assert.Equal(t, testUsername, claims.Subject)
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	t.Parallel()
	accessToken, _, err := GenerateTokenPair(testUserID, testUsername, testRole, testSecret, -1*time.Hour, 7*24*time.Hour)
	require.NoError(t, err)

	_, err = ValidateToken(accessToken, testSecret)
	require.Error(t, err)
}

func TestValidateToken_WrongSecret(t *testing.T) {
	t.Parallel()
	accessToken, _, err := GenerateTokenPair(testUserID, testUsername, testRole, testSecret, 15*time.Minute, 7*24*time.Hour)
	require.NoError(t, err)

	_, err = ValidateToken(accessToken, "wrong-secret")
	require.Error(t, err)
}

func TestValidateToken_MalformedToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		token string
	}{
		{"not a jwt", "not.a.jwt"},
		{"empty string", ""},
		{"random string", "eyJhbGciOiJSUzI1NiJ9.eyJ1aWQiOjQyLCJ1c2VybmFtZSI6ImFkbWluIn0.signature"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ValidateToken(tt.token, testSecret)
			require.Error(t, err)
		})
	}
}

func TestValidateToken_TamperedPayload(t *testing.T) {
	t.Parallel()
	accessToken, _, err := GenerateTokenPair(testUserID, testUsername, testRole, testSecret, 15*time.Minute, 7*24*time.Hour)
	require.NoError(t, err)

	parts := splitJWT(accessToken)
	require.Len(t, parts, 3)
	parts[1] = "dGFtcGVyZWQ"
	tampered := joinJWT(parts)

	_, err = ValidateToken(tampered, testSecret)
	require.Error(t, err)
}

func TestValidateToken_NoneAlgorithm(t *testing.T) {
	t.Parallel()
	token := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
		UserID:   testUserID,
		Username: testUsername,
		Role:     testRole,
	})
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = ValidateToken(tokenString, testSecret)
	require.Error(t, err)
}

func TestClaims_MissingFields(t *testing.T) {
	t.Parallel()
	accessToken, _, err := GenerateTokenPair(0, "", "", testSecret, 15*time.Minute, 7*24*time.Hour)
	require.NoError(t, err)

	claims, err := ValidateToken(accessToken, testSecret)
	require.NoError(t, err)
	assert.Equal(t, int64(0), claims.UserID)
	assert.Equal(t, "", claims.Username)
	assert.Equal(t, "", claims.Role)
}

func splitJWT(token string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	parts = append(parts, token[start:])
	return parts
}

func joinJWT(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "."
		}
		result += p
	}
	return result
}
