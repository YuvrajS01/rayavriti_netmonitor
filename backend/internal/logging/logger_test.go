package logging

import (
	"context"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() *config.Config {
	return &config.Config{
		App: config.AppConfig{Version: "1.1.0", AppEnv: "development"},
		Logging: config.LoggingConfig{Level: "debug", Format: "pretty", FileEnabled: false},
	}
}

func TestWithRequestID_GetRequestID(t *testing.T) {
	t.Parallel()
	ctx := WithRequestID(context.Background(), "req-123")
	assert.Equal(t, "req-123", GetRequestID(ctx))
}

func TestWithUserID_GetUserID(t *testing.T) {
	t.Parallel()
	ctx := WithUserID(context.Background(), "user-456")
	assert.Equal(t, "user-456", GetUserID(ctx))
}

func TestWithTraceID_GetTraceID(t *testing.T) {
	t.Parallel()
	ctx := WithTraceID(context.Background(), "trace-789")
	assert.Equal(t, "trace-789", GetTraceID(ctx))
}

func TestGet_EmptyContext(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", GetRequestID(context.Background()))
	assert.Equal(t, "", GetUserID(context.Background()))
	assert.Equal(t, "", GetTraceID(context.Background()))
}

func TestGenerateRequestID(t *testing.T) {
	t.Parallel()
	id := GenerateRequestID()
	assert.Len(t, id, 16)
	for _, c := range id {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'), "not hex: %c", c)
	}
}

func TestGenerateTraceID(t *testing.T) {
	t.Parallel()
	id := GenerateTraceID("collect")
	require.NotEmpty(t, id)
	assert.Contains(t, id, "collect-")

	id2 := GenerateTraceID("")
	assert.Contains(t, id2, "tr-")
}

func TestParseLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected int
	}{
		{"trace", int(LevelTrace)},
		{"debug", -4},
		{"info", 0},
		{"warn", 4},
		{"error", 8},
		{"unknown", 0},
		{"", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			level := parseLevel(tt.input)
			assert.Equal(t, tt.expected, int(level))
		})
	}
}

func TestLogger_With(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	logger := New(cfg)
	child := logger.With("http")
	require.NotNil(t, child)
	assert.Equal(t, "http", child.component)
	assert.Equal(t, logger.hostname, child.hostname)
	assert.Equal(t, logger.pid, child.pid)
	assert.Equal(t, logger.version, child.version)
}

func TestLogger_Hostname(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	logger := New(cfg)
	assert.NotEmpty(t, logger.Hostname())
}

func TestLogger_PID(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	logger := New(cfg)
	assert.Greater(t, logger.PID(), 0)
}

func TestLogger_Version(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	logger := New(cfg)
	assert.Equal(t, "1.1.0", logger.Version())
}

func TestLogger_LogLevels(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	logger := New(cfg)
	logger.Trace("trace message")
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")
}

func TestLogger_ContextEnrichment(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	logger := New(cfg)
	ctx := WithRequestID(context.Background(), "req-123")
	ctx = WithTraceID(ctx, "trace-456")
	ctx = WithUserID(ctx, "user-789")
	logger.InfoCtx(ctx, "context enriched message")
}
