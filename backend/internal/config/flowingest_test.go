package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FlowIngest(t *testing.T) {
	clearEnv()
	t.Setenv("JWT_SECRET", "test-secret-that-is-at-least-32-characters")

	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.FlowIngest.Enabled)
	assert.Equal(t, ":2055", cfg.FlowIngest.NetFlowAddr)
	assert.Equal(t, ":6343", cfg.FlowIngest.SFlowAddr)
	assert.Equal(t, 2*time.Second, cfg.FlowIngest.FlushInterval)

	// Legacy NETFLOW_PORT still works when NETFLOW_ADDR is unset.
	t.Setenv("NETFLOW_PORT", "9995")
	cfg, err = Load()
	require.NoError(t, err)
	assert.Equal(t, ":9995", cfg.FlowIngest.NetFlowAddr)

	// An explicitly empty address disables that listener.
	t.Setenv("FLOW_INGEST_ENABLED", "true")
	t.Setenv("NETFLOW_ADDR", "127.0.0.1:2056")
	t.Setenv("SFLOW_ADDR", "")
	cfg, err = Load()
	require.NoError(t, err)
	assert.True(t, cfg.FlowIngest.Enabled)
	assert.Equal(t, "127.0.0.1:2056", cfg.FlowIngest.NetFlowAddr)
	assert.Equal(t, "", cfg.FlowIngest.SFlowAddr)
}
