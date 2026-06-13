package logging

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/rayavriti/netmonitor-backend/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	LevelTrace = slog.Level(-8)
	LevelFatal = slog.Level(12)
)

// LogEntry is the structured log record emitted by the system.
type LogEntry struct {
	Timestamp  string   `json:"timestamp"`
	Level      string   `json:"level"`
	Component  string   `json:"component"`
	RequestID  string   `json:"request_id,omitempty"`
	TraceID    string   `json:"trace_id,omitempty"`
	UserID     string   `json:"user_id,omitempty"`
	Message    string   `json:"message"`
	Data       any      `json:"data,omitempty"`
	DurationMs *float64 `json:"duration_ms,omitempty"`
	Error      *string  `json:"error,omitempty"`
	StackTrace *string  `json:"stack_trace,omitempty"`
	Hostname   string   `json:"hostname"`
	PID        int      `json:"pid"`
	Version    string   `json:"version"`
}

// Logger wraps slog with component-scoped, structured logging.
type Logger struct {
	base      *slog.Logger
	component string
	hostname  string
	pid       int
	version   string
}

// New creates a root Logger from the application config.
func New(cfg *config.Config) *Logger {
	level := parseLevel(cfg.Logging.Level)
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.Logging.Format == "json" || cfg.App.AppEnv == "production" {
		handler = slog.NewJSONHandler(buildWriter(cfg), opts)
	} else {
		handler = slog.NewTextHandler(buildWriter(cfg), opts)
	}

	hostname, _ := os.Hostname()
	pid := os.Getpid()

	return &Logger{
		base:     slog.New(handler),
		hostname: hostname,
		pid:      pid,
		version:  cfg.App.Version,
	}
}

func buildWriter(cfg *config.Config) io.Writer {
	if cfg.Logging.FileEnabled {
		lj := &lumberjack.Logger{
			Filename:   cfg.Logging.FilePath,
			MaxSize:    cfg.Logging.FileMaxSizeMB,
			MaxBackups: cfg.Logging.FileMaxBackups,
			MaxAge:     cfg.Logging.FileMaxAgeDays,
			Compress:   cfg.Logging.FileCompress,
		}
		return io.MultiWriter(os.Stdout, lj)
	}
	return os.Stdout
}

// With returns a child Logger scoped to the given component name.
func (l *Logger) With(component string) *Logger {
	return &Logger{
		base:      l.base.With("component", component),
		component: component,
		hostname:  l.hostname,
		pid:       l.pid,
		version:   l.version,
	}
}

// Hostname returns the hostname captured at logger creation.
func (l *Logger) Hostname() string { return l.hostname }

// PID returns the process ID captured at logger creation.
func (l *Logger) PID() int { return l.pid }

// Version returns the application version.
func (l *Logger) Version() string { return l.version }

func (l *Logger) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	// Inject context values as structured fields.
	if reqID := GetRequestID(ctx); reqID != "" {
		args = append(args, "request_id", reqID)
	}
	if traceID := GetTraceID(ctx); traceID != "" {
		args = append(args, "trace_id", traceID)
	}
	if userID := GetUserID(ctx); userID != "" {
		args = append(args, "user_id", userID)
	}
	l.base.Log(ctx, level, msg, args...)
}

func (l *Logger) Trace(msg string, args ...any) {
	l.log(context.Background(), LevelTrace, msg, args...)
}
func (l *Logger) Debug(msg string, args ...any) {
	l.log(context.Background(), slog.LevelDebug, msg, args...)
}
func (l *Logger) Info(msg string, args ...any) {
	l.log(context.Background(), slog.LevelInfo, msg, args...)
}
func (l *Logger) Warn(msg string, args ...any) {
	l.log(context.Background(), slog.LevelWarn, msg, args...)
}
func (l *Logger) Error(msg string, args ...any) {
	l.log(context.Background(), slog.LevelError, msg, args...)
}

func (l *Logger) TraceCtx(ctx context.Context, msg string, args ...any) {
	l.log(ctx, LevelTrace, msg, args...)
}
func (l *Logger) DebugCtx(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelDebug, msg, args...)
}
func (l *Logger) InfoCtx(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelInfo, msg, args...)
}
func (l *Logger) WarnCtx(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelWarn, msg, args...)
}
func (l *Logger) ErrorCtx(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelError, msg, args...)
}

func (l *Logger) Fatal(msg string, args ...any) {
	l.log(context.Background(), LevelFatal, msg, args...)
	os.Exit(1)
}

func parseLevel(s string) slog.Level {
	switch s {
	case "trace":
		return LevelTrace
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
