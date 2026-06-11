package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearEnv() {
	keys := []string{
		"JWT_SECRET", "PORT", "NODE_ENV", "VERSION", "DATABASE_URL", "DATABASE_DSN",
		"DB_MAX_CONNS", "DB_MIN_CONNS", "DB_MAX_CONN_LIFETIME", "DB_HEALTH_CHECK_PERIOD",
		"ADMIN_USERNAME", "ADMIN_PASSWORD", "ACCESS_TOKEN_EXPIRY", "REFRESH_TOKEN_EXPIRY",
		"NETFLOW_PORT", "METRICS_RETENTION_DAYS", "FLOW_RETENTION_DAYS", "ALERTS_RETENTION_DAYS",
		"PORT_DISCOVERY_ENABLED", "CAPTURE_ENABLED", "COLLECTOR_INTERVAL_SEC",
		"LOG_LEVEL", "LOG_FORMAT", "LOG_FILE_ENABLED", "LOG_FILE_PATH",
		"LOG_FILE_MAX_SIZE_MB", "LOG_FILE_MAX_BACKUPS", "LOG_FILE_MAX_AGE_DAYS", "LOG_FILE_COMPRESS",
		"LOG_DB_ENABLED", "LOG_DB_SAMPLE_RATE", "LOG_DB_QUEUE_SIZE", "LOG_DB_DROP_POLICY",
		"LOG_MODULE_LEVELS", "LOG_SLOW_QUERY_MS", "LOG_SLOW_REQUEST_MS",
		"TELEGRAM_TOKEN", "TELEGRAM_CHAT_ID", "TWILIO_SID", "TWILIO_TOKEN", "TWILIO_FROM",
		"STATUS_PAGE_ENABLED", "CORS_ORIGINS",
	}
	for _, k := range keys {
		os.Unsetenv(k)
	}
}

// NOTE: Config tests that modify env vars cannot run in parallel
// because os.Setenv/os.Unsetenv affects the entire process.

func TestLoad_RequiresJWTSecret(t *testing.T) {
	clearEnv()
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoad_DefaultValues(t *testing.T) {
	clearEnv()
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 3000, cfg.App.Port)
	assert.Equal(t, "development", cfg.App.NodeEnv)
	assert.Equal(t, "1.1.0", cfg.App.Version)
	assert.Equal(t, 20, cfg.Database.MaxConns)
	assert.Equal(t, 2, cfg.Database.MinConns)
	assert.Equal(t, 1*time.Hour, cfg.Database.MaxConnLifetime)
	assert.Equal(t, 30*time.Second, cfg.Database.HealthCheckPeriod)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "pretty", cfg.Logging.Format)
	assert.Equal(t, false, cfg.Logging.FileEnabled)
	assert.Equal(t, true, cfg.Logging.FileCompress)
	assert.Equal(t, true, cfg.Logging.DBEnabled)
	assert.Equal(t, 1.0, cfg.Logging.DBSampleRate)
	assert.Equal(t, 10000, cfg.Logging.DBQueueSize)
	assert.Equal(t, "drop_debug", cfg.Logging.DBDropPolicy)
	assert.Equal(t, 60, cfg.Collector.CollectorIntervalSec)
	assert.Equal(t, 30, cfg.Collector.MetricsRetentionDays)
	assert.Equal(t, 15*time.Minute, cfg.Auth.AccessTokenExpiry)
	assert.Equal(t, 7*24*time.Hour, cfg.Auth.RefreshTokenExpiry)
}

func TestLoad_OverrideValues(t *testing.T) {
	clearEnv()
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("PORT", "8080")
	os.Setenv("NODE_ENV", "production")
	os.Setenv("LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("PORT")
		os.Unsetenv("NODE_ENV")
		os.Unsetenv("LOG_LEVEL")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.App.Port)
	assert.Equal(t, "production", cfg.App.NodeEnv)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestLoad_ModuleLevels(t *testing.T) {
	clearEnv()
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("LOG_MODULE_LEVELS", "http=debug,db=warn")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("LOG_MODULE_LEVELS")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.Logging.ModuleLevels["http"])
	assert.Equal(t, "warn", cfg.Logging.ModuleLevels["db"])
}

func TestLoad_InvalidInt(t *testing.T) {
	clearEnv()
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("PORT", "abc")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("PORT")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 3000, cfg.App.Port)
}

func TestLoad_InvalidBool(t *testing.T) {
	clearEnv()
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("LOG_FILE_ENABLED", "maybe")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("LOG_FILE_ENABLED")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, false, cfg.Logging.FileEnabled)
}

func TestLoad_InvalidDuration(t *testing.T) {
	clearEnv()
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("ACCESS_TOKEN_EXPIRY", "xyz")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("ACCESS_TOKEN_EXPIRY")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 15*time.Minute, cfg.Auth.AccessTokenExpiry)
}

func TestLoad_InvalidFloat(t *testing.T) {
	clearEnv()
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("LOG_DB_SAMPLE_RATE", "abc")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("LOG_DB_SAMPLE_RATE")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 1.0, cfg.Logging.DBSampleRate)
}

func TestEnvStr_EmptyReturnsDefault(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "fallback", envStr("NONEXISTENT_VAR_XYZ", "fallback"))
}

func TestEnvInt_EmptyReturnsDefault(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 42, envInt("NONEXISTENT_VAR_XYZ", 42))
}

func TestEnvBool_EmptyReturnsDefault(t *testing.T) {
	t.Parallel()
	assert.Equal(t, true, envBool("NONEXISTENT_VAR_XYZ", true))
}

func TestEnvDuration_EmptyReturnsDefault(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 5*time.Second, envDuration("NONEXISTENT_VAR_XYZ", 5*time.Second))
}

func TestEnvFloat64_EmptyReturnsDefault(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0.5, envFloat64("NONEXISTENT_VAR_XYZ", 0.5))
}

func TestLoad_CORSOrigins(t *testing.T) {
	clearEnv()
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("CORS_ORIGINS", "http://localhost:3000, http://example.com")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("CORS_ORIGINS")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"http://localhost:3000", "http://example.com"}, cfg.App.CORSOrigins)
}
