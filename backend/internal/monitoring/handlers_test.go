package monitoring

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMonitoringHandler_NilSource(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil)
	require.NotNil(t, handler)
}

func TestNewMonitoringHandler_Store(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(&Store{})
	require.NotNil(t, handler)
	require.NotNil(t, handler.store)
}

func TestParseIntQuery_Default(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Equal(t, 100, parseIntQuery(req, "limit", 100))
}

func TestParseIntQuery_Valid(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/?limit=50", nil)
	assert.Equal(t, 50, parseIntQuery(req, "limit", 100))
}

func TestParseIntQuery_Invalid(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/?limit=abc", nil)
	assert.Equal(t, 100, parseIntQuery(req, "limit", 100))
}

func TestSystemLogs_NoStore(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/logs", nil)
	rec := httptest.NewRecorder()
	handler.SystemLogs(rec, req)
	assert.Equal(t, http.StatusNotImplemented, rec.Code)
}

func TestSystemLogsStats_NoStore(t *testing.T) {
	t.Parallel()
	handler := NewMonitoringHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/logs/stats", nil)
	rec := httptest.NewRecorder()
	handler.SystemLogsStats(rec, req)
	assert.Equal(t, http.StatusNotImplemented, rec.Code)
}
