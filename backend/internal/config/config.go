package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Collector CollectorConfig
	Logging  LoggingConfig
	Phase2   Phase2Config
}

type AppConfig struct {
	Port       int
	NodeEnv    string
	Version    string
	CORSOrigins []string
}

type DatabaseConfig struct {
	DSN             string
	MaxConns        int
	MinConns        int
	MaxConnLifetime time.Duration
	HealthCheckPeriod time.Duration
}

type AuthConfig struct {
	JWTSecret           string
	AdminUsername       string
	AdminPassword       string
	AccessTokenExpiry   time.Duration
	RefreshTokenExpiry  time.Duration
}

type CollectorConfig struct {
	NetflowPort           int
	MetricsRetentionDays  int
	FlowRetentionDays     int
	AlertsRetentionDays   int
	PortDiscoveryEnabled  bool
	CaptureEnabled        bool
	CollectorIntervalSec  int
}

type LoggingConfig struct {
	Level          string
	Format         string
	FileEnabled    bool
	FilePath       string
	FileMaxSizeMB  int
	FileMaxBackups int
	FileMaxAgeDays int
	FileCompress   bool
	DBEnabled      bool
	DBSampleRate   float64
	DBQueueSize    int
	DBDropPolicy   string
	ModuleLevels   map[string]string
	SlowQueryMs    int
	SlowRequestMs  int
}

type Phase2Config struct {
	TelegramToken     string
	TelegramChatID    string
	TwilioSID         string
	TwilioToken       string
	TwilioFrom        string
	StatusPageEnabled bool
}

func Load() (*Config, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	moduleLevels := map[string]string{}
	if raw := os.Getenv("LOG_MODULE_LEVELS"); raw != "" {
		for _, pair := range strings.Split(raw, ",") {
			parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
			if len(parts) == 2 {
				moduleLevels[parts[0]] = parts[1]
			}
		}
	}

	cfg := &Config{
		App: AppConfig{
			Port:       envInt("PORT", 3000),
			NodeEnv:    envStr("NODE_ENV", "development"),
			Version:    envStr("VERSION", "1.1.0"),
			CORSOrigins: envSlice("CORS_ORIGINS"),
		},
		Database: DatabaseConfig{
			DSN:               envStr("DATABASE_URL", envStr("DATABASE_DSN", "postgres://postgres:postgres@localhost:5432/netmonitor?sslmode=disable")),
			MaxConns:           envInt("DB_MAX_CONNS", 20),
			MinConns:           envInt("DB_MIN_CONNS", 2),
			MaxConnLifetime:    envDuration("DB_MAX_CONN_LIFETIME", 1*time.Hour),
			HealthCheckPeriod:  envDuration("DB_HEALTH_CHECK_PERIOD", 30*time.Second),
		},
		Auth: AuthConfig{
			JWTSecret:          jwtSecret,
			AdminUsername:      envStr("ADMIN_USERNAME", "admin"),
			AdminPassword:      envStr("ADMIN_PASSWORD", ""),
			AccessTokenExpiry:  envDuration("ACCESS_TOKEN_EXPIRY", 15*time.Minute),
			RefreshTokenExpiry: envDuration("REFRESH_TOKEN_EXPIRY", 7*24*time.Hour),
		},
		Collector: CollectorConfig{
			NetflowPort:          envInt("NETFLOW_PORT", 2055),
			MetricsRetentionDays: envInt("METRICS_RETENTION_DAYS", 30),
			FlowRetentionDays:    envInt("FLOW_RETENTION_DAYS", 7),
			AlertsRetentionDays:  envInt("ALERTS_RETENTION_DAYS", 90),
			PortDiscoveryEnabled: envBool("PORT_DISCOVERY_ENABLED", true),
			CaptureEnabled:       envBool("CAPTURE_ENABLED", true),
			CollectorIntervalSec: envInt("COLLECTOR_INTERVAL_SEC", 60),
		},
		Logging: LoggingConfig{
			Level:          envStr("LOG_LEVEL", "info"),
			Format:         envStr("LOG_FORMAT", "pretty"),
			FileEnabled:    envBool("LOG_FILE_ENABLED", false),
			FilePath:       envStr("LOG_FILE_PATH", "./data/logs/netmonitor.log"),
			FileMaxSizeMB:  envInt("LOG_FILE_MAX_SIZE_MB", 100),
			FileMaxBackups: envInt("LOG_FILE_MAX_BACKUPS", 10),
			FileMaxAgeDays: envInt("LOG_FILE_MAX_AGE_DAYS", 30),
			FileCompress:   envBool("LOG_FILE_COMPRESS", true),
			DBEnabled:      envBool("LOG_DB_ENABLED", true),
			DBSampleRate:   envFloat64("LOG_DB_SAMPLE_RATE", 1.0),
			DBQueueSize:    envInt("LOG_DB_QUEUE_SIZE", 10000),
			DBDropPolicy:   envStr("LOG_DB_DROP_POLICY", "drop_debug"),
			ModuleLevels:   moduleLevels,
			SlowQueryMs:    envInt("LOG_SLOW_QUERY_MS", 100),
			SlowRequestMs:  envInt("LOG_SLOW_REQUEST_MS", 1000),
		},
		Phase2: Phase2Config{
			TelegramToken:     os.Getenv("TELEGRAM_TOKEN"),
			TelegramChatID:    os.Getenv("TELEGRAM_CHAT_ID"),
			TwilioSID:         os.Getenv("TWILIO_SID"),
			TwilioToken:       os.Getenv("TWILIO_TOKEN"),
			TwilioFrom:        os.Getenv("TWILIO_FROM"),
			StatusPageEnabled: envBool("STATUS_PAGE_ENABLED", false),
		},
	}

	return cfg, nil
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envFloat64(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envSlice(key string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	}
	return nil
}
