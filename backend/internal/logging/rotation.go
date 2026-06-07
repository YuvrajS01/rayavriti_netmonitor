package logging

import (
	"io"

	"gopkg.in/natefinch/lumberjack.v2"
)

// RotationConfig controls log file rotation behaviour.
type RotationConfig struct {
	Filename   string // Log file path
	MaxSizeMB  int    // Megabytes before rotation (default 100)
	MaxBackups int    // Number of old log files to keep (default 3)
	MaxAgeDays int    // Days to retain old log files (default 28)
	Compress   bool   // Compress rotated files
}

// NewRotatingWriter returns an io.WriteCloser backed by lumberjack.
func NewRotatingWriter(cfg RotationConfig) io.WriteCloser {
	maxSize := cfg.MaxSizeMB
	if maxSize == 0 {
		maxSize = 100
	}
	maxBackups := cfg.MaxBackups
	if maxBackups == 0 {
		maxBackups = 3
	}
	maxAge := cfg.MaxAgeDays
	if maxAge == 0 {
		maxAge = 28
	}
	return &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   cfg.Compress,
	}
}
