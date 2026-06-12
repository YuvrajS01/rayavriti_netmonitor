//go:build integration

package database

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) (*Postgres, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "netmonitor_test",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	dsn := "postgres://test:test@" + host + ":" + port.Port() + "/netmonitor_test?sslmode=disable"
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	p := &Postgres{pool: pool}
	require.NoError(t, p.RunMigrations(ctx))

	cleanup := func() {
		pool.Close()
		container.Terminate(ctx)
	}
	return p, cleanup
}

func TestIntegration_DeviceCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create
	dev := &models.Device{
		Name:      "Test Router",
		IPAddress: "10.0.0.1",
		Protocol:  "ping",
		Port:      0,
		Enabled:   true,
		Interval:  30,
	}
	created, err := db.CreateDevice(ctx, dev)
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, "Test Router", created.Name)

	// Read
	got, err := db.GetDevice(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	// Update
	updated, err := db.UpdateDevice(ctx, created.ID, &models.Device{Name: "Updated Router"})
	require.NoError(t, err)
	assert.Equal(t, "Updated Router", updated.Name)

	// Delete
	err = db.DeleteDevice(ctx, created.ID)
	require.NoError(t, err)

	// Verify deleted
	_, err = db.GetDevice(ctx, created.ID)
	assert.Error(t, err)
}

func TestIntegration_GetDevicesFiltered(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create test devices
	for _, d := range []models.Device{
		{Name: "Router1", IPAddress: "10.0.0.1", Protocol: "ping", Enabled: true, Interval: 30},
		{Name: "Router2", IPAddress: "10.0.0.2", Protocol: "snmp", Enabled: false, Interval: 30},
		{Name: "Switch1", IPAddress: "10.0.0.3", Protocol: "ping", Enabled: true, Interval: 30},
	} {
		_, err := db.CreateDevice(ctx, &d)
		require.NoError(t, err)
	}

	// Filter by protocol
	devices, total, err := db.GetDevicesFiltered(ctx, DeviceFilter{Protocol: "ping"})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, devices, 2)

	// Filter by enabled
	enabled := true
	devices, total, err = db.GetDevicesFiltered(ctx, DeviceFilter{Enabled: &enabled})
	require.NoError(t, err)
	assert.Equal(t, 2, total)

	// Search
	devices, total, err = db.GetDevicesFiltered(ctx, DeviceFilter{Search: "Router"})
	require.NoError(t, err)
	assert.Equal(t, 2, total)

	// Sort by name desc
	devices, _, err = db.GetDevicesFiltered(ctx, DeviceFilter{SortBy: "name", SortDir: "desc"})
	require.NoError(t, err)
	assert.Equal(t, "Switch1", devices[0].Name)

	// Pagination
	devices, total, err = db.GetDevicesFiltered(ctx, DeviceFilter{Limit: 1, Offset: 1})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, devices, 1)
}

func TestIntegration_RefreshToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create a user first (required for foreign key)
	_, err := db.CreateUser(ctx, &models.User{
		Username:     "testuser",
		PasswordHash: "hash",
		Role:         "viewer",
		Enabled:      true,
	})
	require.NoError(t, err)
	user, err := db.GetUserByUsername(ctx, "testuser")
	require.NoError(t, err)

	// Create refresh token
	tokenHash := "abc123hash"
	expiresAt := time.Now().Add(24 * time.Hour)
	err = db.CreateRefreshToken(ctx, tokenHash, user.ID, expiresAt)
	require.NoError(t, err)

	// Get refresh token
	rt, err := db.GetRefreshToken(ctx, tokenHash)
	require.NoError(t, err)
	assert.Equal(t, user.ID, rt.UserID)

	// Delete refresh token
	err = db.DeleteRefreshToken(ctx, tokenHash)
	require.NoError(t, err)

	// Verify deleted
	_, err = db.GetRefreshToken(ctx, tokenHash)
	assert.Error(t, err)
}

func TestIntegration_DashboardStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	stats, err := db.GetDashboardStats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stats)
}

func TestIntegration_AlertCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create a device first
	dev, err := db.CreateDevice(ctx, &models.Device{
		Name: "Alert Device", IPAddress: "10.0.99.1", Protocol: "ping", Enabled: true, Interval: 30,
	})
	require.NoError(t, err)

	// Create alert
	alert := &models.Alert{
		DeviceID: dev.ID,
		Severity: "warning",
		Message:  "CPU high",
		Status:   "active",
	}
	created, err := db.CreateAlert(ctx, alert)
	require.NoError(t, err)
	assert.NotZero(t, created.ID)

	// Get alert
	got, err := db.GetAlert(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "CPU high", got.Message)

	// Update status
	err = db.UpdateAlertStatus(ctx, created.ID, "acknowledged", "admin")
	require.NoError(t, err)

	// Get alert counts
	counts, err := db.GetAlertCounts(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, counts.Active+counts.Acknowledged, 1)
}

func TestIntegration_MetricsRecordAndQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create device
	dev, err := db.CreateDevice(ctx, &models.Device{
		Name: "Metric Device", IPAddress: "10.0.0.50", Protocol: "ping", Enabled: true, Interval: 30,
	})
	require.NoError(t, err)

	// Record metric
	rt := 42.0
	metric := &models.Metric{
		DeviceID:     dev.ID,
		DeviceName:   dev.Name,
		Protocol:     dev.Protocol,
		Timestamp:    time.Now(),
		Status:       "up",
		ResponseTime: &rt,
	}
	err = db.RecordMetric(ctx, metric)
	require.NoError(t, err)

	// Get latest metrics
	metrics, err := db.GetLatestMetrics(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(metrics), 1)
}
