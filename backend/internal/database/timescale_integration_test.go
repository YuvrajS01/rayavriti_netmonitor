//go:build integration

package database

import (
	"context"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_RetentionPoliciesAreIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	p, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	dropAfter := func(rel string) string {
		var v string
		err := p.pool.QueryRow(ctx, `
			SELECT config->>'drop_after' FROM timescaledb_information.jobs
			WHERE proc_name = 'policy_retention' AND hypertable_name = $1`, rel).Scan(&v)
		if err != nil {
			return ""
		}
		return v
	}
	jobCount := func() int {
		var n int
		require.NoError(t, p.pool.QueryRow(ctx,
			`SELECT count(*) FROM timescaledb_information.jobs WHERE proc_name = 'policy_retention'`).Scan(&n))
		return n
	}

	policies := []RetentionPolicy{{"metrics", 30}, {"flows", 7}, {"metrics_hourly", 400}, {"not_a_table", 5}}
	require.NoError(t, p.ApplyRetentionPolicies(ctx, policies))
	assert.Equal(t, "30 days", dropAfter("metrics"))
	assert.Equal(t, "7 days", dropAfter("flows"))
	jobs := jobCount()
	assert.Equal(t, 3, jobs, "unknown relations are skipped")

	// Re-applying the same config changes nothing.
	require.NoError(t, p.ApplyRetentionPolicies(ctx, policies))
	assert.Equal(t, jobs, jobCount())

	// Changing a value replaces only that policy; 0 removes it.
	require.NoError(t, p.ApplyRetentionPolicies(ctx, []RetentionPolicy{{"metrics", 14}, {"flows", 0}}))
	assert.Equal(t, "14 days", dropAfter("metrics"))
	assert.Equal(t, "", dropAfter("flows"))
}

func TestIntegration_RollupsAndBaselines(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	p, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	dev, err := p.CreateDevice(ctx, &models.Device{Name: "core-sw", IPAddress: "10.0.0.1", Protocol: "ping", Enabled: true})
	require.NoError(t, err)

	rt := func(v float64) *float64 { return &v }
	now := time.Now()
	var batch []*models.Metric
	for i, v := range []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120} {
		status := "up"
		if i == 0 {
			status = "down"
		}
		batch = append(batch, &models.Metric{
			DeviceID: dev.ID, Status: status, ResponseTime: rt(v),
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	require.NoError(t, p.RecordMetricsBatch(ctx, batch))

	// Latest metric per device is the newest row.
	latest, err := p.GetLatestMetrics(ctx)
	require.NoError(t, err)
	require.Len(t, latest, 1)
	assert.Equal(t, "down", latest[0].Status)

	// Real-time aggregation serves the hourly rollup before any refresh.
	stats, err := p.GetDashboardStats(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 12, stats["totalMetrics24h"])
	require.NotNil(t, stats["avgResponseTime"])
	assert.InDelta(t, 65.0, *stats["avgResponseTime"].(*float64), 1e-9)

	// Baselines: population mean/stddev of 10..120 step 10.
	baselines, err := p.GetMetricBaselines(ctx, now.Add(-time.Hour))
	require.NoError(t, err)
	var got *MetricBaseline
	for i := range baselines {
		if baselines[i].DeviceID == dev.ID && baselines[i].Field == "response_time" {
			got = &baselines[i]
		}
	}
	require.NotNil(t, got)
	assert.Equal(t, 12, got.Count)
	assert.InDelta(t, 65.0, got.Mean, 1e-9)
	assert.InDelta(t, 34.5205, got.StdDev, 1e-3)
}

func TestIntegration_ClaimRefreshTokenOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	p, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	user, err := p.GetUserByUsername(ctx, "admin")
	if err != nil || user == nil {
		user, err = p.CreateUser(ctx, &models.User{Username: "claimer", PasswordHash: "x", Role: "viewer", Enabled: true})
		require.NoError(t, err)
	}
	require.NoError(t, p.CreateRefreshToken(ctx, "h1", user.ID, time.Now().Add(time.Hour)))
	first, err := p.ClaimRefreshToken(ctx, "h1")
	require.NoError(t, err)
	second, err := p.ClaimRefreshToken(ctx, "h1")
	require.NoError(t, err)
	assert.True(t, first)
	assert.False(t, second, "a refresh token can only be claimed once")
}

func TestIntegration_UpdateDeviceStatusesOnlyChanged(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	p, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	a, err := p.CreateDevice(ctx, &models.Device{Name: "a", IPAddress: "10.0.0.2", Protocol: "ping", Enabled: true})
	require.NoError(t, err)
	b, err := p.CreateDevice(ctx, &models.Device{Name: "b", IPAddress: "10.0.0.3", Protocol: "ping", Enabled: true})
	require.NoError(t, err)
	_, err = p.UpdateDeviceStatuses(ctx, []int64{a.ID, b.ID}, []string{"up", "up"})
	require.NoError(t, err)

	changed, err := p.UpdateDeviceStatuses(ctx, []int64{a.ID, b.ID}, []string{"up", "down"})
	require.NoError(t, err)
	assert.Equal(t, []int64{b.ID}, changed, "only the device whose status changed is written")

	got, err := p.GetDevice(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, "down", got.Status)
}
