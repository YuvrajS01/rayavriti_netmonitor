//go:build integration

package database

import (
	"context"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_DashboardsSharingAndDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	p, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	alice, err := p.CreateUser(ctx, &models.User{Username: "alice", DisplayName: "Alice Ops", PasswordHash: "x", Role: "network_admin", Enabled: true})
	require.NoError(t, err)
	bob, err := p.CreateUser(ctx, &models.User{Username: "bob", PasswordHash: "x", Role: "viewer", Enabled: true})
	require.NoError(t, err)

	noc, err := p.SaveDashboard(ctx, &models.Dashboard{UserID: alice.ID, Name: "NOC", Layout: models.EmptyDashboardLayout, Shared: true})
	require.NoError(t, err)
	assert.True(t, noc.Shared)
	assert.Equal(t, "Alice Ops", noc.OwnerName)
	_, err = p.SaveDashboard(ctx, &models.Dashboard{UserID: alice.ID, Name: "Private", Layout: models.EmptyDashboardLayout})
	require.NoError(t, err)
	mine, err := p.SaveDashboard(ctx, &models.Dashboard{UserID: bob.ID, Name: "Bob's", Layout: models.EmptyDashboardLayout})
	require.NoError(t, err)

	// Bob sees his own first, then Alice's shared one; never her private one.
	list, err := p.GetDashboards(ctx, bob.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, mine.ID, list[0].ID)
	assert.Equal(t, noc.ID, list[1].ID)

	require.NoError(t, p.SetDefaultDashboard(ctx, bob.ID, &noc.ID))
	list, err = p.GetDashboards(ctx, bob.ID)
	require.NoError(t, err)
	assert.False(t, list[0].IsDefault)
	assert.True(t, list[1].IsDefault, "Bob's default is the shared NOC dashboard")
	list, err = p.GetDashboards(ctx, alice.ID)
	require.NoError(t, err)
	for _, d := range list {
		assert.False(t, d.IsDefault, "defaults are per user")
	}

	// Deleting the dashboard clears the default instead of failing.
	require.NoError(t, p.DeleteDashboard(ctx, noc.ID))
	var def *int64
	require.NoError(t, p.pool.QueryRow(ctx, `SELECT default_dashboard_id FROM users WHERE id=$1`, bob.ID).Scan(&def))
	assert.Nil(t, def)
}
