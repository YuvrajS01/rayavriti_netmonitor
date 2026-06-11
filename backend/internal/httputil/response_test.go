package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendOK(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	SendOK(w, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp Response
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data)
}

func TestSendError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	SendError(w, http.StatusBadRequest, "invalid input")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp Response
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.False(t, resp.Success)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "BAD_REQUEST", resp.Error.Code)
	assert.Equal(t, "invalid input", resp.Error.Message)
}

func TestSendCreated(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	SendCreated(w, map[string]int64{"id": 1})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp Response
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.True(t, resp.Success)
}

func TestSendOKWithMeta(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	SendOKWithMeta(w, []string{"a", "b"}, &ResponseMeta{Page: 1, PageSize: 10, Total: 2})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Meta)
	assert.Equal(t, 1, resp.Meta.Page)
}

func TestSendErrorWithCode(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	SendErrorWithCode(w, http.StatusConflict, "DUPLICATE", "already exists")

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp Response
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "DUPLICATE", resp.Error.Code)
}

func TestParseJSON_ValidBody(t *testing.T) {
	t.Parallel()
	body := `{"name":"test","value":42}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var v map[string]any
	err := ParseJSON(req, &v)
	require.NoError(t, err)
	assert.Equal(t, "test", v["name"])
	assert.Equal(t, float64(42), v["value"])
}

func TestParseJSON_InvalidJSON(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("POST", "/", strings.NewReader("not json"))

	var v map[string]any
	err := ParseJSON(req, &v)
	require.Error(t, err)
}

func TestParseJSON_EmptyBody(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("POST", "/", nil)
	req.ContentLength = 0

	var v map[string]any
	err := ParseJSON(req, &v)
	require.NoError(t, err)
}

func TestHttpStatusToCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status   int
		expected string
	}{
		{http.StatusBadRequest, "BAD_REQUEST"},
		{http.StatusUnauthorized, "UNAUTHORIZED"},
		{http.StatusForbidden, "FORBIDDEN"},
		{http.StatusNotFound, "NOT_FOUND"},
		{http.StatusConflict, "CONFLICT"},
		{http.StatusTooManyRequests, "RATE_LIMITED"},
		{http.StatusInternalServerError, "INTERNAL_ERROR"},
		{http.StatusOK, "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, httpStatusToCode(tt.status))
		})
	}
}
