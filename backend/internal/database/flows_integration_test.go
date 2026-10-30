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

func TestIntegration_RecordFlowsRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	p, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	ip := func(s string) *string { return &s }
	n := func(v int) *int { return &v }
	devID := int64(42)
	now := time.Now().UTC().Truncate(time.Millisecond)
	flows := []models.Flow{
		// Simulator-style flow: no ingest metadata.
		{Timestamp: now.Add(-2 * time.Minute), SrcIP: "10.0.0.1", DstIP: "10.0.0.2", SrcPort: 1234, DstPort: 80,
			Protocol: "TCP", Bytes: 100, Packets: 1, Duration: 0.5},
		// Ingested flow with every new column set, including IPv6.
		{Timestamp: now.Add(-time.Minute), SrcIP: "2001:db8::1", DstIP: "2001:db8::2", SrcPort: 443, DstPort: 50000,
			Protocol: "TCP", Bytes: 150000, Packets: 100, Duration: 25,
			Exporter: ip("192.0.2.10"), DeviceID: &devID, InIf: n(3), OutIf: n(5),
			TCPFlags: n(0x1b), ToS: n(0xb8), SamplingRate: n(100)},
	}
	require.NoError(t, p.RecordFlows(ctx, flows))

	got, total, err := p.GetFlows(ctx, now.Add(-time.Hour), now.Add(time.Hour), 10, 0)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, got, 2)

	ingested, sim := got[0], got[1] // newest first
	assert.Equal(t, "2001:db8::1", ingested.SrcIP)
	assert.Equal(t, "2001:db8::2", ingested.DstIP)
	assert.True(t, flows[1].Timestamp.Equal(ingested.Timestamp))
	assert.Equal(t, "192.0.2.10", *ingested.Exporter)
	assert.Equal(t, int64(42), *ingested.DeviceID)
	assert.Equal(t, 3, *ingested.InIf)
	assert.Equal(t, 5, *ingested.OutIf)
	assert.Equal(t, 0x1b, *ingested.TCPFlags)
	assert.Equal(t, 0xb8, *ingested.ToS)
	assert.Equal(t, 100, *ingested.SamplingRate)
	assert.Equal(t, int64(150000), ingested.Bytes)

	assert.Equal(t, "10.0.0.1", sim.SrcIP)
	assert.Nil(t, sim.Exporter)
	assert.Nil(t, sim.DeviceID)
	assert.Nil(t, sim.InIf)
	assert.Nil(t, sim.TCPFlags)
	assert.Nil(t, sim.SamplingRate)

	talkers, err := p.GetTopTalkers(ctx, now.Add(-time.Hour), now.Add(time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, talkers, 2)
	assert.Equal(t, "2001:db8::1", talkers[0].IP)

	// A malformed IP fails the batch instead of storing garbage.
	assert.Error(t, p.RecordFlows(ctx, []models.Flow{{Timestamp: now, SrcIP: "not-an-ip", DstIP: "10.0.0.2"}}))
}
