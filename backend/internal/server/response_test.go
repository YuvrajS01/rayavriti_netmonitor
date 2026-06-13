package server

import (
	"bytes"
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

	rec := httptest.NewRecorder()
	SendOK(rec, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "value", resp.Data["key"])
}

func TestSendOK_NilData(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	SendOK(rec, nil)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool `json:"success"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestSendCreated(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	SendCreated(rec, map[string]int{"id": 42})

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, float64(42), resp.Data["id"])
}

func TestSendError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	SendError(rec, http.StatusBadRequest, "invalid input")

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "BAD_REQUEST", resp.Error.Code)
	assert.Equal(t, "invalid input", resp.Error.Message)
}

func TestSendError_InternalError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	SendError(rec, http.StatusInternalServerError, "something broke")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "INTERNAL_ERROR", resp.Error.Code)
}

func TestSendError_NotFound(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	SendError(rec, http.StatusNotFound, "not found")

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "NOT_FOUND", resp.Error.Code)
}

func TestSendError_TooManyRequests(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	SendError(rec, http.StatusTooManyRequests, "slow down")

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "RATE_LIMITED", resp.Error.Code)
}

func TestParseJSON(t *testing.T) {
	t.Parallel()

	body := `{"name":"test","port":80}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var target struct {
		Name string `json:"name"`
		Port int    `json:"port"`
	}
	err := ParseJSON(req, &target)
	require.NoError(t, err)
	assert.Equal(t, "test", target.Name)
	assert.Equal(t, 80, target.Port)
}

func TestParseJSON_EmptyBody(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.ContentLength = 0

	var target struct {
		Name string `json:"name"`
	}
	err := ParseJSON(req, &target)
	require.NoError(t, err)
	assert.Empty(t, target.Name)
}

func TestParseJSON_InvalidJSON(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json"))

	var target struct {
		Name string `json:"name"`
	}
	err := ParseJSON(req, &target)
	assert.Error(t, err)
}

func TestSendOK_StringSlice(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	SendOK(rec, []string{"a", "b", "c"})

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data []string `json:"data"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, resp.Data)
}

func TestSendOK_MultipleFields(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	SendOK(rec, map[string]any{
		"count":   5,
		"results": []int{1, 2, 3},
	})

	var resp struct {
		Data struct {
			Count   float64 `json:"count"`
			Results []int   `json:"results"`
		} `json:"data"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, float64(5), resp.Data.Count)
	assert.Equal(t, []int{1, 2, 3}, resp.Data.Results)
}

func TestSendError_Unauthorized(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	SendError(rec, http.StatusUnauthorized, "auth required")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "UNAUTHORIZED", resp.Error.Code)
}

func TestSendError_Forbidden(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	SendError(rec, http.StatusForbidden, "access denied")

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "FORBIDDEN", resp.Error.Code)
}

func TestParseJSON_LargePayload(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	buf.WriteString(`{"values":[`)
	for i := 0; i < 100; i++ {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString("1")
	}
	buf.WriteString(`]}`)

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	var target struct {
		Values []int `json:"values"`
	}
	err := ParseJSON(req, &target)
	require.NoError(t, err)
	assert.Len(t, target.Values, 100)
}

func TestSendError_Conflict(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	SendError(rec, http.StatusConflict, "already exists")

	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "CONFLICT", resp.Error.Code)
}

func TestResponseContentTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		call   func(w http.ResponseWriter)
		status int
	}{
		{"SendOK", func(w http.ResponseWriter) { SendOK(w, nil) }, http.StatusOK},
		{"SendCreated", func(w http.ResponseWriter) { SendCreated(w, nil) }, http.StatusCreated},
		{"SendError", func(w http.ResponseWriter) { SendError(w, http.StatusBadRequest, "err") }, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			tt.call(rec)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		})
	}
}
