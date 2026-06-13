package collectors

import (
	"context"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCollector struct {
	name   string
	result *Result
	err    error
}

func (m *mockCollector) Name() string { return m.name }
func (m *mockCollector) Collect(ctx context.Context, d *models.Device) (*Result, error) {
	return m.result, m.err
}

func TestRegistry_Register_And_Get(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	reg.Register(&mockCollector{name: "ping", result: &Result{Status: "up"}})

	c, ok := reg.Get("ping")
	require.True(t, ok)
	assert.Equal(t, "ping", c.Name())
}

func TestRegistry_Get_UnknownProtocol(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	_, ok := reg.Get("unknown")
	assert.False(t, ok)
}

func TestRegistry_Register_Overwrite(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	reg.Register(&mockCollector{name: "ping", result: &Result{Status: "up"}})
	reg.Register(&mockCollector{name: "ping", result: &Result{Status: "down"}})

	c, ok := reg.Get("ping")
	require.True(t, ok)
	r, err := c.Collect(context.Background(), &models.Device{})
	require.NoError(t, err)
	assert.Equal(t, "down", r.Status)
}

func TestHTTPCollector_Name(t *testing.T) {
	t.Parallel()
	c := HTTPCollector{}
	assert.Equal(t, "http", c.Name())
}

func TestPortCollector_Name(t *testing.T) {
	t.Parallel()
	c := PortCollector{}
	assert.Equal(t, "port", c.Name())
}

func TestSystemCollector_Name(t *testing.T) {
	t.Parallel()
	c := SystemCollector{}
	assert.Equal(t, "system", c.Name())
}

func TestPingCollector_Name(t *testing.T) {
	t.Parallel()
	c := PingCollector{}
	assert.Equal(t, "ping", c.Name())
}

func TestNetFlowCollector_Name(t *testing.T) {
	t.Parallel()
	c := &NetFlowCollector{Port: 2055}
	assert.Equal(t, "netflow", c.Name())
}

func TestNetFlowCollector_Collect_NoOp(t *testing.T) {
	t.Parallel()
	c := &NetFlowCollector{Port: 2055}
	result, err := c.Collect(context.Background(), &models.Device{})
	require.NoError(t, err)
	assert.Equal(t, "up", result.Status)
}

func TestPortCollector_Collect_ClosedPort(t *testing.T) {
	t.Parallel()
	c := PortCollector{}
	device := &models.Device{IPAddress: "127.0.0.1", Port: 19999}
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	assert.Equal(t, "down", result.Status)
}

func TestPortCollector_Collect_DefaultPort(t *testing.T) {
	t.Parallel()
	c := PortCollector{}
	device := &models.Device{IPAddress: "192.0.2.1", Port: 0} // TEST-NET, should be unreachable
	result, err := c.Collect(context.Background(), device)
	require.NoError(t, err)
	assert.Equal(t, "down", result.Status)
}

func TestF64(t *testing.T) {
	t.Parallel()
	v := f64(42.5)
	require.NotNil(t, v)
	assert.Equal(t, 42.5, *v)
}
