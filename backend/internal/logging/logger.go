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

type Logger struct {
	base      *slog.Logger
	component string
}

func New(cfg *config.Config) *Logger {
	level := parseLevel(cfg.Logging.Level)
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.Logging.Format == "json" || cfg.App.NodeEnv == "production" {
		handler = slog.NewJSONHandler(buildWriter(cfg), opts)
	} else {
		handler = slog.NewTextHandler(buildWriter(cfg), opts)
	}

	return &Logger{base: slog.New(handler)}
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

func (l *Logger) With(component string) *Logger {
	return &Logger{
		base:      l.base.With("component", component),
		component: component,
	}
}

func (l *Logger) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	l.base.Log(ctx, level, msg, args...)
}

func (l *Logger) Trace(msg string, args ...any) { l.log(context.Background(), LevelTrace, msg, args...) }
func (l *Logger) Debug(msg string, args ...any) { l.log(context.Background(), slog.LevelDebug, msg, args...) }
func (l *Logger) Info(msg string, args ...any)  { l.log(context.Background(), slog.LevelInfo, msg, args...) }
func (l *Logger) Warn(msg string, args ...any)  { l.log(context.Background(), slog.LevelWarn, msg, args...) }
func (l *Logger) Error(msg string, args ...any) { l.log(context.Background(), slog.LevelError, msg, args...) }

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
