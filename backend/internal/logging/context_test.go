package logging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-abc")
	ctx = WithUserID(ctx, "user-123")
	ctx = WithTraceID(ctx, "tr-xyz")

	assert.Equal(t, "req-abc", GetRequestID(ctx))
	assert.Equal(t, "user-123", GetUserID(ctx))
	assert.Equal(t, "tr-xyz", GetTraceID(ctx))
}

func TestContextOverwrite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctx = WithRequestID(ctx, "first")
	ctx = WithRequestID(ctx, "second")
	assert.Equal(t, "second", GetRequestID(ctx))
}

func TestGenerateRequestID_Unique(t *testing.T) {
	t.Parallel()
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()
	assert.NotEqual(t, id1, id2)
}

func TestGenerateTraceID_WithPrefix(t *testing.T) {
	t.Parallel()
	id := GenerateTraceID("collect")
	assert.Regexp(t, `^collect-[a-f0-9]+$`, id)
}

func TestGenerateTraceID_WithoutPrefix(t *testing.T) {
	t.Parallel()
	id := GenerateTraceID("")
	assert.Regexp(t, `^tr-[a-f0-9]+$`, id)
}
