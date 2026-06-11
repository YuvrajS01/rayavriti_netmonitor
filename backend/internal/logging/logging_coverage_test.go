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
		App:     config.AppConfig{Version: "1.0.0", NodeEnv: "production"},
		Logging: config.LoggingConfig{Level: "info", Format: "json"},
	}
	logger := New(cfg)
	require.NotNil(t, logger)
	assert.Equal(t, "1.0.0", logger.Version())
}

func TestNew_TextFormat(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		App:     config.AppConfig{Version: "2.0.0", NodeEnv: "development"},
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
		App:     config.AppConfig{Version: "1.0.0"},
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

// ── DBSink ───────────────────────────────────────────────────────────────────

func TestDBSink_Coverage_StartAndWrite(t *testing.T) {
	t.Parallel()
	var received []byte
	sink := NewDBSink(100, "drop_debug", func(data []byte) error {
		received = append(received, data...)
		return nil
	})
	sink.Start()

	_, err := sink.Write([]byte("log line 1\n"))
	require.NoError(t, err)
	_, err = sink.Write([]byte("log line 2\n"))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	sink.Stop()

	assert.Contains(t, string(received), "log line 1")
	assert.Contains(t, string(received), "log line 2")
}

func TestDBSink_Coverage_StopDrainsQueue(t *testing.T) {
	t.Parallel()
	var count int
	sink := NewDBSink(100, "drop_debug", func(data []byte) error {
		count++
		return nil
	})
	sink.Start()

	for i := 0; i < 10; i++ {
		_, _ = sink.Write([]byte("line\n"))
	}

	sink.Stop()
	assert.Equal(t, 10, count)
}

func TestDBSink_Coverage_RecursionSafety(t *testing.T) {
	t.Parallel()
	writeCount := 0
	var sink *DBSink
	sink = NewDBSink(100, "drop_debug", func(data []byte) error {
		writeCount++
		_, _ = sink.Write([]byte("recursive write\n"))
		return nil
	})
	sink.Start()

	_, _ = sink.Write([]byte("initial write\n"))
	time.Sleep(100 * time.Millisecond)
	sink.Stop()

	assert.Equal(t, 1, writeCount)
}

func TestDBSink_Coverage_QueueFullDropOldest(t *testing.T) {
	t.Parallel()
	var count int
	sink := NewDBSink(2, "drop_oldest", func(data []byte) error {
		time.Sleep(50 * time.Millisecond)
		count++
		return nil
	})
	sink.Start()

	for i := 0; i < 10; i++ {
		_, _ = sink.Write([]byte("line\n"))
	}

	time.Sleep(300 * time.Millisecond)
	sink.Stop()
	assert.Greater(t, count, 0)
}

func TestDBSink_Coverage_QueueFullDropDebug(t *testing.T) {
	t.Parallel()
	var count int
	sink := NewDBSink(2, "drop_debug", func(data []byte) error {
		time.Sleep(50 * time.Millisecond)
		count++
		return nil
	})
	sink.Start()

	for i := 0; i < 10; i++ {
		_, _ = sink.Write([]byte("line\n"))
	}

	time.Sleep(300 * time.Millisecond)
	sink.Stop()
	assert.Greater(t, count, 0)
}

func TestDBSink_Coverage_DefaultQueueSize(t *testing.T) {
	t.Parallel()
	sink := NewDBSink(0, "drop_debug", func(data []byte) error { return nil })
	assert.Equal(t, 10000, sink.queueSize)
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

// ── redactPasswords ──────────────────────────────────────────────────────────

func TestRedactPasswords_Coverage_CapitalP(t *testing.T) {
	t.Parallel()
	result := redactPasswords(`{"Password":"hunter2"}`)
	assert.Equal(t, `{"Password":"***"}`, result)
}

func TestRedactPasswords_Coverage_Passwd(t *testing.T) {
	t.Parallel()
	result := redactPasswords(`{"passwd":"hunter2"}`)
	assert.Equal(t, `{"passwd":"***"}`, result)
}

func TestRedactPasswords_Coverage_Secret(t *testing.T) {
	t.Parallel()
	result := redactPasswords(`{"secret":"hunter2"}`)
	assert.Equal(t, `{"secret":"***"}`, result)
}

func TestRedactPasswords_Coverage_NoMatch(t *testing.T) {
	t.Parallel()
	input := `{"username":"admin","email":"test@example.com"}`
	result := redactPasswords(input)
	assert.Equal(t, input, result)
}

func TestRedactPasswords_Coverage_MultipleFields(t *testing.T) {
	t.Parallel()
	input := `{"password":"abc","secret":"def"}`
	result := redactPasswords(input)
	assert.Contains(t, result, `"password":"***"`)
	assert.Contains(t, result, `"secret":"***"`)
}

func TestRedactPasswords_Coverage_Empty(t *testing.T) {
	t.Parallel()
	result := redactPasswords("")
	assert.Equal(t, "", result)
}

func TestRedactPasswords_Coverage_Malformed(t *testing.T) {
	t.Parallel()
	result := redactPasswords(`passwordnoquotes`)
	assert.Equal(t, `passwordnoquotes`, result)
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
