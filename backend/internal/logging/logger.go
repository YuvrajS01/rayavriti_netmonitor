package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

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
	runtime   *RuntimeControls
}

// PersistedEvent is the normalized operational log event stored for admin use.
type PersistedEvent struct {
	Timestamp        time.Time      `json:"timestamp"`
	Level            string         `json:"level"`
	Component        string         `json:"component"`
	EventType        string         `json:"eventType,omitempty"`
	Message          string         `json:"message"`
	RequestID        string         `json:"requestId,omitempty"`
	TraceID          string         `json:"traceId,omitempty"`
	UserID           string         `json:"userId,omitempty"`
	Actor            string         `json:"actor,omitempty"`
	RemoteAddr       string         `json:"remoteAddr,omitempty"`
	DeviceID         *int64         `json:"deviceId,omitempty"`
	SensorID         *int64         `json:"sensorId,omitempty"`
	Protocol         string         `json:"protocol,omitempty"`
	Method           string         `json:"method,omitempty"`
	Path             string         `json:"path,omitempty"`
	StatusCode       *int           `json:"statusCode,omitempty"`
	DurationMs       *float64       `json:"durationMs,omitempty"`
	Error            string         `json:"error,omitempty"`
	Hostname         string         `json:"hostname"`
	PID              int            `json:"pid"`
	Version          string         `json:"version"`
	VerboseSessionID *int64         `json:"verboseSessionId,omitempty"`
	Attrs            map[string]any `json:"attrs,omitempty"`
}

type PersistFunc func(context.Context, PersistedEvent)

// VerboseSession represents a temporary runtime logging override.
type VerboseSession struct {
	ID         int64
	Level      string
	Components []string
	DeviceIDs  []int64
	UserIDs    []string
	ExpiresAt  time.Time
}

// RuntimeControls owns dynamic log-level decisions and optional persistence.
type RuntimeControls struct {
	defaultLevel slog.Level
	persist      PersistFunc
	mu           sync.RWMutex
	sessions     map[int64]VerboseSession
}

func NewRuntimeControls(defaultLevel slog.Level) *RuntimeControls {
	return &RuntimeControls{defaultLevel: defaultLevel, sessions: map[int64]VerboseSession{}}
}

func (r *RuntimeControls) Level() slog.Level {
	r.mu.RLock()
	defer r.mu.RUnlock()
	level := r.defaultLevel
	now := time.Now()
	for _, s := range r.sessions {
		if now.After(s.ExpiresAt) {
			continue
		}
		if lv := parseLevel(s.Level); lv < level {
			level = lv
		}
	}
	return level
}

func (r *RuntimeControls) SetPersistFunc(fn PersistFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.persist = fn
}

func (r *RuntimeControls) UpsertVerboseSession(s VerboseSession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.ID] = s
}

func (r *RuntimeControls) StopVerboseSession(id int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, id)
}

func (r *RuntimeControls) matchVerbose(level slog.Level, component string, attrs map[string]any) *int64 {
	if level >= r.defaultLevel {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for id, s := range r.sessions {
		if now.After(s.ExpiresAt) {
			delete(r.sessions, id)
			continue
		}
		if level < parseLevel(s.Level) {
			continue
		}
		if !matchesComponent(component, s.Components) {
			continue
		}
		if len(s.DeviceIDs) > 0 && !containsInt64(s.DeviceIDs, attrInt64(attrs, "device_id")) {
			continue
		}
		if len(s.UserIDs) > 0 && !containsString(s.UserIDs, attrString(attrs, "user_id")) {
			continue
		}
		cp := id
		return &cp
	}
	return nil
}

func (r *RuntimeControls) shouldPersist(level slog.Level, component string, attrs map[string]any) (*int64, bool) {
	if level >= r.defaultLevel {
		return nil, true
	}
	id := r.matchVerbose(level, component, attrs)
	return id, id != nil
}

func (r *RuntimeControls) persistEvent(ctx context.Context, evt PersistedEvent) {
	r.mu.RLock()
	fn := r.persist
	r.mu.RUnlock()
	if fn != nil {
		fn(ctx, evt)
	}
}

func matchesComponent(component string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, c := range allowed {
		c = strings.TrimSpace(c)
		if c == "" || c == "*" || component == c || strings.HasPrefix(component, c+".") {
			return true
		}
	}
	return false
}

func containsInt64(items []int64, target int64) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// New creates a root Logger from the application config.
func New(cfg *config.Config) *Logger {
	level := parseLevel(cfg.Logging.Level)
	runtime := NewRuntimeControls(level)
	opts := &slog.HandlerOptions{Level: runtime}

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
		runtime:  runtime,
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
		runtime:   l.runtime,
	}
}

func (l *Logger) RuntimeControls() *RuntimeControls { return l.runtime }

// Hostname returns the hostname captured at logger creation.
func (l *Logger) Hostname() string { return l.hostname }

// PID returns the process ID captured at logger creation.
func (l *Logger) PID() int { return l.pid }

// Version returns the application version.
func (l *Logger) Version() string { return l.version }

func (l *Logger) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	attrs := attrsMap(args...)
	// Inject context values as structured fields.
	if reqID := GetRequestID(ctx); reqID != "" {
		args = append(args, "request_id", reqID)
		attrs["request_id"] = reqID
	}
	if traceID := GetTraceID(ctx); traceID != "" {
		args = append(args, "trace_id", traceID)
		attrs["trace_id"] = traceID
	}
	if userID := GetUserID(ctx); userID != "" {
		args = append(args, "user_id", userID)
		attrs["user_id"] = userID
	}
	l.base.Log(ctx, level, msg, args...)
	verboseID, ok := l.runtime.shouldPersist(level, l.component, attrs)
	if !ok {
		return
	}
	l.runtime.persistEvent(ctx, l.persistedEvent(level, msg, attrs, verboseID))
}

func (l *Logger) persistedEvent(level slog.Level, msg string, attrs map[string]any, verboseID *int64) PersistedEvent {
	statusCode := attrInt(attrs, "status", "status_code")
	durationMs := attrFloat(attrs, "duration_ms")
	deviceID := attrOptionalInt64(attrs, "device_id")
	sensorID := attrOptionalInt64(attrs, "sensor_id")
	component := l.component
	if component == "" {
		component = attrString(attrs, "component")
	}
	return PersistedEvent{
		Timestamp:        time.Now().UTC(),
		Level:            levelName(level),
		Component:        component,
		EventType:        attrString(attrs, "event"),
		Message:          msg,
		RequestID:        attrString(attrs, "request_id"),
		TraceID:          attrString(attrs, "trace_id"),
		UserID:           attrString(attrs, "user_id"),
		Actor:            attrString(attrs, "actor"),
		RemoteAddr:       firstString(attrs, "remote_addr", "actor_ip"),
		DeviceID:         deviceID,
		SensorID:         sensorID,
		Protocol:         attrString(attrs, "protocol"),
		Method:           attrString(attrs, "method"),
		Path:             attrString(attrs, "path"),
		StatusCode:       statusCode,
		DurationMs:       durationMs,
		Error:            attrString(attrs, "error"),
		Hostname:         l.hostname,
		PID:              l.pid,
		Version:          l.version,
		VerboseSessionID: verboseID,
		Attrs:            redactAttrs(attrs),
	}
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
