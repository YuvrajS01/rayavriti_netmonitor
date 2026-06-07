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
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, reqIDKey, id)
}

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func GetRequestID(ctx context.Context) string {
	v, _ := ctx.Value(reqIDKey).(string)
	return v
}

func GetUserID(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

func GenerateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
