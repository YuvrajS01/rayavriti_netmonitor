package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingDB embeds the Database interface so it satisfies it, then forces all
// metric writes to fail so the buffer has to re-queue popped items.
type failingDB struct {
	database.Database
}

func (f *failingDB) RecordMetricsBatch(ctx context.Context, metrics []*models.Metric) error {
	return errors.New("db unavailable")
}

func (f *failingDB) RecordMetric(ctx context.Context, m *models.Metric) error {
	return errors.New("db unavailable")
}

// succeedingDB accepts writes and counts what it received.
type succeedingDB struct {
	database.Database
	batches int
	singles int
}

func (s *succeedingDB) RecordMetricsBatch(ctx context.Context, metrics []*models.Metric) error {
	s.batches++
	return nil
}

func (s *succeedingDB) RecordMetric(ctx context.Context, m *models.Metric) error {
	s.singles++
	return nil
}

func TestMetricBuffer_RequeuesOnFailure(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	b := NewMetricBuffer(rdb, &failingDB{}, 10, time.Second)

	_, err := rdb.Client().Del(ctx, metricsBufferKey).Result()
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		require.NoError(t, b.Push(ctx, &models.Metric{DeviceID: int64(i)}))
	}

	b.flush(ctx)

	// All items failed persistence and must be re-queued, not dropped.
	llen, err := rdb.Client().LLen(ctx, metricsBufferKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(3), llen)
}

func TestMetricBuffer_FlushSuccess(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	db := &succeedingDB{}
	b := NewMetricBuffer(rdb, db, 20, time.Minute)

	_, err := rdb.Client().Del(ctx, metricsBufferKey).Result()
	require.NoError(t, err)

	for i := 1; i <= 2; i++ {
		require.NoError(t, b.Push(ctx, &models.Metric{DeviceID: int64(i)}))
	}

	b.flush(ctx)

	llen, err := rdb.Client().LLen(ctx, metricsBufferKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), llen, "list should drain on success")
	assert.Equal(t, 1, db.batches)
}
