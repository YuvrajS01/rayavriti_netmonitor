//go:build integration

package campus

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "timescale/timescaledb:2.28.2-pg16",
			ExposedPorts: []string{"5432/tcp"},
			Env:          map[string]string{"POSTGRES_USER": "test", "POSTGRES_PASSWORD": "test", "POSTGRES_DB": "nm"},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "5432")
	dsn := "postgres://test:test@" + host + ":" + port.Port() + "/nm?sslmode=disable"
	db := database.NewPostgres(dsn, nil)
	require.NoError(t, db.Connect(ctx))
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.RunMigrations(ctx))
	return db.Pool()
}

func TestIntegration_MaintenanceWindowSuppresses(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()
	svc := NewMaintenanceService(pool)

	// No windows: nothing is under maintenance (and the query must not fail).
	st, err := svc.IsUnderMaintenance(ctx, 1, nil, "10.0.0.1")
	require.NoError(t, err)
	require.False(t, st.UnderMaintenance)

	_, err = pool.Exec(ctx, `INSERT INTO maintenance_windows
		(name, scope_type, scope_value, schedule_type, start_time, end_time, suppress_alerts)
		VALUES ('core upgrade', 'subnet', '10.0.0.0/24', 'one_time', NOW() - INTERVAL '1 hour', NOW() + INTERVAL '1 hour', TRUE),
		       ('next week', 'global', '*', 'one_time', NOW() + INTERVAL '7 days', NOW() + INTERVAL '8 days', TRUE)`)
	require.NoError(t, err)

	st, err = svc.IsUnderMaintenance(ctx, 1, nil, "10.0.0.1")
	require.NoError(t, err)
	require.True(t, st.UnderMaintenance, "device inside an active subnet window")
	require.True(t, st.SuppressAlerts)

	st, err = svc.IsUnderMaintenance(ctx, 2, nil, "10.9.9.9")
	require.NoError(t, err)
	require.False(t, st.UnderMaintenance, "future global window must not apply yet")
}

func TestIntegration_ParentDownSuppressesChild(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()
	var parent, child int64
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO devices(name, ip_address, protocol, status) VALUES ('core','10.0.0.1','ping','down') RETURNING id`).Scan(&parent))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO devices(name, ip_address, protocol, status, parent_device_id) VALUES ('edge','10.0.0.2','ping','down',$1) RETURNING id`, parent).Scan(&child))

	res, err := NewTopologyService(pool).CheckSuppression(ctx, child)
	require.NoError(t, err)
	require.True(t, res.ShouldSuppress, "child of a down parent must be suppressed")
	require.Equal(t, parent, res.RootCauseDevice.DeviceID)

	_, err = pool.Exec(ctx, `UPDATE devices SET status='up' WHERE id=$1`, parent)
	require.NoError(t, err)
	res, err = NewTopologyService(pool).CheckSuppression(ctx, child)
	require.NoError(t, err)
	require.False(t, res.ShouldSuppress)
}
