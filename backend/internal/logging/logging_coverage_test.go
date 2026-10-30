package logging

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── New with JSON format ──────────────────────────────────────────────────────

func TestNew_JsonFormat(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		App:     config.AppConfig{Version: "1.0.0", AppEnv: "production"},
		Logging: config.LoggingConfig{Level: "info", Format: "json"},
	}
	logger := New(cfg)
	require.NotNil(t, logger)
	assert.Equal(t, "1.0.0", logger.Version())
}

func TestNew_TextFormat(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		App:     config.AppConfig{Version: "2.0.0", AppEnv: "development"},
		Logging: config.LoggingConfig{Level: "debug", Format: "text"},
	}
	logger := New(cfg)
	require.NotNil(t, logger)
}

// ── New with file logging ─────────────────────────────────────────────────────

func TestNew_FileLogging(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := &config.Config{
		App: config.AppConfig{Version: "1.0.0"},
		Logging: config.LoggingConfig{
			Level:          "info",
			Format:         "text",
			FileEnabled:    true,
			FilePath:       logFile,
			FileMaxSizeMB:  1,
			FileMaxBackups: 1,
			FileMaxAgeDays: 1,
		},
	}
	logger := New(cfg)
	require.NotNil(t, logger)
	logger.Info("test log message")

	_, err := os.Stat(logFile)
	assert.NoError(t, err)
}

// ── NewRotatingWriter ─────────────────────────────────────────────────────────

func TestNewRotatingWriter_FullConfig(t *testing.T) {
	t.Parallel()
	w := NewRotatingWriter(RotationConfig{
		Filename:   "/tmp/test-rotation.log",
		MaxSizeMB:  200,
		MaxBackups: 20,
		MaxAgeDays: 60,
		Compress:   true,
	})
	require.NotNil(t, w)
	err := w.Close()
	assert.NoError(t, err)
}

func TestNewRotatingWriter_ZeroValues(t *testing.T) {
	t.Parallel()
	w := NewRotatingWriter(RotationConfig{
		Filename: "/tmp/test-zero.log",
	})
	require.NotNil(t, w)
	err := w.Close()
	assert.NoError(t, err)
}

// ── MultiSink ────────────────────────────────────────────────────────────────

func TestMultiSink_AllWritersSucceed(t *testing.T) {
	t.Parallel()
	var buf1, buf2, buf3 bytes.Buffer
	sink := NewMultiSink(&buf1, &buf2, &buf3)

	n, err := sink.Write([]byte("hello world"))
	require.NoError(t, err)
	assert.Equal(t, 11, n)
	assert.Equal(t, "hello world", buf1.String())
	assert.Equal(t, "hello world", buf2.String())
	assert.Equal(t, "hello world", buf3.String())
}

func TestMultiSink_FirstWriterFails(t *testing.T) {
	t.Parallel()
	fw := &failingWriter{err: io.ErrShortWrite}
	var buf bytes.Buffer
	sink := NewMultiSink(fw, &buf)

	_, err := sink.Write([]byte("test"))
	assert.Error(t, err)
}

type failingWriter struct {
	err error
}

func (f *failingWriter) Write(p []byte) (int, error) {
	return 0, f.err
}

// ── RequestLogger middleware ──────────────────────────────────────────────────

func TestRequestLogger_Coverage_SlowRequest(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	middleware := RequestLogger(logger, 10) // 10ms threshold

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestRequestLogger_Coverage_4xxResponse(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	middleware := RequestLogger(logger, 10000)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("GET", "/notfound", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRequestLogger_Coverage_5xxResponse(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	middleware := RequestLogger(logger, 10000)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest("GET", "/error", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRequestLogger_Coverage_WithBody(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	middleware := RequestLogger(logger, 10000)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := strings.NewReader(`{"username":"admin","password":"secret123"}`)
	req := httptest.NewRequest("POST", "/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(body.Len())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestLogger_Coverage_WithAPIKey(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	middleware := RequestLogger(logger, 10000)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("X-Api-Key", "test-key-123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestLogger_Coverage_WithBearerToken(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	middleware := RequestLogger(logger, 10000)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestLogger_Coverage_WithXFF(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	middleware := RequestLogger(logger, 10000)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestLogger_Coverage_WithQueryParams(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	middleware := RequestLogger(logger, 10000)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/data?foo=bar&baz=1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestLogger_Coverage_WithReferer(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	middleware := RequestLogger(logger, 10000)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Referer", "http://example.com/page")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── responseWriter.Flush and Hijack ──────────────────────────────────────────

func TestResponseWriter_Coverage_Flush(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, status: 200}

	rw.Flush()
}

type hijackableResponseWriter struct {
	http.ResponseWriter
	hijacked bool
}

func (h *hijackableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, nil
}

func TestResponseWriter_Coverage_HijackSupported(t *testing.T) {
	t.Parallel()
	inner := &hijackableResponseWriter{ResponseWriter: httptest.NewRecorder()}
	rw := &responseWriter{ResponseWriter: inner}

	_, _, err := rw.Hijack()
	require.NoError(t, err)
	assert.True(t, inner.hijacked)
}

func TestResponseWriter_Coverage_HijackNotSupported(t *testing.T) {
	t.Parallel()
	inner := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: inner}

	_, _, err := rw.Hijack()
	assert.Error(t, err)
}

func TestResponseWriter_Coverage_Unwrap(t *testing.T) {
	t.Parallel()
	inner := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: inner, status: 200}

	unwrapped := rw.Unwrap()
	assert.Equal(t, inner, unwrapped)
}

func TestResponseWriter_Coverage_DoubleWriteHeader(t *testing.T) {
	t.Parallel()
	inner := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: inner, status: 200}

	rw.WriteHeader(http.StatusNotFound)
	rw.WriteHeader(http.StatusOK)

	assert.Equal(t, http.StatusNotFound, inner.Code)
}

func TestResponseWriter_Coverage_WriteAutoHeader(t *testing.T) {
	t.Parallel()
	inner := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: inner}

	n, err := rw.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, http.StatusOK, inner.Code)
}

// ── StdoutSink, StderrSink ──────────────────────────────────────────────────

func TestStdoutSink_Coverage(t *testing.T) {
	t.Parallel()
	w := StdoutSink()
	assert.Equal(t, os.Stdout, w)
}

func TestStderrSink_Coverage(t *testing.T) {
	t.Parallel()
	w := StderrSink()
	assert.Equal(t, os.Stderr, w)
}
