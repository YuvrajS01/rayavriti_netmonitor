package logging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuditLogger_LogEvent(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	auditLogger := NewAuditLogger(logger)
	assert.NotNil(t, auditLogger)
}

func TestAuditLogger_LogLogin_Success(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	auditLogger := NewAuditLogger(logger)
	auditLogger.LogLogin(context.Background(), 1, "admin", "127.0.0.1", "Mozilla/5.0", true, nil)
}

func TestAuditLogger_LogLogin_Failure(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	auditLogger := NewAuditLogger(logger)
	auditLogger.LogLogin(context.Background(), 0, "unknown", "127.0.0.1", "Mozilla/5.0", false, nil)
}

func TestAuditLogger_LogConfigChange_Delete(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	auditLogger := NewAuditLogger(logger)
	auditLogger.LogConfigChange(context.Background(), "deleted", "devices", 1, "user:admin", "127.0.0.1", nil)
}

func TestAuditLogger_LogConfigChange_Create(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	auditLogger := NewAuditLogger(logger)
	auditLogger.LogConfigChange(context.Background(), "created", "devices", 1, "user:admin", "127.0.0.1", nil)
}

func TestAuditLogger_LogAction(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	auditLogger := NewAuditLogger(logger)
	auditLogger.LogAction(context.Background(), 1, "delete", "device", 42, true)
}

func TestFormatActor(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "system", formatActor(0))
	assert.Equal(t, "user:42", formatActor(42))
}

func TestFormatInt64(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", formatInt64(0))
	assert.Equal(t, "42", formatInt64(42))
}
