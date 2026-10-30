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

func TestIntegration_ReplaceDeviceNeighbors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	p, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	d, err := p.CreateDevice(ctx, &models.Device{Name: "core", IPAddress: "10.0.0.1", Protocol: "snmp", Enabled: true})
	require.NoError(t, err)
	n1 := models.LLDPNeighbor{LocalPortNum: 1, LocalPort: "Gi0/1", RemoteChassisID: "aa:bb:cc:00:00:01", RemotePortID: "Gi1/0/48", RemoteSysName: "access-1"}
	n2 := models.LLDPNeighbor{LocalPortNum: 2, LocalPort: "Gi0/2", RemoteChassisID: "aa:bb:cc:00:00:02", RemotePortID: "Gi1/0/48", RemoteSysName: "access-2"}

	require.NoError(t, p.ReplaceDeviceNeighbors(ctx, d.ID, []models.LLDPNeighbor{n1, n2}))
	var firstSeen time.Time
	require.NoError(t, p.pool.QueryRow(ctx, `SELECT first_seen FROM device_neighbors WHERE device_id=$1 AND local_port_num=1`, d.ID).Scan(&firstSeen))

	// n2 disappears and n1's description changes: n1 keeps first_seen.
	n1.RemoteSysName = "access-1.example.net"
	require.NoError(t, p.ReplaceDeviceNeighbors(ctx, d.ID, []models.LLDPNeighbor{n1}))
	var count int
	var name string
	var gotFirst time.Time
	require.NoError(t, p.pool.QueryRow(ctx, `SELECT count(*) OVER (), remote_sys_name, first_seen FROM device_neighbors WHERE device_id=$1`, d.ID).Scan(&count, &name, &gotFirst))
	assert.Equal(t, 1, count)
	assert.Equal(t, "access-1.example.net", name)
	assert.True(t, gotFirst.Equal(firstSeen), "first_seen must survive a refresh")

	// An empty set clears the device's neighbours.
	require.NoError(t, p.ReplaceDeviceNeighbors(ctx, d.ID, []models.LLDPNeighbor{}))
	require.NoError(t, p.pool.QueryRow(ctx, `SELECT count(*) FROM device_neighbors WHERE device_id=$1`, d.ID).Scan(&count))
	assert.Equal(t, 0, count)
}
