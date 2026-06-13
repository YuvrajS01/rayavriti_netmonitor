package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type ctxKey int

const (
	reqIDKey ctxKey = iota
	userIDKey
	traceIDKey
)

// WithRequestID stores a request ID in the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, reqIDKey, id)
}

// WithUserID stores a user ID in the context.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// WithTraceID stores a trace ID in the context for correlating related operations.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	v, _ := ctx.Value(reqIDKey).(string)
	return v
}

// GetUserID retrieves the user ID from the context.
func GetUserID(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

// GetTraceID retrieves the trace ID from the context.
func GetTraceID(ctx context.Context) string {
	v, _ := ctx.Value(traceIDKey).(string)
	return v
}

// GenerateRequestID creates a random 16-char hex request identifier.
func GenerateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateTraceID creates a random trace identifier with an optional prefix.
func GenerateTraceID(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	if prefix != "" {
		return prefix + "-" + hex.EncodeToString(b)
	}
	return "tr-" + hex.EncodeToString(b)
}
